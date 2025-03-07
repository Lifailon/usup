package sup

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/pkg/errors"

	"gopkg.in/yaml.v2"
)

// Структура Supfile в формате YAML
type Supfile struct {
	Networks Networks `yaml:"networks"`
	Commands Commands `yaml:"commands"`
	Targets  Targets  `yaml:"targets"`
	Env      EnvList  `yaml:"env"`
	Version  string   `yaml:"version"`
}

// Сеть - это группа хостов с дополнительными пользовательскими параметрами переменных (env)
type Network struct {
	Env          EnvList  `yaml:"env"`
	Inventory    string   `yaml:"inventory"`
	Hosts        []string `yaml:"hosts"`
	Bastion      string   `yaml:"bastion"` // Jump host for the environment
	User         string   // `yaml:"user"`
	IdentityFile string   // `yaml:"identity_file"`
}

// Сети - это список сетей, определенных пользователем
type Networks struct {
	Names []string
	nets  map[string]Network
}

func (n *Networks) UnmarshalYAML(unmarshal func(interface{}) error) error {
	err := unmarshal(&n.nets)
	if err != nil {
		return err
	}

	var items yaml.MapSlice
	err = unmarshal(&items)
	if err != nil {
		return err
	}

	n.Names = make([]string, len(items))
	for i, item := range items {
		n.Names[i] = item.Key.(string)
	}

	return nil
}

func (n *Networks) Get(name string) (Network, bool) {
	net, ok := n.nets[name]
	return net, ok
}

// Command представляет команду/команды для удаленного выполнения
type Command struct {
	Name    string   `yaml:"-"`        // Название команды
	Desc    string   `yaml:"desc"`     // Описание команды
	Local   string   `yaml:"local"`    // Для локального запуска
	Run     string   `yaml:"run"`      // Для удаленного запуска
	Script  string   `yaml:"script"`   // Загрузить команду из скрипта и запустить ее удаленно
	Upload  []Upload `yaml:"upload"`   // Структура Upload
	Stdin   bool     `yaml:"stdin"`    // да/нет - присоединить STDOUT локального хоста к STDIN удаленных команд
	Once    bool     `yaml:"once"`     // да/нет - команда должна быть запущена один раз (только на одном хосте).
	Serial  int      `yaml:"serial"`   // Максимальное количество клиентов, обрабатывающих задачу параллельно
	RunOnce bool     `yaml:"run_once"` // да/нет - команда должна быть выполнена только один раз
}

// Список команд (определяемых пользователем)
type Commands struct {
	Names []string
	cmds  map[string]Command
}

func (c *Commands) UnmarshalYAML(unmarshal func(interface{}) error) error {
	err := unmarshal(&c.cmds)
	if err != nil {
		return err
	}

	var items yaml.MapSlice
	err = unmarshal(&items)
	if err != nil {
		return err
	}

	c.Names = make([]string, len(items))
	for i, item := range items {
		c.Names[i] = item.Key.(string)
	}

	return nil
}

func (c *Commands) Get(name string) (Command, bool) {
	cmd, ok := c.cmds[name]
	return cmd, ok
}

// Список целей (определяемых пользователем)
type Targets struct {
	Names   []string
	targets map[string][]string
}

func (t *Targets) UnmarshalYAML(unmarshal func(interface{}) error) error {
	err := unmarshal(&t.targets)
	if err != nil {
		return err
	}

	var items yaml.MapSlice
	err = unmarshal(&items)
	if err != nil {
		return err
	}

	t.Names = make([]string, len(items))
	for i, item := range items {
		t.Names[i] = item.Key.(string)
	}

	return nil
}

func (t *Targets) Get(name string) ([]string, bool) {
	cmds, ok := t.targets[name]
	return cmds, ok
}

// Операция копирования файла из локального хоста по пути из Src в Dst
type Upload struct {
	Src string `yaml:"src"`
	Dst string `yaml:"dst"`
	Exc string `yaml:"exclude"`
}

// Переменная окружения
type EnvVar struct {
	Key   string
	Value string
}

