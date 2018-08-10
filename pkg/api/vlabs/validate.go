package vlabs

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"

	"github.com/Azure/dcos-engine/pkg/api/common"
	validator "gopkg.in/go-playground/validator.v9"
)

var (
	validate        *validator.Validate
	keyvaultIDRegex *regexp.Regexp
	labelValueRegex *regexp.Regexp
	labelKeyRegex   *regexp.Regexp
)

const (
	labelKeyPrefixMaxLength = 253
	labelValueFormat        = "^([A-Za-z0-9][-A-Za-z0-9_.]{0,61})?[A-Za-z0-9]$"
	labelKeyFormat          = "^(([a-zA-Z0-9-]+[.])*[a-zA-Z0-9-]+[/])?([A-Za-z0-9][-A-Za-z0-9_.]{0,61})?[A-Za-z0-9]$"
)

func init() {
	validate = validator.New()
	keyvaultIDRegex = regexp.MustCompile(`^/subscriptions/\S+/resourceGroups/\S+/providers/Microsoft.KeyVault/vaults/[^/\s]+$`)
	labelValueRegex = regexp.MustCompile(labelValueFormat)
	labelKeyRegex = regexp.MustCompile(labelKeyFormat)
}

// Validate implements APIObject
func (a *Properties) Validate(isUpdate bool) error {
	if e := validate.Struct(a); e != nil {
		return handleValidationErrors(e.(validator.ValidationErrors))
	}
	if e := a.validateOrchestratorProfile(isUpdate); e != nil {
		return e
	}
	if e := a.validateMasterProfile(); e != nil {
		return e
	}
	if e := a.validateAgentPoolProfiles(); e != nil {
		return e
	}
	if e := a.validateLinuxProfile(); e != nil {
		return e
	}
	if e := a.validateExtensions(); e != nil {
		return e
	}
	if e := a.validateVNET(); e != nil {
		return e
	}
	return nil
}

func handleValidationErrors(e validator.ValidationErrors) error {
	// Override any version specific validation error message
	// common.HandleValidationErrors if the validation error message is general
	return common.HandleValidationErrors(e)
}

func (a *Properties) validateOrchestratorProfile(isUpdate bool) error {
	o := a.OrchestratorProfile
	// On updates we only need to make sure there is a supported patch version for the minor version
	if !isUpdate {
		switch o.OrchestratorType {
		case DCOS:
			version := common.RationalizeReleaseAndVersion(
				o.OrchestratorType,
				o.OrchestratorRelease,
				o.OrchestratorVersion,
				false)
			if version == "" {
				return fmt.Errorf("the following OrchestratorProfile configuration is not supported: OrchestratorType: %s, OrchestratorRelease: %s, OrchestratorVersion: %s. Please check supported Release or Version for this build of dcos-engine", o.OrchestratorType, o.OrchestratorRelease, o.OrchestratorVersion)
			}
			if o.LinuxBootstrapProfile != nil && len(o.LinuxBootstrapProfile.StaticIP) > 0 {
				if net.ParseIP(o.LinuxBootstrapProfile.StaticIP) == nil {
					return fmt.Errorf("LinuxBootstrapProfile.StaticIP '%s' is an invalid IP address",
						o.LinuxBootstrapProfile.StaticIP)
				}
			}
			if o.WindowsBootstrapProfile != nil && len(o.WindowsBootstrapProfile.StaticIP) > 0 {
				if net.ParseIP(o.WindowsBootstrapProfile.StaticIP) == nil {
					return fmt.Errorf("WindowsBootstrapProfile.StaticIP '%s' is an invalid IP address",
						o.WindowsBootstrapProfile.StaticIP)
				}
			}
		default:
			return fmt.Errorf("OrchestratorProfile has unknown orchestrator: %s", o.OrchestratorType)
		}
	} else {
		switch o.OrchestratorType {
		case DCOS:
			version := common.RationalizeReleaseAndVersion(
				o.OrchestratorType,
				o.OrchestratorRelease,
				o.OrchestratorVersion,
				a.HasWindows())
			if version == "" {
				patchVersion := common.GetValidPatchVersion(o.OrchestratorType, o.OrchestratorVersion, a.HasWindows())
				// if there isn't a supported patch version for this version fail
				if patchVersion == "" {
					if a.HasWindows() {
						return fmt.Errorf("the following OrchestratorProfile configuration is not supported with Windows agentpools: OrchestratorType: \"%s\", OrchestratorRelease: \"%s\", OrchestratorVersion: \"%s\". Please check supported Release or Version for this build of dcos-engine", o.OrchestratorType, o.OrchestratorRelease, o.OrchestratorVersion)
					}
					return fmt.Errorf("the following OrchestratorProfile configuration is not supported: OrchestratorType: \"%s\", OrchestratorRelease: \"%s\", OrchestratorVersion: \"%s\". Please check supported Release or Version for this build of dcos-engine", o.OrchestratorType, o.OrchestratorRelease, o.OrchestratorVersion)
				}
			}
		}
	}
	return nil
}

