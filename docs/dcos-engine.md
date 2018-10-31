# Microsoft DC/OS Engine

The DC/OS Engine (`dcos-engine`) generates ARM (Azure Resource Manager) templates for Docker enabled clusters on Microsoft Azure DCOS orchestrator. The input to dcos-engine is a cluster definition file which describes the desired cluster, including orchestrator, features, and agents.

<a href="#install-dcos-engine"></a>

## Install

Binary downloads for the latest version of dcos-engine for are available [here](https://github.com/Azure/dcos-engine/releases/latest). Download `dcos-engine` for your operating system. Extract the binary and copy it to your `$PATH`.

You can also choose to install dcos-engine using [gofish](https://www.gofi.sh/#about), to do so execute the command `gofish install dcos-engine` . You can install gofish following the [instructions](https://www.gofi.sh/#install) for your OS.

If you would prefer to build `dcos-engine` from source or you are interested in contributing to `dcos-engine` see [building from source](#build-dcos-engine-from-source) below.

## Usage

`dcos-engine` reads a JSON [cluster definition](./clusterdefinition.md) and generates a number of files that may be submitted to Azure Resource Manager (ARM). The generated files include:

1. **apimodel.json**: is an expanded version of the cluster definition provided to the generate command. All default or computed values will be expanded during the generate phase.
2. **azuredeploy.json**: represents a complete description of all Azure resources required to fulfill the cluster definition from `apimodel.json`.
3. **azuredeploy.parameters.json**: the parameters file holds a series of custom variables which are used in various locations throughout `azuredeploy.json`.

### Generate Templates

DC/OS Engine consumes a cluster definition which outlines the desired shape, size, and configuration of DC/OS. There are a number of features that can be enabled through the cluster definition.

<a href="#deployment-usage"></a>

### Deploy Templates

Generated templates can be deployed using [the Azure CLI 2.0](https://github.com/Azure/azure-cli) or [Powershell](https://github.com/Azure/azure-powershell).

#### Deploying with Azure CLI 2.0

Azure CLI 2.0 is the latest CLI maintained and supported by Microsoft. For installation instructions see [the Azure CLI GitHub repository](https://github.com/Azure/azure-cli#installation) for the latest release.

```bash
$ az login

$ az account set --subscription "<SUBSCRIPTION NAME OR ID>"

$ az group create \
    --name "<RESOURCE_GROUP_NAME>" \
    --location "<LOCATION>"

$ az group deployment create \
    --name "<DEPLOYMENT NAME>" \
    --resource-group "<RESOURCE_GROUP_NAME>" \
    --template-file "./_output/<INSTANCE>/azuredeploy.json" \
    --parameters "./_output/<INSTANCE>/azuredeploy.parameters.json"
```

#### Deploying with Powershell

```powershell
Add-AzureRmAccount

Select-AzureRmSubscription -SubscriptionID <SUBSCRIPTION_ID>

New-AzureRmResourceGroup `
    -Name <RESOURCE_GROUP_NAME> `
    -Location <LOCATION>

New-AzureRmResourceGroupDeployment `
    -Name <DEPLOYMENT_NAME> `
    -ResourceGroupName <RESOURCE_GROUP_NAME> `
    -TemplateFile _output\<INSTANCE>\azuredeploy.json `
    -TemplateParameterFile _output\<INSTANCE>\azuredeploy.parameters.json
```

<a href="#build-from-source"></a>

## Build DC/OS Engine from Source

### Docker Development Environment

The easiest way to start hacking on `dcos-engine` is to use a Docker-based environment. If you already have Docker installed then you can get started with a few commands.

* Windows (PowerShell): `.\scripts\devenv.ps1`
* Linux/OSX (bash): `./scripts/devenv.sh`

This script mounts the `dcos-engine` source directory as a volume into the Docker container, which means you can edit your source code in your favorite editor on your machine, while still being able to compile and test inside of the Docker container. This environment mirrors the environment used in the dcos-engine continuous integration (CI) system.

When the script `devenv.ps1` or `devenv.sh` completes, you will be left at a command prompt.

Run the following commands to pull the latest dependencies and build the `dcos-engine` tool.

```
# install and download build dependencies
make bootstrap
# build the `dcos-engine` binary
make build
```

The build process leaves the compiled `dcos-engine` binary in the `bin` directory. Make sure everything completed successfully by running `bin/dcos-engine` without any arguments:

```
# ./bin/dcos-engine
DC/OS Engine deploys and manages DC/OS clusters in Azure

Usage:
  dcos-engine [command]

Available Commands:
  deploy        Deploy an Azure Resource Manager template
  generate      Generate an Azure Resource Manager template
  help          Help about any command
  orchestrators Display info about supported orchestrators
  upgrade       Upgrade an existing DC/OS cluster
  version       Print the version of DC/OS Engine

Flags:
      --debug   enable verbose debug logs
  -h, --help    help for dcos-engine

Use "dcos-engine [command] --help" for more information about a command.
```

## Building on Windows, OSX, and Linux

Building DC/OS Engine from source has a few requirements for each of the platforms. Download and install the pre-reqs for your platform, Windows, Linux, or Mac:

### Prerequisite
1. Go version 1.8 [installation instructions](https://golang.org/doc/install)
2. Git Version Control [installation instructions](https://git-scm.com/download/)

### Windows

Setup steps:

1. Setup your go workspace. This guide assumes you are using `c:\gopath` as your Go workspace:
  1. Type Windows key-R to open the run prompt
  2. Type `rundll32 sysdm.cpl,EditEnvironmentVariables` to open the system variables
  3. Add `c:\go\bin` and `c:\gopath\bin` to your PATH variables
  4. Click "new" and add new environment variable named `GOPATH` and set the value to `c:\gopath`

Build dcos-engine:
  1. Type Windows key-R to open the run prompt
  2. Type `cmd` to open a command prompt
  3. Type `mkdir %GOPATH%` to create your gopath
  4. Type `cd %GOPATH%`
  5. Type `go get -d github.com/Azure/dcos-engine` to download dcos-engine from GitHub
  6. Type `go get all` to get the supporting components
  7. Type `go get -u github.com/go-bindata/go-bindata/...`
  8. Type `cd %GOPATH%\src\github.com\Azure\dcos-engine\pkg\acsengine`
  9. Type `go generate`
  10. Type `cd %GOPATH%\src\github.com\Azure\dcos-engine\pkg\i18n`
  11. Type `go generate`
  12. Type `cd %GOPATH%\src\github.com\Azure\dcos-engine\pkg\operations\dcosupgrade`
  13. Type `go generate`
  14. Type `cd %GOPATH%\src\github.com\Azure\dcos-engine`
  15. Type `go build` to build the project
  16. Type `go install` to install the project
  17. Run `dcos-engine.exe` to see the command line parameters

### OS X and Linux

Setup steps:

  1. Open a command prompt to setup your gopath:
  2. `mkdir $HOME/go`
  3. edit `$HOME/.bash_profile` and add the following lines to setup your go path
  ```
  export GOPATH=$HOME/go
  export PATH=$PATH:/usr/local/go/bin:$GOPATH/bin
  ```
  4. `source $HOME/.bash_profile`

Build dcos-engine:
  1. Type `go get github.com/Azure/dcos-engine` to get the dcos-engine Github project
  2. Type `cd $GOPATH/src/github.com/Azure/dcos-engine` to change to the source directory
  3. Type `make bootstrap` to install supporting components
  4. Type `make build` to build the project
  5. Type `./bin/dcos-engine` to see the command line parameters
