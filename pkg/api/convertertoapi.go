package api

import (
	"github.com/Azure/dcos-engine/pkg/api/common"
	"github.com/Azure/dcos-engine/pkg/api/vlabs"
	"github.com/Azure/dcos-engine/pkg/helpers"
)

///////////////////////////////////////////////////////////
// The converter exposes functions to convert the top level
// ContainerService resource
//
// All other functions are internal helper functions used
// for converting.
///////////////////////////////////////////////////////////

// ConvertVLabsContainerService converts a vlabs ContainerService to an unversioned ContainerService
func ConvertVLabsContainerService(vlabs *vlabs.ContainerService) *ContainerService {
	c := &ContainerService{}
	c.ID = vlabs.ID
	c.Location = helpers.NormalizeAzureRegion(vlabs.Location)
	c.Name = vlabs.Name
	if vlabs.Plan != nil {
		c.Plan = &ResourcePurchasePlan{}
		convertVLabsResourcePurchasePlan(vlabs.Plan, c.Plan)
	}
	c.Tags = map[string]string{}
	for k, v := range vlabs.Tags {
		c.Tags[k] = v
	}
	c.Type = vlabs.Type
	c.Properties = &Properties{}
	convertVLabsProperties(vlabs.Properties, c.Properties)
	return c
}

// convertVLabsResourcePurchasePlan converts a vlabs ResourcePurchasePlan to an unversioned ResourcePurchasePlan
func convertVLabsResourcePurchasePlan(vlabs *vlabs.ResourcePurchasePlan, api *ResourcePurchasePlan) {
	api.Name = vlabs.Name
	api.Product = vlabs.Product
	api.PromotionCode = vlabs.PromotionCode
	api.Publisher = vlabs.Publisher
}

func convertVLabsProperties(vlabs *vlabs.Properties, api *Properties) {
	api.ProvisioningState = ProvisioningState(vlabs.ProvisioningState)
	if vlabs.OrchestratorProfile != nil {
		api.OrchestratorProfile = &OrchestratorProfile{}
		convertVLabsOrchestratorProfile(vlabs, api.OrchestratorProfile)
	}
	if vlabs.MasterProfile != nil {
		api.MasterProfile = &MasterProfile{}
		convertVLabsMasterProfile(vlabs.MasterProfile, api.MasterProfile)
	}
	api.AgentPoolProfiles = []*AgentPoolProfile{}
	for _, p := range vlabs.AgentPoolProfiles {
		apiProfile := &AgentPoolProfile{}
		convertVLabsAgentPoolProfile(p, apiProfile)
		// by default vlabs will use managed disks for all orchestrators but kubernetes as it has encryption at rest.
		if !api.OrchestratorProfile.IsKubernetes() && !api.OrchestratorProfile.IsOpenShift() {
			if len(p.StorageProfile) == 0 {
				apiProfile.StorageProfile = ManagedDisks
			}
		}
		api.AgentPoolProfiles = append(api.AgentPoolProfiles, apiProfile)
	}
	if vlabs.LinuxProfile != nil {
		api.LinuxProfile = &LinuxProfile{}
		convertVLabsLinuxProfile(vlabs.LinuxProfile, api.LinuxProfile)
	}
	api.ExtensionProfiles = []*ExtensionProfile{}
	for _, p := range vlabs.ExtensionProfiles {
		apiExtensionProfile := &ExtensionProfile{}
		convertVLabsExtensionProfile(p, apiExtensionProfile)
		api.ExtensionProfiles = append(api.ExtensionProfiles, apiExtensionProfile)
	}
	if vlabs.WindowsProfile != nil {
		api.WindowsProfile = &WindowsProfile{}
		convertVLabsWindowsProfile(vlabs.WindowsProfile, api.WindowsProfile)
	}
	if vlabs.ServicePrincipalProfile != nil {
		api.ServicePrincipalProfile = &ServicePrincipalProfile{}
		convertVLabsServicePrincipalProfile(vlabs.ServicePrincipalProfile, api.ServicePrincipalProfile)
	}
	if vlabs.CertificateProfile != nil {
		api.CertificateProfile = &CertificateProfile{}
		convertVLabsCertificateProfile(vlabs.CertificateProfile, api.CertificateProfile)
	}

	if vlabs.AADProfile != nil {
		api.AADProfile = &AADProfile{}
		convertVLabsAADProfile(vlabs.AADProfile, api.AADProfile)
	}
	if vlabs.AzProfile != nil {
		api.AzProfile = &AzProfile{}
		convertVLabsAZProfile(vlabs.AzProfile, api.AzProfile)
	}
}

func convertVLabsAZProfile(vlabs *vlabs.AzProfile, api *AzProfile) {
	api.Location = vlabs.Location
	api.ResourceGroup = vlabs.ResourceGroup
	api.SubscriptionID = vlabs.SubscriptionID
	api.TenantID = vlabs.TenantID
}

