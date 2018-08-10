package api

import (
	"fmt"

	"github.com/blang/semver"

	"github.com/Azure/dcos-engine/pkg/api/vlabs"
)

///////////////////////////////////////////////////////////
// The converter exposes functions to convert the top level
// ContainerService resource
//
// All other functions are internal helper functions used
// for converting.
///////////////////////////////////////////////////////////

// ConvertContainerServiceToVLabs converts an unversioned ContainerService to a vlabs ContainerService
func ConvertContainerServiceToVLabs(api *ContainerService) *vlabs.ContainerService {
	vlabsCS := &vlabs.ContainerService{}
	vlabsCS.ID = api.ID
	vlabsCS.Location = api.Location
	vlabsCS.Name = api.Name
	if api.Plan != nil {
		vlabsCS.Plan = &vlabs.ResourcePurchasePlan{}
		convertResourcePurchasePlanToVLabs(api.Plan, vlabsCS.Plan)
	}
	vlabsCS.Tags = map[string]string{}
	for k, v := range api.Tags {
		vlabsCS.Tags[k] = v
	}
	vlabsCS.Type = api.Type
	vlabsCS.Properties = &vlabs.Properties{}
	convertPropertiesToVLabs(api.Properties, vlabsCS.Properties)
	return vlabsCS
}

// ConvertOrchestratorVersionProfileToVLabs converts an unversioned OrchestratorVersionProfile to a vlabs OrchestratorVersionProfile
func ConvertOrchestratorVersionProfileToVLabs(api *OrchestratorVersionProfile) *vlabs.OrchestratorVersionProfile {
	vlabsProfile := &vlabs.OrchestratorVersionProfile{}
	switch api.OrchestratorType {
	case DCOS:
		vlabsProfile.OrchestratorType = vlabs.DCOS
	}
	vlabsProfile.OrchestratorVersion = api.OrchestratorVersion
	vlabsProfile.Default = api.Default
	if api.Upgrades != nil {
		vlabsProfile.Upgrades = make([]string, len(api.Upgrades))
		copy(vlabsProfile.Upgrades, api.Upgrades)
	}
	return vlabsProfile
}

// convertResourcePurchasePlanToVLabs converts a vlabs ResourcePurchasePlan to an unversioned ResourcePurchasePlan
func convertResourcePurchasePlanToVLabs(api *ResourcePurchasePlan, vlabs *vlabs.ResourcePurchasePlan) {
	vlabs.Name = api.Name
	vlabs.Product = api.Product
	vlabs.PromotionCode = api.PromotionCode
	vlabs.Publisher = api.Publisher
}

func convertPropertiesToVLabs(api *Properties, vlabsProps *vlabs.Properties) {
	vlabsProps.ProvisioningState = vlabs.ProvisioningState(api.ProvisioningState)
	if api.OrchestratorProfile != nil {
		vlabsProps.OrchestratorProfile = &vlabs.OrchestratorProfile{}
		convertOrchestratorProfileToVLabs(api.OrchestratorProfile, vlabsProps.OrchestratorProfile)
	}
	if api.MasterProfile != nil {
		vlabsProps.MasterProfile = &vlabs.MasterProfile{}
		convertMasterProfileToVLabs(api.MasterProfile, vlabsProps.MasterProfile)
	}
	vlabsProps.AgentPoolProfiles = []*vlabs.AgentPoolProfile{}
	for _, apiProfile := range api.AgentPoolProfiles {
		vlabsProfile := &vlabs.AgentPoolProfile{}
		convertAgentPoolProfileToVLabs(apiProfile, vlabsProfile)
		vlabsProps.AgentPoolProfiles = append(vlabsProps.AgentPoolProfiles, vlabsProfile)
	}
	if api.LinuxProfile != nil {
		vlabsProps.LinuxProfile = &vlabs.LinuxProfile{}
		convertLinuxProfileToVLabs(api.LinuxProfile, vlabsProps.LinuxProfile)
	}
	vlabsProps.ExtensionProfiles = []*vlabs.ExtensionProfile{}
	for _, extensionProfile := range api.ExtensionProfiles {
		vlabsExtensionProfile := &vlabs.ExtensionProfile{}
		convertExtensionProfileToVLabs(extensionProfile, vlabsExtensionProfile)
		vlabsProps.ExtensionProfiles = append(vlabsProps.ExtensionProfiles, vlabsExtensionProfile)
	}
	if api.WindowsProfile != nil {
		vlabsProps.WindowsProfile = &vlabs.WindowsProfile{}
		convertWindowsProfileToVLabs(api.WindowsProfile, vlabsProps.WindowsProfile)
	}
	if api.ServicePrincipalProfile != nil {
		vlabsProps.ServicePrincipalProfile = &vlabs.ServicePrincipalProfile{}
		convertServicePrincipalProfileToVLabs(api.ServicePrincipalProfile, vlabsProps.ServicePrincipalProfile)
	}
}