func (a *Properties) validateMasterProfile() error {
	m := a.MasterProfile

	if m.ImageRef != nil {
		if err := m.ImageRef.validateImageNameAndGroup(); err != nil {
			return err
		}
	}
	return validateDNSName(m.DNSPrefix)
}

func (a *Properties) validateAgentPoolProfiles() error {

	profileNames := make(map[string]bool)
	for _, agentPoolProfile := range a.AgentPoolProfiles {

		if e := validatePoolName(agentPoolProfile.Name); e != nil {
			return e
		}

		// validate that each AgentPoolProfile Name is unique
		if _, ok := profileNames[agentPoolProfile.Name]; ok {
			return fmt.Errorf("profile name '%s' already exists, profile names must be unique across pools", agentPoolProfile.Name)
		}
		profileNames[agentPoolProfile.Name] = true

		if e := validatePoolOSType(agentPoolProfile.OSType); e != nil {
			return e
		}

		if e := agentPoolProfile.validateOrchestratorSpecificProperties(a.OrchestratorProfile.OrchestratorType); e != nil {
			return e
		}

		if agentPoolProfile.ImageRef != nil {
			return agentPoolProfile.ImageRef.validateImageNameAndGroup()
		}

		if e := agentPoolProfile.validateAvailabilityProfile(a.OrchestratorProfile.OrchestratorType); e != nil {
			return e
		}

		if e := agentPoolProfile.validateRoles(a.OrchestratorProfile.OrchestratorType); e != nil {
			return e
		}

		if e := agentPoolProfile.validateCustomNodeLabels(a.OrchestratorProfile.OrchestratorType); e != nil {
			return e
		}

		if e := agentPoolProfile.validateWindows(a.OrchestratorProfile, a.WindowsProfile); agentPoolProfile.OSType == Windows && e != nil {
			return e
		}
	}
	return nil
}

func (a *Properties) validateLinuxProfile() error {
	if e := validate.Var(a.LinuxProfile.SSH.PublicKeys[0].KeyData, "required"); e != nil {
		return fmt.Errorf("KeyData in LinuxProfile.SSH.PublicKeys cannot be empty string")
	}
	return validateKeyVaultSecrets(a.LinuxProfile.Secrets, false)
}

func (a *Properties) validateExtensions() error {
	for _, agentPool := range a.AgentPoolProfiles {
		if len(agentPool.Extensions) != 0 && (len(agentPool.AvailabilityProfile) == 0 || agentPool.IsVirtualMachineScaleSets()) {
			return fmt.Errorf("Extensions are currently not supported with VirtualMachineScaleSets. Please specify \"availabilityProfile\": \"%s\"", AvailabilitySet)
		}
	}

	for _, extension := range a.ExtensionProfiles {
		if extension.ExtensionParametersKeyVaultRef != nil {
			if e := validate.Var(extension.ExtensionParametersKeyVaultRef.VaultID, "required"); e != nil {
				return fmt.Errorf("the Keyvault ID must be specified for Extension %s", extension.Name)
			}
			if e := validate.Var(extension.ExtensionParametersKeyVaultRef.SecretName, "required"); e != nil {
				return fmt.Errorf("the Keyvault Secret must be specified for Extension %s", extension.Name)
			}
			if !keyvaultIDRegex.MatchString(extension.ExtensionParametersKeyVaultRef.VaultID) {
				return fmt.Errorf("Extension %s's keyvault secret reference is of incorrect format", extension.Name)
			}
		}
	}
	return nil
}

