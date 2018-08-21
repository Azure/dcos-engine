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

	& cmd /c "c:\AzureData\7z\7z.exe e .\dcos_generate_config.windows.tar.xz -so | c:\AzureData\7z\7z.exe x -si -ttar"
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

var winNodeUpgradeScript = `
filter Timestamp {"[\$(Get-Date -Format o)] \$_"}

function Write-Log(\$message)
{
    \$msg = \$message | Timestamp
    Write-Output \$msg
}
\$upgradeScriptURL = "WIN_UPGRADE_SCRIPT_URL"
\$upgradeDir = "C:\AzureData\upgrade\NEW_VERSION"
\$log = "C:\AzureData\upgrade_NEW_VERSION.log"
\$adminUser = "ADMIN_USER"
\$password = "ADMIN_PASSWORD"
try {
	Start-Transcript -Path \$log -append
	Write-Log "Starting node upgrade to DCOS NEW_VERSION"
	Remove-Item -Recurse -Force -ErrorAction SilentlyContinue \$upgradeDir
	New-Item -ItemType Directory -Force -Path \$upgradeDir
	cd \$upgradeDir

	[Environment]::SetEnvironmentVariable("SYSTEMD_SERVICE_USERNAME", "\$env:computername\\\$adminUser", "Machine")
	[Environment]::SetEnvironmentVariable("SYSTEMD_SERVICE_PASSWORD", \$password, "Machine")

	[Environment]::SetEnvironmentVariable("SYSTEMD_SERVICE_USERNAME", "\$env:computername\\\$adminUser", "Process")
	[Environment]::SetEnvironmentVariable("SYSTEMD_SERVICE_PASSWORD", \$password, "Process")

	& curl.exe --keepalive-time 2 -fLsS --retry 20 -Y 100000 -y 60 -o dcos_node_upgrade.ps1 \$upgradeScriptURL
	if (\$LASTEXITCODE -ne 0) {
		throw "Failed to download \$upgradeScriptURL"
	}
	.\dcos_node_upgrade.ps1
}catch {
	Write-Log "Failed to upgrade Windows agent node: \$_"
	Stop-Transcript
    exit 1
}
Write-Log "Successfully upgraded Windows agent node"
Stop-Transcript
exit 0
`

func (uc *UpgradeCluster) createWindowsAgentScript(masterDNS, winUpgradeScriptURL, newVersion string) error {
	winNodeScript := strings.Replace(winNodeUpgradeScript, "NEW_VERSION", newVersion, -1)
	winNodeScript = strings.Replace(winNodeScript, "WIN_UPGRADE_SCRIPT_URL", winUpgradeScriptURL, -1)
	winNodeScript = strings.Replace(winNodeScript, "ADMIN_USER", uc.ClusterTopology.DataModel.Properties.WindowsProfile.AdminUsername, -1)
	winNodeScript = strings.Replace(winNodeScript, "ADMIN_PASSWORD", uc.ClusterTopology.DataModel.Properties.WindowsProfile.AdminPassword, -1)
	winNodeScriptName := fmt.Sprintf("node_upgrade.%s.ps1", uc.ClusterTopology.DataModel.Properties.OrchestratorProfile.OrchestratorVersion)
	// copy script to master
	uc.Logger.Infof("Copy Windows agent script to master")
	_, strErr, err := operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, fmt.Sprintf("cat << END > %s\n%s\nEND\n",
		winNodeScriptName, winNodeScript))
	if err != nil {
		uc.Logger.Errorf(strErr)
		return err
	}
	return nil
}

func (uc *UpgradeCluster) upgradeWindowsBootstrapNode(masterDNS, winBootstrapIP, newVersion string) (string, error) {
	winBootstrapScript := strings.Replace(winBootstrapUpgradeScript, "CURR_VERSION", uc.CurrentDcosVersion, -1)
	winBootstrapScript = strings.Replace(winBootstrapScript, "NEW_VERSION", newVersion, -1)
	winBootstrapScript = strings.Replace(winBootstrapScript, "WIN_BOOTSTRAP_URL", uc.ClusterTopology.DataModel.Properties.OrchestratorProfile.WindowsBootstrapProfile.BootstrapURL, -1)

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
	uc.Logger.Infof("Upgrading Windows agent %s", agent.Hostname)
	cmdCheckVersion := fmt.Sprintf("ssh -i .ssh/id_rsa_cluster -o ConnectTimeout=30 -o StrictHostKeyChecking=no %s type C:\\\\opt\\\\mesosphere\\\\etc\\\\dcos-version.json", agent.Hostname)
	strOut, strErr, err := operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, cmdCheckVersion)
	if err != nil {
		uc.Logger.Errorf(strErr)
		return err
	}
	uc.Logger.Infof("Current DCOS Version for %s\n%s", agent.Hostname, strings.TrimSpace(strOut))
	// copy script to the node
	uc.Logger.Infof("Copy script to agent %s", agent.Hostname)
	winNodeScriptName := fmt.Sprintf("node_upgrade.%s.ps1", uc.ClusterTopology.DataModel.Properties.OrchestratorProfile.OrchestratorVersion)
	cmd := fmt.Sprintf("scp -i .ssh/id_rsa_cluster -o ConnectTimeout=30 -o StrictHostKeyChecking=no %s %s:C:\\\\AzureData\\\\%s",
		winNodeScriptName, agent.Hostname, winNodeScriptName)
	_, strErr, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, cmd)
	if err != nil {
		uc.Logger.Errorf(strErr)
		return err
	}
	// run the script
	uc.Logger.Infof("Run script on agent %s", agent.Hostname)
	cmd = fmt.Sprintf("ssh -i .ssh/id_rsa_cluster -o ConnectTimeout=30 -o StrictHostKeyChecking=no %s powershell.exe -ExecutionPolicy Unrestricted -Command \"C:\\\\AzureData\\\\%s\"",
		agent.Hostname, winNodeScriptName)
	_, strErr, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, cmd)
	if err != nil {
		uc.Logger.Errorf(strErr)
		return err
	}
	// check new version
	strOut, strErr, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, cmdCheckVersion)
	if err != nil {
		uc.Logger.Errorf(strErr)
		return err
	}
	uc.Logger.Infof("New DCOS Version for %s\n%s", agent.Hostname, strings.TrimSpace(strOut))
	return nil
}
