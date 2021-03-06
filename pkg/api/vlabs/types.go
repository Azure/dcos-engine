package vlabs

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ResourcePurchasePlan defines resource plan as required by ARM
// for billing purposes.
type ResourcePurchasePlan struct {
	Name          string `json:"name,omitempty"`
	Product       string `json:"product,omitempty"`
	PromotionCode string `json:"promotionCode,omitempty"`
	Publisher     string `json:"publisher,omitempty"`
}

// ContainerService complies with the ARM model of
// resource definition in a JSON template.
type ContainerService struct {
	ID       string                `json:"id,omitempty"`
	Location string                `json:"location,omitempty"`
	Name     string                `json:"name,omitempty"`
	Plan     *ResourcePurchasePlan `json:"plan,omitempty"`
	Tags     map[string]string     `json:"tags,omitempty"`
	Type     string                `json:"type,omitempty"`

	Properties *Properties `json:"properties"`
}

// Properties represents the ACS cluster definition
type Properties struct {
	ProvisioningState       ProvisioningState        `json:"provisioningState,omitempty"`
	OrchestratorProfile     *OrchestratorProfile     `json:"orchestratorProfile,omitempty" validate:"required"`
	MasterProfile           *MasterProfile           `json:"masterProfile,omitempty" validate:"required"`
	AgentPoolProfiles       []*AgentPoolProfile      `json:"agentPoolProfiles,omitempty" validate:"dive,required"`
	LinuxProfile            *LinuxProfile            `json:"linuxProfile,omitempty" validate:"required"`
	ExtensionProfiles       []*ExtensionProfile      `json:"extensionProfiles,omitempty"`
	WindowsProfile          *WindowsProfile          `json:"windowsProfile,omitempty"`
	ServicePrincipalProfile *ServicePrincipalProfile `json:"servicePrincipalProfile,omitempty"`
}

// ServicePrincipalProfile contains the client and secret used by the cluster for Azure Resource CRUD
// The 'Secret' and 'KeyvaultSecretRef' parameters are mutually exclusive
// The 'Secret' parameter should be a secret in plain text.
// The 'KeyvaultSecretRef' parameter is a reference to a secret in a keyvault.
type ServicePrincipalProfile struct {
	ClientID          string             `json:"clientId,omitempty"`
	Secret            string             `json:"secret,omitempty"`
	ObjectID          string             `json:"objectId,omitempty"`
	KeyvaultSecretRef *KeyvaultSecretRef `json:"keyvaultSecretRef,omitempty"`
}

// KeyvaultSecretRef is a reference to a secret in a keyvault.
// The format of 'VaultID' value should be
// "/subscriptions/<SUB_ID>/resourceGroups/<RG_NAME>/providers/Microsoft.KeyVault/vaults/<KV_NAME>"
// where:
//    <SUB_ID> is the subscription ID of the keyvault
//    <RG_NAME> is the resource group of the keyvault
//    <KV_NAME> is the name of the keyvault
// The 'SecretName' is the name of the secret in the keyvault
// The 'SecretVersion' (optional) is the version of the secret (default: the latest version)
type KeyvaultSecretRef struct {
	VaultID       string `json:"vaultID" validate:"required"`
	SecretName    string `json:"secretName" validate:"required"`
	SecretVersion string `json:"version,omitempty"`
}

// LinuxProfile represents the linux parameters passed to the cluster
type LinuxProfile struct {
	AdminUsername string `json:"adminUsername" validate:"required"`
	SSH           struct {
		PublicKeys []PublicKey `json:"publicKeys" validate:"required,len=1"`
	} `json:"ssh" validate:"required"`
	Secrets            []KeyVaultSecrets   `json:"secrets,omitempty"`
	ScriptRootURL      string              `json:"scriptroot,omitempty"`
	CustomSearchDomain *CustomSearchDomain `json:"customSearchDomain,omitempty"`
	CustomNodesDNS     *CustomNodesDNS     `json:"customNodesDNS,omitempty"`
}

