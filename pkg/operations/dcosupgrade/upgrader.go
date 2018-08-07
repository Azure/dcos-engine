package dcosupgrade

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/Azure/dcos-engine/pkg/acsengine"
	"github.com/Azure/dcos-engine/pkg/operations"
)

type agentAttr struct {
	OS       string `json:"os"`
	PublicIP string `json:"public_ip,omitempty"`
}

type agentInfo struct {
	ID         string    `json:"id"`
	Hostname   string    `json:"hostname"`
	Attributes agentAttr `json:"attributes"`
}

type agentList struct {
	Agents []agentInfo `json:"slaves"`
}

var bootstrapUpgradeScript = `#!/bin/bash

echo "Starting upgrade configuration"
if [ ! -e /opt/azure/dcos/upgrade/NEW_VERSION/upgrade_url ]; then
  echo "Setting up bootstrap node"
  rm -rf /opt/azure/dcos/upgrade/NEW_VERSION
  mkdir -p /opt/azure/dcos/upgrade/NEW_VERSION/genconf
  cp /opt/azure/dcos/genconf/config.yaml /opt/azure/dcos/genconf/ip-detect /opt/azure/dcos/upgrade/NEW_VERSION/genconf/
  cd /opt/azure/dcos/upgrade/NEW_VERSION/
  curl -s -O https://dcos-mirror.azureedge.net/dcos/NEW_DASHED_VERSION/dcos_generate_config.sh
  bash dcos_generate_config.sh --generate-node-upgrade-script CURR_VERSION | tee /opt/azure/dcos/upgrade/NEW_VERSION/log
  process=\$(docker ps -f ancestor=nginx -q)
  if [ ! -z "\$process" ]; then
    echo "Stopping nginx service \$process"
    docker kill \$process
  fi
  echo "Starting nginx service"
  docker run -d -p 8086:80 -v \$PWD/genconf/serve:/usr/share/nginx/html:ro nginx
  docker ps
  grep 'Node upgrade script URL' /opt/azure/dcos/upgrade/NEW_VERSION/log | awk -F ': ' '{print \$2}' | cat > /opt/azure/dcos/upgrade/NEW_VERSION/upgrade_url
fi
upgrade_url=\$(cat /opt/azure/dcos/upgrade/NEW_VERSION/upgrade_url)
if [ -z \${upgrade_url} ]; then
  rm -f /opt/azure/dcos/upgrade/NEW_VERSION/upgrade_url
  echo "Failed to set up bootstrap node. Please try again"
  exit 1
else
  echo "Setting up bootstrap node completed. Node upgrade script URL \${upgrade_url}"
fi
`

var nodeUpgradeScript = `#!/bin/bash

echo "Starting node upgrade"
mkdir -p /opt/azure/dcos/upgrade/NEW_VERSION
cd /opt/azure/dcos/upgrade/NEW_VERSION
curl -O UPGRADE_SCRIPT_URL
bash ./dcos_node_upgrade.sh

`

var winBootstrapUpgradeScript = `
filter Timestamp {"[\$(Get-Date -Format o)] \$_"}

function Write-Log(\$message)
{
    \$msg = \$message | Timestamp
    Write-Output \$msg
}

try {
	Write-Log "Starting upgrade configuration"
	\$BootstrapURL = "https://dcosdevstorage.blob.core.windows.net/dcos-build/testing/paulall/dcos_generate_config.windows.tar.xz"
	\$upgradeDir = "C:\AzureData\upgrade\NEW_VERSION"
	\$genconfDir = Join-Path \$upgradeDir "genconf"
	\$logPath = Join-Path \$upgradeDir "dcos_generate_config.log"
	\$upgradeUrlPath = Join-Path \$upgradeDir "upgrade_url"

	if ( -Not (Test-Path \$upgradeUrlPath)) {
		Write-Log "Setting up Windows bootstrap node for upgrade"
		Remove-Item -Recurse -Force -ErrorAction SilentlyContinue \$upgradeDir
		New-Item -ItemType Directory -Force -Path \$genconfDir
		cp "c:\temp\genconf\config.yaml" \$genconfDir
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
		\$url = ($match.Line -replace 'Node upgrade script URL:','').Trim()
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

		Set-Content -Path \$upgradeUrlPath -Value \$url -Encoding Ascii
	}
	\$url = Get-Content -Path \$upgradeUrlPath -Encoding Ascii
	if (-Not \$url) {
		Remove-Item $upgradeUrlPath -Force
		throw "Failed to set up bootstrap node. Please try again"
	} else {
		Write-Output "Setting up bootstrap node completed. Node upgrade script URL \$url"
	}
} catch {
    Write-Log "Failed to upgrade Windows bootstrap node: \$_"
    exit 1
}
Write-Output "Setting up bootstrap node completed"
exit 0
`

