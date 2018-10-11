package acsengine

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	//log "github.com/sirupsen/logrus"
	"github.com/Azure/dcos-engine/pkg/api"
	"github.com/Azure/dcos-engine/pkg/api/common"
	"github.com/ghodss/yaml"
)

var commonTemplateFiles = []string{agentOutputs, agentParams, classicParams, masterOutputs, iaasOutputs, masterParams, windowsParams}
var dcosTemplateFiles = []string{dcosBaseFile, dcosAgentResourcesVMAS, dcosAgentResourcesVMSS, dcosAgentVars, dcosMasterResources, dcosBootstrapResources, dcosBootstrapWinResources, dcosMasterVars, dcosParams, dcosWindowsAgentResourcesVMAS, dcosWindowsAgentResourcesVMSS, dcosBootstrapVars, dcosBootstrapParams}

var keyvaultSecretPathRe *regexp.Regexp

func init() {
	keyvaultSecretPathRe = regexp.MustCompile(`^(/subscriptions/\S+/resourceGroups/\S+/providers/Microsoft.KeyVault/vaults/\S+)/secrets/([^/\s]+)(/(\S+))?$`)
}

// GenerateClusterID creates a unique 8 string cluster ID
func GenerateClusterID(properties *api.Properties) string {
	uniqueNameSuffixSize := 8
	// the name suffix uniquely identifies the cluster and is generated off a hash
	// from the master dns name
	h := fnv.New64a()
	if properties.MasterProfile != nil {
		h.Write([]byte(properties.MasterProfile.DNSPrefix))
	} else {
		h.Write([]byte(properties.AgentPoolProfiles[0].Name))
	}
	rand.Seed(int64(h.Sum64()))
	return fmt.Sprintf("%08d", rand.Uint32())[:uniqueNameSuffixSize]
}

// formatAzureProdFQDNs constructs all possible Azure prod fqdn
func formatAzureProdFQDNs(fqdnPrefix string) []string {
	var fqdns []string
	for _, location := range AzureLocations {
		fqdns = append(fqdns, FormatAzureProdFQDN(fqdnPrefix, location))
	}
	return fqdns
}

// FormatAzureProdFQDN constructs an Azure prod fqdn
func FormatAzureProdFQDN(fqdnPrefix string, location string) string {
	var FQDNFormat string
	switch getCloudTargetEnv(location) {
	case azureChinaCloud:
		FQDNFormat = AzureChinaCloudSpec.EndpointConfig.ResourceManagerVMDNSSuffix
	case azureGermanCloud:
		FQDNFormat = AzureGermanCloudSpec.EndpointConfig.ResourceManagerVMDNSSuffix
	case azureUSGovernmentCloud:
		FQDNFormat = AzureUSGovernmentCloud.EndpointConfig.ResourceManagerVMDNSSuffix
	default:
		FQDNFormat = AzureCloudSpec.EndpointConfig.ResourceManagerVMDNSSuffix
	}
	return fmt.Sprintf("%s.%s."+FQDNFormat, fqdnPrefix, location)
}

//getCloudSpecConfig returns the container images url configurations based on the deploy target environment
//for example: if the target is the public azure, then the default container image url should be k8s-gcrio.azureedge.net/...
//if the target is azure china, then the default container image should be mirror.azure.cn:5000/google_container/...
func getCloudSpecConfig(location string) AzureEnvironmentSpecConfig {
	switch getCloudTargetEnv(location) {
	case azureChinaCloud:
		return AzureChinaCloudSpec
	case azureGermanCloud:
		return AzureGermanCloudSpec
	case azureUSGovernmentCloud:
		return AzureUSGovernmentCloud
	default:
		return AzureCloudSpec
	}
}

// validateDistro checks if the requested orchestrator type is supported on the requested Linux distro.
func validateDistro(cs *api.ContainerService) bool {
	// Check Master distro
	if cs.Properties.MasterProfile != nil && cs.Properties.MasterProfile.Distro == api.RHEL {
		log.Fatalf("Orchestrator type %s not suported on RHEL Master", cs.Properties.OrchestratorProfile.OrchestratorType)
		return false
	}
	// Check Agent distros
	for _, agentProfile := range cs.Properties.AgentPoolProfiles {
		if agentProfile.Distro == api.RHEL {
			log.Fatalf("Orchestrator type %s not suported on RHEL Agent", cs.Properties.OrchestratorProfile.OrchestratorType)
			return false
		}
	}
	return true
}

// getCloudTargetEnv determines and returns whether the region is a sovereign cloud which
// have their own data compliance regulations (China/Germany/USGov) or standard
//  Azure public cloud
func getCloudTargetEnv(location string) string {
	loc := strings.ToLower(strings.Join(strings.Fields(location), ""))
	switch {
	case loc == "chinaeast" || loc == "chinanorth" || loc == "chinaeast2" || loc == "chinanorth2":
		return azureChinaCloud
	case loc == "germanynortheast" || loc == "germanycentral":
		return azureGermanCloud
	case strings.HasPrefix(loc, "usgov") || strings.HasPrefix(loc, "usdod"):
		return azureUSGovernmentCloud
	default:
		return azurePublicCloud
	}
}