func convertExtensionProfileToVLabs(api *ExtensionProfile, obj *vlabs.ExtensionProfile) {
	obj.Name = api.Name
	obj.Version = api.Version
	obj.ExtensionParameters = api.ExtensionParameters
	if api.ExtensionParametersKeyVaultRef != nil {
		obj.ExtensionParametersKeyVaultRef = &vlabs.KeyvaultSecretRef{
			VaultID:       api.ExtensionParametersKeyVaultRef.VaultID,
			SecretName:    api.ExtensionParametersKeyVaultRef.SecretName,
			SecretVersion: api.ExtensionParametersKeyVaultRef.SecretVersion,
		}
	}
	obj.RootURL = api.RootURL
	obj.Script = api.Script
	obj.URLQuery = api.URLQuery
}

func convertExtensionToVLabs(api *Extension, vlabs *vlabs.Extension) {
	vlabs.Name = api.Name
	vlabs.SingleOrAll = api.SingleOrAll
	vlabs.Template = api.Template
}

func convertLinuxProfileToVLabs(obj *LinuxProfile, vlabsProfile *vlabs.LinuxProfile) {
	vlabsProfile.AdminUsername = obj.AdminUsername
	vlabsProfile.SSH.PublicKeys = []vlabs.PublicKey{}
	for _, d := range obj.SSH.PublicKeys {
		vlabsProfile.SSH.PublicKeys = append(vlabsProfile.SSH.PublicKeys,
			vlabs.PublicKey{KeyData: d.KeyData})
	}
	vlabsProfile.Secrets = []vlabs.KeyVaultSecrets{}
	for _, s := range obj.Secrets {
		secret := &vlabs.KeyVaultSecrets{}
		convertKeyVaultSecretsToVlabs(&s, secret)
		vlabsProfile.Secrets = append(vlabsProfile.Secrets, *secret)
	}
	vlabsProfile.ScriptRootURL = obj.ScriptRootURL
	if obj.CustomSearchDomain != nil {
		vlabsProfile.CustomSearchDomain = &vlabs.CustomSearchDomain{}
		vlabsProfile.CustomSearchDomain.Name = obj.CustomSearchDomain.Name
		vlabsProfile.CustomSearchDomain.RealmUser = obj.CustomSearchDomain.RealmUser
		vlabsProfile.CustomSearchDomain.RealmPassword = obj.CustomSearchDomain.RealmPassword
	}

	if obj.CustomNodesDNS != nil {
		vlabsProfile.CustomNodesDNS = &vlabs.CustomNodesDNS{}
		vlabsProfile.CustomNodesDNS.DNSServer = obj.CustomNodesDNS.DNSServer
	}
}

func convertWindowsProfileToVLabs(api *WindowsProfile, vlabsProfile *vlabs.WindowsProfile) {
	vlabsProfile.AdminUsername = api.AdminUsername
	vlabsProfile.AdminPassword = api.AdminPassword
	vlabsProfile.ImageVersion = api.ImageVersion
	vlabsProfile.WindowsImageSourceURL = api.WindowsImageSourceURL
	vlabsProfile.WindowsPublisher = api.WindowsPublisher
	vlabsProfile.WindowsOffer = api.WindowsOffer
	vlabsProfile.WindowsSku = api.WindowsSku
	vlabsProfile.Secrets = []vlabs.KeyVaultSecrets{}
	for _, s := range api.Secrets {
		secret := &vlabs.KeyVaultSecrets{}
		convertKeyVaultSecretsToVlabs(&s, secret)
		vlabsProfile.Secrets = append(vlabsProfile.Secrets, *secret)
	}
}

