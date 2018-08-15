package dcosupgrade

import (
	"fmt"
	"strings"

	"github.com/Azure/dcos-engine/pkg/acsengine"
	"github.com/Azure/dcos-engine/pkg/operations"
)

var winBootstrapUpgradeScript = `
filter Timestamp {"[\$(Get-Date -Format o)] \$_"}

function Write-Log(\$message)
{
    \$msg = \$message | Timestamp
    Write-Output \$msg
}

function CreateDockerStart(\$fileName, \$log, \$volume)
{
    \$content = "Start-Transcript -path \$log -append"
    Set-Content -Path \$fileName -Value \$content
	\$content = 'Write-Output ("[{0}] {1}" -f (Get-Date -Format o), "Starting docker container")'
	Add-Content -Path \$fileName -Value \$content
	\$content = "& docker.exe run --rm -d --network customnat -p 8086:80 -v \$volume nginx:1803"
	Add-Content -Path \$fileName -Value \$content
	\$content = '
if (\$LASTEXITCODE -ne 0) {
    Write-Output ("[{0}] {1}" -f (Get-Date -Format o), "Failed to run docker image")
    Stop-Transcript
    Exit 1
}
Write-Output ("[{0}] {1}" -f (Get-Date -Format o), "Successfully started docker container")
Stop-Transcript
Exit 0
'
    Add-Content -Path \$fileName -Value \$content
}

try {
	Write-Log "Starting upgrade configuration"
	\$BootstrapURL = "WIN_BOOTSTRAP_URL"
	\$upgradeDir = "C:\AzureData\upgrade\NEW_VERSION"
	\$genconfDir = Join-Path \$upgradeDir "genconf"
	\$logPath = Join-Path \$upgradeDir "dcos_generate_config.log"
	\$upgradeUrlPath = Join-Path \$upgradeDir "upgrade_url"

	if ( -Not (Test-Path \$upgradeUrlPath)) {
		Write-Log "Setting up Windows bootstrap node for upgrade"
		Remove-Item -Recurse -Force -ErrorAction SilentlyContinue \$upgradeDir
		New-Item -ItemType Directory -Force -Path \$genconfDir
		\$path = Join-Path \$genconfDir "config.yaml"
		cp "C:\AzureData\config-win.NEW_VERSION.yaml" \$path
		cp "c:\temp\genconf\ip-detect.ps1" \$genconfDir
		cd \$upgradeDir

		\$path = Join-Path \$upgradeDir "dcos_generate_config.windows.tar.xz"
		& curl.exe --keepalive-time 2 -fLsS --retry 20 -Y 100000 -y 60 -o \$path \$BootstrapURL
		if (\$LASTEXITCODE -ne 0) {
			throw "Failed to download \$BootstrapURL"
		}

		& tar -xvf .\dcos_generate_config.windows.tar.xz
		if (\$LASTEXITCODE -ne 0) {
			throw "Failed to untar dcos_generate_config.windows.tar.xz"
		}

		& .\dcos_generate_config.ps1 --generate-node-upgrade-script CURR_VERSION > \$logPath
		if (\$LASTEXITCODE -ne 0) {
			throw "Failed to run dcos_generate_config.ps1"
		}

		# Fetch upgrade script URL
		\$match = Select-String -Path \$logPath -Pattern "Node upgrade script URL:" -CaseSensitive
		if (-Not \$match) {
			throw "Missing Node upgrade script URL in \$logPath"
		}
		\$url = (\$match.Line -replace 'Node upgrade script URL:','').Trim()
		if (-Not \$url) {
			throw "Bad Node upgrade script URL in \$logPath"
		}

		# Stop docker container
		\$process = docker ps -q
		if (\$process) {
			Write-Log "Stopping nginx service \$process"
			& docker.exe kill \$process
		}
		Write-Log "Starting nginx service"

		# Run docker container with nginx
		cd c:\docker

		# only create customnat if it does not exist
		\$a = docker network ls | select-string -pattern "customnat"
		if (\$a.count -eq 0)
		{
			& docker.exe network create --driver="nat" --opt "com.docker.network.windowsshim.disable_gatewaydns=true" "customnat"
			if (\$LASTEXITCODE -ne 0) {
				throw "Failed to create customnat docker network"
			}
		}

		& docker.exe build --network customnat -t nginx:1803 c:\docker
		if (\$LASTEXITCODE -ne 0) {
			throw "Failed to build docker image"
		}

		\$volume = (\$genconfDir+"/serve/:c:/nginx/html:ro")
		& docker.exe run --rm -d --network customnat -p 8086:80 -v \$volume nginx:1803
		if (\$LASTEXITCODE -ne 0) {
			throw "Failed to run docker image"
		}

		CreateDockerStart "c:\docker\StartDocker.ps1" "c:\docker\StartDocker.log" \$volume

		Set-Content -Path \$upgradeUrlPath -Value \$url -Encoding Ascii
	}
	\$url = Get-Content -Path \$upgradeUrlPath -Encoding Ascii
	if (-Not \$url) {
		Remove-Item $upgradeUrlPath -Force
		throw "Failed to set up bootstrap node. Please try again"
	} else {
		# keep Write-Output - used in parsing
		Write-Output "Setting up bootstrap node completed. Node upgrade script URL \$url"
	}
} catch {
    Write-Log "Failed to upgrade Windows bootstrap node: \$_"
    exit 1
}
Write-Log "Setting up bootstrap node completed"
exit 0
`