func generateIPList(count int, firstAddr string) []string {
	ipaddr := net.ParseIP(firstAddr).To4()
	if ipaddr == nil {
		panic(fmt.Sprintf("IPAddr '%s' is an invalid IP address", firstAddr))
	}
	ret := make([]string, count)
	for i := 0; i < count; i++ {
		ret[i] = fmt.Sprintf("%d.%d.%d.%d", ipaddr[0], ipaddr[1], ipaddr[2], ipaddr[3]+byte(i))
	}
	return ret
}

func addValue(m paramsMap, k string, v interface{}) {
	m[k] = paramsMap{
		"value": v,
	}
}

func addKeyvaultReference(m paramsMap, k string, vaultID, secretName, secretVersion string) {
	m[k] = paramsMap{
		"reference": &KeyVaultRef{
			KeyVault: KeyVaultID{
				ID: vaultID,
			},
			SecretName:    secretName,
			SecretVersion: secretVersion,
		},
	}
}

func addSecret(m paramsMap, k string, v interface{}, encode bool) {
	str, ok := v.(string)
	if !ok {
		addValue(m, k, v)
		return
	}
	parts := keyvaultSecretPathRe.FindStringSubmatch(str)
	if parts == nil || len(parts) != 5 {
		if encode {
			addValue(m, k, base64.StdEncoding.EncodeToString([]byte(str)))
		} else {
			addValue(m, k, str)
		}
		return
	}
	addKeyvaultReference(m, k, parts[1], parts[2], parts[4])
}

// getStorageAccountType returns the support managed disk storage tier for a give VM size
func getStorageAccountType(sizeName string) (string, error) {
	spl := strings.Split(sizeName, "_")
	if len(spl) < 2 {
		return "", fmt.Errorf("Invalid sizeName: %s", sizeName)
	}
	capability := spl[1]
	if strings.Contains(strings.ToLower(capability), "s") {
		return "Premium_LRS", nil
	}
	return "Standard_LRS", nil
}

func makeExtensionScriptCommands(extension *api.Extension, extensionProfiles []*api.ExtensionProfile) string {
	if extension == nil {
		return ""
	}
	var extensionProfile *api.ExtensionProfile
	for _, eP := range extensionProfiles {
		if strings.EqualFold(eP.Name, extension.Name) {
			extensionProfile = eP
			break
		}
	}

	if extensionProfile == nil {
		panic(fmt.Sprintf("%s extension referenced was not found in the extension profile", extension.Name))
	}

	extensionsParameterReference := fmt.Sprintf("parameters('%sParameters')", extensionProfile.Name)
	scriptURL := getExtensionURL(extensionProfile.RootURL, extensionProfile.Name, extensionProfile.Version, extensionProfile.Script, extensionProfile.URLQuery)
	scriptFilePath := fmt.Sprintf("/opt/azure/containers/extensions/%s/%s", extensionProfile.Name, extensionProfile.Script)
	return fmt.Sprintf("- /usr/bin/curl --retry 5 --retry-delay 10 --retry-max-time 30 -o %s --create-dirs \"%s\" \n- /bin/chmod 744 %s \n- %s ',%s,' > /var/log/%s-output.log",
		scriptFilePath, scriptURL, scriptFilePath, scriptFilePath, extensionsParameterReference, extensionProfile.Name)
}

func getWindowsExtensionScriptData(extension *api.Extension, extensionProfiles []*api.ExtensionProfile) (string, string, string) {
	if extension == nil {
		return "", "", ""
	}
	var extensionProfile *api.ExtensionProfile
	for _, eP := range extensionProfiles {
		if strings.EqualFold(eP.Name, extension.Name) {
			extensionProfile = eP
			break
		}
	}

	if extensionProfile == nil {
		panic(fmt.Sprintf("%s extension referenced was not found in the extension profile", extension.Name))
	}

	scriptURL := getExtensionURL(extensionProfile.RootURL, extensionProfile.Name, extensionProfile.Version, extensionProfile.Script, extensionProfile.URLQuery)
	scriptFileDir := fmt.Sprintf("$env:SystemDrive:/AzureData/extensions/%s", extensionProfile.Name)
	scriptFilePath := fmt.Sprintf("%s/%s", scriptFileDir, extensionProfile.Script)
	scriptURL = "\"" + scriptURL + "\""
	scriptFileDir = "\"" + scriptFileDir + "\""
	scriptFilePath = "\"" + scriptFilePath + "\""
	return scriptURL, scriptFileDir, scriptFilePath
}

func getDCOSWindowsAgentPreprovisionParameters(cs *api.ContainerService, profile *api.AgentPoolProfile) string {
	extension := profile.PreprovisionExtension

	var extensionProfile *api.ExtensionProfile

	for _, eP := range cs.Properties.ExtensionProfiles {
		if strings.EqualFold(eP.Name, extension.Name) {
			extensionProfile = eP
			break
		}
	}
	if extensionProfile == nil {
		panic(fmt.Sprintf("%s extension referenced was not found in the extension profile", extension.Name))
	}
	parms := extensionProfile.ExtensionParameters
	return parms
}

