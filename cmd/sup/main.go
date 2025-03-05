package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Lifailon/sup"
	"github.com/mikkeloscar/sshconfig"
	"github.com/pkg/errors"
)

var (
	supfile     string
	envVars     flagStringSlice
	sshConfig   string
	onlyHosts   string
	exceptHosts string

	debug         bool
	disablePrefix bool

	showVersion bool
	showHelp    bool

	ErrUsage            = errors.New("Usage: sup [OPTIONS] NETWORK COMMAND [...]\n       sup [ --help | -v | --version ]")
	ErrUnknownNetwork   = errors.New("Unknown network")
	ErrNetworkNoHosts   = errors.New("No hosts defined for a given network")
	ErrCmd              = errors.New("Unknown command/target")
	ErrTargetNoCommands = errors.New("No commands defined for a given target")
	ErrConfigFile       = errors.New("Unknown ssh_config file")

	configURL string
)

type flagStringSlice []string

func (f *flagStringSlice) String() string {
	return fmt.Sprintf("%v", *f)
}

func (f *flagStringSlice) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func init() {
	flag.StringVar(&supfile, "f", "", "Custom path to file configuration")
	flag.StringVar(&configURL, "u", "", "Url path to file configuration")
	flag.Var(&envVars, "e", "Set environment variables")
	flag.Var(&envVars, "env", "Set environment variables")
	flag.StringVar(&sshConfig, "sshconfig", "", "Read SSH Config file, ie. ~/.ssh/config file")
	flag.StringVar(&onlyHosts, "only", "", "Filter hosts using regexp")
	flag.StringVar(&exceptHosts, "except", "", "Filter out hosts using regexp")

	flag.BoolVar(&debug, "D", false, "Enable debug mode")
	flag.BoolVar(&debug, "debug", false, "Enable debug mode")
	flag.BoolVar(&disablePrefix, "disable-prefix", false, "Disable hostname prefix")

	flag.BoolVar(&showVersion, "v", false, "Print version")
	flag.BoolVar(&showVersion, "version", false, "Print version")
	flag.BoolVar(&showHelp, "h", false, "Show help")
	flag.BoolVar(&showHelp, "help", false, "Show help")
}