func convertVLabsExtensionProfile(vlabs *vlabs.ExtensionProfile, api *ExtensionProfile) {
	api.Name = vlabs.Name
	api.Version = vlabs.Version
	api.ExtensionParameters = vlabs.ExtensionParameters
	if vlabs.ExtensionParametersKeyVaultRef != nil {
		api.ExtensionParametersKeyVaultRef = &KeyvaultSecretRef{
			VaultID:       vlabs.ExtensionParametersKeyVaultRef.VaultID,
			SecretName:    vlabs.ExtensionParametersKeyVaultRef.SecretName,
			SecretVersion: vlabs.ExtensionParametersKeyVaultRef.SecretVersion,
		}
	}
	api.RootURL = vlabs.RootURL
	api.Script = vlabs.Script
	api.URLQuery = vlabs.URLQuery
}

func convertVLabsExtension(vlabs *vlabs.Extension, api *Extension) {
	api.Name = vlabs.Name
	api.SingleOrAll = vlabs.SingleOrAll
	api.Template = vlabs.Template
}

func convertVLabsLinuxProfile(vlabs *vlabs.LinuxProfile, api *LinuxProfile) {
	api.AdminUsername = vlabs.AdminUsername
	api.SSH.PublicKeys = []PublicKey{}
	for _, d := range vlabs.SSH.PublicKeys {
		api.SSH.PublicKeys = append(api.SSH.PublicKeys,
			PublicKey{KeyData: d.KeyData})
	}
	api.Secrets = []KeyVaultSecrets{}
	for _, s := range vlabs.Secrets {
		secret := &KeyVaultSecrets{}
		convertVLabsKeyVaultSecrets(&s, secret)
		api.Secrets = append(api.Secrets, *secret)
	}
	api.ScriptRootURL = vlabs.ScriptRootURL
	if vlabs.CustomSearchDomain != nil {
		api.CustomSearchDomain = &CustomSearchDomain{}
		api.CustomSearchDomain.Name = vlabs.CustomSearchDomain.Name
		api.CustomSearchDomain.RealmUser = vlabs.CustomSearchDomain.RealmUser
		api.CustomSearchDomain.RealmPassword = vlabs.CustomSearchDomain.RealmPassword
	}

	if vlabs.CustomNodesDNS != nil {
		api.CustomNodesDNS = &CustomNodesDNS{}
		api.CustomNodesDNS.DNSServer = vlabs.CustomNodesDNS.DNSServer
	}
}

func convertVLabsWindowsProfile(vlabs *vlabs.WindowsProfile, api *WindowsProfile) {
	api.AdminUsername = vlabs.AdminUsername
	api.AdminPassword = vlabs.AdminPassword
	api.ImageVersion = vlabs.ImageVersion
	api.WindowsImageSourceURL = vlabs.WindowsImageSourceURL
	api.WindowsPublisher = vlabs.WindowsPublisher
	api.WindowsOffer = vlabs.WindowsOffer
	api.WindowsSku = vlabs.WindowsSku
	api.Secrets = []KeyVaultSecrets{}
	for _, s := range vlabs.Secrets {
		secret := &KeyVaultSecrets{}
		convertVLabsKeyVaultSecrets(&s, secret)
		api.Secrets = append(api.Secrets, *secret)
	}
}

func convertVLabsOrchestratorProfile(vp *vlabs.Properties, api *OrchestratorProfile) {
	vlabscs := vp.OrchestratorProfile
	api.OrchestratorType = vlabscs.OrchestratorType
	switch api.OrchestratorType {
	case DCOS:
		if vlabscs.DcosConfig != nil {
			api.DcosConfig = &DcosConfig{}
			convertVLabsDcosConfig(vlabscs.DcosConfig, api.DcosConfig)
		}
		api.OrchestratorVersion = common.RationalizeReleaseAndVersion(
			vlabscs.OrchestratorType,
			vlabscs.OrchestratorRelease,
			vlabscs.OrchestratorVersion,
			false)
	}
}

func convertVLabsDcosConfig(vlabs *vlabs.DcosConfig, api *DcosConfig) {
	api.DcosBootstrapURL = vlabs.DcosBootstrapURL
	api.DcosWindowsBootstrapURL = vlabs.DcosWindowsBootstrapURL

	if len(vlabs.Registry) > 0 {
		api.Registry = vlabs.Registry
	}

	if len(vlabs.RegistryUser) > 0 {
		api.RegistryUser = vlabs.RegistryUser
	}

	if len(vlabs.RegistryPass) > 0 {
		api.RegistryPass = vlabs.RegistryPass
	}
	api.DcosRepositoryURL = vlabs.DcosRepositoryURL
	api.DcosClusterPackageListID = vlabs.DcosClusterPackageListID
	api.DcosProviderPackageID = vlabs.DcosProviderPackageID

	if vlabs.BootstrapProfile != nil {
		api.BootstrapProfile = &BootstrapProfile{
			VMSize:       vlabs.BootstrapProfile.VMSize,
			OSDiskSizeGB: vlabs.BootstrapProfile.OSDiskSizeGB,
			OAuthEnabled: vlabs.BootstrapProfile.OAuthEnabled,
			StaticIP:     vlabs.BootstrapProfile.StaticIP,
			Subnet:       vlabs.BootstrapProfile.Subnet,
		}
	}
}

