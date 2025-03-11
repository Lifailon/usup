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
                    if (params.target == "null") {
                        sh """
                            ${env.WORKSPACE}/usup -u https://raw.githubusercontent.com/${params.repoPath}/refs/heads/main/${params.fileName} ${params.network} ${params.command}
                        """
                    } else {
                        sh """
                            ${env.WORKSPACE}/usup -u https://raw.githubusercontent.com/${params.repoPath}/refs/heads/main/${params.fileName} ${params.network} ${params.target}
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
                    rm ${env.WORKSPACE}/usup
                """
            }
        }
    }
}
