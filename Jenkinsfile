def withGithubNotifier(String buildContext, Closure<Void> job) {
   setGitHubPullRequestStatus(context: buildContext, message: 'Build started', state: "PENDING")
   try {
        job()
        setGitHubPullRequestStatus(context: buildContext, message: 'Job status: SUCCESS', state: "SUCCESS")
        
   } catch(e) {
        echo 'Err: Build failed with Error -> ' + e.toString()
        setGitHubPullRequestStatus(context: buildContext, message: 'Job status: FAILURE', state: "ERROR")
        throw e
   } finally {
        if (fileExists("${WORKSPACE}/gopath/src/github.com/Azure/dcos-engine/test/src/_logs")) { archiveArtifacts allowEmptyArchive: true, artifacts: '**/_logs/*' } 
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
    withCredentials([usernamePassword(credentialsId: env.AZURE_CREDENTIALS_ID,
                                      passwordVariable: 'SERVICE_PRINCIPAL_CLIENT_SECRET',
                                      usernameVariable: 'SERVICE_PRINCIPAL_CLIENT_ID')]) {
    withEnv(["GOPATH=$WORKSPACE/gopath",
             "PATH+GO=/usr/local/go/bin:$WORKSPACE/gopath/bin",
             "AZURE_CONFIG_DIR=$WORKSPACE/azure_config_dir",
             "DCOS_ENGINE_EXE=$WORKSPACE/gopath/src/github.com/Azure/dcos-engine/bin/dcos-engine",
             "SUBSCRIPTION_ID=${env.SUBSCRIPTION_ID}",
             "TENANT_ID=${env.TENANT_ID}",
             "STAGE_TIMEOUT_MIN=60",
             "TEST_CONFIG=dcos1.12-hyb.json",
             "NUM_OF_RETRIES=1",
             "AUTOCLEAN=false",
             "RESOURCE_GROUP_PREFIX=r-pr-${env.GITHUB_PR_NUMBER}-hyb-rs4",
             "LINUX_VMSIZE=Standard_D2s_v3",
             "WINDOWS_VMSIZE=Standard_D8s_v3",
             "WINDOWS_IMAGE=MicrosoftWindowsServer,WindowsServerSemiAnnual,Datacenter-Core-1803-with-Containers-smalldisk",
             "WINDOWS_MARATHON_APP=iis-rs4-marathon-template.json",
             "KEYVAULT_NAME=${env.KEYVAULT_NAME}",
             "SSH_KEY_SECRET_NAME=${env.SSH_KEY_SECRET_NAME}",
             "WINDOWS_PASSWORD_SECRET_NAME=${env.WINDOWS_PASSWORD_SECRET_NAME}"]) {
                println "Bash shell executes -> $command"
                sh command
             }
        }
}

def dcosEngineTesting = { dcos_engine_testing_env('cd gopath/src/github.com/Azure/dcos-engine && make bootstrap && make test') }
def dcosRegressionTesting = { dcos_engine_testing_env("""
                                 cd gopath/src/github.com/Azure/dcos-engine &&
                                 make build &&
                                 cd test/src && 
                                 ./dcos-engine-test -d \${DCOS_ENGINE_EXE} -c ../config/\${TEST_CONFIG}
                             """) }

runBuild('jenkins/unit-testing', env.TEST_NODE_NAME,   dcosEngineTesting)
runBuild('jenkins/regression-dcos-1.11-hyb-rs4', env.TEST_NODE_NAME, dcosRegressionTesting)