// GetDCOSDefaultBootstrapInstallerURL returns default DCOS Bootstrap installer URL
func GetDCOSDefaultBootstrapInstallerURL(orchestratorVersion string) string {
	switch orchestratorVersion {
	case common.DCOSVersion1Dot12Dot0:
		return "https://dcos-mirror.azureedge.net/dcos/1-12-0/dcos_generate_config.sh"
	case common.DCOSVersion1Dot11Dot6:
		return "https://dcos-mirror.azureedge.net/dcos/1-11-6/dcos_generate_config.sh"
	case common.DCOSVersion1Dot11Dot5:
		return "https://dcos-mirror.azureedge.net/dcos/1-11-5/dcos_generate_config.sh"
	case common.DCOSVersion1Dot11Dot4:
		return "https://dcos-mirror.azureedge.net/dcos/1-11-4/dcos_generate_config.sh"
	case common.DCOSVersion1Dot11Dot2:
		return "https://dcos-mirror.azureedge.net/dcos/1-11-2/dcos_generate_config.sh"
	default:
		return ""
	}
}

// GetDCOSDefaultWindowsBootstrapInstallerURL returns default DCOS Windows Bootstrap installer URL
func GetDCOSDefaultWindowsBootstrapInstallerURL(orchestratorVersion string) string {
	switch orchestratorVersion {
	case common.DCOSVersion1Dot12Dot0:
		return "https://dcos-mirror.azureedge.net/dcos/1-12-0/dcos_generate_config.windows.tar.xz"
	case common.DCOSVersion1Dot11Dot2, common.DCOSVersion1Dot11Dot4, common.DCOSVersion1Dot11Dot5, common.DCOSVersion1Dot11Dot6:
		return "https://dcos-mirror.azureedge.net/dcos/1-11-6/dcos_generate_config.windows.tar.xz"
	default:
		return ""
	}
}

func isCustomVNET(a []*api.AgentPoolProfile) bool {
	if a != nil {
		for _, agentPoolProfile := range a {
			if !agentPoolProfile.IsCustomVNET() {
				return false
			}
		}
		return true
	}
	return false
}

func getDCOSCustomDataPublicIPStr(orchestratorType string, masterCount int) string {
	if orchestratorType == api.DCOS {
		var buf bytes.Buffer
		for i := 0; i < masterCount; i++ {
			buf.WriteString(fmt.Sprintf("reference(variables('masterVMNic')[%d]).ipConfigurations[0].properties.privateIPAddress,", i))
			if i < (masterCount - 1) {
				buf.WriteString(`'\\\", \\\"', `)
			}
		}
		return buf.String()
	}
	return ""
}

func getDCOSMasterCustomNodeLabels() string {
	// return empty string for DCOS since no attribtutes needed on master
	return ""
}

func getDCOSAgentCustomNodeLabels(profile *api.AgentPoolProfile) string {
	var buf bytes.Buffer
	var attrstring string
	buf.WriteString("")
	// always write MESOS_ATTRIBUTES because
	// the provision script will add FD/UD attributes
	// at node provisioning time
	if len(profile.OSType) > 0 {
		attrstring = fmt.Sprintf("MESOS_ATTRIBUTES=\"os:%s", profile.OSType)
	} else {
		attrstring = fmt.Sprintf("MESOS_ATTRIBUTES=\"os:%s", api.Linux)
	}

	if len(profile.Ports) > 0 {
		attrstring += ";public_ip:true"
	}

	buf.WriteString(attrstring)
	if len(profile.CustomNodeLabels) > 0 {
		for k, v := range profile.CustomNodeLabels {
			buf.WriteString(fmt.Sprintf(";%s:%s", k, v))
		}
	}
	buf.WriteString("\"")
	return buf.String()
}

func getDCOSWindowsAgentCustomAttributes(profile *api.AgentPoolProfile) string {
	var buf bytes.Buffer
	var attrstring string
	buf.WriteString("")
	if len(profile.OSType) > 0 {
		attrstring = fmt.Sprintf("os:%s", profile.OSType)
	} else {
		attrstring = fmt.Sprintf("os:%s", api.Windows)
	}
	if len(profile.Ports) > 0 {
		attrstring += ";public_ip:true"
	}
	buf.WriteString(attrstring)
	if len(profile.CustomNodeLabels) > 0 {
		for k, v := range profile.CustomNodeLabels {
			buf.WriteString(fmt.Sprintf(";%s:%s", k, v))
		}
	}
	return buf.String()
}

func getVNETAddressPrefixes(properties *api.Properties) string {
	visitedSubnets := make(map[string]bool)
	var buf bytes.Buffer
	buf.WriteString(`"[variables('masterSubnet')]"`)
	visitedSubnets[properties.MasterProfile.Subnet] = true
	for _, profile := range properties.AgentPoolProfiles {
		if _, ok := visitedSubnets[profile.Subnet]; !ok {
			buf.WriteString(fmt.Sprintf(",\n            \"[variables('%sSubnet')]\"", profile.Name))
		}
	}
	return buf.String()
}

func getVNETSubnetDependencies(properties *api.Properties) string {
	agentString := `        "[concat('Microsoft.Network/networkSecurityGroups/', variables('%sNSGName'))]"`
	var buf bytes.Buffer
	for index, agentProfile := range properties.AgentPoolProfiles {
		if index > 0 {
			buf.WriteString(",\n")
		}
		buf.WriteString(fmt.Sprintf(agentString, agentProfile.Name))
	}
	return buf.String()
}

