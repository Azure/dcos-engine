package dcosupgrade

import (
	"encoding/json"
	"fmt"
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
	Agents []*agentInfo `json:"slaves"`
}

var bootstrapUpgradeScript = `#!/bin/bash

echo "Setting up bootstrap node"
rm -rf /opt/azure/dcos/upgrade/NEW_VERSION
mkdir -p /opt/azure/dcos/upgrade/NEW_VERSION/genconf
cp /opt/azure/dcos/genconf/ip-detect /opt/azure/dcos/upgrade/NEW_VERSION/genconf/ip-detect
cp config.NEW_VERSION.yaml /opt/azure/dcos/upgrade/NEW_VERSION/genconf/config.yaml
dns=\$(grep search /etc/resolv.conf | cut -d " " -f 2)
sed -i "/dns_search:/c dns_search: \$dns" /opt/azure/dcos/upgrade/NEW_VERSION/genconf/config.yaml
cd /opt/azure/dcos/upgrade/NEW_VERSION/
curl -fsSL -O BOOTSTRAP_URL
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

func (uc *UpgradeCluster) runUpgrade() error {
	if uc.ClusterTopology.DataModel.Properties.OrchestratorProfile.LinuxBootstrapProfile == nil {
		return fmt.Errorf("LinuxBootstrapProfile is not set")
	}
	newVersion := uc.ClusterTopology.DataModel.Properties.OrchestratorProfile.OrchestratorVersion
	masterDNS := acsengine.FormatAzureProdFQDN(uc.ClusterTopology.DataModel.Properties.MasterProfile.DNSPrefix, uc.ClusterTopology.DataModel.Location)

	// get the agents
	strOut, strErr, err := operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, "curl -fsSL http://leader.mesos:5050/slaves")
	if err != nil {
		uc.Logger.Errorf(strErr)
		return err
	}
	agents := &agentList{}
	if err = json.Unmarshal([]byte(strOut), agents); err != nil {
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
	bootstrapIP := uc.ClusterTopology.DataModel.Properties.OrchestratorProfile.LinuxBootstrapProfile.StaticIP
	uc.Logger.Infof("masterDNS:%s masterCount:%d", masterDNS, masterCount)
	uc.Logger.Infof("bootstrapIP:%s", bootstrapIP)

	if hasWindowsAgents {
		uc.Logger.Warnf("DC/OS upgrade for Windows agents is currently unsupported")
	}
	// copy SSH key to master
	uc.Logger.Infof("Copy SSH key to master")
	_, strErr, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, fmt.Sprintf("cat << END > .ssh/id_rsa_cluster\n%s\nEND\n", string(uc.SSHKey)))
	if err != nil {
		uc.Logger.Errorf(strErr)
		return err
	}
	// set SSH key permissions
	_, strErr, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, "chmod 600 .ssh/id_rsa_cluster")
	if err != nil {
		uc.Logger.Errorf(strErr)
		return err
	}
	// upgrade bootstrap node
	bootstrapScript := strings.Replace(bootstrapUpgradeScript, "CURR_VERSION", uc.CurrentDcosVersion, -1)
	bootstrapScript = strings.Replace(bootstrapScript, "NEW_VERSION", newVersion, -1)
	bootstrapScript = strings.Replace(bootstrapScript, "BOOTSTRAP_URL", uc.ClusterTopology.DataModel.Properties.OrchestratorProfile.LinuxBootstrapProfile.BootstrapURL, -1)

	upgradeScriptURL, err := uc.upgradeBootstrapNode(masterDNS, bootstrapIP, bootstrapScript)
	if err != nil {
		return err
	}
	uc.Logger.Infof("upgradeScriptURL %s", upgradeScriptURL)

	nodeScript := strings.Replace(nodeUpgradeScript, "NEW_VERSION", newVersion, -1)
	nodeScript = strings.Replace(nodeScript, "UPGRADE_SCRIPT_URL", upgradeScriptURL, -1)

	// upgrade master nodes
	if err = uc.upgradeMasterNodes(masterDNS, masterCount, nodeScript); err != nil {
		return err
	}

	// upgrade agent nodes
	for _, agent := range agents.Agents {
		if strings.Compare(agent.Attributes.OS, "Windows") == 0 {
			if err = uc.upgradeWindowsAgent(masterDNS, agent); err != nil {
				return err
			}
		} else {
			if err = uc.upgradeLinuxAgent(masterDNS, agent); err != nil {
				return err
			}
		}
	}
	return nil
}

func (uc *UpgradeCluster) upgradeBootstrapNode(masterDNS, bootstrapIP, bootstrapScript string) (string, error) {
	// copy bootstrap script to master
	uc.Logger.Infof("Copy bootstrap script to master")
	strOut, strErr, err := operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, fmt.Sprintf("cat << END > bootstrap_upgrade.sh\n%s\nEND\n", bootstrapScript))
	if err != nil {
		uc.Logger.Errorf(strErr)
		return "", err
	}
	// set script permissions
	_, strErr, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, "chmod 755 ./bootstrap_upgrade.sh")
	if err != nil {
		uc.Logger.Errorf(strErr)
		return "", err
	}
	// copy bootstrap config to master
	configFilename := fmt.Sprintf("config.%s.yaml", uc.DataModel.Properties.OrchestratorProfile.OrchestratorVersion)
	uc.Logger.Infof("Copy bootstrap config to master")
	_, strErr, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, fmt.Sprintf("cat << END > %s\n%s\nEND\n",
		configFilename, acsengine.GetDCOSBootstrapConfig(uc.DataModel)))
	if err != nil {
		uc.Logger.Errorf(strErr)
		return "", err
	}
	// copy bootstrap script to the bootstrap node
	uc.Logger.Infof("Copy bootstrap script to the bootstrap node")
	cmd := fmt.Sprintf("scp -i .ssh/id_rsa_cluster -o ConnectTimeout=30 -o StrictHostKeyChecking=no bootstrap_upgrade.sh %s:", bootstrapIP)
	_, strErr, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, cmd)
	if err != nil {
		uc.Logger.Errorf(strErr)
		return "", err
	}
	// copy bootstrap config to the bootstrap node
	uc.Logger.Infof("Copy bootstrap config to the bootstrap node")
	cmd = fmt.Sprintf("scp -i .ssh/id_rsa_cluster -o ConnectTimeout=30 -o StrictHostKeyChecking=no %s %s:", configFilename, bootstrapIP)
	_, strErr, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, cmd)
	if err != nil {
		uc.Logger.Errorf(strErr)
		return "", err
	}
	// run bootstrap script
	uc.Logger.Infof("Run bootstrap upgrade script")
	cmd = fmt.Sprintf("ssh -i .ssh/id_rsa_cluster -o ConnectTimeout=30 -o StrictHostKeyChecking=no %s sudo ./bootstrap_upgrade.sh", bootstrapIP)
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

func (uc *UpgradeCluster) upgradeMasterNodes(masterDNS string, masterCount int, nodeScript string) error {
	// run master upgrade script
	catCmd := fmt.Sprintf("cat << END > node_upgrade.sh\n%s\nEND\n", nodeScript)
	for i := 0; i < masterCount; i++ {
		uc.Logger.Infof("Upgrading master node #%d", i+1)
		port := 2200 + i
		// check current version
		strOut, strErr, err := operations.RemoteRun("azureuser", masterDNS, port, uc.SSHKey, "cat /opt/mesosphere/etc/dcos-version.json")
		if err != nil {
			uc.Logger.Errorf(strErr)
			return err
		}
		uc.Logger.Infof("Current DCOS Version for %s:%d\n%s", masterDNS, port, strings.TrimSpace(strOut))
		// copy script to the node
		uc.Logger.Infof("Copy script to master node")
		_, strErr, err = operations.RemoteRun("azureuser", masterDNS, port, uc.SSHKey, catCmd)
		if err != nil {
			uc.Logger.Errorf(strErr)
			return err
		}
		// set script permissions
		_, strErr, err = operations.RemoteRun("azureuser", masterDNS, port, uc.SSHKey, "chmod 755 ./node_upgrade.sh")
		if err != nil {
			uc.Logger.Errorf(strErr)
			return err
		}
		// run the script
		uc.Logger.Infof("Run script on master node")
		_, strErr, err = operations.RemoteRun("azureuser", masterDNS, port, uc.SSHKey, "sudo ./node_upgrade.sh")
		if err != nil {
			uc.Logger.Errorf(strErr)
			return err
		}
		// check new version
		strOut, strErr, err = operations.RemoteRun("azureuser", masterDNS, port, uc.SSHKey, "cat /opt/mesosphere/etc/dcos-version.json")
		if err != nil {
			uc.Logger.Errorf(strErr)
			return err
		}
		uc.Logger.Infof("New DCOS Version for %s:%d\n%s", masterDNS, port, strings.TrimSpace(strOut))
	}
	return nil
}

func (uc *UpgradeCluster) upgradeLinuxAgent(masterDNS string, agent *agentInfo) error {
	uc.Logger.Infof("Upgrading Linux agent %s", agent.Hostname)
	// check current version
	cmdCheckVersion := fmt.Sprintf("ssh -i .ssh/id_rsa_cluster -o ConnectTimeout=30 -o StrictHostKeyChecking=no %s cat /opt/mesosphere/etc/dcos-version.json", agent.Hostname)
	strOut, strErr, err := operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, cmdCheckVersion)
	if err != nil {
		uc.Logger.Errorf(strErr)
		return err
	}
	uc.Logger.Infof("Current DCOS Version for %s\n%s", agent.Hostname, strings.TrimSpace(strOut))
	// copy script to the node
	uc.Logger.Infof("Copy script to agent %s", agent.Hostname)
	cmd := fmt.Sprintf("scp -i .ssh/id_rsa_cluster -o ConnectTimeout=30 -o StrictHostKeyChecking=no node_upgrade.sh %s:", agent.Hostname)
	_, strErr, err = operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, cmd)
	if err != nil {
		uc.Logger.Errorf(strErr)
		return err
	}
	// run the script
	uc.Logger.Infof("Run script on agent %s", agent.Hostname)
	cmd = fmt.Sprintf("ssh -i .ssh/id_rsa_cluster -o ConnectTimeout=30 -o StrictHostKeyChecking=no %s sudo ./node_upgrade.sh", agent.Hostname)
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
	uc.Logger.Infof("Current DCOS Version for %s\n%s", agent.Hostname, strings.TrimSpace(strOut))
	return nil
}

func (uc *UpgradeCluster) upgradeWindowsAgent(masterDNS string, agent *agentInfo) error {
	uc.Logger.Infof("Skipping upgrade of Windows agent %s", agent.Hostname)
	return nil
}
