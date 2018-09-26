package acsengine

const (
	// DefaultFirstConsecutiveStaticIP specifies the static IP address on master 0 for DCOS or Swarm
	DefaultFirstConsecutiveStaticIP = "172.16.0.5"
	// DefaultDCOSMasterSubnet specifies the default master subnet for a DCOS cluster
	DefaultDCOSMasterSubnet = "192.168.255.0/24"
	// DefaultDCOSFirstConsecutiveStaticIP  specifies the static IP address on master 0 for a DCOS cluster
	DefaultDCOSFirstConsecutiveStaticIP = "192.168.255.5"
	// DefaultDCOSBootstrapStaticIP specifies the static IP address on bootstrap for a DCOS cluster
	DefaultDCOSBootstrapStaticIP = "192.168.255.240"
	// DefaultDCOSWindowsBootstrapStaticIP specifies the static IP address on Windows bootstrap for a DCOS cluster
	DefaultDCOSWindowsBootstrapStaticIP = "192.168.255.241"
	// DefaultDockerBridgeSubnet specifies the default subnet for the docker bridge network for masters and agents.
	DefaultDockerBridgeSubnet = "172.17.0.1/16"
	// DefaultAgentSubnetTemplate specifies a default agent subnet
	DefaultAgentSubnetTemplate = "10.%d.0.0/16"
	// DefaultContainerRuntime is docker
	DefaultContainerRuntime = "docker"
	// DefaultGeneratorCode specifies the source generator of the cluster template.
	DefaultGeneratorCode = "dcos-engine"
	// DefaultOrchestratorName specifies the 3 character orchestrator code of the cluster template and affects resource naming.
	DefaultOrchestratorName = "dcos"
	// DefaultWindowsDockerVersion specifies default docker version installed on Windows nodes
	DefaultWindowsDockerVersion = "18.03.1-ee-1"
	// DefaultBootstrapVMSize specifies default bootstrap VM size
	DefaultBootstrapVMSize = "Standard_D2s_v3"
	// DefaultWindowsBootstrapVMSize specifies default Windows bootstrap VM size
	DefaultWindowsBootstrapVMSize = "Standard_D4s_v3"
)

const (
	// DCOSMaster represents the master node type
	DCOSMaster DCOSNodeType = "DCOSMaster"
	// DCOSPrivateAgent represents the private agent node type
	DCOSPrivateAgent DCOSNodeType = "DCOSPrivateAgent"
	// DCOSPublicAgent represents the public agent node type
	DCOSPublicAgent DCOSNodeType = "DCOSPublicAgent"
)

const (
	//DefaultExtensionsRootURL  Root URL for extensions
	DefaultExtensionsRootURL = "https://raw.githubusercontent.com/Azure/dcos-engine/master/"
	// DefaultDockerEngineRepo for grabbing docker engine packages
	DefaultDockerEngineRepo = "https://download.docker.com/linux/ubuntu"
	// DefaultDockerComposeURL for grabbing docker images
	DefaultDockerComposeURL = "https://github.com/docker/compose/releases/download"

	//AzureEdgeDCOSBootstrapDownloadURL is the azure edge CDN download url
	AzureEdgeDCOSBootstrapDownloadURL = "https://dcosio.azureedge.net/dcos/%s/bootstrap/%s.bootstrap.tar.xz"
	//AzureChinaCloudDCOSBootstrapDownloadURL is the China specific DCOS package download url.
	AzureChinaCloudDCOSBootstrapDownloadURL = "https://acsengine.blob.core.chinacloudapi.cn/dcos/%s.bootstrap.tar.xz"
	//AzureEdgeDCOSWindowsBootstrapDownloadURL
)

const (
	dcosProvisionSource    = "dcos/dcosprovisionsource.sh"
	dcosProvision          = "dcos/dcosprovision.sh"
	dcosBootstrapProvision = "dcos/bootstrapprovision.sh"
	dcosBootstrapConfig111 = "dcos/dcos1.11.bootstrap-config.yaml"
	dcosCustomData111      = "dcos/dcos1.11.customdata.t"

	dcosBootstrapWindowsProvision = "dcos/bootstrapWindowsProvision.ps1"
	dcosBootstrapWindowsConfig111 = "dcos/dcos1.11.bootstrapwin-config.yaml"
	dcosWindowsProvision          = "dcos/dcosWindowsProvision.ps1"
)

const (
	agentOutputs                  = "agentoutputs.t"
	agentParams                   = "agentparams.t"
	classicParams                 = "classicparams.t"
	dcosAgentResourcesVMAS        = "dcos/dcosagentresourcesvmas.t"
	dcosWindowsAgentResourcesVMAS = "dcos/dcosWindowsAgentResourcesVmas.t"
	dcosAgentResourcesVMSS        = "dcos/dcosagentresourcesvmss.t"
	dcosWindowsAgentResourcesVMSS = "dcos/dcosWindowsAgentResourcesVmss.t"
	dcosAgentVars                 = "dcos/dcosagentvars.t"
	dcosParams                    = "dcos/dcosparams.t"
	dcosBaseFile                  = "dcos/dcosbase.t"
	dcosBootstrapVars             = "dcos/bootstrapvars.t"
	dcosBootstrapParams           = "dcos/bootstrapparams.t"
	dcosBootstrapResources        = "dcos/bootstrapresources.t"
	dcosBootstrapWinResources     = "dcos/bootstrapwinresources.t"
	dcosBootstrapCustomdata       = "dcos/bootstrapcustomdata.yml"
	dcosMasterVars                = "dcos/dcosmastervars.t"
	dcosMasterResources           = "dcos/dcosmasterresources.t"
	iaasOutputs                   = "iaasoutputs.t"
	masterOutputs                 = "masteroutputs.t"
	masterParams                  = "masterparams.t"
	windowsParams                 = "windowsparams.t"
)

const (
	azurePublicCloud       = "AzurePublicCloud"
	azureChinaCloud        = "AzureChinaCloud"
	azureGermanCloud       = "AzureGermanCloud"
	azureUSGovernmentCloud = "AzureUSGovernmentCloud"
)
