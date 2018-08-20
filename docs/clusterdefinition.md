# Microsoft DC/OS Engine - Cluster Definition

## Cluster Defintions for apiVersion "vlabs"

Here are the cluster definitions for apiVersion "vlabs":

### apiVersion

|Name|Required|Description|
|---|---|---|
|apiVersion|yes|The version of the template.  For "vlabs" the value is "vlabs"|

### orchestratorProfile
`orchestratorProfile` describes the orchestrator settings.

|Name|Required|Description|
|---|---|---|
|orchestratorType|yes|Specifies the orchestrator type for the cluster|

Here are the valid values for the orchestrator types:

1. `DCOS` - this represents the [DC/OS orchestrator](dcos.md).


### masterProfile
`masterProfile` describes the settings for master configuration.

|Name|Required|Description|
|---|---|---|
|count|yes|Masters have count value of 1, 3, or 5 masters|
|dnsPrefix|yes|The dns prefix for the master FQDN.  The master FQDN is used for SSH or commandline access. This must be a unique name. ([bring your own VNET examples](../examples/vnet))|
|subjectAltNames|no|An array of fully qualified domain names using which a user can reach API server. These domains are added as Subject Alternative Names to the generated API server certificate. **NOTE**: These domains **will not** be automatically provisioned.|
|firstConsecutiveStaticIP|only required when vnetSubnetId specified|The IP address of the first master.  IP Addresses will be assigned consecutively to additional master nodes|
|vmsize|yes|Describes a valid [Azure VM Sizes](https://azure.microsoft.com/en-us/documentation/articles/virtual-machines-windows-sizes/). These are restricted to machines with at least 2 cores and 100GB of ephemeral disk space|
|osDiskSizeGB|no|Describes the OS Disk Size in GB|
|vnetSubnetId|no|Specifies the Id of an alternate VNET subnet.  The subnet id must specify a valid VNET ID owned by the same subscription. ([bring your own VNET examples](../examples/vnet))|
|extensions|no|This is an array of extensions. This indicates that the extension be run on a single master.  The name in the extensions array must exactly match the extension name in the extensionProfiles|
|vnetCidr|no|Specifies the VNET cidr when using a custom VNET ([bring your own VNET examples](../examples/vnet))|
|imageReference.name|no|The name of the Linux OS image. Needs to be used in conjunction with resourceGroup, below|
|imageReference.resourceGroup|no|Resource group that contains the Linux OS image. Needs to be used in conjunction with name, above|
|distro|no|Select Master(s) Operating System (Linux only). Currently supported value is: `ubuntu`|
|customFiles|no|The custom files to be provisioned to the master nodes. Defined as an array of json objects with each defined as `"source":"absolute-local-path", "dest":"absolute-path-on-masternodes"`.[See examples](../examples/customfiles) |

### agentPoolProfiles
A cluster can have 0 to 12 agent pool profiles. Agent Pool Profiles are used for creating agents with different capabilities such as VMSizes, VMSS or Availability Set, Public/Private access, user-defined OS Images, [attached storage disks](../examples/disks-storageaccount), [attached managed disks](../examples/disks-managed), or [Windows](../examples/windows).

|Name|Required|Description|
|---|---|---|
|availabilityProfile|no|Supported values are `VirtualMachineScaleSets` (default) and `AvailabilitySet`.|
|count|yes|Describes the node count|
|scaleSetPriority|no|Supported values are `Regular` (default) and `Low`. Only applies to clusters with availabilityProfile `VirtualMachineScaleSets`. Enables the usage of [Low-priority VMs on Scale Sets](https://docs.microsoft.com/en-us/azure/virtual-machine-scale-sets/virtual-machine-scale-sets-use-low-priority).|
|scaleSetEvictionPolicy|no|Supported values are `Delete` (default) and `Deallocate`. Only applies to clusters with availabilityProfile of `VirtualMachineScaleSets` and scaleSetPriority of `Low`.|
|diskSizesGB|no|Describes an array of up to 4 attached disk sizes.  Valid disk size values are between 1 and 1024|
|dnsPrefix|Required if agents are to be exposed publically with a load balancer|The dns prefix that forms the FQDN to access the loadbalancer for this agent pool. This must be a unique name among all agent pools.|
|name|yes|This is the unique name for the agent pool profile. The resources of the agent pool profile are derived from this name|
|ports|only required if needed for exposing services publically|Describes an array of ports need for exposing publically.  A tcp probe is configured for each port and only opens to an agent node if the agent node is listening on that port.  A maximum of 150 ports may be specified.|
|storageProfile|no|Specifies the storage profile to use.  Valid values are [ManagedDisks](../examples/disks-managed) or [StorageAccount](../examples/disks-storageaccount). Defaults to `ManagedDisks`|
|vmsize|yes|Describes a valid [Azure VM Sizes](https://azure.microsoft.com/en-us/documentation/articles/virtual-machines-windows-sizes/).  These are restricted to machines with at least 2 cores|
|osDiskSizeGB|no|Describes the OS Disk Size in GB|
|vnetSubnetId|no|Specifies the Id of an alternate VNET subnet.  The subnet id must specify a valid VNET ID owned by the same subscription. ([bring your own VNET examples](../examples/vnet))|
|imageReference.name|no|The name of a a Linux OS image. Needs to be used in conjunction with resourceGroup, below|
|imageReference.resourceGroup|no|Resource group that contains the Linux OS image. Needs to be used in conjunction with name, above|
|osType|no|Specifies the agent pool's Operating System. Supported values are `Windows` and `Linux`. Defaults to `Linux`|
|distro|no|Specifies the agent pool's Linux distribution. Supported value is `ubuntu`|
|acceleratedNetworkingEnabled|no|Use [Azure Accelerated Networking](https://azure.microsoft.com/en-us/blog/maximize-your-vm-s-performance-with-accelerated-networking-now-generally-available-for-both-windows-and-linux/) feature for agents (You must select a VM SKU that support Accelerated Networking)|

### linuxProfile

`linuxProfile` provides the linux configuration for each linux node in the cluster

|Name|Required|Description|
|---|---|---|
|adminUsername|yes|Describes the username to be used on all linux clusters|
|ssh.publicKeys.keyData|yes|The public SSH key used for authenticating access to all Linux nodes in the cluster.  Here are instructions for [generating a public/private key pair](ssh.md#ssh-key-generation)|
|secrets|no|Specifies an array of key vaults to pull secrets from and what secrets to pull from each|
|customSearchDomain.name|no|describes the search domain to be used on all linux clusters|
|customSearchDomain.realmUser|no|describes the realm user with permissions to update dns registries on Windows Server DNS|
|customSearchDomain.realmPassword|no|describes the realm user password to update dns registries on Windows Server DNS|
|customNodesDNS.dnsServer|no|describes the IP address of the DNS Server|

#### secrets
`secrets` details which certificates to install on the masters and nodes in the cluster.

A cluster can have a list of key vaults to install certs from.

On linux boxes the certs are saved on under the directory "/var/lib/waagent/". 2 files are saved per certificate:

1. `{thumbprint}.crt` : this is the full cert chain saved in PEM format
2. `{thumbprint}.prv` : this is the private key saved in PEM format

|Name|Required|Description|
|---|---|---|
|sourceVault.id|yes|The azure resource manager id of the key vault to pull secrets from|
|vaultCertificates.certificateUrl|yes|Keyvault URL to this cert including the version|
format for `sourceVault.id`, can be obtained in cli, or found in the portal: /subscriptions/{subscription-id}/resourceGroups/{resource-group}/providers/Microsoft.KeyVault/vaults/{keyvaultname}

format for `vaultCertificates.certificateUrl`, can be obtained in cli, or found in the portal:
https://{keyvaultname}.vault.azure.net:443/secrets/{secretName}/{version}
