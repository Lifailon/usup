def SSH_KEY_FILE = ""

pipeline {
    agent any
    options {
        // Add support for standard ANSI escape sequences via plugin ansiColor (https://plugins.jenkins.io/ansicolor)
        ansiColor('xterm')
    }
    stages {
        stage('Get ssh key') {
            steps {
                script {
                    SSH_KEY_FILE = sh(
                        script: "echo $HOME",
                        returnStdout: true
                    ).trim() + "/.ssh/id_rsa"
                    withCredentials(
                        [
                            sshUserPrivateKey(
                                credentialsId: params.credentials,
                                keyFileVariable: 'SSH_KEY',
                                passphraseVariable: ''
                            )
                        ]
                    ) {
                        writeFile(
                            file: SSH_KEY_FILE,
                            text: readFile(SSH_KEY)
                        )
                        sh "chmod 600 ${SSH_KEY_FILE}"
                    }
                }
            }
        }
        stage('Install usup') {
            steps {
                script {
                    // Get arch on agent and download usup binary from GitHub
                    sh """
                        GITHUB_LATEST_VERSION=\$(curl -L -sS -H 'Accept: application/json' https://github.com/Lifailon/usup/releases/latest | sed -e 's/.*"tag_name":"\\([^"]*\\)".*/\\1/')
                        ARCH=\$(uname -m)
                        case \$ARCH in
                            x86_64|amd64) ARCH="amd64" ;;
                            aarch64) ARCH="arm64" ;;
                        esac
                        BIN_URL="https://github.com/Lifailon/usup/releases/download/\$GITHUB_LATEST_VERSION/usup-\$GITHUB_LATEST_VERSION-linux-\$ARCH"
                        curl -L -sS "\$BIN_URL" -o ${env.WORKSPACE}/usup
                        chmod +x ${env.WORKSPACE}/usup
                        ${env.WORKSPACE}/usup -v
                    """
                }
            }
        }
        stage('Run usup') {
            steps {
                script {
                    // Get host list from param
                    writeFile file: 'hostlist', text: params.localHostList
                    def hostlist = readFile('hostlist')
                    echo "Local host list in file:\n${hostlist}"

                    // Check params
                    def options = "-u https://raw.githubusercontent.com/${params.repoPath}/refs/heads/main/${params.fileName}"
                    sh "${env.WORKSPACE}/usup ${options} || true"
                    sh "${env.WORKSPACE}/usup ${options} ${params.network} || true"

                    // Get env from param
                    def envVars = params.envVars.trim().split('\n')
                    def envParams = []
                    for (line in envVars) {
                        if (line.trim() ==~ /^[A-Za-z_][A-Za-z0-9_]*=.*/) {
                            envParams << "-e \"${line.trim()}\""
                        }
                    }
                    def envList = envParams.join(' ')
                    echo "Environment variables list for flag: ${envList}"

                    // Set options
                    if (envParams.size() > 0) {
                        options += " ${envList}"
                    }

                    // Run usup
                    if (params.target == "null") {
                        sh """
                            ${env.WORKSPACE}/usup ${options} ${params.network} ${params.command}
                        """
                    } else {
                        sh """
                            ${env.WORKSPACE}/usup ${options} ${params.network} ${params.target}
                        """
                    }
                }
            }
        }
    }
    post {
        always {
            script {
                sh """
                    rm -f ${env.WORKSPACE}/usup ${SSH_KEY_FILE}
                """
            }
        }
    }
}