// PublicKey represents an SSH key for LinuxProfile
type PublicKey struct {
	KeyData string `json:"keyData"`
}

// CustomSearchDomain represents the Search Domain when the custom vnet has a windows server DNS as a nameserver.
type CustomSearchDomain struct {
	Name          string `json:"name,omitempty"`
	RealmUser     string `json:"realmUser,omitempty"`
	RealmPassword string `json:"realmPassword,omitempty"`
}

// CustomNodesDNS represents the Search Domain
type CustomNodesDNS struct {
	DNSServer string `json:"dnsServer,omitempty"`
}

// WindowsProfile represents the windows parameters passed to the cluster
type WindowsProfile struct {
	AdminUsername         string            `json:"adminUsername,omitempty"`
	AdminPassword         string            `json:"adminPassword,omitempty"`
	ImageVersion          string            `json:"imageVersion,omitempty"`
	WindowsImageSourceURL string            `json:"WindowsImageSourceUrl"`
	WindowsPublisher      string            `json:"WindowsPublisher"`
	WindowsOffer          string            `json:"WindowsOffer"`
	WindowsSku            string            `json:"WindowsSku"`
	Secrets               []KeyVaultSecrets `json:"secrets,omitempty"`
}

// ProvisioningState represents the current state of container service resource.
type ProvisioningState string

const (
	// Creating means ContainerService resource is being created.
	Creating ProvisioningState = "Creating"
	// Updating means an existing ContainerService resource is being updated
	Updating ProvisioningState = "Updating"
	// Failed means resource is in failed state
	Failed ProvisioningState = "Failed"
	// Succeeded means resource created succeeded during last create/update
	Succeeded ProvisioningState = "Succeeded"
	// Deleting means resource is in the process of being deleted
	Deleting ProvisioningState = "Deleting"
	// Migrating means resource is being migrated from one subscription or
	// resource group to another
	Migrating ProvisioningState = "Migrating"
)

// OrchestratorProfile contains Orchestrator properties
type OrchestratorProfile struct {
	OrchestratorType        string            `json:"orchestratorType" validate:"required"`
	OrchestratorRelease     string            `json:"orchestratorRelease,omitempty"`
	OrchestratorVersion     string            `json:"orchestratorVersion,omitempty"`
	OAuthEnabled            bool              `json:"oauthEnabled,omitempty"`
	OpenAccess              bool              `json:"openAccess,omitempty"`
	LinuxBootstrapProfile   *BootstrapProfile `json:"linuxBootstrapProfile,omitempty"`
	WindowsBootstrapProfile *BootstrapProfile `json:"windowsBootstrapProfile,omitempty"`
	Registry                string            `json:"registry,omitempty"`
	RegistryUser            string            `json:"registryUser,omitempty"`
	RegistryPass            string            `json:"registryPassword,omitempty"`
}

// UnmarshalJSON unmarshal json using the default behavior
// And do fields manipulation, such as populating default value
func (o *OrchestratorProfile) UnmarshalJSON(b []byte) error {
	// Need to have a alias type to avoid circular unmarshal
	type aliasOrchestratorProfile OrchestratorProfile
	op := aliasOrchestratorProfile{}
	if e := json.Unmarshal(b, &op); e != nil {
		return e
	}
	*o = OrchestratorProfile(op)
	// Unmarshal OrchestratorType, format it as well
	orchestratorType := o.OrchestratorType
	switch {
	case strings.EqualFold(orchestratorType, DCOS):
		o.OrchestratorType = DCOS
	default:
		return fmt.Errorf("OrchestratorType has unknown orchestrator: %s", orchestratorType)
	}
	return nil
}

// PrivateCluster defines the configuration for a private cluster
type PrivateCluster struct {
	Enabled *bool `json:"enabled,omitempty"`
}

// CustomFile has source as the full absolute source path to a file and dest
// is the full absolute desired destination path to put the file on a master node
type CustomFile struct {
	Source string `json:"source,omitempty"`
	Dest   string `json:"dest,omitempty"`
}

