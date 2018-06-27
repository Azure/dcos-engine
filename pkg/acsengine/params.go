package acsengine

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/Azure/dcos-engine/pkg/api"
)

func getParameters(cs *api.ContainerService, isClassicMode bool, generatorCode string) (paramsMap, error) {
	properties := cs.Properties
	location := cs.Location
	parametersMap := paramsMap{}
	cloudSpecConfig := getCloudSpecConfig(location)

	// Master Parameters
	addValue(parametersMap, "location", location)

	// Identify Master distro
	masterDistro := getMasterDistro(properties.MasterProfile)
	if properties.MasterProfile != nil && properties.MasterProfile.ImageRef != nil {
		addValue(parametersMap, "osImageName", properties.MasterProfile.ImageRef.Name)
		addValue(parametersMap, "osImageResourceGroup", properties.MasterProfile.ImageRef.ResourceGroup)
	}
	// TODO: Choose the correct image config based on the version
	// for the openshift orchestrator
	addValue(parametersMap, "osImageOffer", cloudSpecConfig.OSImageConfig[masterDistro].ImageOffer)
	addValue(parametersMap, "osImageSKU", cloudSpecConfig.OSImageConfig[masterDistro].ImageSku)
	addValue(parametersMap, "osImagePublisher", cloudSpecConfig.OSImageConfig[masterDistro].ImagePublisher)
	addValue(parametersMap, "osImageVersion", cloudSpecConfig.OSImageConfig[masterDistro].ImageVersion)

	addValue(parametersMap, "fqdnEndpointSuffix", cloudSpecConfig.EndpointConfig.ResourceManagerVMDNSSuffix)
	addValue(parametersMap, "targetEnvironment", getCloudTargetEnv(location))
	addValue(parametersMap, "linuxAdminUsername", properties.LinuxProfile.AdminUsername)
	if properties.LinuxProfile.CustomSearchDomain != nil {
		addValue(parametersMap, "searchDomainName", properties.LinuxProfile.CustomSearchDomain.Name)
		addValue(parametersMap, "searchDomainRealmUser", properties.LinuxProfile.CustomSearchDomain.RealmUser)
		addValue(parametersMap, "searchDomainRealmPassword", properties.LinuxProfile.CustomSearchDomain.RealmPassword)
	}
	if properties.LinuxProfile.CustomNodesDNS != nil {
		addValue(parametersMap, "dnsServer", properties.LinuxProfile.CustomNodesDNS.DNSServer)
	}
	// masterEndpointDNSNamePrefix is the basis for storage account creation across dcos, swarm, and k8s
	if properties.MasterProfile != nil {
		// MasterProfile exists, uses master DNS prefix
		addValue(parametersMap, "masterEndpointDNSNamePrefix", properties.MasterProfile.DNSPrefix)
	}
	if properties.MasterProfile != nil {
		if properties.MasterProfile.IsCustomVNET() {
			addValue(parametersMap, "masterVnetSubnetID", properties.MasterProfile.VnetSubnetID)
		} else {
			addValue(parametersMap, "masterSubnet", properties.MasterProfile.Subnet)
		}
		addValue(parametersMap, "firstConsecutiveStaticIP", properties.MasterProfile.FirstConsecutiveStaticIP)
		addValue(parametersMap, "masterVMSize", properties.MasterProfile.VMSize)
		if isClassicMode {
			addValue(parametersMap, "masterCount", properties.MasterProfile.Count)
		}
	}
	addValue(parametersMap, "sshRSAPublicKey", properties.LinuxProfile.SSH.PublicKeys[0].KeyData)
	for i, s := range properties.LinuxProfile.Secrets {
		addValue(parametersMap, fmt.Sprintf("linuxKeyVaultID%d", i), s.SourceVault.ID)
		for j, c := range s.VaultCertificates {
			addValue(parametersMap, fmt.Sprintf("linuxKeyVaultID%dCertificateURL%d", i, j), c.CertificateURL)
		}
	}

	if strings.HasPrefix(properties.OrchestratorProfile.OrchestratorType, api.DCOS) {
		dcosBootstrapURL := getDCOSDefaultBootstrapInstallerURL(properties.OrchestratorProfile)
		dcosWindowsBootstrapURL := getDCOSDefaultWindowsBootstrapInstallerURL(properties.OrchestratorProfile)

		if properties.OrchestratorProfile.DcosConfig != nil {
			if properties.OrchestratorProfile.DcosConfig.DcosWindowsBootstrapURL != "" {
				dcosWindowsBootstrapURL = properties.OrchestratorProfile.DcosConfig.DcosWindowsBootstrapURL
			}
			if properties.OrchestratorProfile.DcosConfig.DcosBootstrapURL != "" {
				dcosBootstrapURL = properties.OrchestratorProfile.DcosConfig.DcosBootstrapURL
			}
			if len(properties.OrchestratorProfile.DcosConfig.Registry) > 0 {
				addValue(parametersMap, "registry", properties.OrchestratorProfile.DcosConfig.Registry)
				addValue(parametersMap, "registryKey", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", properties.OrchestratorProfile.DcosConfig.RegistryUser, properties.OrchestratorProfile.DcosConfig.RegistryPass))))
			}
		}

		addValue(parametersMap, "dcosBootstrapURL", dcosBootstrapURL)
		addValue(parametersMap, "dcosWindowsBootstrapURL", dcosWindowsBootstrapURL)

		if properties.OrchestratorProfile.DcosConfig.BootstrapProfile != nil {
			addValue(parametersMap, "bootstrapStaticIP", properties.OrchestratorProfile.DcosConfig.BootstrapProfile.StaticIP)
			addValue(parametersMap, "bootstrapVMSize", properties.OrchestratorProfile.DcosConfig.BootstrapProfile.VMSize)
		}
	}

	// Agent parameters
	for _, agentProfile := range properties.AgentPoolProfiles {
		addValue(parametersMap, fmt.Sprintf("%sCount", agentProfile.Name), agentProfile.Count)
		addValue(parametersMap, fmt.Sprintf("%sVMSize", agentProfile.Name), agentProfile.VMSize)
		if agentProfile.IsCustomVNET() {
			addValue(parametersMap, fmt.Sprintf("%sVnetSubnetID", agentProfile.Name), agentProfile.VnetSubnetID)
		} else {
			addValue(parametersMap, fmt.Sprintf("%sSubnet", agentProfile.Name), agentProfile.Subnet)
		}
		if len(agentProfile.Ports) > 0 {
			addValue(parametersMap, fmt.Sprintf("%sEndpointDNSNamePrefix", agentProfile.Name), agentProfile.DNSPrefix)
		}

		// Unless distro is defined, default distro is configured by defaults#setAgentNetworkDefaults
		//   Ignores Windows OS
		if !(agentProfile.OSType == api.Windows) {
			if agentProfile.ImageRef != nil {
				addValue(parametersMap, fmt.Sprintf("%sosImageName", agentProfile.Name), agentProfile.ImageRef.Name)
				addValue(parametersMap, fmt.Sprintf("%sosImageResourceGroup", agentProfile.Name), agentProfile.ImageRef.ResourceGroup)
			}
			addValue(parametersMap, fmt.Sprintf("%sosImageOffer", agentProfile.Name), cloudSpecConfig.OSImageConfig[agentProfile.Distro].ImageOffer)
			addValue(parametersMap, fmt.Sprintf("%sosImageSKU", agentProfile.Name), cloudSpecConfig.OSImageConfig[agentProfile.Distro].ImageSku)
			addValue(parametersMap, fmt.Sprintf("%sosImagePublisher", agentProfile.Name), cloudSpecConfig.OSImageConfig[agentProfile.Distro].ImagePublisher)
			addValue(parametersMap, fmt.Sprintf("%sosImageVersion", agentProfile.Name), cloudSpecConfig.OSImageConfig[agentProfile.Distro].ImageVersion)
		}
	}

	// Windows parameters
	if properties.HasWindows() {
		addValue(parametersMap, "windowsAdminUsername", properties.WindowsProfile.AdminUsername)
		addSecret(parametersMap, "windowsAdminPassword", properties.WindowsProfile.AdminPassword, false)
		if properties.WindowsProfile.ImageVersion != "" {
			addValue(parametersMap, "agentWindowsVersion", properties.WindowsProfile.ImageVersion)
		}
		if properties.WindowsProfile.WindowsImageSourceURL != "" {
			addValue(parametersMap, "agentWindowsSourceUrl", properties.WindowsProfile.WindowsImageSourceURL)
		}
		if properties.WindowsProfile.WindowsPublisher != "" {
			addValue(parametersMap, "agentWindowsPublisher", properties.WindowsProfile.WindowsPublisher)
		}
		if properties.WindowsProfile.WindowsOffer != "" {
			addValue(parametersMap, "agentWindowsOffer", properties.WindowsProfile.WindowsOffer)
		}
		if properties.WindowsProfile.WindowsSku != "" {
			addValue(parametersMap, "agentWindowsSku", properties.WindowsProfile.WindowsSku)
		}
		for i, s := range properties.WindowsProfile.Secrets {
			addValue(parametersMap, fmt.Sprintf("windowsKeyVaultID%d", i), s.SourceVault.ID)
			for j, c := range s.VaultCertificates {
				addValue(parametersMap, fmt.Sprintf("windowsKeyVaultID%dCertificateURL%d", i, j), c.CertificateURL)
				addValue(parametersMap, fmt.Sprintf("windowsKeyVaultID%dCertificateStore%d", i, j), c.CertificateStore)
			}
		}
	}

	for _, extension := range properties.ExtensionProfiles {
		if extension.ExtensionParametersKeyVaultRef != nil {
			addKeyvaultReference(parametersMap, fmt.Sprintf("%sParameters", extension.Name),
				extension.ExtensionParametersKeyVaultRef.VaultID,
				extension.ExtensionParametersKeyVaultRef.SecretName,
				extension.ExtensionParametersKeyVaultRef.SecretVersion)
		} else {
			addValue(parametersMap, fmt.Sprintf("%sParameters", extension.Name), extension.ExtensionParameters)
		}
	}

	return parametersMap, nil
}