func getVNETSubnets(properties *api.Properties, addNSG bool) string {
	masterString := `{
            "name": "[variables('masterSubnetName')]",
            "properties": {
              "addressPrefix": "[variables('masterSubnet')]"
            }
          }`
	agentString := `          {
            "name": "[variables('%sSubnetName')]",
            "properties": {
              "addressPrefix": "[variables('%sSubnet')]"
            }
          }`
	agentStringNSG := `          {
            "name": "[variables('%sSubnetName')]",
            "properties": {
              "addressPrefix": "[variables('%sSubnet')]",
              "networkSecurityGroup": {
                "id": "[resourceId('Microsoft.Network/networkSecurityGroups', variables('%sNSGName'))]"
              }
            }
          }`
	var buf bytes.Buffer
	buf.WriteString(masterString)
	for _, agentProfile := range properties.AgentPoolProfiles {
		buf.WriteString(",\n")
		if addNSG {
			buf.WriteString(fmt.Sprintf(agentStringNSG, agentProfile.Name, agentProfile.Name, agentProfile.Name))
		} else {
			buf.WriteString(fmt.Sprintf(agentString, agentProfile.Name, agentProfile.Name))
		}

	}
	return buf.String()
}

func getLBRule(name string, port int) string {
	return fmt.Sprintf(`	          {
            "name": "LBRule%d",
            "properties": {
              "backendAddressPool": {
                "id": "[concat(variables('%sLbID'), '/backendAddressPools/', variables('%sLbBackendPoolName'))]"
              },
              "backendPort": %d,
              "enableFloatingIP": false,
              "frontendIPConfiguration": {
                "id": "[variables('%sLbIPConfigID')]"
              },
              "frontendPort": %d,
              "idleTimeoutInMinutes": 5,
              "loadDistribution": "Default",
              "probe": {
                "id": "[concat(variables('%sLbID'),'/probes/tcp%dProbe')]"
              },
              "protocol": "tcp"
            }
          }`, port, name, name, port, name, port, name, port)
}

func getLBRules(name string, ports []int) string {
	var buf bytes.Buffer
	for index, port := range ports {
		if index > 0 {
			buf.WriteString(",\n")
		}
		buf.WriteString(getLBRule(name, port))
	}
	return buf.String()
}

func getProbe(port int) string {
	return fmt.Sprintf(`          {
            "name": "tcp%dProbe",
            "properties": {
              "intervalInSeconds": "5",
              "numberOfProbes": "2",
              "port": %d,
              "protocol": "tcp"
            }
          }`, port, port)
}

func getProbes(ports []int) string {
	var buf bytes.Buffer
	for index, port := range ports {
		if index > 0 {
			buf.WriteString(",\n")
		}
		buf.WriteString(getProbe(port))
	}
	return buf.String()
}

func getSecurityRule(port int, portIndex int) string {
	// BaseLBPriority specifies the base lb priority.
	BaseLBPriority := 200
	return fmt.Sprintf(`          {
            "name": "Allow_%d",
            "properties": {
              "access": "Allow",
              "description": "Allow traffic from the Internet to port %d",
              "destinationAddressPrefix": "*",
              "destinationPortRange": "%d",
              "direction": "Inbound",
              "priority": %d,
              "protocol": "*",
              "sourceAddressPrefix": "Internet",
              "sourcePortRange": "*"
            }
          }`, port, port, port, BaseLBPriority+portIndex)
}

func getDataDisks(a *api.AgentPoolProfile) string {
	if !a.HasDisks() {
		return ""
	}
	var buf bytes.Buffer
	buf.WriteString("\"dataDisks\": [\n")
	dataDisks := `            {
              "createOption": "Empty",
              "diskSizeGB": "%d",
              "lun": %d,
              "name": "[concat(variables('%sVMNamePrefix'), copyIndex(),'-datadisk%d')]",
              "vhd": {
                "uri": "[concat('http://',variables('storageAccountPrefixes')[mod(add(add(div(copyIndex(),variables('maxVMsPerStorageAccount')),variables('%sStorageAccountOffset')),variables('dataStorageAccountPrefixSeed')),variables('storageAccountPrefixesCount'))],variables('storageAccountPrefixes')[div(add(add(div(copyIndex(),variables('maxVMsPerStorageAccount')),variables('%sStorageAccountOffset')),variables('dataStorageAccountPrefixSeed')),variables('storageAccountPrefixesCount'))],variables('%sDataAccountName'),'.blob.core.windows.net/vhds/',variables('%sVMNamePrefix'),copyIndex(), '--datadisk%d.vhd')]"
              }
            }`
	managedDataDisks := `            {
              "diskSizeGB": "%d",
              "lun": %d,
              "createOption": "Empty"
            }`
	for i, diskSize := range a.DiskSizesGB {
		if i > 0 {
			buf.WriteString(",\n")
		}
		if a.StorageProfile == api.StorageAccount {
			buf.WriteString(fmt.Sprintf(dataDisks, diskSize, i, a.Name, i, a.Name, a.Name, a.Name, a.Name, i))
		} else if a.StorageProfile == api.ManagedDisks {
			buf.WriteString(fmt.Sprintf(managedDataDisks, diskSize, i))
		}
	}
	buf.WriteString("\n          ],")
	return buf.String()
}

func getSecurityRules(ports []int) string {
	var buf bytes.Buffer
	for index, port := range ports {
		if index > 0 {
			buf.WriteString(",\n")
		}
		buf.WriteString(getSecurityRule(port, index))
	}
	return buf.String()
}