// BootstrapProfile represents the definition of the DCOS bootstrap node used to deploy the cluster
type BootstrapProfile struct {
	BootstrapURL  string                 `json:"bootstrapURL,omitempty"`
	DockerVersion string                 `json:"dockerVersion,omitempty"`
	Hosted        bool                   `json:"hosted,omitempty"`
	VMSize        string                 `json:"vmSize,omitempty"`
	OSDiskSizeGB  int                    `json:"osDiskSizeGB,omitempty"`
	StaticIP      string                 `json:"staticIP,omitempty"`
	HasPublicIP   bool                   `json:"hasPublicIP,omitempty"`
	Subnet        string                 `json:"subnet,omitempty"`
	EnableIPv6    bool                   `json:"enableIPv6,omitempty"`
	ExtraConfigs  map[string]interface{} `json:"extraConfigs,omitempty"`
}

// MasterProfile represents the definition of the master cluster
type MasterProfile struct {
	Count                    int             `json:"count" validate:"required,eq=1|eq=3|eq=5"`
	DNSPrefix                string          `json:"dnsPrefix" validate:"required"`
	SubjectAltNames          []string        `json:"subjectAltNames"`
	VMSize                   string          `json:"vmSize" validate:"required"`
	OSDiskSizeGB             int             `json:"osDiskSizeGB,omitempty" validate:"min=0,max=1023"`
	VnetSubnetID             string          `json:"vnetSubnetID,omitempty"`
	VnetCidr                 string          `json:"vnetCidr,omitempty"`
	FirstConsecutiveStaticIP string          `json:"firstConsecutiveStaticIP,omitempty"`
	IPAddressCount           int             `json:"ipAddressCount,omitempty" validate:"min=0,max=256"`
	StorageProfile           string          `json:"storageProfile,omitempty" validate:"eq=StorageAccount|eq=ManagedDisks|len=0"`
	HTTPSourceAddressPrefix  string          `json:"HTTPSourceAddressPrefix,omitempty"`
	PreProvisionExtension    *Extension      `json:"preProvisionExtension"`
	PostProvisionExtension   *Extension      `json:"postProvisionExtension"`
	Extensions               []Extension     `json:"extensions"`
	Distro                   Distro          `json:"distro,omitempty"`
	ImageRef                 *ImageReference `json:"imageReference,omitempty"`
	CustomFiles              *[]CustomFile   `json:"customFiles,omitempty"`

	// subnet is internal
	subnet string

	// Master LB public endpoint/FQDN with port
	// The format will be FQDN:2376
	// Not used during PUT, returned as part of GET
	FQDN string `json:"fqdn,omitempty"`
}

// ImageReference represents a reference to an Image resource in Azure.
type ImageReference struct {
	Name          string `json:"name,omitempty"`
	ResourceGroup string `json:"resourceGroup,omitempty"`
}

// ClassicAgentPoolProfileType represents types of classic profiles
type ClassicAgentPoolProfileType string

// ExtensionProfile represents an extension definition
type ExtensionProfile struct {
	Name                           string             `json:"name"`
	Version                        string             `json:"version"`
	ExtensionParameters            string             `json:"extensionParameters,omitempty"`
	ExtensionParametersKeyVaultRef *KeyvaultSecretRef `json:"parametersKeyvaultSecretRef,omitempty"`
	RootURL                        string             `json:"rootURL,omitempty"`
	// This is only needed for preprovision extensions and it needs to be a bash script
	Script   string `json:"script,omitempty"`
	URLQuery string `json:"urlQuery,omitempty"`
}

// Extension represents an extension definition in the master or agentPoolProfile
type Extension struct {
	Name        string `json:"name"`
	SingleOrAll string `json:"singleOrAll"`
	Template    string `json:"template"`
}

