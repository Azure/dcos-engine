package acsengine

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/Azure/dcos-engine/pkg/api"
	"github.com/Azure/dcos-engine/pkg/api/common"
	"github.com/Azure/dcos-engine/pkg/helpers"
	"github.com/Azure/dcos-engine/pkg/i18n"
)

// TemplateGenerator represents the object that performs the template generation.
type TemplateGenerator struct {
	ClassicMode bool
	Translator  *i18n.Translator
}

// InitializeTemplateGenerator creates a new template generator object
func InitializeTemplateGenerator(ctx Context, classicMode bool) (*TemplateGenerator, error) {
	t := &TemplateGenerator{
		ClassicMode: classicMode,
		Translator:  ctx.Translator,
	}

	if err := t.verifyFiles(); err != nil {
		return nil, err
	}

	return t, nil
}

// GenerateTemplate generates the template from the API Model
func (t *TemplateGenerator) GenerateTemplate(containerService *api.ContainerService, generatorCode string, isUpgrade bool, acsengineVersion string) (templateRaw string, parametersRaw string, certsGenerated bool, err error) {
	// named return values are used in order to set err in case of a panic
	templateRaw = ""
	parametersRaw = ""
	err = nil

	var templ *template.Template

	properties := containerService.Properties
	// save the current orchestrator version and restore it after deploying.
	// this allows us to deploy agents on the most recent patch without updating the orchestator version in the object
	orchVersion := properties.OrchestratorProfile.OrchestratorVersion
	defer func() {
		properties.OrchestratorProfile.OrchestratorVersion = orchVersion
	}()
	if certsGenerated, err = setPropertiesDefaults(containerService, isUpgrade); err != nil {
		return templateRaw, parametersRaw, certsGenerated, err
	}

	templ = template.New("acs template").Funcs(t.getTemplateFuncMap(containerService))

	files, baseFile, e := t.prepareTemplateFiles(properties)
	if e != nil {
		return "", "", false, e
	}

	for _, file := range files {
		bytes, e := Asset(file)
		if e != nil {
			err = t.Translator.Errorf("Error reading file %s, Error: %s", file, e.Error())
			return templateRaw, parametersRaw, certsGenerated, err
		}
		if _, err = templ.New(file).Parse(string(bytes)); err != nil {
			return templateRaw, parametersRaw, certsGenerated, err
		}
	}
	// template generation may have panics in the called functions.  This catches those panics
	// and ensures the panic is returned as an error
	defer func() {
		if r := recover(); r != nil {
			s := debug.Stack()
			err = fmt.Errorf("%v - %s", r, s)

			// invalidate the template and the parameters
			templateRaw = ""
			parametersRaw = ""
		}
	}()

	if !validateDistro(containerService) {
		return templateRaw, parametersRaw, certsGenerated, fmt.Errorf("Invalid distro")
	}

	var b bytes.Buffer
	if err = templ.ExecuteTemplate(&b, baseFile, properties); err != nil {
		return templateRaw, parametersRaw, certsGenerated, err
	}
	templateRaw = b.String()

	var parametersMap paramsMap
	if parametersMap, err = getParameters(containerService, t.ClassicMode, generatorCode, acsengineVersion); err != nil {
		return templateRaw, parametersRaw, certsGenerated, err
	}

	var parameterBytes []byte
	if parameterBytes, err = helpers.JSONMarshal(parametersMap, false); err != nil {
		return templateRaw, parametersRaw, certsGenerated, err
	}
	parametersRaw = string(parameterBytes)

	return templateRaw, parametersRaw, certsGenerated, err
}

func (t *TemplateGenerator) verifyFiles() error {
	allFiles := commonTemplateFiles
	allFiles = append(allFiles, dcosTemplateFiles...)
	allFiles = append(allFiles, dcos2TemplateFiles...)
	for _, file := range allFiles {
		if _, err := Asset(file); err != nil {
			return t.Translator.Errorf("template file %s does not exist", file)
		}
	}
	return nil
}

func (t *TemplateGenerator) prepareTemplateFiles(properties *api.Properties) ([]string, string, error) {
	var files []string
	var baseFile string
	switch properties.OrchestratorProfile.OrchestratorType {
	case api.DCOS:
		if properties.OrchestratorProfile.DcosConfig == nil || properties.OrchestratorProfile.DcosConfig.BootstrapProfile == nil {
			files = append(commonTemplateFiles, dcosTemplateFiles...)
			baseFile = dcosBaseFile
		} else {
			files = append(commonTemplateFiles, dcos2TemplateFiles...)
			baseFile = dcos2BaseFile
		}
	default:
		return nil, "", t.Translator.Errorf("orchestrator '%s' is unsupported", properties.OrchestratorProfile.OrchestratorType)
	}

	return files, baseFile, nil
}

