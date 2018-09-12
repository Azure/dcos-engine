package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/Azure/dcos-engine/pkg/acsengine"
	"github.com/Azure/dcos-engine/pkg/api"
	"github.com/Azure/dcos-engine/pkg/armhelpers"
	"github.com/Azure/dcos-engine/pkg/helpers"
	"github.com/Azure/dcos-engine/pkg/i18n"
	"github.com/Azure/dcos-engine/pkg/operations/dcosupgrade"
	"github.com/blang/semver"
	"github.com/leonelquinteros/gotext"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	dcosUpgradeName             = "upgrade"
	dcosUpgradeShortDescription = "Upgrade an existing DC/OS cluster"
	dcosUpgradeLongDescription  = "Upgrade an existing DC/OS cluster"
)

type dcosUpgradeCmd struct {
	authArgs

	// user input
	resourceGroupName   string
	deploymentDirectory string
	upgradeVersion      string
	location            string
	sshPrivateKeyPath   string
	lnxBootstrapURL     string
	winBootstrapURL     string

	// derived
	containerService   *api.ContainerService
	apiVersion         string
	currentDcosVersion string
	client             armhelpers.ACSEngineClient
	locale             *gotext.Locale
	nameSuffix         string
	sshPrivateKey      []byte
}

func newDcosUpgradeCmd() *cobra.Command {
	uc := dcosUpgradeCmd{}

	dcosUpgradeCmd := &cobra.Command{
		Use:   dcosUpgradeName,
		Short: dcosUpgradeShortDescription,
		Long:  dcosUpgradeLongDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			return uc.run(cmd, args)
		},
	}

	f := dcosUpgradeCmd.Flags()
	f.StringVarP(&uc.location, "location", "l", "", "location the cluster is deployed in (required)")
	f.StringVarP(&uc.resourceGroupName, "resource-group", "g", "", "the resource group where the cluster is deployed (required)")
	f.StringVar(&uc.deploymentDirectory, "deployment-dir", "", "the location of the output from `generate` (required)")
	f.StringVar(&uc.sshPrivateKeyPath, "ssh-private-key-path", "", "ssh private key path (default: <deployment-dir>/id_rsa)")
	f.StringVar(&uc.upgradeVersion, "upgrade-version", "", "desired DC/OS version (required)")
	f.StringVar(&uc.lnxBootstrapURL, "linux-bootstrap-url", "", "Linux bootstrap installer URL (optional)")
	f.StringVar(&uc.winBootstrapURL, "windows-bootstrap-url", "", "Windows bootstrap installer URL (optional)")
	addAuthFlags(&uc.authArgs, f)

	return dcosUpgradeCmd
}

func (uc *dcosUpgradeCmd) validate(cmd *cobra.Command) error {
	log.Infoln("validating...")

	var err error

	uc.locale, err = i18n.LoadTranslations()
	if err != nil {
		return fmt.Errorf("error loading translation files: %s", err.Error())
	}

	if len(uc.resourceGroupName) == 0 {
		cmd.Usage()
		return fmt.Errorf("--resource-group must be specified")
	}

	if len(uc.location) == 0 {
		cmd.Usage()
		return fmt.Errorf("--location must be specified")
	}
	uc.location = helpers.NormalizeAzureRegion(uc.location)

	if len(uc.upgradeVersion) == 0 {
		cmd.Usage()
		return fmt.Errorf("--upgrade-version must be specified")
	}

	if len(uc.lnxBootstrapURL) == 0 {
		uc.lnxBootstrapURL = acsengine.GetDCOSDefaultBootstrapInstallerURL(uc.upgradeVersion)
	}
	if len(uc.lnxBootstrapURL) == 0 {
		return fmt.Errorf("DCOS %s is not supported", uc.upgradeVersion)
	}

	if len(uc.winBootstrapURL) == 0 {
		uc.winBootstrapURL = acsengine.GetDCOSDefaultWindowsBootstrapInstallerURL(uc.upgradeVersion)
	}
	if len(uc.winBootstrapURL) == 0 {
		return fmt.Errorf("DCOS %s is not supported", uc.upgradeVersion)
	}

	if len(uc.deploymentDirectory) == 0 {
		cmd.Usage()
		return fmt.Errorf("--deployment-dir must be specified")
	}

	if len(uc.sshPrivateKeyPath) == 0 {
		uc.sshPrivateKeyPath = filepath.Join(uc.deploymentDirectory, "id_rsa")
	}
	if uc.sshPrivateKey, err = ioutil.ReadFile(uc.sshPrivateKeyPath); err != nil {
		cmd.Usage()
		return fmt.Errorf("ssh-private-key-path must be specified: %s", err)
	}

	if err = uc.authArgs.validateAuthArgs(); err != nil {
		return fmt.Errorf("%s", err)
	}
	return nil
}