func (uc *UpgradeCluster) runUpgrade() error {
	if uc.ClusterTopology.DataModel.Properties.OrchestratorProfile.DcosConfig == nil ||
		uc.ClusterTopology.DataModel.Properties.OrchestratorProfile.DcosConfig.BootstrapProfile == nil {
		return fmt.Errorf("BootstrapProfile is not set")
	}
	newVersion := uc.ClusterTopology.DataModel.Properties.OrchestratorProfile.OrchestratorVersion
	dashedVersion := strings.Replace(newVersion, ".", "-", -1)

	masterDNS := acsengine.FormatAzureProdFQDN(uc.ClusterTopology.DataModel.Properties.MasterProfile.DNSPrefix, uc.ClusterTopology.DataModel.Location)

	// get the agents
	out, err := operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey,
		fmt.Sprintf("curl -s http://%s:5050/slaves", uc.ClusterTopology.DataModel.Properties.MasterProfile.FirstConsecutiveStaticIP))
	if err != nil {
		uc.Logger.Errorf(out)
		return err
	}
	agents := &agentList{}
	if err = json.Unmarshal([]byte(out), agents); err != nil {
		return err
	}

	var hasWindowsAgents bool
	for _, agent := range agents.Agents {
		if strings.Compare(agent.Attributes.OS, "Windows") == 0 {
			hasWindowsAgents = true
			break
		}
	}

	masterCount := uc.ClusterTopology.DataModel.Properties.MasterProfile.Count
	bootstrapIP := uc.ClusterTopology.DataModel.Properties.OrchestratorProfile.DcosConfig.BootstrapProfile.StaticIP
	uc.Logger.Infof("masterDNS:%s masterCount:%d bootstrapIP:%s", masterDNS, masterCount, bootstrapIP)

	var winBootstrapIP string
	if hasWindowsAgents {
		// winBootstrapIP is next to bootstrapIP
		ip := net.ParseIP(bootstrapIP)
		if ip == nil {
			return fmt.Errorf("Invalid IP format '%s'", bootstrapIP)
		}
		ip = ip.To4()
		if ip == nil {
			return fmt.Errorf("Failed to convert IP '%s' to IPv4", bootstrapIP)
		}
		ip[3]++ // check for rollover
		winBootstrapIP = ip.String()
		uc.Logger.Infof("Windows bootstrap IP:%s", winBootstrapIP)
	}

	// copy SSH key to master
	uc.Logger.Infof("Copy SSH key to master")
	out, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, fmt.Sprintf("cat << END > .ssh/id_rsa_cluster\n%s\nEND\n", string(uc.SSHKey)))
	if err != nil {
		uc.Logger.Errorf(out)
		return err
	}
	// set SSH key permissions
	out, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, "chmod 600 .ssh/id_rsa_cluster")
	if err != nil {
		uc.Logger.Errorf(out)
		return err
	}
	// upgrade bootstrap node
	bootstrapScript := strings.Replace(bootstrapUpgradeScript, "CURR_VERSION", uc.CurrentDcosVersion, -1)
	bootstrapScript = strings.Replace(bootstrapScript, "NEW_VERSION", newVersion, -1)
	bootstrapScript = strings.Replace(bootstrapScript, "NEW_DASHED_VERSION", dashedVersion, -1)

	upgradeScriptURL, err := uc.upgradeBootstrapNode(masterDNS, bootstrapIP, bootstrapScript)
	if err != nil {
		return err
	}
	uc.Logger.Infof("upgradeScriptURL %s", upgradeScriptURL)
	// upgrade Windows bootstrap node
	var winUpgradeScriptURL string
	if hasWindowsAgents {
		winBootstrapScript := strings.Replace(winBootstrapUpgradeScript, "CURR_VERSION", uc.CurrentDcosVersion, -1)
		winBootstrapScript = strings.Replace(winBootstrapScript, "NEW_VERSION", newVersion, -1)
		winBootstrapScript = strings.Replace(winBootstrapScript, "NEW_DASHED_VERSION", dashedVersion, -1)

		winUpgradeScriptURL, err = uc.upgradeWinBootstrapNode(masterDNS, winBootstrapIP, winBootstrapScript)
		if err != nil {
			return err
		}
	}
	uc.Logger.Infof("winUpgradeScriptURL %s", winUpgradeScriptURL)

	nodeScript := strings.Replace(nodeUpgradeScript, "NEW_VERSION", newVersion, -1)
	nodeScript = strings.Replace(nodeScript, "UPGRADE_SCRIPT_URL", upgradeScriptURL, -1)

	// upgrade master nodes
	if err = uc.upgradeMasterNodes(masterDNS, masterCount, nodeScript); err != nil {
		return err
	}

	// upgrade agent nodes
	return uc.upgradeAgentNodes(masterDNS, agents)
}