func (a *Properties) validateVNET() error {
	isCustomVNET := a.MasterProfile.IsCustomVNET()
	for _, agentPool := range a.AgentPoolProfiles {
		if agentPool.IsCustomVNET() != isCustomVNET {
			return fmt.Errorf("Multiple VNET Subnet configurations specified.  The master profile and each agent pool profile must all specify a custom VNET Subnet, or none at all")
		}
	}
	if isCustomVNET {
		subscription, resourcegroup, vnetname, _, e := common.GetVNETSubnetIDComponents(a.MasterProfile.VnetSubnetID)
		if e != nil {
			return e
		}

		for _, agentPool := range a.AgentPoolProfiles {
			agentSubID, agentRG, agentVNET, _, err := common.GetVNETSubnetIDComponents(agentPool.VnetSubnetID)
			if err != nil {
				return err
			}
			if agentSubID != subscription ||
				agentRG != resourcegroup ||
				agentVNET != vnetname {
				return errors.New("Multiple VNETS specified.  The master profile and each agent pool must reference the same VNET (but it is ok to reference different subnets on that VNET)")
			}
		}

		masterFirstIP := net.ParseIP(a.MasterProfile.FirstConsecutiveStaticIP)
		if masterFirstIP == nil {
			return fmt.Errorf("MasterProfile.FirstConsecutiveStaticIP (with VNET Subnet specification) '%s' is an invalid IP address", a.MasterProfile.FirstConsecutiveStaticIP)
		}

		if a.MasterProfile.VnetCidr != "" {
			_, _, err := net.ParseCIDR(a.MasterProfile.VnetCidr)
			if err != nil {
				return fmt.Errorf("MasterProfile.VnetCidr '%s' contains invalid cidr notation", a.MasterProfile.VnetCidr)
			}
		}
	}
	return nil
}

func (a *AgentPoolProfile) validateAvailabilityProfile(orchestratorType string) error {
	switch a.AvailabilityProfile {
	case AvailabilitySet:
	case VirtualMachineScaleSets:
	case "":
	default:
		{
			return fmt.Errorf("unknown availability profile type '%s' for agent pool '%s'.  Specify either %s, or %s", a.AvailabilityProfile, a.Name, AvailabilitySet, VirtualMachineScaleSets)
		}
	}
	return nil
}

