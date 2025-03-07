package sup

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"

	"github.com/goware/prefixer"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

const VERSION = "0.6.0"

type Stackup struct {
	conf   *Supfile
	debug  bool
	prefix bool
}

func New(conf *Supfile) (*Stackup, error) {
	return &Stackup{
		conf: conf,
	}, nil
}

// Последовательный запуск набора команд на нескольких хостах, определенных в network
// TODO: Раздробить функциюю на несколько маленьких
func (sup *Stackup) Run(network *Network, envVars EnvList, commands ...*Command) error {
	if len(commands) == 0 {
		return errors.New("no commands to be run")
	}

	env := envVars.AsExport()

	// Create clients for every host (either SSH or Localhost).
	var bastion *SSHClient
	if network.Bastion != "" {
		bastion = &SSHClient{}
		if err := bastion.Connect(network.Bastion); err != nil {
			return errors.Wrap(err, "connecting to bastion failed")
		}
	}

	var wg sync.WaitGroup
	clientCh := make(chan Client, len(network.Hosts))
	errCh := make(chan error, len(network.Hosts))

	for i, host := range network.Hosts {
		wg.Add(1)
		go func(i int, host string) {
			defer wg.Done()

			// Localhost client
			if host == "localhost" {
				local := &LocalhostClient{
					env: env + `export SUP_HOST="` + host + `";`,
				}
				if err := local.Connect(host); err != nil {
					errCh <- errors.Wrap(err, "connecting to localhost failed")
					return
				}
				clientCh <- local
				return
			}

			// SSH client
			remote := &SSHClient{
				env:   env + `export SUP_HOST="` + host + `";`,
				user:  network.User,
				color: Colors[i%len(Colors)],
			}

			if bastion != nil {
				if err := remote.ConnectWith(host, bastion.DialThrough); err != nil {
					errCh <- errors.Wrap(err, "connecting to remote host through bastion failed")
					return
				}
			} else {
				if err := remote.Connect(host); err != nil {
					errCh <- errors.Wrap(err, "connecting to remote host failed")
					return
				}
			}
			clientCh <- remote
		}(i, host)
	}
	wg.Wait()
	close(clientCh)
	close(errCh)

	maxLen := 0
	var clients []Client
	for client := range clientCh {
		if remote, ok := client.(*SSHClient); ok {
			defer remote.Close()
		}
		_, prefixLen := client.Prefix()
		if prefixLen > maxLen {
			maxLen = prefixLen
		}
		clients = append(clients, client)
	}
	for err := range errCh {
		return errors.Wrap(err, "connecting to clients failed")
	}

	// Запуск одной команды или последовательное выполнение нескольких команд из targets
	for _, cmd := range commands {
		// Перевод из команды в задачу (task)
		tasks, err := sup.createTasks(cmd, clients, env)
		if err != nil {
			return errors.Wrap(err, "creating task failed")
		}

		// Последовательное выполнение задач
		for _, task := range tasks {
			var writers []io.Writer
			var wg sync.WaitGroup

			// Выполнять задания на предоставленных клиентах
			for _, c := range task.Clients {
				var prefix string
				var prefixLen int
				if sup.prefix {
					prefix, prefixLen = c.Prefix()
					if len(prefix) < maxLen { // Left padding
						prefix = strings.Repeat(" ", maxLen-prefixLen) + prefix
					}
				}

				err := c.Run(task)
				if err != nil {
					return errors.Wrap(err, prefix+"task failed")
				}

				// Копировать STDOUT задач
				wg.Add(1)
				go func(c Client) {
					defer wg.Done()
					_, err := io.Copy(os.Stdout, prefixer.New(c.Stdout(), prefix))
					if err != nil && err != io.EOF {
						// TODO: io.Copy() вообще не должна возвращать io.EOF.
						// Ошибка восходящего потока или ошибка prefixer.WriteTo() ?
						fmt.Fprintf(os.Stderr, "%v", errors.Wrap(err, prefix+"reading STDOUT failed"))
					}
				}(c)

				// Копировать STDERR задач
				wg.Add(1)
				go func(c Client) {
					defer wg.Done()
					_, err := io.Copy(os.Stderr, prefixer.New(c.Stderr(), prefix))
					if err != nil && err != io.EOF {
						fmt.Fprintf(os.Stderr, "%v", errors.Wrap(err, prefix+"reading STDERR failed"))
					}
				}(c)

				writers = append(writers, c.Stdin())
			}

			// Копировать STDIN задач
			if task.Input != nil {
				go func() {
					writer := io.MultiWriter(writers...)
					_, err := io.Copy(writer, task.Input)
					if err != nil && err != io.EOF {
						fmt.Fprintf(os.Stderr, "%v", errors.Wrap(err, "copying STDIN failed"))
					}
					// TODO: Использовать MultiWriteCloser (его нет в Stdlib), чтобы вместо этого мы могли writer.Close()?
					for _, c := range clients {
						c.WriteClose()
					}
				}()
			}

			// Ловит сигналы OS и передает их всем активным клиентам
			trap := make(chan os.Signal, 1)
			signal.Notify(trap, os.Interrupt)
			go func() {
				for {
					select {
					case sig, ok := <-trap:
						if !ok {
							return
						}
						for _, c := range task.Clients {
							err := c.Signal(sig)
							if err != nil {
								fmt.Fprintf(os.Stderr, "%v", errors.Wrap(err, "sending signal failed"))
							}
						}
					}
				}
			}()

			// Сначала дождаться выполнения всех операций ввода-вывода
			wg.Wait()

			// Проверить, что каждый клиент закончил выполнение задачи
			for _, c := range task.Clients {
				wg.Add(1)
				go func(c Client) {
					defer wg.Done()
					if err := c.Wait(); err != nil {
						var prefix string
						if sup.prefix {
							var prefixLen int
							prefix, prefixLen = c.Prefix()
							if len(prefix) < maxLen { // Left padding.
								prefix = strings.Repeat(" ", maxLen-prefixLen) + prefix
							}
						}
						if e, ok := err.(*ssh.ExitError); ok && e.ExitStatus() != 15 {
							// TODO: Сохранять все ошибки и выводить их после Wait()
							fmt.Fprintf(os.Stderr, "%s%v\n", prefix, e)
							os.Exit(e.ExitStatus())
						}
						fmt.Fprintf(os.Stderr, "%s%v\n", prefix, err)

						// TODO: Здесь не должно быть os.Exit(1). Вместо этого нужно собирать статусы выхода для последующего использования
						os.Exit(1)
					}
				}(c)
			}

			// Ожидание завершения выполнения всех команд
			wg.Wait()

			// Прекратить получение сигналов для текущих клиентов
			signal.Stop(trap)
			close(trap)
		}
	}

	return nil
}

func (sup *Stackup) Debug(value bool) {
	sup.debug = value
}

func (sup *Stackup) Prefix(value bool) {
	sup.prefix = value
}
