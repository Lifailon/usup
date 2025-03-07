package sup

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
)

// Задание представляет собой набор команд, которые нужно выполнить
// Основная структура Task
type Task struct {
	Run     string
	Input   io.Reader
	Clients []Client
	TTY     bool
}

func (sup *Stackup) createTasks(cmd *Command, clients []Client, env string) ([]*Task, error) {
	var tasks []*Task

	cwd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "resolving CWD failed")
	}

	// Upload task
	for _, upload := range cmd.Upload {
		uploadFile, err := ResolveLocalPath(cwd, upload.Src, env)
		if err != nil {
			return nil, errors.Wrap(err, "upload: "+upload.Src)
		}
		uploadTarReader, err := NewTarStreamReader(cwd, uploadFile, upload.Exc)
		if err != nil {
			return nil, errors.Wrap(err, "upload: "+upload.Src)
		}

		task := Task{
			Run:   RemoteTarCommand(upload.Dst),
			Input: uploadTarReader,
			TTY:   false,
		}

		if cmd.Once {
			task.Clients = []Client{clients[0]}
			tasks = append(tasks, &task)
		} else if cmd.Serial > 0 {
			// Каждая клиентская группа серийных задач (из target) выполняется последовательно
			for i := 0; i < len(clients); i += cmd.Serial {
				j := i + cmd.Serial
				if j > len(clients) {
					j = len(clients)
				}
				copy := task
				copy.Clients = clients[i:j]
				tasks = append(tasks, &copy)
			}
		} else {
			task.Clients = clients
			tasks = append(tasks, &task)
		}
	}

	// Script - читать файл как многострочную команду
	if cmd.Script != "" {
		f, err := os.Open(cmd.Script)
		if err != nil {
			return nil, errors.Wrap(err, "can't open script")
		}
		data, err := ioutil.ReadAll(f)
		if err != nil {
			return nil, errors.Wrap(err, "can't read script")
		}

		task := Task{
			Run: string(data),
			TTY: true,
		}
		if sup.debug {
			task.Run = "set -x;" + task.Run
		}
		if cmd.Stdin {
			task.Input = os.Stdin
		}
		if cmd.Once {
			task.Clients = []Client{clients[0]}
			tasks = append(tasks, &task)
		} else if cmd.Serial > 0 {
			// Каждая клиентская группа серийных задач (из target) выполняется последовательно
			for i := 0; i < len(clients); i += cmd.Serial {
				j := i + cmd.Serial
				if j > len(clients) {
					j = len(clients)
				}
				copy := task
				copy.Clients = clients[i:j]
				tasks = append(tasks, &copy)
			}
		} else {
			task.Clients = clients
			tasks = append(tasks, &task)
		}
	}

	// Локальная команда (localhost)
	if cmd.Local != "" {
		local := &LocalhostClient{
			env: env + `export SUP_HOST="localhost";`,
		}
		local.Connect("localhost")
		task := &Task{
			Run:     cmd.Local,
			Clients: []Client{local},
			TTY:     true,
		}
		if sup.debug {
			task.Run = "set -x;" + task.Run
		}
		if cmd.Stdin {
			task.Input = os.Stdin
		}
		tasks = append(tasks, task)
	}

	// Удаленная команда (ssh)
	if cmd.Run != "" {
		task := Task{
			Run: cmd.Run,
			TTY: true,
		}
		if sup.debug {
			task.Run = "set -x;" + task.Run
		}
		if cmd.Stdin {
			task.Input = os.Stdin
		}
		if cmd.Once {
			task.Clients = []Client{clients[0]}
			tasks = append(tasks, &task)
		} else if cmd.Serial > 0 {
			// Каждая клиентская группа серийных задач (из target) выполняется последовательно
			for i := 0; i < len(clients); i += cmd.Serial {
				j := i + cmd.Serial
				if j > len(clients) {
					j = len(clients)
				}
				copy := task
				copy.Clients = clients[i:j]
				tasks = append(tasks, &copy)
			}
		} else {
			task.Clients = clients
			tasks = append(tasks, &task)
		}
	}

	return tasks, nil
}

type ErrTask struct {
	Task   *Task
	Reason string
}

func (e ErrTask) Error() string {
	return fmt.Sprintf(`Run("%v"): %v`, e.Task, e.Reason)
}