// getTemplateFuncMap returns all functions used in template generation
func (t *TemplateGenerator) getTemplateFuncMap(cs *api.ContainerService) template.FuncMap {
	return template.FuncMap{
		"IsHostedMaster": func() bool {
			return cs.Properties.HostedMasterProfile != nil
		},
		"IsDCOS19": func() bool {
			return cs.Properties.OrchestratorProfile.OrchestratorType == api.DCOS &&
				(cs.Properties.OrchestratorProfile.OrchestratorVersion == common.DCOSVersion1Dot9Dot0 ||
					cs.Properties.OrchestratorProfile.OrchestratorVersion == common.DCOSVersion1Dot9Dot8)
		},
		"IsKubernetesVersionGe": func(version string) bool {
			return cs.Properties.OrchestratorProfile.IsKubernetes() && common.IsKubernetesVersionGe(cs.Properties.OrchestratorProfile.OrchestratorVersion, version)
		},
		"IsKubernetesVersionLt": func(version string) bool {
			return cs.Properties.OrchestratorProfile.IsKubernetes() && !common.IsKubernetesVersionGe(cs.Properties.OrchestratorProfile.OrchestratorVersion, version)
		},
		"GetMasterKubernetesLabels": func(rg string) string {
			var buf bytes.Buffer
			buf.WriteString("kubernetes.io/role=master")
			buf.WriteString(fmt.Sprintf(",kubernetes.azure.com/cluster=%s", rg))
			return buf.String()
		},
		"GetAgentKubernetesLabels": func(profile *api.AgentPoolProfile, rg string) string {
			var buf bytes.Buffer
			buf.WriteString(fmt.Sprintf("kubernetes.io/role=agent,agentpool=%s", profile.Name))
			if profile.StorageProfile == api.ManagedDisks {
				storagetier, _ := getStorageAccountType(profile.VMSize)
				buf.WriteString(fmt.Sprintf(",storageprofile=managed,storagetier=%s", storagetier))
			}
			buf.WriteString(fmt.Sprintf(",kubernetes.azure.com/cluster=%s", rg))
			for k, v := range profile.CustomNodeLabels {
				buf.WriteString(fmt.Sprintf(",%s=%s", k, v))
			}
			return buf.String()
		},
		"GetKubeletConfigKeyVals": func(kc *api.KubernetesConfig) string {
			if kc == nil {
				return ""
			}
			kubeletConfig := cs.Properties.OrchestratorProfile.KubernetesConfig.KubeletConfig
			if kc.KubeletConfig != nil {
				kubeletConfig = kc.KubeletConfig
			}
			// Order by key for consistency
			keys := []string{}
			for key := range kubeletConfig {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			var buf bytes.Buffer
			for _, key := range keys {
				buf.WriteString(fmt.Sprintf("%s=%s ", key, kubeletConfig[key]))
			}
			return buf.String()
		},
		"GetK8sRuntimeConfigKeyVals": func(config map[string]string) string {
			// Order by key for consistency
			keys := []string{}
			for key := range config {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			var buf bytes.Buffer
			for _, key := range keys {
				buf.WriteString(fmt.Sprintf("\\\"%s=%s\\\", ", key, config[key]))
			}
			return strings.TrimSuffix(buf.String(), ", ")
		},
		"HasPrivateRegistry": func() bool {
			if cs.Properties.OrchestratorProfile.DcosConfig != nil {
				return len(cs.Properties.OrchestratorProfile.DcosConfig.Registry) > 0
			}
			return false
		},
		"RequiresFakeAgentOutput": func() bool {
			return cs.Properties.OrchestratorProfile.IsKubernetes() || cs.Properties.OrchestratorProfile.IsOpenShift()
		},
		"IsSwarmMode": func() bool {
			return cs.Properties.OrchestratorProfile.IsSwarmMode()
		},
		"IsKubernetes": func() bool {
			return cs.Properties.OrchestratorProfile.IsKubernetes()
		},
		"IsOpenShift": func() bool {
			return cs.Properties.OrchestratorProfile.IsOpenShift()
		},
		"IsPublic": func(ports []int) bool {
			return len(ports) > 0
		},
		"IsAzureCNI": func() bool {
			return cs.Properties.OrchestratorProfile.IsAzureCNI()
		},
		"RequireRouteTable": func() bool {
			return cs.Properties.OrchestratorProfile.RequireRouteTable()
		},
		"IsPrivateCluster": func() bool {
			if !cs.Properties.OrchestratorProfile.IsKubernetes() {
				return false
			}
			return helpers.IsTrueBoolPointer(cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster.Enabled)
		},
		"UseManagedIdentity": func() bool {
			return cs.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity
		},
		"UseInstanceMetadata": func() bool {
			return helpers.IsTrueBoolPointer(cs.Properties.OrchestratorProfile.KubernetesConfig.UseInstanceMetadata)
		},
		"GetVNETSubnetDependencies": func() string {
			return getVNETSubnetDependencies(cs.Properties)
		},
		"GetLBRules": func(name string, ports []int) string {
			return getLBRules(name, ports)
		},
		"GetProbes": func(ports []int) string {
			return getProbes(ports)
		},
		"GetSecurityRules": func(ports []int) string {
			return getSecurityRules(ports)
		},
		"GetUniqueNameSuffix": func() string {
			return GenerateClusterID(cs.Properties)
		},
		"GetVNETAddressPrefixes": func() string {
			return getVNETAddressPrefixes(cs.Properties)
		},
		"GetVNETSubnets": func(addNSG bool) string {
			return getVNETSubnets(cs.Properties, addNSG)
		},
		"GetDataDisks": func(profile *api.AgentPoolProfile) string {
			return getDataDisks(profile)
		},
		"HasBootstrap": func() bool {
			return cs.Properties.OrchestratorProfile.DcosConfig != nil && cs.Properties.OrchestratorProfile.DcosConfig.BootstrapProfile != nil
		},
		"HasBootstrapPublicIP": func() bool {
			return false
		},
		"IsHostedBootstrap": func() bool {
			return false
		},
		"GetDCOSBootstrapCustomData": func() string {
			masterIPList := generateIPList(cs.Properties.MasterProfile.Count, cs.Properties.MasterProfile.FirstConsecutiveStaticIP)
			for i, v := range masterIPList {
				masterIPList[i] = "    - " + v
			}

			str := getSingleLineDCOSCustomData(
				cs.Properties.OrchestratorProfile.OrchestratorType,
				dcos2BootstrapCustomdata, 0,
				map[string]string{
					"PROVISION_SOURCE_STR":    getDCOSProvisionScript(dcosProvisionSource),
					"PROVISION_STR":           getDCOSProvisionScript(dcos2BootstrapProvision),
					"MASTER_IP_LIST":          strings.Join(masterIPList, "\n"),
					"BOOTSTRAP_IP":            cs.Properties.OrchestratorProfile.DcosConfig.BootstrapProfile.StaticIP,
					"BOOTSTRAP_OAUTH_ENABLED": strconv.FormatBool(cs.Properties.OrchestratorProfile.DcosConfig.BootstrapProfile.OAuthEnabled)})

			return fmt.Sprintf("\"customData\": \"[base64(concat('#cloud-config\\n\\n', '%s'))]\",", str)
		},
		"GetDCOSMasterCustomData": func() string {
			masterAttributeContents := getDCOSMasterCustomNodeLabels()
			masterPreprovisionExtension := ""
			if cs.Properties.MasterProfile.PreprovisionExtension != nil {
				masterPreprovisionExtension += "\n"
				masterPreprovisionExtension += makeMasterExtensionScriptCommands(cs)
			}
			var bootstrapIP string
			if cs.Properties.OrchestratorProfile.DcosConfig != nil && cs.Properties.OrchestratorProfile.DcosConfig.BootstrapProfile != nil {
				bootstrapIP = cs.Properties.OrchestratorProfile.DcosConfig.BootstrapProfile.StaticIP
			}

			str := getSingleLineDCOSCustomData(
				cs.Properties.OrchestratorProfile.OrchestratorType,
				getDCOSCustomDataTemplate(cs.Properties.OrchestratorProfile.OrchestratorType, cs.Properties.OrchestratorProfile.OrchestratorVersion),
				cs.Properties.MasterProfile.Count,
				map[string]string{
					"PROVISION_SOURCE_STR":   getDCOSProvisionScript(dcosProvisionSource),
					"PROVISION_STR":          getDCOSMasterProvisionScript(cs.Properties.OrchestratorProfile, bootstrapIP),
					"ATTRIBUTES_STR":         masterAttributeContents,
					"PREPROVISION_EXTENSION": masterPreprovisionExtension,
					"ROLENAME":               "master"})

			return fmt.Sprintf("\"customData\": \"[base64(concat('#cloud-config\\n\\n', '%s'))]\",", str)
		},
		"GetDCOSAgentCustomData": func(profile *api.AgentPoolProfile) string {
			attributeContents := getDCOSAgentCustomNodeLabels(profile)
			agentPreprovisionExtension := ""
			if profile.PreprovisionExtension != nil {
				agentPreprovisionExtension += "\n"
				agentPreprovisionExtension += makeAgentExtensionScriptCommands(cs, profile)
			}
			var agentRoleName, bootstrapIP string
			if len(profile.Ports) > 0 {
				agentRoleName = "slave_public"
			} else {
				agentRoleName = "slave"
			}
			if cs.Properties.OrchestratorProfile.DcosConfig != nil && cs.Properties.OrchestratorProfile.DcosConfig.BootstrapProfile != nil {
				bootstrapIP = cs.Properties.OrchestratorProfile.DcosConfig.BootstrapProfile.StaticIP
			}

			str := getSingleLineDCOSCustomData(
				cs.Properties.OrchestratorProfile.OrchestratorType,
				getDCOSCustomDataTemplate(cs.Properties.OrchestratorProfile.OrchestratorType, cs.Properties.OrchestratorProfile.OrchestratorVersion),
				cs.Properties.MasterProfile.Count,
				map[string]string{
					"PROVISION_SOURCE_STR":   getDCOSProvisionScript(dcosProvisionSource),
					"PROVISION_STR":          getDCOSAgentProvisionScript(profile, cs.Properties.OrchestratorProfile, bootstrapIP),
					"ATTRIBUTES_STR":         attributeContents,
					"PREPROVISION_EXTENSION": agentPreprovisionExtension,
					"ROLENAME":               agentRoleName})

			return fmt.Sprintf("\"customData\": \"[base64(concat('#cloud-config\\n\\n', '%s'))]\",", str)
		},
		"GetDCOSWindowsAgentCustomData": func(profile *api.AgentPoolProfile) string {
			agentPreprovisionExtension := ""
			if profile.PreprovisionExtension != nil {
				agentPreprovisionExtension += "\n"
				agentPreprovisionExtension += makeAgentExtensionScriptCommands(cs, profile)
			}
			b, err := Asset(dcosWindowsProvision)
			if err != nil {
				// this should never happen and this is a bug
				panic(fmt.Sprintf("BUG: %s", err.Error()))
			}
			// translate the parameters
			csStr := string(b)
			csStr = strings.Replace(csStr, "PREPROVISION_EXTENSION", agentPreprovisionExtension, -1)
			csStr = strings.Replace(csStr, "\r\n", "\n", -1)
			str := getBase64CustomScriptFromStr(csStr)
			return fmt.Sprintf("\"customData\": \"%s\"", str)
		},
		"GetDCOSWindowsAgentCustomNodeAttributes": func(profile *api.AgentPoolProfile) string {
			return getDCOSWindowsAgentCustomAttributes(profile)
		},
		"GetDCOSWindowsAgentPreprovisionParameters": func(profile *api.AgentPoolProfile) string {
			agentPreprovisionExtensionParameters := ""
			if profile.PreprovisionExtension != nil {
				agentPreprovisionExtensionParameters = getDCOSWindowsAgentPreprovisionParameters(cs, profile)
			}
			return agentPreprovisionExtensionParameters
		},
		"GetMasterAllowedSizes": func() string {
			if t.ClassicMode {
				return GetClassicAllowedSizes()
			} else if cs.Properties.OrchestratorProfile.OrchestratorType == api.DCOS {
				return GetDCOSMasterAllowedSizes()
			}
			return GetMasterAgentAllowedSizes()
		},
		"GetAgentAllowedSizes": func() string {
			if t.ClassicMode {
				return GetClassicAllowedSizes()
			} else if cs.Properties.OrchestratorProfile.IsKubernetes() || cs.Properties.OrchestratorProfile.IsOpenShift() {
				return GetKubernetesAgentAllowedSizes()
			}
			return GetMasterAgentAllowedSizes()
		},
		"getSwarmVersions": func() string {
			return getSwarmVersions(api.SwarmVersion, api.SwarmDockerComposeVersion)
		},
		"GetSwarmModeVersions": func() string {
			return getSwarmVersions(api.DockerCEVersion, api.DockerCEDockerComposeVersion)
		},
		"GetSizeMap": func() string {
			if t.ClassicMode {
				return GetClassicSizeMap()
			}
			return GetSizeMap()
		},
		"GetClassicMode": func() bool {
			return t.ClassicMode
		},
		"Base64": func(s string) string {
			return base64.StdEncoding.EncodeToString([]byte(s))
		},
		"GetDefaultInternalLbStaticIPOffset": func() int {
			return DefaultInternalLbStaticIPOffset
		},
		"WriteLinkedTemplatesForExtensions": func() string {
			extensions := getLinkedTemplatesForExtensions(cs.Properties)
			return extensions
		},
		"GetLocation": func() string {
			return cs.Location
		},
		"WrapAsVariable": func(s string) string {
			return fmt.Sprintf("',variables('%s'),'", s)
		},
		"WrapAsVerbatim": func(s string) string {
			return fmt.Sprintf("',%s,'", s)
		},
		"AnyAgentUsesAvailabilitySets": func() bool {
			for _, agentProfile := range cs.Properties.AgentPoolProfiles {
				if agentProfile.IsAvailabilitySets() {
					return true
				}
			}
			return false
		},
		"AnyAgentUsesVirtualMachineScaleSets": func() bool {
			for _, agentProfile := range cs.Properties.AgentPoolProfiles {
				if agentProfile.IsVirtualMachineScaleSets() {
					return true
				}
			}
			return false
		},
		"HasLinuxAgents": func() bool {
			for _, agentProfile := range cs.Properties.AgentPoolProfiles {
				if agentProfile.IsLinux() {
					return true
				}
			}
			return false
		},
		"IsNVIDIADevicePluginEnabled": func() bool {
			return cs.Properties.IsNVIDIADevicePluginEnabled()
		},
		"IsNSeriesSKU": func(profile *api.AgentPoolProfile) bool {
			return isNSeriesSKU(profile)
		},
		"GetGPUDriversInstallScript": func(profile *api.AgentPoolProfile) string {
			return getGPUDriversInstallScript(profile)
		},
		"HasLinuxSecrets": func() bool {
			return cs.Properties.LinuxProfile.HasSecrets()
		},
		"HasCustomSearchDomain": func() bool {
			return cs.Properties.LinuxProfile.HasSearchDomain()
		},
		"HasCustomNodesDNS": func() bool {
			return cs.Properties.LinuxProfile.HasCustomNodesDNS()
		},
		"HasWindowsSecrets": func() bool {
			return cs.Properties.WindowsProfile.HasSecrets()
		},
		"HasWindowsCustomImage": func() bool {
			return cs.Properties.WindowsProfile.HasCustomImage()
		},
		"GetConfigurationScriptRootURL": func() string {
			if cs.Properties.LinuxProfile.ScriptRootURL == "" {
				return DefaultConfigurationScriptRootURL
			}
			return cs.Properties.LinuxProfile.ScriptRootURL
		},
		"GetMasterOSImageOffer": func() string {
			cloudSpecConfig := getCloudSpecConfig(cs.Location)
			return fmt.Sprintf("\"%s\"", cloudSpecConfig.OSImageConfig[cs.Properties.MasterProfile.Distro].ImageOffer)
		},
		"GetMasterOSImagePublisher": func() string {
			cloudSpecConfig := getCloudSpecConfig(cs.Location)
			return fmt.Sprintf("\"%s\"", cloudSpecConfig.OSImageConfig[cs.Properties.MasterProfile.Distro].ImagePublisher)
		},
		"GetMasterOSImageSKU": func() string {
			cloudSpecConfig := getCloudSpecConfig(cs.Location)
			return fmt.Sprintf("\"%s\"", cloudSpecConfig.OSImageConfig[cs.Properties.MasterProfile.Distro].ImageSku)
		},
		"GetMasterOSImageVersion": func() string {
			cloudSpecConfig := getCloudSpecConfig(cs.Location)
			return fmt.Sprintf("\"%s\"", cloudSpecConfig.OSImageConfig[cs.Properties.MasterProfile.Distro].ImageVersion)
		},
		"GetAgentOSImageOffer": func(profile *api.AgentPoolProfile) string {
			cloudSpecConfig := getCloudSpecConfig(cs.Location)
			return fmt.Sprintf("\"%s\"", cloudSpecConfig.OSImageConfig[profile.Distro].ImageOffer)
		},
		"GetAgentOSImagePublisher": func(profile *api.AgentPoolProfile) string {
			cloudSpecConfig := getCloudSpecConfig(cs.Location)
			return fmt.Sprintf("\"%s\"", cloudSpecConfig.OSImageConfig[profile.Distro].ImagePublisher)
		},
		"GetAgentOSImageSKU": func(profile *api.AgentPoolProfile) string {
			cloudSpecConfig := getCloudSpecConfig(cs.Location)
			return fmt.Sprintf("\"%s\"", cloudSpecConfig.OSImageConfig[profile.Distro].ImageSku)
		},
		"GetAgentOSImageVersion": func(profile *api.AgentPoolProfile) string {
			cloudSpecConfig := getCloudSpecConfig(cs.Location)
			return fmt.Sprintf("\"%s\"", cloudSpecConfig.OSImageConfig[profile.Distro].ImageVersion)
		},
		"UseAgentCustomImage": func(profile *api.AgentPoolProfile) bool {
			imageRef := profile.ImageRef
			return imageRef != nil && len(imageRef.Name) > 0 && len(imageRef.ResourceGroup) > 0
		},
		"UseMasterCustomImage": func() bool {
			imageRef := cs.Properties.MasterProfile.ImageRef
			return imageRef != nil && len(imageRef.Name) > 0 && len(imageRef.ResourceGroup) > 0
		},
		"GetMasterEtcdServerPort": func() int {
			return DefaultMasterEtcdServerPort
		},
		"GetMasterEtcdClientPort": func() int {
			return DefaultMasterEtcdClientPort
		},
		"PopulateClassicModeDefaultValue": func(attr string) string {
			var val string
			if !t.ClassicMode {
				val = ""
			} else {
				switch attr {
				case "cloudProviderBackoff":
					val = strconv.FormatBool(cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoff)
				case "cloudProviderBackoffRetries":
					val = strconv.Itoa(cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoffRetries)
				case "cloudProviderBackoffExponent":
					val = strconv.FormatFloat(cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoffExponent, 'f', -1, 64)
				case "cloudProviderBackoffDuration":
					val = strconv.Itoa(cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoffDuration)
				case "cloudProviderBackoffJitter":
					val = strconv.FormatFloat(cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderBackoffJitter, 'f', -1, 64)
				case "cloudProviderRatelimit":
					val = strconv.FormatBool(cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimit)
				case "cloudProviderRatelimitQPS":
					val = strconv.FormatFloat(cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitQPS, 'f', -1, 64)
				case "cloudProviderRatelimitBucket":
					val = strconv.Itoa(cs.Properties.OrchestratorProfile.KubernetesConfig.CloudProviderRateLimitBucket)
				case "caPrivateKey":
					// The base64 encoded "NotAvailable"
					val = "Tm90QXZhaWxhYmxlCg=="
				case "dockerBridgeCidr":
					val = DefaultDockerBridgeSubnet
				case "gchighthreshold":
					val = strconv.Itoa(cs.Properties.OrchestratorProfile.KubernetesConfig.GCHighThreshold)
				case "gclowthreshold":
					val = strconv.Itoa(cs.Properties.OrchestratorProfile.KubernetesConfig.GCLowThreshold)
				case "generatorCode":
					val = DefaultGeneratorCode
				case "orchestratorName":
					val = DefaultOrchestratorName
				default:
					val = ""
				}
			}
			return fmt.Sprintf("\"defaultValue\": \"%s\",", val)
		},
		"AdminGroupID": func() bool {
			return cs.Properties.AADProfile != nil && cs.Properties.AADProfile.AdminGroupID != ""
		},
		// inspired by http://stackoverflow.com/questions/18276173/calling-a-template-with-several-pipeline-parameters/18276968#18276968
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, errors.New("invalid dict call")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, errors.New("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"loop": func(min, max int) []int {
			var s []int
			for i := min; i <= max; i++ {
				s = append(s, i)
			}
			return s
		},
		"subtract": func(a, b int) int {
			return a - b
		},
		"IsCustomVNET": func() bool {
			return isCustomVNET(cs.Properties.AgentPoolProfiles)
		},
	}
}
