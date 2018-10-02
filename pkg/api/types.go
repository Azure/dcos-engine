package api

import (
	neturl "net/url"
	"strings"

	"github.com/Azure/dcos-engine/pkg/api/vlabs"
)

// TypeMeta describes an individual API model object
type TypeMeta struct {
	// APIVersion is on every object
	APIVersion string `json:"apiVersion"`
}

// ResourcePurchasePlan defines resource plan as required by ARM
// for billing purposes.
type ResourcePurchasePlan struct {
	Name          string `json:"name"`
	Product       string `json:"product"`
	PromotionCode string `json:"promotionCode"`
	Publisher     string `json:"publisher"`
}

// ContainerService complies with the ARM model of
// resource definition in a JSON template.
type ContainerService struct {
	ID       string                `json:"id"`
	Location string                `json:"location"`
	Name     string                `json:"name"`
	Plan     *ResourcePurchasePlan `json:"plan,omitempty"`
	Tags     map[string]string     `json:"tags"`
	Type     string                `json:"type"`

	Properties *Properties `json:"properties,omitempty"`
}

// Properties represents the ACS cluster definition
type Properties struct {
	ProvisioningState       ProvisioningState        `json:"provisioningState,omitempty"`
	OrchestratorProfile     *OrchestratorProfile     `json:"orchestratorProfile,omitempty"`
	MasterProfile           *MasterProfile           `json:"masterProfile,omitempty"`
	AgentPoolProfiles       []*AgentPoolProfile      `json:"agentPoolProfiles,omitempty"`
	LinuxProfile            *LinuxProfile            `json:"linuxProfile,omitempty"`
	WindowsProfile          *WindowsProfile          `json:"windowsProfile,omitempty"`
	ExtensionProfiles       []*ExtensionProfile      `json:"extensionProfiles"`
	DiagnosticsProfile      *DiagnosticsProfile      `json:"diagnosticsProfile,omitempty"`
	ServicePrincipalProfile *ServicePrincipalProfile `json:"servicePrincipalProfile,omitempty"`
	CustomProfile           *CustomProfile           `json:"customProfile,omitempty"`
}

// ServicePrincipalProfile contains the client and secret used by the cluster for Azure Resource CRUD
type ServicePrincipalProfile struct {
	ClientID          string             `json:"clientId"`
	Secret            string             `json:"secret,omitempty"`
	ObjectID          string             `json:"objectId,omitempty"`
	KeyvaultSecretRef *KeyvaultSecretRef `json:"keyvaultSecretRef,omitempty"`
}

// KeyvaultSecretRef specifies path to the Azure keyvault along with secret name and (optionaly) version
// for Service Principal's secret
type KeyvaultSecretRef struct {
	VaultID       string `json:"vaultID"`
	SecretName    string `json:"secretName"`
	SecretVersion string `json:"version,omitempty"`
}