func convertOrchestratorProfileToVLabs(api *OrchestratorProfile, o *vlabs.OrchestratorProfile) {
	o.OrchestratorType = api.OrchestratorType

	if api.OrchestratorVersion != "" {
		o.OrchestratorVersion = api.OrchestratorVersion
		sv, _ := semver.Make(o.OrchestratorVersion)
		o.OrchestratorRelease = fmt.Sprintf("%d.%d", sv.Major, sv.Minor)
	}
	o.OAuthEnabled = api.OAuthEnabled
	if api.LinuxBootstrapProfile != nil {
		o.LinuxBootstrapProfile = &vlabs.BootstrapProfile{
			BootstrapURL: api.LinuxBootstrapProfile.BootstrapURL,
			Hosted:       api.LinuxBootstrapProfile.Hosted,
			VMSize:       api.LinuxBootstrapProfile.VMSize,
			OSDiskSizeGB: api.LinuxBootstrapProfile.OSDiskSizeGB,
			StaticIP:     api.LinuxBootstrapProfile.StaticIP,
			Subnet:       api.LinuxBootstrapProfile.Subnet,
		}
	}
	if api.WindowsBootstrapProfile != nil {
		o.WindowsBootstrapProfile = &vlabs.BootstrapProfile{
			BootstrapURL: api.WindowsBootstrapProfile.BootstrapURL,
			Hosted:       api.WindowsBootstrapProfile.Hosted,
			VMSize:       api.WindowsBootstrapProfile.VMSize,
			OSDiskSizeGB: api.WindowsBootstrapProfile.OSDiskSizeGB,
			StaticIP:     api.WindowsBootstrapProfile.StaticIP,
			Subnet:       api.WindowsBootstrapProfile.Subnet,
		}
	}
	o.Registry = api.Registry
	o.RegistryUser = api.RegistryUser
	o.RegistryPass = api.RegistryPass
}

func convertCustomFilesToVlabs(a *MasterProfile, v *vlabs.MasterProfile) {
	if a.CustomFiles != nil {
		v.CustomFiles = &[]vlabs.CustomFile{}
		for i := range *a.CustomFiles {
			*v.CustomFiles = append(*v.CustomFiles, vlabs.CustomFile{
				Dest:   (*a.CustomFiles)[i].Dest,
				Source: (*a.CustomFiles)[i].Source,
			})
		}
	}
}

func convertMasterProfileToVLabs(api *MasterProfile, vlabsProfile *vlabs.MasterProfile) {
	vlabsProfile.Count = api.Count
	vlabsProfile.DNSPrefix = api.DNSPrefix
	vlabsProfile.SubjectAltNames = api.SubjectAltNames
	vlabsProfile.VMSize = api.VMSize
	vlabsProfile.OSDiskSizeGB = api.OSDiskSizeGB
	vlabsProfile.VnetSubnetID = api.VnetSubnetID
	vlabsProfile.FirstConsecutiveStaticIP = api.FirstConsecutiveStaticIP
	vlabsProfile.VnetCidr = api.VnetCidr
	vlabsProfile.SetSubnet(api.Subnet)
	vlabsProfile.FQDN = api.FQDN
	vlabsProfile.StorageProfile = api.StorageProfile
	if api.PreprovisionExtension != nil {
		vlabsExtension := &vlabs.Extension{}
		convertExtensionToVLabs(api.PreprovisionExtension, vlabsExtension)
		vlabsProfile.PreProvisionExtension = vlabsExtension
	}
	vlabsProfile.Extensions = []vlabs.Extension{}
	for _, extension := range api.Extensions {
		vlabsExtension := &vlabs.Extension{}
		convertExtensionToVLabs(&extension, vlabsExtension)
		vlabsProfile.Extensions = append(vlabsProfile.Extensions, *vlabsExtension)
	}
	vlabsProfile.Distro = vlabs.Distro(api.Distro)
	if api.ImageRef != nil {
		vlabsProfile.ImageRef = &vlabs.ImageReference{}
		vlabsProfile.ImageRef.Name = api.ImageRef.Name
		vlabsProfile.ImageRef.ResourceGroup = api.ImageRef.ResourceGroup
	}

	convertCustomFilesToVlabs(api, vlabsProfile)
}