func (uc *UpgradeCluster) upgradeBootstrapNode(masterDNS, bootstrapIP, bootstrapScript string) (string, error) {
	// copy bootstrap script to master
	uc.Logger.Infof("Copy bootstrap script to master")
	out, err := operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, fmt.Sprintf("cat << END > bootstrap_upgrade.sh\n%s\nEND\n", bootstrapScript))
	if err != nil {
		uc.Logger.Errorf(out)
		return "", err
	}
	// set script permissions
	out, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, "chmod 755 ./bootstrap_upgrade.sh")
	if err != nil {
		uc.Logger.Errorf(out)
		return "", err
	}
	// copy bootstrap script to the bootstrap node
	uc.Logger.Infof("Copy bootstrap script to the bootstrap node")
	cmd := fmt.Sprintf("scp -i .ssh/id_rsa_cluster -o ConnectTimeout=30 -o StrictHostKeyChecking=no bootstrap_upgrade.sh %s:", bootstrapIP)
	out, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, cmd)
	if err != nil {
		uc.Logger.Errorf(out)
		return "", err
	}
	// run bootstrap script
	uc.Logger.Infof("Run bootstrap upgrade script")
	cmd = fmt.Sprintf("ssh -i .ssh/id_rsa_cluster -o ConnectTimeout=30 -o StrictHostKeyChecking=no %s sudo ./bootstrap_upgrade.sh", bootstrapIP)
	out, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, cmd)
	if err != nil {
		uc.Logger.Errorf(out)
		return "", err
	}
	uc.Logger.Info(out)
	// retrieve upgrade script URL
	var url string
	arr := strings.Split(out, "\n")
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

func (uc *UpgradeCluster) upgradeWinBootstrapNode(masterDNS, winBootstrapIP, winBootstrapScript string) (string, error) {
	// copy bootstrap script to master
	uc.Logger.Infof("Copy Windows bootstrap script to master")
	out, err := operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, fmt.Sprintf("cat << END > winBootstrapUpgrade.ps1\n%s\nEND\n", winBootstrapScript))
	if err != nil {
		uc.Logger.Errorf(out)
		return "", err
	}
	// copy bootstrap script to the bootstrap node
	uc.Logger.Infof("Copy Windows bootstrap script to Windows bootstrap node")
	cmd := fmt.Sprintf("scp -i .ssh/id_rsa_cluster -o ConnectTimeout=30 -o StrictHostKeyChecking=no winBootstrapUpgrade.ps1 %s:C:\\\\AzureData\\\\winBootstrapUpgrade.ps1",
		winBootstrapIP)
	out, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, cmd)
	if err != nil {
		uc.Logger.Errorf(out)
		return "", err
	}
	// run bootstrap script
	uc.Logger.Infof("Run Windows bootstrap upgrade script")
	cmd = fmt.Sprintf("ssh -i .ssh/id_rsa_cluster -o ConnectTimeout=30 -o StrictHostKeyChecking=no %s powershell.exe -ExecutionPolicy Unrestricted -command \"C:\\\\AzureData\\\\winBootstrapUpgrade.ps1\"",
		winBootstrapIP)
	out, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, cmd)
	if err != nil {
		uc.Logger.Errorf(out)
		return "", err
	}
	uc.Logger.Info(out)
	// retrieve upgrade script URL
	var url string
	arr := strings.Split(out, "\n")
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