func (uc *UpgradeCluster) upgradeWindowsBootstrapNode(masterDNS, winBootstrapIP, winBootstrapScript string) (string, error) {
	// copy bootstrap script to master
	uc.Logger.Infof("Copy Windows bootstrap script to master")
	strOut, strErr, err := operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, fmt.Sprintf("cat << END > winBootstrapUpgrade.ps1\n%s\nEND\n", winBootstrapScript))
	if err != nil {
		uc.Logger.Errorf(strErr)
		return "", err
	}
	// copy bootstrap config to master
	configFilename := fmt.Sprintf("config-win.%s.yaml", uc.DataModel.Properties.OrchestratorProfile.OrchestratorVersion)
	uc.Logger.Infof("Copy Windows bootstrap config to master")
	strOut, strErr, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, fmt.Sprintf("cat << END > %s\n%s\nEND\n",
		configFilename, acsengine.GetDCOSWindowsBootstrapConfig(uc.DataModel)))
	if err != nil {
		uc.Logger.Errorf(strErr)
		return "", err
	}
	// copy bootstrap script to bootstrap node
	uc.Logger.Infof("Copy Windows bootstrap script to Windows bootstrap node")
	cmd := fmt.Sprintf("scp -i .ssh/id_rsa_cluster -o ConnectTimeout=30 -o StrictHostKeyChecking=no winBootstrapUpgrade.ps1 %s:C:\\\\AzureData\\\\winBootstrapUpgrade.ps1",
		winBootstrapIP)
	strOut, strErr, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, cmd)
	if err != nil {
		uc.Logger.Errorf(strErr)
		return "", err
	}
	// copy bootstrap config to bootstrap node
	uc.Logger.Infof("Copy Windows bootstrap config to Windows bootstrap node")
	cmd = fmt.Sprintf("scp -i .ssh/id_rsa_cluster -o ConnectTimeout=30 -o StrictHostKeyChecking=no %s %s:C:\\\\AzureData\\\\%s",
		configFilename, winBootstrapIP, configFilename)
	strOut, strErr, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, cmd)
	if err != nil {
		uc.Logger.Errorf(strErr)
		return "", err
	}
	// run bootstrap script
	uc.Logger.Infof("Run Windows bootstrap upgrade script")
	cmd = fmt.Sprintf("ssh -i .ssh/id_rsa_cluster -o ConnectTimeout=30 -o StrictHostKeyChecking=no %s powershell.exe -ExecutionPolicy Unrestricted -command \"C:\\\\AzureData\\\\winBootstrapUpgrade.ps1\"",
		winBootstrapIP)
	strOut, strErr, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, cmd)
	if err != nil {
		uc.Logger.Errorf(strErr)
		return "", err
	}
	uc.Logger.Info(strOut)
	// retrieve upgrade script URL
	var url string
	arr := strings.Split(strOut, "\n")
	prefix := "Setting up bootstrap node completed. Node upgrade script URL"
	for _, str := range arr {
		if strings.HasPrefix(str, prefix) {
			url = strings.TrimSpace(str[len(prefix):])
			break
		}
	}
	if len(url) == 0 {
		return "", fmt.Errorf("Undefined upgrade script URL")
	}
	return url, nil
}

func (uc *UpgradeCluster) upgradeWindowsAgent(masterDNS string, agent *agentInfo) error {
	uc.Logger.Infof("Skipping upgrade of Windows agent %s", agent.Hostname)
	return nil
}