// getSingleLineForTemplate returns the file as a single line for embedding in an arm template
func (t *TemplateGenerator) getSingleLineForTemplate(textFilename string, cs *api.ContainerService, profile interface{}) (string, error) {
	b, err := Asset(textFilename)
	if err != nil {
		return "", t.Translator.Errorf("yaml file %s does not exist", textFilename)
	}

	// use go templates to process the text filename
	templ := template.New("customdata template").Funcs(t.getTemplateFuncMap(cs))
	if _, err = templ.New(textFilename).Parse(string(b)); err != nil {
		return "", t.Translator.Errorf("error parsing file %s: %v", textFilename, err)
	}

	var buffer bytes.Buffer
	if err = templ.ExecuteTemplate(&buffer, textFilename, profile); err != nil {
		return "", t.Translator.Errorf("error executing template for file %s: %v", textFilename, err)
	}
	expandedTemplate := buffer.String()

	textStr := escapeSingleLine(string(expandedTemplate))

	return textStr, nil
}

func escapeSingleLine(escapedStr string) string {
	// template.JSEscapeString leaves undesirable chars that don't work with pretty print
	escapedStr = strings.Replace(escapedStr, "\\", "\\\\", -1)
	escapedStr = strings.Replace(escapedStr, "\r\n", "\\n", -1)
	escapedStr = strings.Replace(escapedStr, "\n", "\\n", -1)
	escapedStr = strings.Replace(escapedStr, "\"", "\\\"", -1)
	return escapedStr
}

// getBase64CustomScript will return a base64 of the CSE
func getBase64CustomScript(csFilename string) string {
	b, err := Asset(csFilename)
	if err != nil {
		// this should never happen and this is a bug
		panic(fmt.Sprintf("BUG: %s", err.Error()))
	}
	// translate the parameters
	csStr := string(b)
	csStr = strings.Replace(csStr, "\r\n", "\n", -1)
	return getBase64CustomScriptFromStr(csStr)
}

// getBase64CustomScript will return a base64 of the CSE
func getBase64CustomScriptFromStr(str string) string {
	var gzipB bytes.Buffer
	w := gzip.NewWriter(&gzipB)
	w.Write([]byte(str))
	w.Close()
	return base64.StdEncoding.EncodeToString(gzipB.Bytes())
}

// GetDCOSBootstrapConfig returns DCOS bootstrap config
func GetDCOSBootstrapConfig(cs *api.ContainerService) string {
	if cs.Properties.OrchestratorProfile.OrchestratorType != api.DCOS {
		panic(fmt.Sprintf("BUG: invalid orchestrator %s", cs.Properties.OrchestratorProfile.OrchestratorType))
	}
	var configFName string
	switch cs.Properties.OrchestratorProfile.OrchestratorVersion {
	case common.DCOSVersion1Dot11Dot2, common.DCOSVersion1Dot11Dot4, common.DCOSVersion1Dot11Dot5, common.DCOSVersion1Dot11Dot6, common.DCOSVersion1Dot12Dot0:
		configFName = dcosBootstrapConfig111
	default:
		panic(fmt.Sprintf("BUG: invalid orchestrator version %s", cs.Properties.OrchestratorProfile.OrchestratorVersion))
	}

	bp, err := Asset(configFName)
	if err != nil {
		panic(fmt.Sprintf("BUG: %s", err.Error()))
	}

	masterIPList := generateIPList(cs.Properties.MasterProfile.Count, cs.Properties.MasterProfile.FirstConsecutiveStaticIP)
	for i, v := range masterIPList {
		masterIPList[i] = "- " + v
	}

	config := string(bp)
	config = strings.Replace(config, "MASTER_IP_LIST", strings.Join(masterIPList, "\n"), -1)
	config = strings.Replace(config, "BOOTSTRAP_IP", cs.Properties.OrchestratorProfile.LinuxBootstrapProfile.StaticIP, -1)
	config = strings.Replace(config, "BOOTSTRAP_OAUTH_ENABLED", strconv.FormatBool(cs.Properties.OrchestratorProfile.OAuthEnabled), -1)
	config = strings.Replace(config, "BOOTSTRAP_LINUX_ENABLE_IPV6", strconv.FormatBool(cs.Properties.OrchestratorProfile.LinuxBootstrapProfile.EnableIPv6), -1)

	if len(cs.Properties.OrchestratorProfile.LinuxBootstrapProfile.ExtraConfigs) != 0 {
		config = combineYAML(config, cs.Properties.OrchestratorProfile.LinuxBootstrapProfile.ExtraConfigs)
	}

	return config
}