func (uc *dcosUpgradeCmd) loadCluster(cmd *cobra.Command) error {
	var err error

	if uc.client, err = uc.authArgs.getClient(); err != nil {
		return fmt.Errorf("Failed to get client: %s", err)
	}

	_, err = uc.client.EnsureResourceGroup(uc.resourceGroupName, uc.location, nil)
	if err != nil {
		return fmt.Errorf("Error ensuring resource group: %s", err)
	}

	// load apimodel from the deployment directory
	apiModelPath := path.Join(uc.deploymentDirectory, "apimodel.json")

	if _, err = os.Stat(apiModelPath); os.IsNotExist(err) {
		return fmt.Errorf("specified api model does not exist (%s)", apiModelPath)
	}

	apiloader := &api.Apiloader{
		Translator: &i18n.Translator{
			Locale: uc.locale,
		},
	}
	uc.containerService, uc.apiVersion, err = apiloader.LoadContainerServiceFromFile(apiModelPath, true, true, nil)
	if err != nil {
		return fmt.Errorf("error parsing the api model: %s", err.Error())
	}
	uc.currentDcosVersion = uc.containerService.Properties.OrchestratorProfile.OrchestratorVersion

	cv, err := semver.Make(uc.currentDcosVersion)
	if err != nil {
		return fmt.Errorf("invalid format of current version %s", uc.currentDcosVersion)
	}
	uv, err := semver.Make(uc.upgradeVersion)
	if err != nil {
		return fmt.Errorf("invalid format of desired version %s", uc.upgradeVersion)
	}
	if uv.LT(cv) {
		return fmt.Errorf("cannot downgrade DC/OS from %s to %s", uc.currentDcosVersion, uc.upgradeVersion)
	}

	if len(uc.containerService.Location) == 0 {
		uc.containerService.Location = uc.location
	} else if uc.containerService.Location != uc.location {
		return fmt.Errorf("--location does not match api model location")
	}

	// get available upgrades for container service
	orchestratorInfo, err := api.GetOrchestratorVersionProfile(uc.containerService.Properties.OrchestratorProfile)
	if err != nil {
		return fmt.Errorf("error getting list of available upgrades: %s", err.Error())
	}
	// add the current version if upgrade has failed
	orchestratorInfo.Upgrades = append(orchestratorInfo.Upgrades, uc.containerService.Properties.OrchestratorProfile.OrchestratorVersion)

	// validate desired upgrade version and set goal state
	found := false
	for _, up := range orchestratorInfo.Upgrades {
		if up == uc.upgradeVersion {
			uc.containerService.Properties.OrchestratorProfile.OrchestratorVersion = uc.upgradeVersion
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("upgrade to DCOS %s is not supported", uc.upgradeVersion)
	}

	uc.containerService.Properties.OrchestratorProfile.LinuxBootstrapProfile.BootstrapURL = uc.lnxBootstrapURL
	uc.containerService.Properties.OrchestratorProfile.WindowsBootstrapProfile.BootstrapURL = uc.winBootstrapURL

	// Read name suffix to identify nodes in the resource group that belong
	// to this cluster.
	templatePath := path.Join(uc.deploymentDirectory, "azuredeploy.json")
	contents, _ := ioutil.ReadFile(templatePath)

	var template interface{}
	json.Unmarshal(contents, &template)

	templateMap := template.(map[string]interface{})
	templateParameters := templateMap["parameters"].(map[string]interface{})

	nameSuffixParam := templateParameters["nameSuffix"].(map[string]interface{})
	uc.nameSuffix = nameSuffixParam["defaultValue"].(string)
	log.Infof("Name suffix: %s", uc.nameSuffix)
	return nil
}

func (uc *dcosUpgradeCmd) run(cmd *cobra.Command, args []string) error {
	err := uc.validate(cmd)
	if err != nil {
		log.Fatalf("error validating upgrade command: %v", err)
	}

	err = uc.loadCluster(cmd)
	if err != nil {
		log.Fatalf("error loading existing cluster: %v", err)
	}

	upgradeCluster := dcosupgrade.UpgradeCluster{
		Translator: &i18n.Translator{
			Locale: uc.locale,
		},
		Logger: log.NewEntry(log.New()),
		Client: uc.client,
	}

	if err = upgradeCluster.UpgradeCluster(uc.authArgs.SubscriptionID, uc.resourceGroupName, uc.currentDcosVersion,
		uc.containerService, uc.nameSuffix, uc.sshPrivateKey); err != nil {
		log.Fatalf("Error upgrading cluster: %v", err)
	}

	apiloader := &api.Apiloader{
		Translator: &i18n.Translator{
			Locale: uc.locale,
		},
	}
	b, err := apiloader.SerializeContainerService(uc.containerService, uc.apiVersion)
	if err != nil {
		return err
	}

	f := acsengine.FileSaver{
		Translator: &i18n.Translator{
			Locale: uc.locale,
		},
	}

	return f.SaveFile(uc.deploymentDirectory, "apimodel.json", b)
}
