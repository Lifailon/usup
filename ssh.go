package sup

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

// Клиент - это обертка над ssh соединением/сессией
type SSHClient struct {
	conn         *ssh.Client
	sess         *ssh.Session
	user         string
	host         string
	remoteStdin  io.WriteCloser
	remoteStdout io.Reader
	remoteStderr io.Reader
	connOpened   bool
	sessOpened   bool
	running      bool
	env          string // export FOO="bar"; export BAR="baz";
	color        string
}

type ErrConnect struct {
	User   string
	Host   string
	Reason string
}

func (e ErrConnect) Error() string {
	return fmt.Sprintf(`Connect("%v@%v"): %v`, e.User, e.Host, e.Reason)
}

// Парсим строку формата <user>@<host:port>
func (c *SSHClient) parseHost(host string) error {
	c.host = host

	// Удалить протокол "ssh://" в начале строки хоста
	if strings.HasPrefix(c.host, "ssh://") {
		c.host = c.host[6:]
	}

	// Разбить на два значения до и после "@" (в имени пользователя может быть "@")
	if at := strings.LastIndex(c.host, "@"); at != -1 {
		c.user = c.host[:at]
		c.host = c.host[at+1:]
	}

	// Добавить текущего системного пользователя по умолчанию, если не установлено
	if c.user == "" {
		u, err := user.Current()
		if err != nil {
			return err
		}
		c.user = u.Username
		// Удаляем доменную часть, если она есть
		if strings.Contains(c.user, "\\") {
			c.user = strings.Split(c.user, "\\")[1]
		}
	}

	// Исключаем символ "/" в адресе хоста
	if strings.Contains(c.host, "/") {
		return ErrConnect{c.user, c.host, "unexpected slash in the host URL"}
	}

	// Добавить порт по умолчанию, если не установлен
	if !strings.Contains(c.host, ":") {
		c.host += ":22"
	}

	return nil
}

var initAuthMethodOnce sync.Once
var authMethod ssh.AuthMethod

// Инициализация метода аутентификации SSH
func initAuthMethod() {
	// Определяем переменную (массив) для ключей
	var signers []ssh.Signer

	// ОТКЛЮЧЕНО
	// Если есть запущенный ssh-agent, использовать его ключи
	// Подключение к сокету по пути из переменной окружения и извлечение содержимого приватного ключа в массив signers
	// sock, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	// if err == nil {
	// 	agent := agent.NewClient(sock)
	// 	signers, _ = agent.Signers()
	// }

	// Найти и прочитать закрытый SSH ключ пользователя из стандартных путей
	var envPath string
	// Определяем домашний каталог поиска для Windows и Linux
	if runtime.GOOS == "windows" {
		envPath = os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH") + "\\.ssh\\id_*"
	} else {
		envPath = os.Getenv("HOME") + "/.ssh/id_*"
	}
	files, _ := filepath.Glob(envPath)
	// Проходимся по всем файлам
	for _, file := range files {
		// Пропускаем публичные ключи
		if strings.HasSuffix(file, ".pub") {
			continue
		}

		// Читаем файл
		data, err := ioutil.ReadFile(file)
		if err != nil {
			log.Fatal("error reading ssh key", err)
			continue
		}

		// Парсим приватный ssh ключ из байтового среза и добавляем в массив signers
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			log.Fatal("error parsing ssh key", err)
			continue
		}

		// Добавляем ключ в массив
		signers = append(signers, signer)

		// Вывод публичного ключа для отладки
		// fmt.Printf("Signer: %+v\n", signer)

		// Останавливаем цикл поиска после успешного добавляения ключа
		// break
	}

	authMethod = ssh.PublicKeys(signers...)
}

// SSHDialFunc can dial an ssh server and return a client
type SSHDialFunc func(net, addr string, config *ssh.ClientConfig) (*ssh.Client, error)

// Connect creates SSH connection to a specified host.
// It expects the host of the form "[ssh://]host[:port]".
func (c *SSHClient) Connect(host string) error {
	return c.ConnectWith(host, ssh.Dial)
}