func convertCustomFilesToAPI(v *vlabs.MasterProfile, a *MasterProfile) {
	if v.CustomFiles != nil {
		a.CustomFiles = &[]CustomFile{}
		for i := range *v.CustomFiles {
			*a.CustomFiles = append(*a.CustomFiles, CustomFile{
				Dest:   (*v.CustomFiles)[i].Dest,
				Source: (*v.CustomFiles)[i].Source,
			})
		}
	}
}

func convertPrivateJumpboxProfileToAPI(v *vlabs.PrivateJumpboxProfile, a *PrivateJumpboxProfile) {
	a.Name = v.Name
	a.OSDiskSizeGB = v.OSDiskSizeGB
	a.VMSize = v.VMSize
	a.PublicKey = v.PublicKey
	a.Username = v.Username
	a.StorageProfile = v.StorageProfile
}

func convertVLabsMasterProfile(vlabs *vlabs.MasterProfile, api *MasterProfile) {
	api.Count = vlabs.Count
	api.DNSPrefix = vlabs.DNSPrefix
	api.SubjectAltNames = vlabs.SubjectAltNames
	api.VMSize = vlabs.VMSize
	api.OSDiskSizeGB = vlabs.OSDiskSizeGB
	api.VnetSubnetID = vlabs.VnetSubnetID
	api.FirstConsecutiveStaticIP = vlabs.FirstConsecutiveStaticIP
	api.VnetCidr = vlabs.VnetCidr
	api.Subnet = vlabs.GetSubnet()
	api.IPAddressCount = vlabs.IPAddressCount
	api.FQDN = vlabs.FQDN
	api.StorageProfile = vlabs.StorageProfile
	api.HTTPSourceAddressPrefix = vlabs.HTTPSourceAddressPrefix
	api.OAuthEnabled = vlabs.OAuthEnabled
	// by default vlabs will use managed disks as it has encryption at rest
	if len(api.StorageProfile) == 0 {
		api.StorageProfile = ManagedDisks
	}

	if vlabs.PreProvisionExtension != nil {
		apiExtension := &Extension{}
		convertVLabsExtension(vlabs.PreProvisionExtension, apiExtension)
		api.PreprovisionExtension = apiExtension
	}

	api.Extensions = []Extension{}
	for _, extension := range vlabs.Extensions {
		apiExtension := &Extension{}
		convertVLabsExtension(&extension, apiExtension)
		api.Extensions = append(api.Extensions, *apiExtension)
	}

	api.Distro = Distro(vlabs.Distro)

	if vlabs.ImageRef != nil {
		api.ImageRef = &ImageReference{}
		api.ImageRef.Name = vlabs.ImageRef.Name
		api.ImageRef.ResourceGroup = vlabs.ImageRef.ResourceGroup
	}

	convertCustomFilesToAPI(vlabs, api)
}

func convertVLabsAgentPoolProfile(vlabs *vlabs.AgentPoolProfile, api *AgentPoolProfile) {
	api.Name = vlabs.Name
	api.Count = vlabs.Count
	api.VMSize = vlabs.VMSize
	api.OSDiskSizeGB = vlabs.OSDiskSizeGB
	api.DNSPrefix = vlabs.DNSPrefix
	api.OSType = OSType(vlabs.OSType)
	api.Ports = []int{}
	api.Ports = append(api.Ports, vlabs.Ports...)
	api.AvailabilityProfile = vlabs.AvailabilityProfile
	api.ScaleSetPriority = vlabs.ScaleSetPriority
	api.ScaleSetEvictionPolicy = vlabs.ScaleSetEvictionPolicy
	api.StorageProfile = vlabs.StorageProfile
	api.DiskSizesGB = []int{}
	api.DiskSizesGB = append(api.DiskSizesGB, vlabs.DiskSizesGB...)
	api.VnetSubnetID = vlabs.VnetSubnetID
	api.Subnet = vlabs.GetSubnet()
	api.IPAddressCount = vlabs.IPAddressCount
	api.FQDN = vlabs.FQDN
	api.AcceleratedNetworkingEnabled = vlabs.AcceleratedNetworkingEnabled

	api.CustomNodeLabels = map[string]string{}
	for k, v := range vlabs.CustomNodeLabels {
		api.CustomNodeLabels[k] = v
	}

	if vlabs.PreProvisionExtension != nil {
		apiExtension := &Extension{}
		convertVLabsExtension(vlabs.PreProvisionExtension, apiExtension)
		api.PreprovisionExtension = apiExtension
	}

	api.Extensions = []Extension{}
	for _, extension := range vlabs.Extensions {
		apiExtension := &Extension{}
		convertVLabsExtension(&extension, apiExtension)
		api.Extensions = append(api.Extensions, *apiExtension)
	}
	api.Distro = Distro(vlabs.Distro)
	if vlabs.ImageRef != nil {
		api.ImageRef = &ImageReference{}
		api.ImageRef.Name = vlabs.ImageRef.Name
		api.ImageRef.ResourceGroup = vlabs.ImageRef.ResourceGroup
	}
	api.Role = AgentPoolProfileRole(vlabs.Role)
}

