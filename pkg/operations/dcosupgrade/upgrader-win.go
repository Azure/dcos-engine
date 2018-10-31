package dcosupgrade

import (
	"fmt"
	"strings"

	"github.com/Azure/dcos-engine/pkg/acsengine"
	"github.com/Azure/dcos-engine/pkg/operations"
)

const (
	winBootstrapUpgradeScriptFile = "windowsupgradescripts/winBootstrapUpgradeScript.ps1"
	winNodeUpgradeScriptFile      = "windowsupgradescripts/winNodeUpgradeScript.ps1"
)

func (uc *UpgradeCluster) createWindowsAgentScript(masterDNS, winUpgradeScriptURL, newVersion string) error {
	winNodeUpgradeScript, err := Asset(winNodeUpgradeScriptFile)
	if err != nil {
		uc.Logger.Fatal(err)
	}
	winNodeScript := strings.Replace(string(winNodeUpgradeScript), "NEW_VERSION", newVersion, -1)
	winNodeScript = strings.Replace(winNodeScript, "WIN_UPGRADE_SCRIPT_URL", winUpgradeScriptURL, -1)
	winNodeScript = strings.Replace(winNodeScript, "ADMIN_USER", uc.ClusterTopology.DataModel.Properties.WindowsProfile.AdminUsername, -1)
	winNodeScript = strings.Replace(winNodeScript, "ADMIN_PASSWORD", uc.ClusterTopology.DataModel.Properties.WindowsProfile.AdminPassword, -1)
	winNodeScriptName := fmt.Sprintf("node_upgrade.%s.ps1", uc.ClusterTopology.DataModel.Properties.OrchestratorProfile.OrchestratorVersion)
	// copy script to master
	uc.Logger.Infof("Copy Windows agent script to master")
	_, strErr, err := operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, fmt.Sprintf("cat << 'END' > %s\n%s\nEND\n",
		winNodeScriptName, winNodeScript))
	if err != nil {
		uc.Logger.Errorf(strErr)
		return err
	}
	return nil
}

func (uc *UpgradeCluster) upgradeWindowsBootstrapNode(masterDNS, winBootstrapIP, newVersion string) (string, error) {
	winBootstrapUpgradeScript, err := Asset(winBootstrapUpgradeScriptFile)
	if err != nil {
		uc.Logger.Fatal(err)
	}
	winBootstrapScript := strings.Replace(string(winBootstrapUpgradeScript), "CURR_VERSION", uc.CurrentDcosVersion, -1)
	winBootstrapScript = strings.Replace(winBootstrapScript, "NEW_VERSION", newVersion, -1)
	winBootstrapScript = strings.Replace(winBootstrapScript, "WIN_BOOTSTRAP_URL", uc.ClusterTopology.DataModel.Properties.OrchestratorProfile.WindowsBootstrapProfile.BootstrapURL, -1)

	// copy bootstrap script to master
	uc.Logger.Infof("Copy Windows bootstrap script to master")
	strOut, strErr, err := operations.RemoteRun("azureuser", masterDNS, 2200, uc.SSHKey, fmt.Sprintf("cat << 'END' > winBootstrapUpgrade.ps1\n%s\nEND\n", winBootstrapScript))
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
	dcosVer, err := getDCOSVersion(strOut)
	if err != nil {
		uc.Logger.Errorf("failed to parse dcos-version.json")
		return err
	}
	// partial upgrade case
	if uc.CurrentDcosVersion != uc.ClusterTopology.DataModel.Properties.OrchestratorProfile.OrchestratorVersion &&
		dcosVer.Version == uc.ClusterTopology.DataModel.Properties.OrchestratorProfile.OrchestratorVersion {
		uc.Logger.Infof("Agent node is up-to-date. Skipping upgrade")
		return nil
	}
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
