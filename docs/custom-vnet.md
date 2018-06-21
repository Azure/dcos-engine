# Using a custom virtual network with DC/OS Engine
In this tutorial you are going to learn how to use [DC/OS Engine](https://github.com/Azure/dcos-engine) to deploy a brand new cluster into an existing or pre-created virtual network.
By doing this, you will be able to control the properties of the virtual network or integrate a new cluster into your existing infrastructure.

## Prerequisites
You can run this walkthrough on OS X, Windows, or Linux.
- You need an Azure subscription. If you don't have one, you can [sign up for an account](https://azure.microsoft.com/).
- Install the [Azure CLI 2.0](/cli/azure/install-az-cli2).
- Install the [DC/OS Engine](https://github.com/Azure/dcos-engine/blob/master/docs/dcos-engine.md)

## Create the virtual network

First, you need to create a new resource group
```bash
az group create -n <resource group> -l <location>
```

*You need a virtual network before creating the new cluster. If you already have one, you can skip this step.*

For this example, we deployed a virtual network that contains two subnets, one for master nodes and one for agent nodes:

- 10.100.0.0/24
- 10.200.0.0/24

```bash
az network vnet create -g <resource group> -n CustomVNET --address-prefixes 10.100.0.0/24 10.200.0.0/24 --subnet-name MasterSubnet --subnet-prefix 10.100.0.0/24
az network vnet subnet create --name AgentSubnet --address-prefix 10.200.0.0/24 -g <resource group> --vnet-name CustomVNET
```

Once the deployment is completed you should see the virtual network in the resource group.


## Create the template for DC/OS Engine
DC/OS Engine uses a JSON template in input and generates the ARM template and ARM parameters files in output.

There are a lot of examples available on the [DC/OS Engine GitHub](https://github.com/Azure/dcos-engine/tree/master/examples) and you can find [one dedicated for virtual network](https://github.com/Azure/dcos-engine/blob/master/examples/vnet/README.md).

In this case, we are going to use the following template:

```json
{
  "apiVersion": "vlabs",
  "properties": {
    "orchestratorProfile": {
      "orchestratorType": "DCOS"
    },
    "masterProfile": {
      "count": 3,
      "dnsPrefix": "",
      "vmSize": "Standard_D2_v2",
      "vnetSubnetId": "/subscriptions/SUBSCRIPTION_ID/resourceGroups/RESOURCE_GROUP_NAME/providers/Microsoft.Network/virtualNetworks/ExampleCustomVNET/subnets/ExampleMasterSubnet",
      "firstConsecutiveStaticIP": "10.100.0.5"
    },
    "agentPoolProfiles": [
      {
        "name": "agentprivate",
        "count": 3,
        "vmSize": "Standard_D2_v2",
        "vnetSubnetId": "/subscriptions/SUBSCRIPTION_ID/resourceGroups/RESOURCE_GROUP_NAME/providers/Microsoft.Network/virtualNetworks/ExampleCustomVNET/subnets/ExampleAgentSubnet"
      },
      {
        "name": "agentpublic",
        "count": 3,
        "vmSize": "Standard_D2_v2",
        "dnsPrefix": "",
        "vnetSubnetId": "/subscriptions/SUBSCRIPTION_ID/resourceGroups/RESOURCE_GROUP_NAME/providers/Microsoft.Network/virtualNetworks/ExampleCustomVNET/subnets/ExampleAgentSubnet",
        "ports": [
          80,
          443,
          8080
        ]
      }
    ],
    "linuxProfile": {
      "adminUsername": "azureuser",
      "ssh": {
        "publicKeys": [
          {
            "keyData": ""
          }
        ]
      }
    }
  }
}
```

As you can see, for all node pools definition (master or agents) you can use the **vnetSubnetId** and **firstConsecutiveStaticIP** properties to defines the virtual network where you want to deploy the cluster and the first IP address that should be used by the first machine in the pool.

*Note: Make sure the the vnetSubnetId matches with your subnet, by giving your **SUBSCRIPTION_ID**, **RESOURCE_GROUP_NAME**, virtual network and subnet names. You also need to fill DNS prefix for all the public pools you want to create, give an SSH keys...*

## Generate the cluster Azure Resource Manager template
Once your are ready with the cluster definition file, you can use DC/OS Engine to generate the ARM template that will be used to deploy the cluster on Azure:

```bash
dcos-engine generate dcos.json
```

This command will output three files in `_output/`:

- apimodel.json: this is the cluster definition file you gave to DC/OS Engine
- azuredeploy.json: this is the Azure Resource Manager JSON template that you are going to use to deploy the cluster
- azuredeploy.parameters.json: this is the parameters file that you are going to use to deploy the cluster

## Deploy the Azure Container Service cluster
Now that you have generated the ARM templates and its parameters file using DC/OS Engine, you can use Azure CLI 2.0 to start the deployment of the cluster:

```bash
az group deployment create -g <resource group> --name "ClusterDeployment" --template-file azuredeploy.json --parameters "@azuredeploy.parameters.json"
```