func (uc *UpgradeCluster) upgradeMasterNodes(masterDNS string, masterCount int, nodeScript string) error {
	// run master upgrade script
	catCmd := fmt.Sprintf("cat << END > node_upgrade.sh\n%s\nEND\n", nodeScript)
	for i := 0; i < masterCount; i++ {
		uc.Logger.Infof("Upgrading master node #%d", i+1)
		// check current version
		out, err := operations.RemoteRun("azureuser", masterDNS, 2200+i, uc.SSHKey, "grep version /opt/mesosphere/etc/dcos-version.json | cut -d '\"' -f 4")
		if err != nil {
			uc.Logger.Errorf(out)
			return err
		}
		if strings.TrimSpace(out) == uc.ClusterTopology.DataModel.Properties.OrchestratorProfile.OrchestratorVersion {
			uc.Logger.Infof("Master node is up-to-date. Skipping upgrade")
			continue
		}
		// copy script to the node
		out, err = operations.RemoteRun("azureuser", masterDNS, 2200+i, uc.SSHKey, catCmd)
		if err != nil {
			uc.Logger.Errorf(out)
			return err
		}
		// set script permissions
		out, err = operations.RemoteRun("azureuser", masterDNS, 2200+i, uc.SSHKey, "chmod 755 ./node_upgrade.sh")
		if err != nil {
			uc.Logger.Errorf(out)
			return err
		}
		// run the script
		out, err = operations.RemoteRun("azureuser", masterDNS, 2200+i, uc.SSHKey, "sudo ./node_upgrade.sh")
		if err != nil {
			uc.Logger.Errorf(out)
			return err
		}
		uc.Logger.Info(out)
	}
	return nil
}

func (uc *UpgradeCluster) upgradeAgentNodes(masterDNS string, agents *agentList) error {
	for _, agent := range agents.Agents {
		uc.Logger.Infof("Upgrading %s agent %s", agent.Attributes.OS, agent.Hostname)

		// check current version
		cmd := fmt.Sprintf("ssh -i .ssh/id_rsa_cluster -o ConnectTimeout=30 -o StrictHostKeyChecking=no %s grep version /opt/mesosphere/etc/dcos-version.json | cut -d '\"' -f 4", agent.Hostname)
		out, err := operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, cmd)
		if err != nil {
			uc.Logger.Errorf(out)
			return err
		}
		if strings.TrimSpace(out) == uc.ClusterTopology.DataModel.Properties.OrchestratorProfile.OrchestratorVersion {
			uc.Logger.Infof("Agent node is up-to-date. Skipping upgrade")
			continue
		}
		// copy script to the node
		cmd = fmt.Sprintf("scp -i .ssh/id_rsa_cluster -o ConnectTimeout=30 -o StrictHostKeyChecking=no node_upgrade.sh %s:", agent.Hostname)
		out, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, cmd)
		if err != nil {
			uc.Logger.Errorf(out)
			return err
		}
		// run the script
		cmd = fmt.Sprintf("ssh -i .ssh/id_rsa_cluster -o ConnectTimeout=30 -o StrictHostKeyChecking=no %s sudo ./node_upgrade.sh", agent.Hostname)
		out, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, cmd)
		if err != nil {
			uc.Logger.Errorf(out)
			return err
		}
		uc.Logger.Info(out)
	}
	return nil
}