// Создает ssh соединение с указанным хостом с использованием dialer для авторизации по ключу
func (c *SSHClient) ConnectWith(host string, dialer SSHDialFunc) error {
	// Уже подключен
	if c.connOpened {
		return fmt.Errorf("already connected")
	}

	initAuthMethodOnce.Do(initAuthMethod)

	err := c.parseHost(host)
	if err != nil {
		return err
	}

	config := &ssh.ClientConfig{
		User: c.user,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	c.conn, err = dialer("tcp", c.host, config)
	if err != nil {
		return ErrConnect{c.user, c.host, err.Error()}
	}
	c.connOpened = true

	return nil
}

// Run запускает команду task.Run удаленно на хосте c.host
func (c *SSHClient) Run(task *Task) error {
	// Сессия уже запущена
	if c.running {
		return fmt.Errorf("session already running")
	}

	// Сессия уже подключена
	if c.sessOpened {
		return fmt.Errorf("session already connected")
	}

	sess, err := c.conn.NewSession()
	if err != nil {
		return err
	}

	c.remoteStdin, err = sess.StdinPipe()
	if err != nil {
		return err
	}

	c.remoteStdout, err = sess.StdoutPipe()
	if err != nil {
		return err
	}

	c.remoteStderr, err = sess.StderrPipe()
	if err != nil {
		return err
	}

	if task.TTY {
		// Set up terminal modes
		modes := ssh.TerminalModes{
			ssh.ECHO:          0,     // disable echoing
			ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
			ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
		}

		// Request pseudo terminal
		if err := sess.RequestPty("xterm", 80, 40, modes); err != nil {
			return ErrTask{task, fmt.Sprintf("request for pseudo terminal failed: %s", err)}
		}
	}

	// Запуск удаленной команды
	if err := sess.Start(c.env + task.Run); err != nil {
		return ErrTask{task, err.Error()}
	}

	c.sess = sess
	c.sessOpened = true
	c.running = true
	return nil
}

// Дожидается окончания выполнения удаленной команды и выходит из системы (закрывает ssh сессию)
func (c *SSHClient) Wait() error {
	if !c.running {
		return fmt.Errorf("trying to wait on stopped session")
	}

	err := c.sess.Wait()
	c.sess.Close()
	c.running = false
	c.sessOpened = false

	return err
}

// Создает новое ssh соединение с сервером через уже существующие ssh подключение
func (sc *SSHClient) DialThrough(net, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	conn, err := sc.conn.Dial(net, addr)
	if err != nil {
		return nil, err
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

// Close closes the underlying SSH connection and session.
func (c *SSHClient) Close() error {
	if c.sessOpened {
		c.sess.Close()
		c.sessOpened = false
	}

	// Попытка закрыть уже закрытое соединение
	if !c.connOpened {
		return fmt.Errorf("trying to close the already closed connection")
	}

	err := c.conn.Close()
	c.connOpened = false
	c.running = false

	return err
}

func (c *SSHClient) Stdin() io.WriteCloser {
	return c.remoteStdin
}

func (c *SSHClient) Stderr() io.Reader {
	return c.remoteStderr
}

func (c *SSHClient) Stdout() io.Reader {
	return c.remoteStdout
}

func (c *SSHClient) Prefix() (string, int) {
	host := c.user + "@" + c.host + " | "
	return c.color + host + ResetColor, len(host)
}

func (c *SSHClient) Write(p []byte) (n int, err error) {
	return c.remoteStdin.Write(p)
}

func (c *SSHClient) WriteClose() error {
	return c.remoteStdin.Close()
}

func (c *SSHClient) Signal(sig os.Signal) error {
	if !c.sessOpened {
		return fmt.Errorf("session is not open")
	}

	switch sig {
	case os.Interrupt:
		c.remoteStdin.Write([]byte("\x03"))
		return c.sess.Signal(ssh.SIGINT)
		// Сигнал SIGHUP не работает
		// https://github.com/golang/go/issues/4115#issuecomment-66070418
		// return c.sess.Signal(ssh.SIGHUP)
	default:
		return fmt.Errorf("%v not supported", sig)
	}
}
