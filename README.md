# Update > Stack Up

A very simple deployment tool that runs a given set of bash commands on multiple hosts in parallel. It reads `usupfile.yml/supfile.yml` (`yaml` configuration), which defines networks (groups of hosts), global variables, commands and targets (groups of commands).

The goal is to revive the [sup](https://github.com/pressly/sup) project, which has not been supported since 2018. First of all, to solve common problems (for example, an error when connecting via ssh), expand the functionality (for example, add reading the host list or configuration from the url) and implement a simple user interface.

## Install

Run the command in the console to quickly install or update the stable version for Linux or macOS system:

```shell
curl -sS https://raw.githubusercontent.com/Lifailon/usup/main/install.sh | bash
```

To install on Windows, download the binary file from the [releases](https://github.com/Lifailon/usup/releases) page.

## Usage

```bash
usup [OPTIONS] NETWORK COMMAND

usup dev date
usup -u https://raw.githubusercontent.com/Lifailon/usup/refs/heads/main/usupfile.yml dev date
```

### Supported file names

Usup will look for the following file names, in order of priority:

```
usupfile.yml
usupfile.yaml
Usupfile.yml
Usupfile.yaml
supfile.yml
supfile.yaml
Supfile.yml
Supfile.yaml
```

### Options

| Option                                  | Description                         |
| -                                       | -                                   |
| `-f usupfile.yml`                       | Custom path to file configuration   |
| `-u https://example.com/usupfile.yml`   | Url path to file configuration      |
| `-e`, `--env=[]`                        | Set environment variables           |
| `--only REGEXP`                         | Filter hosts matching regexp        |
| `--except REGEXP`                       | Filter out hosts matching regexp    |
| `-D`, `--debug`                         | Enable debug/verbose mode           |
| `--disable-prefix`                      | Disable hostname prefix             |
| `-h`, `--help`                          | Show help/usage                     |
| `-v`, `--version`                       | Print version                       |

## Network

Static and dynamic host list.

```yaml
networks:
  local:
    hosts:
      - localhost
  dev:
    hosts:
      - lifailon@192.168.3.101:2121
      - lifailon@192.168.3.104:2121
      - lifailon@192.168.3.105:2121
  bsd:
    # Read host list from URL in Linux
    inventory: curl -s https://raw.githubusercontent.com/Lifailon/usup/refs/heads/main/hostlist
    # Windows PowerShell or PowerShell Core
    # inventory: irm https://raw.githubusercontent.com/Lifailon/usup/refs/heads/main/hostlist
    # Local from file
    # inventory: cat ./hostlist
```

## Variables and Command

```yaml
env:
  FILE_NAME: test
  FILE_FORMAT: txt

networks:
  local:
    hosts:
      - localhost

commands:
  echo:
    desc: Print filename from env vars
    run: echo $FILE_NAME.$FILE_FORMAT

  file:
    desc: Creat new test file
    run: echo "This is test" > ./$FILE_NAME.$FILE_FORMAT
```

`usup local echo` output the contents of the variables

`usup local file` create test file on the local machine

### Serial and once command

`serial: N` constraints a command to be run on `N` hosts at a time at maximum.

```yaml
commands:
  echo:
    desc: Print filename from env vars
    run: echo $FILE_NAME.$FILE_FORMAT
    serial: 2
```

`once: true` constraints a command to be run only on one host.

```yaml
commands:
  file:
    desc: Creat new test file
    run: echo "This is test" > ./$FILE_NAME.$FILE_FORMAT
    once: true
```

`usup dev echo file`

### Local command

Runs command always on localhost.

```yaml
commands:
    build:
        desc: Build in Windows
        local: go build -o ./bin/sup.exe ./cmd/sup
```

### Upload command

Uploads files/directories to all remote hosts (uses `tar` under the hood).

```yaml
commands:
  upload:
    desc: Upload dist files to all hosts
    upload:
      - src: ./$FILE_NAME.$FILE_FORMAT
        dst: /tmp/
```

### Interactive Bash on all hosts

```yaml
commands:
  bash:
    desc: Interactive Bash on all hosts
    stdin: true
    run: bash
```

Send commands to all hosts simultaneously for execution.

```bash
echo 'sudo apt-get update -y && sudo apt-get upgrade -y' | usup production bash
# or
usup dev bash
# ^C
```

## Target

Target is an alias for multiple commands. Each command will be run on all hosts in parallel,
`usup` will check return status from all hosts, and run subsequent commands on success only
(thus any error on any host will interrupt the process).

```yaml
targets:
  get:
    - uptime
    - date
  up:
    - upload
    - cat
```

`usup dev get` get uptime and current time in the system from all hosts simultaneously

`usup dev up` download and read the file

### Default environment variables available in Supfile

| Variable Name     | Description                                               |
| -                 | -                                                         |
| `$SUP_HOST`       | Current host                                              |
| `$SUP_NETWORK`    | Current network                                           |
| `$SUP_USER`       | User who invoked sup command                              |
| `$SUP_TIME`       | Date/time of sup command invocation                       |
| `$SUP_ENV`        | Environment variables provided on sup command invocation  |

## License

Licensed under the [MIT License](./LICENSE).