// GetDCOSWindowsBootstrapConfig returns DCOS Windows bootstrap config
func GetDCOSWindowsBootstrapConfig(cs *api.ContainerService) string {
	if cs.Properties.OrchestratorProfile.OrchestratorType != api.DCOS {
		panic(fmt.Sprintf("BUG: invalid orchestrator %s", cs.Properties.OrchestratorProfile.OrchestratorType))
	}
	var configFName string
	switch cs.Properties.OrchestratorProfile.OrchestratorVersion {
	case common.DCOSVersion1Dot11Dot2, common.DCOSVersion1Dot11Dot4, common.DCOSVersion1Dot11Dot5, common.DCOSVersion1Dot11Dot6, common.DCOSVersion1Dot12Dot0:
		configFName = dcosBootstrapWindowsConfig111
	default:
		panic(fmt.Sprintf("BUG: invalid orchestrator version %s", cs.Properties.OrchestratorProfile.OrchestratorVersion))
	}

	bp, err := Asset(configFName)
	if err != nil {
		panic(fmt.Sprintf("BUG: %s", err.Error()))
	}

	masterIPList := generateIPList(cs.Properties.MasterProfile.Count, cs.Properties.MasterProfile.FirstConsecutiveStaticIP)
	for i, v := range masterIPList {
		masterIPList[i] = "- " + v
	}

	config := string(bp)
	config = strings.Replace(config, "MASTER_IP_LIST", strings.Join(masterIPList, "\n"), -1)
	config = strings.Replace(config, "BOOTSTRAP_IP", cs.Properties.OrchestratorProfile.WindowsBootstrapProfile.StaticIP, -1)
	config = strings.Replace(config, "BOOTSTRAP_OAUTH_ENABLED", strconv.FormatBool(cs.Properties.OrchestratorProfile.OAuthEnabled), -1)
	config = strings.Replace(config, "BOOTSTRAP_WINDOWS_ENABLE_IPV6", strconv.FormatBool(cs.Properties.OrchestratorProfile.WindowsBootstrapProfile.EnableIPv6), -1)

	if len(cs.Properties.OrchestratorProfile.WindowsBootstrapProfile.ExtraConfigs) != 0 {
		config = combineYAML(config, cs.Properties.OrchestratorProfile.WindowsBootstrapProfile.ExtraConfigs)
	}

	return config
}

func combineYAML(config string, extra_config map[string]interface{}) string {
	var config_map map[string]interface{}
	if err := yaml.Unmarshal([]byte(config), &config_map); err != nil {
		panic(err)
	}

	for k, v := range extra_config {
		if value, exist := config_map[k]; exist {
			log.Fatalf("Error: %s has the value %s and cannot be set in ExtraConfigs", k, value)
		} else {
			config_map[k] = v
		}
	}

	new_config, err := yaml.Marshal(config_map)
	if err != nil {
		panic(err)
	}
	return string(new_config)
}

func getDCOSBootstrapConfig(cs *api.ContainerService) string {
	config := GetDCOSBootstrapConfig(cs)
	return strings.Replace(strings.Replace(config, "\r\n", "\n", -1), "\n", "\n\n    ", -1)
}

func getDCOSProvisionScript(script string) string {
	// add the provision script
	bp, err := Asset(script)
	if err != nil {
		panic(fmt.Sprintf("BUG: %s", err.Error()))
	}

	provisionScript := string(bp)
	if strings.Contains(provisionScript, "'") {
		panic(fmt.Sprintf("BUG: %s may not contain character '", script))
	}

	return strings.Replace(strings.Replace(provisionScript, "\r\n", "\n", -1), "\n", "\n\n    ", -1)
}

func getDCOSAgentProvisionScript(profile *api.AgentPoolProfile, orchProfile *api.OrchestratorProfile, bootstrapIP string) string {
	// add the provision script
	scriptname := dcosProvision

	bp, err := Asset(scriptname)
	if err != nil {
		panic(fmt.Sprintf("BUG: %s", err.Error()))
	}

	provisionScript := string(bp)
	if strings.Contains(provisionScript, "'") {
		panic(fmt.Sprintf("BUG: %s may not contain character '", scriptname))
	}

	// the embedded roleFileContents
	var roleFileContents string
	if len(profile.Ports) > 0 {
		// public agents
		roleFileContents = "touch /etc/mesosphere/roles/slave_public"
	} else {
		roleFileContents = "touch /etc/mesosphere/roles/slave"
	}
	provisionScript = strings.Replace(provisionScript, "ROLESFILECONTENTS", roleFileContents, -1)
	provisionScript = strings.Replace(provisionScript, "BOOTSTRAP_IP", bootstrapIP, -1)

	var b bytes.Buffer
	b.WriteString(provisionScript)
	b.WriteString("\n")

	if len(orchProfile.Registry) == 0 {
		b.WriteString("rm /etc/docker.tar.gz\n")
	}

	return strings.Replace(strings.Replace(b.String(), "\r\n", "\n", -1), "\n", "\n\n    ", -1)
}

func getDCOSMasterProvisionScript(orchProfile *api.OrchestratorProfile, bootstrapIP string) string {
	scriptname := dcosProvision

	// add the provision script
	bp, err := Asset(scriptname)
	if err != nil {
		panic(fmt.Sprintf("BUG: %s", err.Error()))
	}

	provisionScript := string(bp)
	if strings.Contains(provisionScript, "'") {
		panic(fmt.Sprintf("BUG: %s may not contain character '", scriptname))
	}

	// the embedded roleFileContents
	roleFileContents := `touch /etc/mesosphere/roles/master
touch /etc/mesosphere/roles/azure_master`
	provisionScript = strings.Replace(provisionScript, "ROLESFILECONTENTS", roleFileContents, -1)
	provisionScript = strings.Replace(provisionScript, "BOOTSTRAP_IP", bootstrapIP, -1)

	var b bytes.Buffer
	b.WriteString(provisionScript)
	b.WriteString("\n")

	return strings.Replace(strings.Replace(b.String(), "\r\n", "\n", -1), "\n", "\n\n    ", -1)
}

func getDCOSCustomDataTemplate(orchestratorType, orchestratorVersion string) string {
	switch orchestratorType {
	case api.DCOS:
		switch orchestratorVersion {
		case common.DCOSVersion1Dot11Dot2, common.DCOSVersion1Dot11Dot4, common.DCOSVersion1Dot11Dot5, common.DCOSVersion1Dot11Dot6, common.DCOSVersion1Dot12Dot0:
			return dcosCustomData111
		}
	default:
		// it is a bug to get here
		panic(fmt.Sprintf("BUG: invalid orchestrator %s", orchestratorType))
	}
	return ""
}

