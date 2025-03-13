pipeline {
    agent any
    stages {
        stage('Install usup') {
            steps {
                script {
                    sh """
                        GITHUB_LATEST_VERSION=\$(curl -L -sS -H 'Accept: application/json' https://github.com/Lifailon/usup/releases/latest | sed -e 's/.*"tag_name":"\\([^"]*\\)".*/\\1/')
                        BIN_URL="https://github.com/Lifailon/usup/releases/download/\$GITHUB_LATEST_VERSION/usup-\$GITHUB_LATEST_VERSION-linux-amd64"
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
                        sh "${env.WORKSPACE}/usup ${options} ${params.network} ${params.command}"
                    } else {
                        sh "${env.WORKSPACE}/usup ${options} ${params.network} ${params.target}"
                    }
                }
            }
        }
    }
    post {
        always {
            script {
                sh """
                    rm ${env.WORKSPACE}/usup
                """
            }
        }
    }
}