func convertVLabsKeyVaultSecrets(vlabs *vlabs.KeyVaultSecrets, api *KeyVaultSecrets) {
	api.SourceVault = &KeyVaultID{ID: vlabs.SourceVault.ID}
	api.VaultCertificates = []KeyVaultCertificate{}
	for _, c := range vlabs.VaultCertificates {
		cert := KeyVaultCertificate{}
		cert.CertificateStore = c.CertificateStore
		cert.CertificateURL = c.CertificateURL
		api.VaultCertificates = append(api.VaultCertificates, cert)
	}
}

func convertVLabsServicePrincipalProfile(vlabs *vlabs.ServicePrincipalProfile, api *ServicePrincipalProfile) {
	api.ClientID = vlabs.ClientID
	api.Secret = vlabs.Secret
	api.ObjectID = vlabs.ObjectID
	if vlabs.KeyvaultSecretRef != nil {
		api.KeyvaultSecretRef = &KeyvaultSecretRef{
			VaultID:       vlabs.KeyvaultSecretRef.VaultID,
			SecretName:    vlabs.KeyvaultSecretRef.SecretName,
			SecretVersion: vlabs.KeyvaultSecretRef.SecretVersion,
		}
	}
}

func convertVLabsCertificateProfile(vlabs *vlabs.CertificateProfile, api *CertificateProfile) {
	api.CaCertificate = vlabs.CaCertificate
	api.CaPrivateKey = vlabs.CaPrivateKey
	api.APIServerCertificate = vlabs.APIServerCertificate
	api.APIServerPrivateKey = vlabs.APIServerPrivateKey
	api.ClientCertificate = vlabs.ClientCertificate
	api.ClientPrivateKey = vlabs.ClientPrivateKey
	api.KubeConfigCertificate = vlabs.KubeConfigCertificate
	api.KubeConfigPrivateKey = vlabs.KubeConfigPrivateKey
	api.EtcdServerCertificate = vlabs.EtcdServerCertificate
	api.EtcdServerPrivateKey = vlabs.EtcdServerPrivateKey
	api.EtcdClientCertificate = vlabs.EtcdClientCertificate
	api.EtcdClientPrivateKey = vlabs.EtcdClientPrivateKey
	api.EtcdPeerCertificates = vlabs.EtcdPeerCertificates
	api.EtcdPeerPrivateKeys = vlabs.EtcdPeerPrivateKeys
}

func convertVLabsAADProfile(vlabs *vlabs.AADProfile, api *AADProfile) {
	api.ClientAppID = vlabs.ClientAppID
	api.ServerAppID = vlabs.ServerAppID
	api.TenantID = vlabs.TenantID
	api.AdminGroupID = vlabs.AdminGroupID
	api.Authenticator = OIDC
}

func addDCOSPublicAgentPool(api *Properties) {
	publicPool := &AgentPoolProfile{}
	// tag this agent pool with a known suffix string
	publicPool.Name = api.AgentPoolProfiles[0].Name + publicAgentPoolSuffix
	// move DNS prefix to public pool
	publicPool.DNSPrefix = api.AgentPoolProfiles[0].DNSPrefix
	api.AgentPoolProfiles[0].DNSPrefix = ""
	publicPool.VMSize = api.AgentPoolProfiles[0].VMSize // - use same VMsize for public pool
	publicPool.OSType = api.AgentPoolProfiles[0].OSType // - use same OSType for public pool
	api.AgentPoolProfiles[0].Ports = nil
	for _, port := range [3]int{80, 443, 8080} {
		publicPool.Ports = append(publicPool.Ports, port)
	}
	// - VM Count for public agents is based on the following:
	// 1 master => 1 VM
	// 3, 5 master => 3 VMsize
	if api.MasterProfile.Count == 1 {
		publicPool.Count = 1
	} else {
		publicPool.Count = 3
	}
	api.AgentPoolProfiles = append(api.AgentPoolProfiles, publicPool)
}
