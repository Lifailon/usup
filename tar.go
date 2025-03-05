package sup

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

// Копирование директорий и файлов по ssh с использованием TAR.
// tar -C . -cvzf - $SRC | ssh $HOST "tar -C $DST -xvzf -"

// RemoteTarCommand возвращает команду, которая должна быть запущена на удаленном SSH-хосте
// TODO: Проверка относительной директории.
func RemoteTarCommand(dir string) string {
	return fmt.Sprintf("tar -C \"%s\" -xzf -", dir)
}

func LocalTarCmdArgs(path, exclude string) []string {
	args := []string{}

	// Добавлены паттерны для исключения из tar compress
	excludes := strings.Split(exclude, ",")
	for _, exclude := range excludes {
		trimmed := strings.TrimSpace(exclude)
		if trimmed != "" {
			args = append(args, `--exclude=`+trimmed)
		}
	}

	args = append(args, "-C", ".", "-czf", "-", path)
	return args
}

// NewTarStreamReader создает устройство чтения потока tar из локального пути
// TODO: Вместо этого использовать "archive/tar".
func NewTarStreamReader(cwd, path, exclude string) (io.Reader, error) {
	cmd := exec.Command("tar", LocalTarCmdArgs(path, exclude)...)
	cmd.Dir = cwd
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "tar: stdout pipe failed")
	}

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "tar: starting cmd failed")
	}

	return stdout, nil
}