// AgentPoolProfile represents an agent pool definition
type AgentPoolProfile struct {
	Name                         string               `json:"name" validate:"required"`
	Count                        int                  `json:"count" validate:"required,min=1,max=100"`
	VMSize                       string               `json:"vmSize" validate:"required"`
	OSDiskSizeGB                 int                  `json:"osDiskSizeGB,omitempty" validate:"min=0,max=1023"`
	DNSPrefix                    string               `json:"dnsPrefix,omitempty"`
	OSType                       OSType               `json:"osType,omitempty"`
	Ports                        []int                `json:"ports,omitempty" validate:"dive,min=1,max=65535"`
	AvailabilityProfile          string               `json:"availabilityProfile"`
	ScaleSetPriority             string               `json:"scaleSetPriority,omitempty" validate:"eq=Regular|eq=Low|len=0"`
	ScaleSetEvictionPolicy       string               `json:"scaleSetEvictionPolicy,omitempty" validate:"eq=Delete|eq=Deallocate|len=0"`
	StorageProfile               string               `json:"storageProfile" validate:"eq=StorageAccount|eq=ManagedDisks|len=0"`
	DiskSizesGB                  []int                `json:"diskSizesGB,omitempty" validate:"max=4,dive,min=1,max=1023"`
	VnetSubnetID                 string               `json:"vnetSubnetID,omitempty"`
	IPAddressCount               int                  `json:"ipAddressCount,omitempty" validate:"min=0,max=256"`
	Distro                       Distro               `json:"distro,omitempty"`
	ImageRef                     *ImageReference      `json:"imageReference,omitempty"`
	Role                         AgentPoolProfileRole `json:"role,omitempty"`
	AcceleratedNetworkingEnabled bool                 `json:"acceleratedNetworkingEnabled,omitempty"`

	// subnet is internal
	subnet string

	FQDN                   string            `json:"fqdn"`
	CustomNodeLabels       map[string]string `json:"customNodeLabels,omitempty"`
	PreProvisionExtension  *Extension        `json:"preProvisionExtension"`
	PostProvisionExtension *Extension        `json:"postProvisionExtension"`
	Extensions             []Extension       `json:"extensions"`
}

// AgentPoolProfileRole represents an agent role
type AgentPoolProfileRole string

// KeyVaultSecrets specifies certificates to install on the pool
// of machines from a given key vault
// the key vault specified must have been granted read permissions to CRP
type KeyVaultSecrets struct {
	SourceVault       *KeyVaultID           `json:"sourceVault,omitempty"`
	VaultCertificates []KeyVaultCertificate `json:"vaultCertificates,omitempty"`
}

// KeyVaultID specifies a key vault
type KeyVaultID struct {
	ID string `json:"id,omitempty"`
}

// KeyVaultCertificate specifies a certificate to install
// On Linux, the certificate file is placed under the /var/lib/waagent directory
// with the file name <UppercaseThumbprint>.crt for the X509 certificate file
// and <UppercaseThumbprint>.prv for the private key. Both of these files are .pem formatted.
// On windows the certificate will be saved in the specified store.
type KeyVaultCertificate struct {
	CertificateURL   string `json:"certificateUrl,omitempty"`
	CertificateStore string `json:"certificateStore,omitempty"`
}

// OSType represents OS types of agents
type OSType string

// Distro represents Linux distro to use for Linux VMs
type Distro string

// HasWindows returns true if the cluster contains windows
func (p *Properties) HasWindows() bool {
	for _, agentPoolProfile := range p.AgentPoolProfiles {
		if agentPoolProfile.OSType == Windows {
			return true
		}
	}
	return false
}

// IsCustomVNET returns true if the customer brought their own VNET
func (m *MasterProfile) IsCustomVNET() bool {
	return len(m.VnetSubnetID) > 0
}

// GetSubnet returns the read-only subnet for the master
func (m *MasterProfile) GetSubnet() string {
	return m.subnet
}

// SetSubnet sets the read-only subnet for the master
func (m *MasterProfile) SetSubnet(subnet string) {
	m.subnet = subnet
}