// getSingleLineForTemplate returns the file as a single line for embedding in an arm template
func getSingleLineDCOSCustomData(orchestratorType, yamlFilename string, masterCount int, replaceMap map[string]string) string {
	b, err := Asset(yamlFilename)
	if err != nil {
		panic(fmt.Sprintf("BUG getting yaml custom data file: %s", err.Error()))
	}
	yamlStr := string(b)
	for k, v := range replaceMap {
		yamlStr = strings.Replace(yamlStr, k, v, -1)
	}

	// convert to json
	jsonBytes, err4 := yaml.YAMLToJSON([]byte(yamlStr))
	if err4 != nil {
		panic(fmt.Sprintf("BUG: %s", err4.Error()))
	}
	yamlStr = string(jsonBytes)

	// convert to one line
	yamlStr = strings.Replace(yamlStr, "\\", "\\\\", -1)
	yamlStr = strings.Replace(yamlStr, "\r\n", "\\n", -1)
	yamlStr = strings.Replace(yamlStr, "\n", "\\n", -1)
	yamlStr = strings.Replace(yamlStr, "\"", "\\\"", -1)

	// variable replacement
	rVariable, e1 := regexp.Compile("{{{([^}]*)}}}")
	if e1 != nil {
		panic(fmt.Sprintf("BUG: %s", e1.Error()))
	}
	yamlStr = rVariable.ReplaceAllString(yamlStr, "',variables('$1'),'")

	// replace the internal values
	publicIPStr := getDCOSCustomDataPublicIPStr(orchestratorType, masterCount)
	yamlStr = strings.Replace(yamlStr, "DCOSCUSTOMDATAPUBLICIPSTR", publicIPStr, -1)

	return yamlStr
}

func buildYamlFileWithWriteFiles(files []string) string {
	clusterYamlFile := `#cloud-config

write_files:
%s
`
	writeFileBlock := ` -  encoding: gzip
    content: !!binary |
        %s
    path: /opt/azure/containers/%s
    permissions: "0744"
`

	filelines := ""
	for _, file := range files {
		b64GzipString := getBase64CustomScript(file)
		fileNoPath := strings.TrimPrefix(file, "swarm/")
		filelines = filelines + fmt.Sprintf(writeFileBlock, b64GzipString, fileNoPath)
	}
	return fmt.Sprintf(clusterYamlFile, filelines)
}

// Identifies Master distro to use for master parameters
func getMasterDistro(m *api.MasterProfile) api.Distro {
	// Use Ubuntu distro if MasterProfile is not defined (e.g. agents-only)
	if m == nil {
		return api.Ubuntu
	}

	// MasterProfile.Distro configured by defaults#setMasterNetworkDefaults
	return m.Distro
}

// getLinkedTemplatesForExtensions returns the
// Microsoft.Resources/deployments for each extension
//func getLinkedTemplatesForExtensions(properties api.Properties) string {
func getLinkedTemplatesForExtensions(properties *api.Properties) string {
	var result string

	extensions := properties.ExtensionProfiles
	masterProfileExtensions := properties.MasterProfile.Extensions
	orchestratorType := properties.OrchestratorProfile.OrchestratorType

	for err, extensionProfile := range extensions {
		_ = err

		masterOptedForExtension, singleOrAll := validateProfileOptedForExtension(extensionProfile.Name, masterProfileExtensions)
		if masterOptedForExtension {
			result += ","
			dta, e := getMasterLinkedTemplateText(properties.MasterProfile, orchestratorType, extensionProfile, singleOrAll)
			if e != nil {
				fmt.Println(e.Error())
				return ""
			}
			result += dta
		}

		for _, agentPoolProfile := range properties.AgentPoolProfiles {
			poolProfileExtensions := agentPoolProfile.Extensions
			poolOptedForExtension, singleOrAll := validateProfileOptedForExtension(extensionProfile.Name, poolProfileExtensions)
			if poolOptedForExtension {
				result += ","
				dta, e := getAgentPoolLinkedTemplateText(agentPoolProfile, orchestratorType, extensionProfile, singleOrAll)
				if e != nil {
					fmt.Println(e.Error())
					return ""
				}
				result += dta
			}

		}
	}

	return result
}

func getMasterLinkedTemplateText(masterProfile *api.MasterProfile, orchestratorType string, extensionProfile *api.ExtensionProfile, singleOrAll string) (string, error) {
	extTargetVMNamePrefix := "variables('masterVMNamePrefix')"

	loopCount := "[variables('masterCount')]"
	loopOffset := ""

	if strings.EqualFold(singleOrAll, "single") {
		loopCount = "1"
	}
	return internalGetPoolLinkedTemplateText(extTargetVMNamePrefix, orchestratorType, loopCount,
		loopOffset, extensionProfile)
}

