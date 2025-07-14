def SSH_KEY_FILE = ""

pipeline {
    agent any
    options {
        timeout(time: 10, unit: 'MINUTES')
        // Plugin: https://plugins.jenkins.io/ansicolor
        ansiColor('xterm')
    }
    // Params: https://www.jenkins.io/doc/pipeline/steps/pipeline-input-step
    parameters {
        // String Parameter
        string(
            name: 'repoPath',
            description: 'Set the repository on GitHub that contains the configurations.\nFormat: <USERNAME/REPOSITORY>.',
            defaultValue: 'Lifailon/usup',
            trim: true
        )
        // Active Choices Reactive Parameter
        // Plugin: https://plugins.jenkins.io/uno-choice
        reactiveChoice(
            name: 'repoBranch',
            description: 'Select branch.',
            choiceType: 'PT_RADIO',
            filterable: false,
            script: [
                $class: 'GroovyScript',
                script: [
                    sandbox: true,
                    script: '''
                        import groovy.json.JsonSlurper

                        def url = "https://api.github.com/repos/${repoPath}/branches"
                        def URL = new URL(url)
                        def connection = URL.openConnection()
                        connection.requestMethod = 'GET'
                        connection.setRequestProperty("Accept", "application/vnd.github.v3+json")
                        def response = connection.inputStream.text

                        def json = new JsonSlurper().parseText(response)
                        def branches = json.collect { it.name }
                        return branches as List
                    '''
                ]
            ],
            referencedParameters: 'repoPath'
        )
        reactiveChoice(
            name: 'fileName',
            description: 'Select configuration file in yml or yaml format.',
            choiceType: 'PT_SINGLE_SELECT',
            filterable: true,
            filterLength: 1,
            script: [
                $class: 'GroovyScript',
                script: [
                    sandbox: true,
                    script: '''
                        import groovy.json.JsonSlurper

                        def url = "https://api.github.com/repos/${repoPath}/git/trees/${repoBranch}?recursive=1"
                        def URL = new URL(url)
                        def connection = URL.openConnection()
                        connection.requestMethod = 'GET'
                        def response = connection.inputStream.text

                        def json = new JsonSlurper().parseText(response)
                        def yamlFiles = json.tree.findAll { 
                            (it.path.endsWith('.yml') || it.path.endsWith('.yaml')) && 
                            !it.path.split('/').any { dir -> dir.startsWith('.') }
                        }.collect { it.path }
                        return yamlFiles as List
                    '''
                ]
            ],
            referencedParameters: 'repoPath,repoBranch'
        )
        reactiveChoice(
            name: 'network',
            description: 'Select network (aliace for host list).',
            choiceType: 'PT_SINGLE_SELECT',
            filterable: true,
            filterLength: 1,
            script: [
                $class: 'GroovyScript',
                script: [
                    sandbox: true,
                    script: '''
                        import org.yaml.snakeyaml.Yaml

                        def url = "https://raw.githubusercontent.com/${repoPath}/refs/heads/${repoBranch}/${fileName}"
                        def supfile = new URL(url).getText()

                        def yaml = new Yaml()
                        def data = yaml.load(supfile)

                        return data.networks.keySet() as List
                    '''
                ]
            ],
            referencedParameters: 'repoPath,repoBranch,fileName'
        )
        // Multi-line String Parameter
        text(
            name: 'localHostList',
            defaultValue: 'lifailon@192.168.3.105:2121\nlifailon@192.168.3.106:2121',
            description: 'Set the host list.\nEach host on a new line in the format <USERNAME@HOSTNAME:PORT>.\n⚠️ To use the parameter, select the network: "local-host-list".'
        )
        reactiveChoice(
            name: 'command',
            description: 'Select command for execution.',
            choiceType: 'PT_SINGLE_SELECT',
            filterable: true,
            filterLength: 1,
            script: [
                $class: 'GroovyScript',
                script: [
                    sandbox: true,
                    script: '''
                        import org.yaml.snakeyaml.Yaml

                        def url = "https://raw.githubusercontent.com/${repoPath}/refs/heads/${repoBranch}/${fileName}"
                        def supfile = new URL(url).getText()

                        def yaml = new Yaml()
                        def data = yaml.load(supfile)

                        return data.commands.keySet() as List
                    '''
                ]
            ],
            referencedParameters: 'repoPath,repoBranch,fileName'
        )
        reactiveChoice(
            name: 'target',
            description: 'Select target (alias for a group of commands) to execution.\n⚠️ Use "null" to execution the selected command.',
            choiceType: 'PT_SINGLE_SELECT',
            filterable: true,
            filterLength: 1,
            script: [
                $class: 'GroovyScript',
                script: [
                    sandbox: true,
                    script: '''
                        import org.yaml.snakeyaml.Yaml

                        def url = "https://raw.githubusercontent.com/${repoPath}/refs/heads/${repoBranch}/${fileName}"
                        def supfile = new URL(url).getText()

                        def yaml = new Yaml()
                        def data = yaml.load(supfile)

                        def targetsList = data.targets.keySet() as List
                        targetsList.add(0, null)
                        return targetsList
                    '''
                ]
            ],
            referencedParameters: 'repoPath,repoBranch,fileName'
        )
        activeChoiceHtml(
            name: 'env',
            description: 'List of environment variables used (read-only parameter).',
            choiceType: 'ET_UNORDERED_LIST',
            script: [
                $class: 'GroovyScript',
                script: [
                    sandbox: true,
                    script: '''
                        import org.yaml.snakeyaml.Yaml

                        def url = "https://raw.githubusercontent.com/${repoPath}/refs/heads/${repoBranch}/${fileName}"
                        def supfile = new URL(url).getText()

                        def yaml = new Yaml()
                        def data = yaml.load(supfile)

                        def keyValueList = []
                        for (entry in data.env.entrySet()) {
                            keyValueList.add("${entry.key}=${entry.value}")
                        }
                        return keyValueList as List
                    '''
                ]
            ],
            referencedParameters: 'repoPath,repoBranch,fileName'
        )
        text(
            name: 'envVars',
            description: 'Change variable values in the format <KEY=VALUE> (each variable on a new line).\nExample: COMMAND=uptime (used in "run-env" command).'
        )
        // Credentials Parameter
        credentials(
            name: 'credentials',
            description: 'SSH Username with private key from Jenkins Credentials for ssh connection.',
            credentialType: 'SSH Username with private key'
        )
        // Boolean Parameter
        booleanParam(
            name: "debug",
            defaultValue: false,
            description: 'Enable debug mode.'
        )
    }
    stages {
        stage('Check params') {
            steps {
                script {
                    echo "Selected repository: ${params.repoPath}"
                    echo "Selected branch: ${params.repoBranch}"
                    echo "Selected fileName: ${params.fileName}"
                    echo "Selected network: ${params.network}"
                    echo "Selected localHostList: ${params.localHostList}"
                    echo "Selected command: ${params.command}"
                    echo "Selected target: ${params.target}"
                    echo "Selected env: ${params.env}"
                    echo "Selected envVars: ${params.envVars}"
                    echo "Selected credentials: ${params.credentials}"
                }
            }
        }
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

                    // Add debug mode
                    if (params.debug) {
                        options += " -D"
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