// IsManagedDisks returns true if the master specified managed disks
func (m *MasterProfile) IsManagedDisks() bool {
	return m.StorageProfile == ManagedDisks
}

// IsStorageAccount returns true if the master specified storage account
func (m *MasterProfile) IsStorageAccount() bool {
	return m.StorageProfile == StorageAccount
}

// IsRHEL returns true if the master specified a RHEL distro
func (m *MasterProfile) IsRHEL() bool {
	return m.Distro == RHEL
}

// IsCoreOS returns true if the master specified a CoreOS distro
func (m *MasterProfile) IsCoreOS() bool {
	return m.Distro == CoreOS
}

// IsCustomVNET returns true if the customer brought their own VNET
func (a *AgentPoolProfile) IsCustomVNET() bool {
	return len(a.VnetSubnetID) > 0
}

// IsWindows returns true if the agent pool is windows
func (a *AgentPoolProfile) IsWindows() bool {
	return a.OSType == Windows
}

// IsLinux returns true if the agent pool is linux
func (a *AgentPoolProfile) IsLinux() bool {
	return a.OSType == Linux
}

// IsRHEL returns true if the agent pool specified a RHEL distro
func (a *AgentPoolProfile) IsRHEL() bool {
	return a.OSType == Linux && a.Distro == RHEL
}

// IsCoreOS returns true if the agent specified a CoreOS distro
func (a *AgentPoolProfile) IsCoreOS() bool {
	return a.OSType == Linux && a.Distro == CoreOS
}

// IsAvailabilitySets returns true if the customer specified disks
func (a *AgentPoolProfile) IsAvailabilitySets() bool {
	return a.AvailabilityProfile == AvailabilitySet
}

// IsVirtualMachineScaleSets returns true if the agent pool availability profile is VMSS
func (a *AgentPoolProfile) IsVirtualMachineScaleSets() bool {
	return a.AvailabilityProfile == VirtualMachineScaleSets
}

// IsNSeriesSKU returns true if the agent pool contains an N-series (NVIDIA GPU) VM
func (a *AgentPoolProfile) IsNSeriesSKU() bool {
	return strings.Contains(a.VMSize, "Standard_N")
}

// IsManagedDisks returns true if the customer specified managed disks
func (a *AgentPoolProfile) IsManagedDisks() bool {
	return a.StorageProfile == ManagedDisks
}

// IsStorageAccount returns true if the customer specified storage account
func (a *AgentPoolProfile) IsStorageAccount() bool {
	return a.StorageProfile == StorageAccount
}

// HasDisks returns true if the customer specified disks
func (a *AgentPoolProfile) HasDisks() bool {
	return len(a.DiskSizesGB) > 0
}

// GetSubnet returns the read-only subnet for the agent pool
func (a *AgentPoolProfile) GetSubnet() string {
	return a.subnet
}

// SetSubnet sets the read-only subnet for the agent pool
func (a *AgentPoolProfile) SetSubnet(subnet string) {
	a.subnet = subnet
}

// IsAcceleratedNetworkingEnabled returns true if the customer enabled Accelerated Networking
func (a *AgentPoolProfile) IsAcceleratedNetworkingEnabled() bool {
	return a.AcceleratedNetworkingEnabled
}

// HasSearchDomain returns true if the customer specified secrets to install
func (l *LinuxProfile) HasSearchDomain() bool {
	if l.CustomSearchDomain != nil {
		if l.CustomSearchDomain.Name != "" && l.CustomSearchDomain.RealmPassword != "" && l.CustomSearchDomain.RealmUser != "" {
			return true
		}
	}
	return false
}

// HasCustomNodesDNS returns true if the customer specified secrets to install
func (l *LinuxProfile) HasCustomNodesDNS() bool {
	if l.CustomNodesDNS != nil {
		if l.CustomNodesDNS.DNSServer != "" {
			return true
		}
	}
	return false
}