func convertKeyVaultSecretsToVlabs(api *KeyVaultSecrets, vlabsSecrets *vlabs.KeyVaultSecrets) {
	vlabsSecrets.SourceVault = &vlabs.KeyVaultID{ID: api.SourceVault.ID}
	vlabsSecrets.VaultCertificates = []vlabs.KeyVaultCertificate{}
	for _, c := range api.VaultCertificates {
		cert := vlabs.KeyVaultCertificate{}
		cert.CertificateStore = c.CertificateStore
		cert.CertificateURL = c.CertificateURL
		vlabsSecrets.VaultCertificates = append(vlabsSecrets.VaultCertificates, cert)
	}
}

func convertAgentPoolProfileToVLabs(api *AgentPoolProfile, p *vlabs.AgentPoolProfile) {
	p.Name = api.Name
	p.Count = api.Count
	p.VMSize = api.VMSize
	p.OSDiskSizeGB = api.OSDiskSizeGB
	p.DNSPrefix = api.DNSPrefix
	p.OSType = vlabs.OSType(api.OSType)
	p.Ports = []int{}
	p.Ports = append(p.Ports, api.Ports...)
	p.AvailabilityProfile = api.AvailabilityProfile
	p.ScaleSetPriority = api.ScaleSetPriority
	p.ScaleSetEvictionPolicy = api.ScaleSetEvictionPolicy
	p.StorageProfile = api.StorageProfile
	p.DiskSizesGB = []int{}
	p.DiskSizesGB = append(p.DiskSizesGB, api.DiskSizesGB...)
	p.VnetSubnetID = api.VnetSubnetID
	p.SetSubnet(api.Subnet)
	p.FQDN = api.FQDN
	p.CustomNodeLabels = map[string]string{}
	p.AcceleratedNetworkingEnabled = api.AcceleratedNetworkingEnabled

	for k, v := range api.CustomNodeLabels {
		p.CustomNodeLabels[k] = v
	}

	if api.PreprovisionExtension != nil {
		vlabsExtension := &vlabs.Extension{}
		convertExtensionToVLabs(api.PreprovisionExtension, vlabsExtension)
		p.PreProvisionExtension = vlabsExtension
	}

	p.Extensions = []vlabs.Extension{}
	for _, extension := range api.Extensions {
		vlabsExtension := &vlabs.Extension{}
		convertExtensionToVLabs(&extension, vlabsExtension)
		p.Extensions = append(p.Extensions, *vlabsExtension)
	}
	p.Distro = vlabs.Distro(api.Distro)
	if api.ImageRef != nil {
		p.ImageRef = &vlabs.ImageReference{}
		p.ImageRef.Name = api.ImageRef.Name
		p.ImageRef.ResourceGroup = api.ImageRef.ResourceGroup
	}
	p.Role = vlabs.AgentPoolProfileRole(api.Role)
}

func convertServicePrincipalProfileToVLabs(api *ServicePrincipalProfile, v *vlabs.ServicePrincipalProfile) {
	v.ClientID = api.ClientID
	v.Secret = api.Secret
	v.ObjectID = api.ObjectID
	if api.KeyvaultSecretRef != nil {
		v.KeyvaultSecretRef = &vlabs.KeyvaultSecretRef{
			VaultID:       api.KeyvaultSecretRef.VaultID,
			SecretName:    api.KeyvaultSecretRef.SecretName,
			SecretVersion: api.KeyvaultSecretRef.SecretVersion,
		}
	}
}