// Вывести список доступных networks/hosts
func networkUsage(conf *sup.Supfile) {
	w := &tabwriter.Writer{}
	w.Init(os.Stderr, 4, 4, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintln(w, "Networks:\t")
	for _, name := range conf.Networks.Names {
		fmt.Fprintf(w, "- %v\n", name)
		network, _ := conf.Networks.Get(name)
		for _, host := range network.Hosts {
			fmt.Fprintf(w, "\t- %v\n", host)
		}
	}
	fmt.Fprintln(w)
}

// Вывести список доступных targets/commands
func cmdUsage(conf *sup.Supfile) {
	w := &tabwriter.Writer{}
	w.Init(os.Stderr, 4, 4, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintln(w, "Targets:\t")
	for _, name := range conf.Targets.Names {
		cmds, _ := conf.Targets.Get(name)
		fmt.Fprintf(w, "- %v\t%v\n", name, strings.Join(cmds, " "))
	}
	fmt.Fprintln(w, "\t")
	fmt.Fprintln(w, "Commands:\t")
	for _, name := range conf.Commands.Names {
		cmd, _ := conf.Commands.Get(name)
		fmt.Fprintf(w, "- %v\t%v\n", name, cmd.Desc)
	}
	fmt.Fprintln(w)
}

// Анализирует аргументы и возвращает сеть/хосты или таргеты/команды, доступные для выполнения
// При ошибке выводит help и завершает работу
func parseArgs(conf *sup.Supfile) (*sup.Network, []*sup.Command, error) {
	var commands []*sup.Command

	// Если аргументы отсутствуют
	args := flag.Args()
	if len(args) < 1 {
		networkUsage(conf)
		return nil, nil, ErrUsage
	}

	// Проверяем первый аргумент на соответствие со списком сетей, что бы пройти дальше
	network, ok := conf.Networks.Get(args[0])
	if !ok {
		networkUsage(conf)
		return nil, nil, ErrUnknownNetwork
	}

	// Разбор флага "--env", переопределяющие значения, определенные в конфигурации env
	for _, env := range envVars {
		if len(env) == 0 {
			continue
		}
		i := strings.Index(env, "=")
		if i < 0 {
			if len(env) > 0 {
				network.Env.Set(env, "")
			}
			continue
		}
		network.Env.Set(env[:i], env[i+1:])
	}

	// Заполняем массив хостов из выбранной группы network
	hosts, err := network.ParseInventory()
	if err != nil {
		return nil, nil, err
	}
	network.Hosts = append(network.Hosts, hosts...)

	// Проверка, что массив хостов не пустой
	if len(network.Hosts) == 0 {
		networkUsage(conf)
		return nil, nil, ErrNetworkNoHosts
	}

	// Проверка второго аргумента (выводим команды и таргеты)
	if len(args) < 2 {
		cmdUsage(conf)
		return nil, nil, ErrUsage
	}

	// Заполняем network.Env из Supfile, если не было инициализации через аргументы
	if network.Env == nil {
		network.Env = make(sup.EnvList, 0)
	}

	// Добавить переменную окружения по умолчанию с названием текущей сети
	network.Env.Set("SUP_NETWORK", args[0])

	// Добавить SUP_TIME
	network.Env.Set("SUP_TIME", time.Now().UTC().Format(time.RFC3339))
	if os.Getenv("SUP_TIME") != "" {
		network.Env.Set("SUP_TIME", os.Getenv("SUP_TIME"))
	}

	// Добавить SUP_USER
	if os.Getenv("SUP_USER") != "" {
		network.Env.Set("SUP_USER", os.Getenv("SUP_USER"))
	} else {
		network.Env.Set("SUP_USER", os.Getenv("USER"))
	}

	for _, cmd := range args[1:] {
		// Повторять команды из target
		target, isTarget := conf.Targets.Get(cmd)
		if isTarget {
			// Заполнить массив commands
			for _, cmd := range target {
				command, isCommand := conf.Commands.Get(cmd)
				if !isCommand {
					cmdUsage(conf)
					return nil, nil, fmt.Errorf("%v: %v", ErrCmd, cmd)
				}
				command.Name = cmd
				commands = append(commands, &command)
			}
		}

		// Или добавляем одну команду в commands
		command, isCommand := conf.Commands.Get(cmd)
		if isCommand {
			command.Name = cmd
			commands = append(commands, &command)
		}

		if !isTarget && !isCommand {
			cmdUsage(conf)
			return nil, nil, fmt.Errorf("%v: %v", ErrCmd, cmd)
		}
	}

	// Возвращяем сеть (хосты + переменные) и массив команд
	return &network, commands, nil
}

func resolvePath(path string) string {
	if path == "" {
		return ""
	}
	if path[:2] == "~/" {
		usr, err := user.Current()
		if err == nil {
			path = filepath.Join(usr.HomeDir, path[2:])
		}
	}
	return path
}

// Проверяет, существует ли файл
func checkFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Логика для загрузки конфигурации из URL через http.Get
func loadConfigFromURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error loading configuration from URL: %v", err)
	}
	defer resp.Body.Close()

	// Читаем тело ответа
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error when reading response from URL: %v", err)
	}

	return data, nil
}