func (a *AgentPoolProfile) validateRoles(orchestratorType string) error {
	validRoles := []AgentPoolProfileRole{AgentPoolProfileRoleEmpty}

	var found bool
	for _, validRole := range validRoles {
		if a.Role == validRole {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("Role %q is not supported for Orchestrator %s", a.Role, orchestratorType)
	}
	return nil
}

func (a *AgentPoolProfile) validateCustomNodeLabels(orchestratorType string) error {
	return nil
}

func (a *AgentPoolProfile) validateWindows(o *OrchestratorProfile, w *WindowsProfile) error {
	if w != nil {
		if e := w.Validate(o.OrchestratorType); e != nil {
			return e
		}
	} else {
		return fmt.Errorf("WindowsProfile is required when the cluster definition contains Windows agent pool(s)")
	}
	return nil
}

func (a *AgentPoolProfile) validateOrchestratorSpecificProperties(orchestratorType string) error {
	if a.DNSPrefix != "" {
		if e := validateDNSName(a.DNSPrefix); e != nil {
			return e
		}
		if len(a.Ports) > 0 {
			if e := validateUniquePorts(a.Ports, a.Name); e != nil {
				return e
			}
		} else {
			a.Ports = []int{80, 443, 8080}
		}
	} else {
		if e := validate.Var(a.Ports, "len=0"); e != nil {
			return fmt.Errorf("AgentPoolProfile.Ports must be empty when AgentPoolProfile.DNSPrefix is empty for Orchestrator: %s", string(orchestratorType))
		}
	}

	if len(a.DiskSizesGB) > 0 {
		if e := validate.Var(a.StorageProfile, "eq=StorageAccount|eq=ManagedDisks"); e != nil {
			return fmt.Errorf("property 'StorageProfile' must be set to either '%s' or '%s' when attaching disks", StorageAccount, ManagedDisks)
		}
		if e := validate.Var(a.AvailabilityProfile, "eq=VirtualMachineScaleSets|eq=AvailabilitySet"); e != nil {
			return fmt.Errorf("property 'AvailabilityProfile' must be set to either '%s' or '%s' when attaching disks", VirtualMachineScaleSets, AvailabilitySet)
		}
		if a.StorageProfile == StorageAccount && (a.AvailabilityProfile == VirtualMachineScaleSets) {
			return fmt.Errorf("VirtualMachineScaleSets does not support storage account attached disks.  Instead specify 'StorageAccount': '%s' or specify AvailabilityProfile '%s'", ManagedDisks, AvailabilitySet)
		}
	}
	if len(a.Ports) == 0 && len(a.DNSPrefix) > 0 {
		return fmt.Errorf("AgentPoolProfile.Ports must be non empty when AgentPoolProfile.DNSPrefix is specified")
	}
	return nil
}

func validateKeyVaultSecrets(secrets []KeyVaultSecrets, requireCertificateStore bool) error {
	for _, s := range secrets {
		if len(s.VaultCertificates) == 0 {
			return fmt.Errorf("Invalid KeyVaultSecrets must have no empty VaultCertificates")
		}
		if s.SourceVault == nil {
			return fmt.Errorf("missing SourceVault in KeyVaultSecrets")
		}
		if s.SourceVault.ID == "" {
			return fmt.Errorf("KeyVaultSecrets must have a SourceVault.ID")
		}
		for _, c := range s.VaultCertificates {
			if _, e := url.Parse(c.CertificateURL); e != nil {
				return fmt.Errorf("Certificate url was invalid. received error %s", e)
			}
			if e := validateName(c.CertificateStore, "KeyVaultCertificate.CertificateStore"); requireCertificateStore && e != nil {
				return fmt.Errorf("%s for certificates in a WindowsProfile", e)
			}
		}
	}
	return nil
}

// Validate ensures that the WindowsProfile is valid
func (w *WindowsProfile) Validate(orchestratorType string) error {
	if e := validate.Var(w.AdminUsername, "required"); e != nil {
		return fmt.Errorf("WindowsProfile.AdminUsername is required, when agent pool specifies windows")
	}
	if e := validate.Var(w.AdminPassword, "required"); e != nil {
		return fmt.Errorf("WindowsProfile.AdminPassword is required, when agent pool specifies windows")
	}
	return validateKeyVaultSecrets(w.Secrets, true)
}

func validateName(name string, label string) error {
	if name == "" {
		return fmt.Errorf("%s must be a non-empty value", label)
	}
	return nil
}

func validatePoolName(poolName string) error {
	// we will cap at length of 12 and all lowercase letters since this makes up the VMName
	poolNameRegex := `^([a-z][a-z0-9]{0,11})$`
	re, err := regexp.Compile(poolNameRegex)
	if err != nil {
		return err
	}
	submatches := re.FindStringSubmatch(poolName)
	if len(submatches) != 2 {
		return fmt.Errorf("pool name '%s' is invalid. A pool name must start with a lowercase letter, have max length of 12, and only have characters a-z0-9", poolName)
	}
	return nil
}

func validatePoolOSType(os OSType) error {
	if os != Linux && os != Windows && os != "" {
		return fmt.Errorf("AgentPoolProfile.osType must be either Linux or Windows")
	}
	return nil
}

func validateDNSName(dnsName string) error {
	dnsNameRegex := `^([A-Za-z][A-Za-z0-9-]{1,43}[A-Za-z0-9])$`
	re, err := regexp.Compile(dnsNameRegex)
	if err != nil {
		return err
	}
	if !re.MatchString(dnsName) {
		return fmt.Errorf("DNS name '%s' is invalid. The DNS name must contain between 3 and 45 characters.  The name can contain only letters, numbers, and hyphens.  The name must start with a letter and must end with a letter or a number (length was %d)", dnsName, len(dnsName))
	}
	return nil
}

func validateUniquePorts(ports []int, name string) error {
	portMap := make(map[int]bool)
	for _, port := range ports {
		if _, ok := portMap[port]; ok {
			return fmt.Errorf("agent profile '%s' has duplicate port '%d', ports must be unique", name, port)
		}
		portMap[port] = true
	}
	return nil
}

func (i *ImageReference) validateImageNameAndGroup() error {
	if i.Name == "" && i.ResourceGroup != "" {
		return errors.New("imageName needs to be specified when imageResourceGroup is provided")
	}
	if i.Name != "" && i.ResourceGroup == "" {
		return errors.New("imageResourceGroup needs to be specified when imageName is provided")
	}
	return nil
}