// LinuxProfile represents the linux parameters passed to the cluster
type LinuxProfile struct {
	AdminUsername string `json:"adminUsername"`
	SSH           struct {
		PublicKeys []PublicKey `json:"publicKeys"`
	} `json:"ssh"`
	Secrets            []KeyVaultSecrets   `json:"secrets,omitempty"`
	Distro             Distro              `json:"distro,omitempty"`
	ScriptRootURL      string              `json:"scriptroot,omitempty"`
	CustomSearchDomain *CustomSearchDomain `json:"customSearchDomain,omitempty"`
	CustomNodesDNS     *CustomNodesDNS     `json:"CustomNodesDNS,omitempty"`
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

// CustomNodesDNS represents the Search Domain when the custom vnet for a custom DNS as a nameserver.
type CustomNodesDNS struct {
	DNSServer string `json:"dnsServer,omitempty"`
}

// WindowsProfile represents the windows parameters passed to the cluster
type WindowsProfile struct {
	AdminUsername         string            `json:"adminUsername"`
	AdminPassword         string            `json:"adminPassword"`
	ImageVersion          string            `json:"imageVersion"`
	WindowsImageSourceURL string            `json:"windowsImageSourceURL"`
	WindowsPublisher      string            `json:"windowsPublisher"`
	WindowsOffer          string            `json:"windowsOffer"`
	WindowsSku            string            `json:"windowsSku"`
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
	// Upgrading means an existing ContainerService resource is being upgraded
	Upgrading ProvisioningState = "Upgrading"
)

// OrchestratorProfile contains Orchestrator properties
type OrchestratorProfile struct {
	OrchestratorType        string            `json:"orchestratorType"`
	OrchestratorVersion     string            `json:"orchestratorVersion"`
	OAuthEnabled            bool              `json:"oauthEnabled,omitempty"`
	OpenAccess              bool              `json:"openAccess,omitempty"`
	LinuxBootstrapProfile   *BootstrapProfile `json:"linuxBootstrapProfile,omitempty"`
	WindowsBootstrapProfile *BootstrapProfile `json:"windowsBootstrapProfile,omitempty"`
	Registry                string            `json:"registry,omitempty"`
	RegistryUser            string            `json:"registryUser,omitempty"`
	RegistryPass            string            `json:"registryPassword,omitempty"`
}

// OrchestratorVersionProfile contains information of a supported orchestrator version:
type OrchestratorVersionProfile struct {
	// Orchestrator type and version
	OrchestratorProfile
	// Whether this orchestrator version is deployed by default if orchestrator release is not specified
	Default bool `json:"default,omitempty"`
	// List of available upgrades for this orchestrator version
	Upgrades []string `json:"upgrades,omitempty"`
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
	BootstrapURL  string `json:"bootstrapURL,omitempty"`
	DockerVersion string `json:"dockerVersion,omitempty"`
	Hosted        bool   `json:"hosted,omitempty"`
	VMSize        string `json:"vmSize,omitempty"`
	OSDiskSizeGB  int    `json:"osDiskSizeGB,omitempty"`
	StaticIP      string `json:"staticIP,omitempty"`
	HasPublicIP   bool   `json:"hasPublicIP,omitempty"`
	Subnet        string `json:"subnet,omitempty"`
	EnableIPv6    bool   `json:"enableIPv6,omitempty"`
}

// MasterProfile represents the definition of the master cluster
type MasterProfile struct {
	Count                    int             `json:"count"`
	DNSPrefix                string          `json:"dnsPrefix"`
	SubjectAltNames          []string        `json:"subjectAltNames"`
	VMSize                   string          `json:"vmSize"`
	OSDiskSizeGB             int             `json:"osDiskSizeGB,omitempty"`
	VnetSubnetID             string          `json:"vnetSubnetID,omitempty"`
	VnetCidr                 string          `json:"vnetCidr,omitempty"`
	FirstConsecutiveStaticIP string          `json:"firstConsecutiveStaticIP,omitempty"`
	Subnet                   string          `json:"subnet"`
	IPAddressCount           int             `json:"ipAddressCount,omitempty"`
	StorageProfile           string          `json:"storageProfile,omitempty"`
	HTTPSourceAddressPrefix  string          `json:"HTTPSourceAddressPrefix,omitempty"`
	PreprovisionExtension    *Extension      `json:"preProvisionExtension"`
	PostprovisionExtension   *Extension      `json:"postProvisionExtension"`
	Extensions               []Extension     `json:"extensions"`
	Distro                   Distro          `json:"distro,omitempty"`
	ImageRef                 *ImageReference `json:"imageReference,omitempty"`
	CustomFiles              *[]CustomFile   `json:"customFiles,omitempty"`

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
	Name                         string               `json:"name"`
	Count                        int                  `json:"count"`
	VMSize                       string               `json:"vmSize"`
	OSDiskSizeGB                 int                  `json:"osDiskSizeGB,omitempty"`
	DNSPrefix                    string               `json:"dnsPrefix,omitempty"`
	OSType                       OSType               `json:"osType,omitempty"`
	Ports                        []int                `json:"ports,omitempty"`
	AvailabilityProfile          string               `json:"availabilityProfile"`
	ScaleSetPriority             string               `json:"scaleSetPriority,omitempty"`
	ScaleSetEvictionPolicy       string               `json:"scaleSetEvictionPolicy,omitempty"`
	StorageProfile               string               `json:"storageProfile,omitempty"`
	DiskSizesGB                  []int                `json:"diskSizesGB,omitempty"`
	VnetSubnetID                 string               `json:"vnetSubnetID,omitempty"`
	Subnet                       string               `json:"subnet"`
	IPAddressCount               int                  `json:"ipAddressCount,omitempty"`
	Distro                       Distro               `json:"distro,omitempty"`
	Role                         AgentPoolProfileRole `json:"role,omitempty"`
	AcceleratedNetworkingEnabled bool                 `json:"acceleratedNetworkingEnabled,omitempty"`

	FQDN                   string            `json:"fqdn,omitempty"`
	CustomNodeLabels       map[string]string `json:"customNodeLabels,omitempty"`
	PreprovisionExtension  *Extension        `json:"preProvisionExtension"`
	PostprovisionExtension *Extension        `json:"postProvisionExtension"`
	Extensions             []Extension       `json:"extensions"`
	ImageRef               *ImageReference   `json:"imageReference,omitempty"`
}

// AgentPoolProfileRole represents an agent role
type AgentPoolProfileRole string

// DiagnosticsProfile setting to enable/disable capturing
// diagnostics for VMs hosting container cluster.
type DiagnosticsProfile struct {
	VMDiagnostics *VMDiagnostics `json:"vmDiagnostics"`
}

// VMDiagnostics contains settings to on/off boot diagnostics collection
// in RD Host
type VMDiagnostics struct {
	Enabled bool `json:"enabled"`

	// Specifies storage account Uri where Boot Diagnostics (CRP &
	// VMSS BootDiagostics) and VM Diagnostics logs (using Linux
	// Diagnostics Extension) will be stored. Uri will be of standard
	// blob domain. i.e. https://storageaccount.blob.core.windows.net/
	// This field is readonly as ACS RP will create a storage account
	// for the customer.
	StorageURL *neturl.URL `json:"storageUrl"`
}

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

// CustomProfile specifies custom properties that are used for
// cluster instantiation.  Should not be used by most users.
type CustomProfile struct {
	Orchestrator string `json:"orchestrator,omitempty"`
}

// VlabsARMContainerService is the type we read and write from file
// needed because the json that is sent to ARM and dcos-engine
// is different from the json that the ACS RP Api gets from ARM
type VlabsARMContainerService struct {
	TypeMeta
	*vlabs.ContainerService
}

// HasWindows returns true if the cluster contains windows
func (p *Properties) HasWindows() bool {
	for _, agentPoolProfile := range p.AgentPoolProfiles {
		if agentPoolProfile.OSType == Windows {
			return true
		}
	}
	return false
}

// HasManagedDisks returns true if the cluster contains Managed Disks
func (p *Properties) HasManagedDisks() bool {
	if p.MasterProfile != nil && p.MasterProfile.StorageProfile == ManagedDisks {
		return true
	}
	for _, agentPoolProfile := range p.AgentPoolProfiles {
		if agentPoolProfile.StorageProfile == ManagedDisks {
			return true
		}
	}
	return false
}

// HasStorageAccountDisks returns true if the cluster contains Storage Account Disks
func (p *Properties) HasStorageAccountDisks() bool {
	if p.MasterProfile != nil && p.MasterProfile.StorageProfile == StorageAccount {
		return true
	}
	for _, agentPoolProfile := range p.AgentPoolProfiles {
		if agentPoolProfile.StorageProfile == StorageAccount {
			return true
		}
	}
	return false
}

// TotalNodes returns the total number of nodes in the cluster configuration
func (p *Properties) TotalNodes() int {
	var totalNodes int
	if p.MasterProfile != nil {
		totalNodes = p.MasterProfile.Count
	}
	for _, pool := range p.AgentPoolProfiles {
		totalNodes = totalNodes + pool.Count
	}
	return totalNodes
}

// HasVirtualMachineScaleSets returns true if the cluster contains Virtual Machine Scale Sets
func (p *Properties) HasVirtualMachineScaleSets() bool {
	for _, agentPoolProfile := range p.AgentPoolProfiles {
		if agentPoolProfile.AvailabilityProfile == VirtualMachineScaleSets {
			return true
		}
	}
	return false
}

// IsCustomVNET returns true if the customer brought their own VNET
func (m *MasterProfile) IsCustomVNET() bool {
	return len(m.VnetSubnetID) > 0
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

// IsLowPriorityScaleSet returns true if the VMSS is Low Priority
func (a *AgentPoolProfile) IsLowPriorityScaleSet() bool {
	return a.AvailabilityProfile == VirtualMachineScaleSets && a.ScaleSetPriority == ScaleSetPriorityLow
}

// IsManagedDisks returns true if the customer specified disks
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

// IsAcceleratedNetworkingEnabled returns true if the customer enabled Accelerated Networking
func (a *AgentPoolProfile) IsAcceleratedNetworkingEnabled() bool {
	return a.AcceleratedNetworkingEnabled
}

// HasSecrets returns true if the customer specified secrets to install
func (w *WindowsProfile) HasSecrets() bool {
	return len(w.Secrets) > 0
}

// HasCustomImage returns true if there is a custom windows os image url specified
func (w *WindowsProfile) HasCustomImage() bool {
	return len(w.WindowsImageSourceURL) > 0
}

// HasSecrets returns true if the customer specified secrets to install
func (l *LinuxProfile) HasSecrets() bool {
	return len(l.Secrets) > 0
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

// HasCustomNodesDNS returns true if the customer specified a dns server
func (l *LinuxProfile) HasCustomNodesDNS() bool {
	if l.CustomNodesDNS != nil {
		if l.CustomNodesDNS.DNSServer != "" {
			return true
		}
	}
	return false
}

// IsDCOS returns true if this template is for DCOS orchestrator
func (o *OrchestratorProfile) IsDCOS() bool {
	return o.OrchestratorType == DCOS
}

// RequireRouteTable returns true if this deployment requires routing table
func (o *OrchestratorProfile) RequireRouteTable() bool {
	return false
}

func isNSeriesSKU(p *Properties) bool {
	for _, profile := range p.AgentPoolProfiles {
		if strings.Contains(profile.VMSize, "Standard_N") {
			return true
		}
	}
	return false
}
