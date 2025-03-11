// String Parameter: repoPath
// Description: Format: <UserName/Repository>
// def repoPath = "Lifailon/usup"

// Active Choices Reactive Parameter: repoBranch
// Description: Select branch
// Referenced parameters: repoPath

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

// Active Choices Reactive Parameter: fileName
// Description: Select configuration file (supfile in yml/yaml format)
// Referenced parameters: repoPath,repoBranch

import groovy.json.JsonSlurper

def url = "https://api.github.com/repos/${repoPath}/git/trees/${repoBranch}?recursive=1"
def URL = new URL(url)
def connection = URL.openConnection()
connection.requestMethod = 'GET'
def response = connection.inputStream.text

def json = new JsonSlurper().parseText(response)
def yamlFiles = json.tree.findAll { it.path.endsWith('.yml') || it.path.endsWith('.yaml') }.collect { it.path }
return yamlFiles as List

// Active Choices Reactive Parameter: network
// Description: Select network (host list)
// Referenced parameters: repoPath,repoBranch,fileName

import org.yaml.snakeyaml.Yaml

def url = "https://raw.githubusercontent.com/${repoPath}/refs/heads/${repoBranch}/${fileName}"
def supfile = new URL(url).getText()

def yaml = new Yaml()
def data = yaml.load(supfile)

return data.networks.keySet() as List

// Active Choices Reactive Parameter: command
// Description: Select command for run
// Referenced parameters: repoPath,repoBranch,fileName

import org.yaml.snakeyaml.Yaml

def url = "https://raw.githubusercontent.com/${repoPath}/refs/heads/${repoBranch}/${fileName}"
def supfile = new URL(url).getText()

def yaml = new Yaml()
def data = yaml.load(supfile)

return data.commands.keySet() as List

// Active Choices Reactive Parameter: target
// Description: Select target (groups of commands) for run or using null for command run
// Referenced parameters: repoPath,repoBranch,fileName

import org.yaml.snakeyaml.Yaml

def url = "https://raw.githubusercontent.com/${repoPath}/refs/heads/${repoBranch}/${fileName}"
def supfile = new URL(url).getText()

def yaml = new Yaml()
def data = yaml.load(supfile)

def targetsList = data.targets.keySet() as List
targetsList.add(0, null)
return targetsList

// Active Choices Reactive Reference Parameter: env
// Description: Environment variables list
// Referenced parameters: repoPath,repoBranch,fileName

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