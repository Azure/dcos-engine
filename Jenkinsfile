def withGithubNotifier(String buildContext, Closure<Void> job) {
   setGitHubPullRequestStatus(context: buildContext, message: 'Build started', state: "PENDING")
   try {
        job()
        setGitHubPullRequestStatus(context: buildContext, message: 'Job status: SUCCESS', state: "SUCCESS")
        
   } catch(e) {
        echo 'Err: Build failed with Error -> ' + e.toString()
        setGitHubPullRequestStatus(context: buildContext, message: 'Job status: FAILURE', state: "ERROR")
        throw e
   }
}

def runBuild(String buildContext, String nodeName, Closure<Void> task) {
    node(nodeName) {
        stage(buildContext.tokenize('/').last()) {
            checkout scm
            withGithubNotifier ( buildContext, task )
        }
    }
}

def dcos_engine_testing_env(String command) {
    withCredentials([usernamePassword(credentialsId: env.JENKINS_CREDENTIALS_ID,
                                      passwordVariable: 'JENKINS_PASSWORD', 
                                      usernameVariable: 'JENKINS_USER'),
                     usernamePassword(credentialsId: env.AZURE_CREDENTIALS_ID,
                                      passwordVariable: 'AZURE_SERVICE_PRINCIPAL_PASSWORD',
                                      usernameVariable: 'AZURE_SERVICE_PRINCIPAL_ID'),
                     usernamePassword(credentialsId: env.DOCKER_CREDENTIALS_ID,
                                      passwordVariable: 'DOCKER_HUB_USER_PASSWORD',
                                      usernameVariable: 'DOCKER_HUB_USER')]) {
    withEnv(["GOPATH=$WORKSPACE/gopath",
             "PATH+GO=/usr/local/go/bin:$WORKSPACE/gopath/bin"]) {
                println "Bash shell executes -> $command"
                sh command
             }
        }
}


def dcosEngineTesting = { dcos_engine_testing_env('cd gopath/src/github.com/Azure/dcos-engine && make bootstrap && make test') }

runBuild('jenkins/unit-testing', env.TEST_NODE_NAME,   dcosEngineTesting)