func (e EnvVar) String() string {
	return e.Key + `=` + e.Value
}

// Возвращает переменную окружения в виде оператора экспорта bash
func (e EnvVar) AsExport() string {
	return `export ` + e.Key + `="` + e.Value + `";`
}

// EnvList - это список переменных окружения, который отображается на карту YAML,
// но сохраняет порядок, позволяя поздним переменным ссылаться на ранние.
type EnvList []*EnvVar

func (e EnvList) Slice() []string {
	envs := make([]string, len(e))
	for i, env := range e {
		envs[i] = env.String()
	}
	return envs
}

func (e *EnvList) UnmarshalYAML(unmarshal func(interface{}) error) error {
	items := []yaml.MapItem{}

	err := unmarshal(&items)
	if err != nil {
		return err
	}

	*e = make(EnvList, 0, len(items))

	for _, v := range items {
		e.Set(fmt.Sprintf("%v", v.Key), fmt.Sprintf("%v", v.Value))
	}

	return nil
}

// Задать для ключа значение, равное значению в этом списке
func (e *EnvList) Set(key, value string) {
	for i, v := range *e {
		if v.Key == key {
			(*e)[i].Value = value
			return
		}
	}

	*e = append(*e, &EnvVar{
		Key:   key,
		Value: value,
	})
}

func (e *EnvList) ResolveValues() error {
	if len(*e) == 0 {
		return nil
	}

	exports := ""
	for i, v := range *e {
		exports += v.AsExport()

		cmd := exec.Command("bash", "-c", exports+"echo -n "+v.Value+";")
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		cmd.Dir = cwd
		resolvedValue, err := cmd.Output()
		if err != nil {
			return errors.Wrapf(err, "resolving env var %v failed", v.Key)
		}

		(*e)[i].Value = string(resolvedValue)
	}

	return nil
}

func (e *EnvList) AsExport() string {
	// Переработайте все переменные (env) в строку вида:
	// `export FOO="bar"; export BAR="baz";`.
	exports := ``
	for _, v := range *e {
		exports += v.AsExport() + " "
	}
	return exports
}

type ErrMustUpdate struct {
	Msg string
}

type ErrUnsupportedSupfileVersion struct {
	Msg string
}

func (e ErrMustUpdate) Error() string {
	return fmt.Sprintf("%v\n\nPlease update sup by `go get -u github.com/Lifailon/usup/cmd/sup`", e.Msg)
}

func (e ErrUnsupportedSupfileVersion) Error() string {
	return fmt.Sprintf("%v\n\nCheck your Supfile version (available latest version: v0.6.0)", e.Msg)
}

// Парсим конфигурационный файл и возвращаем Supfile или ошибку
func NewSupfile(data []byte) (*Supfile, error) {
	var conf Supfile

	if err := yaml.Unmarshal(data, &conf); err != nil {
		return nil, err
	}

	return &conf, nil
}

// Запустить команду инвентаризации (если она была предоставлена), и добавить выходные строки команды к списку хостов, заданному вручную
func (n Network) ParseInventory() ([]string, error) {
	if n.Inventory == "" {
		return nil, nil
	}

	// Выполняем команду в Windows или Linux для чтения файла из Inventory
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Проверяем, установлен ли PowerShell Core
		if _, err := exec.LookPath("pwsh"); err == nil {
			cmd = exec.Command("pwsh", "-Command", n.Inventory)
		} else {
			// cmd = exec.Command("cmd", "/C", n.Inventory)
			cmd = exec.Command("powershell", "-Command", n.Inventory)
		}
	} else {
		cmd = exec.Command("sh", "-c", n.Inventory)
	}
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, n.Env.Slice()...)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var hosts []string
	buf := bytes.NewBuffer(output)
	for {
		host, err := buf.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		host = strings.TrimSpace(host)
		// Пропускать пустые строки и комментарии
		if host == "" || host[:1] == "#" {
			continue
		}

		hosts = append(hosts, host)
	}
	return hosts, nil
}