func main() {
	flag.Parse()

	if showHelp {
		fmt.Fprintln(os.Stderr, ErrUsage, "\n\nOptions:")
		flag.PrintDefaults()
		return
	}

	if showVersion {
		fmt.Fprintln(os.Stderr, sup.VERSION)
		return
	}

	// Если указан config-url, загружаем конфигурацию из него
	var data []byte
	var err error

	if configURL != "" {
		// Загрузка конфигурации с URL
		data, err = loadConfigFromURL(configURL)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		// Если переменная конфигурационного файла пустая (не передано название файла через флаг параметра), то устанавливаем значение по умолчанию
	} else if supfile == "" {
		// Массив из названия файлов конфигурации по умолчанию (приоритет слева направо)
		files := []string{"./lazysup.yml", "lazysup.yaml", "./Lazysup.yml", "Lazysup.yaml", "./supfile.yml", "./supfile.yaml", "./Supfile.yml", "./Supfile.yaml"}

		// Ищем первый существующий файл
		for _, file := range files {
			if checkFileExists(file) {
				supfile = file
				break
			}
		}

		// Если ни один файл не найден, возвращяем ошибку
		if supfile == "" {
			fmt.Fprintln(os.Stderr, "Error: сonfiguration file not found")
			os.Exit(1)
		}
	}

	// Читаем найденный файл
	if configURL == "" {
		data, err = os.ReadFile(supfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error read сonfiguration file %s: %v\n", supfile, err)
			os.Exit(1)
		}
	}

	// Загружаем конфигурацию
	conf, err := sup.NewSupfile(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing сonfiguration file:", err)
		os.Exit(1)
	}

	// Парсинг сети и команды из аргументов для выполнения
	network, commands, err := parseArgs(conf)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// --only flag filters hosts
	if onlyHosts != "" {
		expr, err := regexp.CompilePOSIX(onlyHosts)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		var hosts []string
		for _, host := range network.Hosts {
			if expr.MatchString(host) {
				hosts = append(hosts, host)
			}
		}
		if len(hosts) == 0 {
			fmt.Fprintln(os.Stderr, fmt.Errorf("no hosts match --only '%v' regexp", onlyHosts))
			os.Exit(1)
		}
		network.Hosts = hosts
	}

	// --except flag filters out hosts
	if exceptHosts != "" {
		expr, err := regexp.CompilePOSIX(exceptHosts)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		var hosts []string
		for _, host := range network.Hosts {
			if !expr.MatchString(host) {
				hosts = append(hosts, host)
			}
		}
		if len(hosts) == 0 {
			fmt.Fprintln(os.Stderr, fmt.Errorf("no hosts left after --except '%v' regexp", onlyHosts))
			os.Exit(1)
		}
		network.Hosts = hosts
	}

	// --sshconfig flag location for ssh_config file
	if sshConfig != "" {
		confHosts, err := sshconfig.ParseSSHConfig(resolvePath(sshConfig))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		// flatten Host -> *SSHHost, not the prettiest
		// but will do
		confMap := map[string]*sshconfig.SSHHost{}
		for _, conf := range confHosts {
			for _, host := range conf.Host {
				confMap[host] = conf
			}
		}

		// check network.Hosts for match
		for _, host := range network.Hosts {
			conf, found := confMap[host]
			if found {
				network.User = conf.User
				network.IdentityFile = resolvePath(conf.IdentityFile)
				network.Hosts = []string{fmt.Sprintf("%s:%d", conf.HostName, conf.Port)}
			}
		}
	}

	var vars sup.EnvList
	for _, val := range append(conf.Env, network.Env...) {
		vars.Set(val.Key, val.Value)
	}
	if err := vars.ResolveValues(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Парсинг флагов cli --env, определяет $SUP_ENV и переопределяет значения, определенные в Supfile
	var cliVars sup.EnvList
	for _, env := range envVars {
		if len(env) == 0 {
			continue
		}
		i := strings.Index(env, "=")
		if i < 0 {
			if len(env) > 0 {
				vars.Set(env, "")
			}
			continue
		}
		vars.Set(env[:i], env[i+1:])
		cliVars.Set(env[:i], env[i+1:])
	}

	// SUP_ENV генерируется только cli
	// Разделить цикл, чтобы исключить дублирование
	supEnv := ""
	for _, v := range cliVars {
		supEnv += fmt.Sprintf(" -e %v=%q", v.Key, v.Value)
	}
	vars.Set("SUP_ENV", strings.TrimSpace(supEnv))

	// Создайте новое приложение Stackup
	app, err := sup.New(conf)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	app.Debug(debug)
	app.Prefix(!disablePrefix)

	// Запустить все команды в указанной сети
	err = app.Run(network, vars, commands...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