func getAgentPoolLinkedTemplateText(agentPoolProfile *api.AgentPoolProfile, orchestratorType string, extensionProfile *api.ExtensionProfile, singleOrAll string) (string, error) {
	extTargetVMNamePrefix := fmt.Sprintf("variables('%sVMNamePrefix')", agentPoolProfile.Name)
	loopCount := fmt.Sprintf("[variables('%sCount'))]", agentPoolProfile.Name)
	loopOffset := ""

	// Availability sets can have an offset since we don't redeploy vms.
	// So we don't want to rerun these extensions in scale up scenarios.
	if agentPoolProfile.IsAvailabilitySets() {
		loopCount = fmt.Sprintf("[sub(variables('%sCount'), variables('%sOffset'))]",
			agentPoolProfile.Name, agentPoolProfile.Name)
		loopOffset = fmt.Sprintf("variables('%sOffset')", agentPoolProfile.Name)
	}

	if strings.EqualFold(singleOrAll, "single") {
		loopCount = "1"
	}

	return internalGetPoolLinkedTemplateText(extTargetVMNamePrefix, orchestratorType, loopCount,
		loopOffset, extensionProfile)
}

func internalGetPoolLinkedTemplateText(extTargetVMNamePrefix, orchestratorType, loopCount, loopOffset string, extensionProfile *api.ExtensionProfile) (string, error) {
	dta, e := getLinkedTemplateTextForURL(extensionProfile.RootURL, orchestratorType, extensionProfile.Name, extensionProfile.Version, extensionProfile.URLQuery)
	if e != nil {
		return "", e
	}
	if strings.Contains(extTargetVMNamePrefix, "master") {
		dta = strings.Replace(dta, "EXTENSION_TARGET_VM_TYPE", "master", -1)
	} else {
		dta = strings.Replace(dta, "EXTENSION_TARGET_VM_TYPE", "agent", -1)
	}
	extensionsParameterReference := fmt.Sprintf("[parameters('%sParameters')]", extensionProfile.Name)
	dta = strings.Replace(dta, "EXTENSION_PARAMETERS_REPLACE", extensionsParameterReference, -1)
	dta = strings.Replace(dta, "EXTENSION_URL_REPLACE", extensionProfile.RootURL, -1)
	dta = strings.Replace(dta, "EXTENSION_TARGET_VM_NAME_PREFIX", extTargetVMNamePrefix, -1)
	if _, err := strconv.Atoi(loopCount); err == nil {
		dta = strings.Replace(dta, "\"EXTENSION_LOOP_COUNT\"", loopCount, -1)
	} else {
		dta = strings.Replace(dta, "EXTENSION_LOOP_COUNT", loopCount, -1)
	}

	dta = strings.Replace(dta, "EXTENSION_LOOP_OFFSET", loopOffset, -1)
	return dta, nil
}

func validateProfileOptedForExtension(extensionName string, profileExtensions []api.Extension) (bool, string) {
	for _, extension := range profileExtensions {
		if extensionName == extension.Name {
			return true, extension.SingleOrAll
		}
	}
	return false, ""
}

// getLinkedTemplateTextForURL returns the string data from
// template-link.json in the following directory:
// extensionsRootURL/extensions/extensionName/version
// It returns an error if the extension cannot be found
// or loaded.  getLinkedTemplateTextForURL provides the ability
// to pass a root extensions url for testing
func getLinkedTemplateTextForURL(rootURL, orchestrator, extensionName, version, query string) (string, error) {
	supportsExtension, err := orchestratorSupportsExtension(rootURL, orchestrator, extensionName, version, query)
	if !supportsExtension {
		return "", fmt.Errorf("Extension not supported for orchestrator. Error: %s", err)
	}

	templateLinkBytes, err := getExtensionResource(rootURL, extensionName, version, "template-link.json", query)
	if err != nil {
		return "", err
	}

	return string(templateLinkBytes), nil
}

func orchestratorSupportsExtension(rootURL, orchestrator, extensionName, version, query string) (bool, error) {
	orchestratorBytes, err := getExtensionResource(rootURL, extensionName, version, "supported-orchestrators.json", query)
	if err != nil {
		return false, err
	}

	var supportedOrchestrators []string
	err = json.Unmarshal(orchestratorBytes, &supportedOrchestrators)
	if err != nil {
		return false, fmt.Errorf("Unable to parse supported-orchestrators.json for Extension %s Version %s", extensionName, version)
	}

	if !stringInSlice(orchestrator, supportedOrchestrators) {
		return false, fmt.Errorf("Orchestrator: %s not in list of supported orchestrators for Extension: %s Version %s", orchestrator, extensionName, version)
	}

	return true, nil
}

func getExtensionResource(rootURL, extensionName, version, fileName, query string) ([]byte, error) {
	requestURL := getExtensionURL(rootURL, extensionName, version, fileName, query)

	res, err := http.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf("Unable to GET extension resource for extension: %s with version %s with filename %s at URL: %s Error: %s", extensionName, version, fileName, requestURL, err)
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("Unable to GET extension resource for extension: %s with version %s with filename %s at URL: %s StatusCode: %s: Status: %s", extensionName, version, fileName, requestURL, strconv.Itoa(res.StatusCode), res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("Unable to GET extension resource for extension: %s with version %s  with filename %s at URL: %s Error: %s", extensionName, version, fileName, requestURL, err)
	}

	return body, nil
}

func getExtensionURL(rootURL, extensionName, version, fileName, query string) string {
	extensionsDir := "extensions"
	if !strings.HasSuffix(rootURL, "/") {
		rootURL = rootURL + "/"
	}
	url := rootURL + extensionsDir + "/" + extensionName + "/" + version + "/" + fileName
	if query != "" {
		url += "?" + query
	}
	return url
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
