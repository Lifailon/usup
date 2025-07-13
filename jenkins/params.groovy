// 1. String Parameter: repoPath
// Description: Set the repository address on GitHub.
// Format: <USERNAME/REPOSITORY>.
// def repoPath = "Lifailon/usup"

// 2. Active Choices Reactive Parameter: repoBranch

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

// Description: Select branch.
// Referenced parameters: repoPath

// 3. Active Choices Reactive Parameter: fileName

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

// Description: Select configuration file in yml or yaml format.
// Referenced parameters: repoPath,repoBranch

// 4. Active Choices Reactive Parameter: network

import org.yaml.snakeyaml.Yaml

def url = "https://raw.githubusercontent.com/${repoPath}/refs/heads/${repoBranch}/${fileName}"
def supfile = new URL(url).getText()

def yaml = new Yaml()
def data = yaml.load(supfile)

return data.networks.keySet() as List

// Description: Select network (aliace for host list).
// Referenced parameters: repoPath,repoBranch,fileName

// 5. Multi-line String Parameter: localHostList
// Description: Set the host list.
// Each host on a new line in the format <USERNAME@HOSTNAME:PORT>.
// ⚠️ To use the parameter, select the network: "local-host-list".
// def repoPath = "Lifailon/usup"

// 6. Active Choices Reactive Parameter: command

import org.yaml.snakeyaml.Yaml

def url = "https://raw.githubusercontent.com/${repoPath}/refs/heads/${repoBranch}/${fileName}"
def supfile = new URL(url).getText()

def yaml = new Yaml()
def data = yaml.load(supfile)

return data.commands.keySet() as List

// Description: Select command for execution.
// Referenced parameters: repoPath,repoBranch,fileName

// 7. Active Choices Reactive Parameter: target
// Description: Select target (alias for a group of commands) to execution.
// ⚠️ Use "null" to execution the selected command.
// Referenced parameters: repoPath,repoBranch,fileName

import org.yaml.snakeyaml.Yaml

def url = "https://raw.githubusercontent.com/${repoPath}/refs/heads/${repoBranch}/${fileName}"
def supfile = new URL(url).getText()

def yaml = new Yaml()
def data = yaml.load(supfile)

def targetsList = data.targets.keySet() as List
targetsList.add(0, null)
return targetsList

// 8. Active Choices Reactive Reference Parameter: env

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

// Description: List of environment variables used.
// Choice Type: Bullet items list
// Referenced parameters: repoPath,repoBranch,fileName

// 9. Multi-line String Parameter: envVars
// Description: Change variable values in the format <KEY=VALUE> (each variable on a new line).

// 10. String Parameter: credentials
// Description: ID SSH Username with private key from Jenkins Credentials for ssh connection.