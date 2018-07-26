{{if HasBootstrapPublicIP}}
    {
      "apiVersion": "[variables('apiVersionDefault')]",
      "location": "[variables('location')]",
      "name": "[variables('bootstrapWinPublicIPAddressName')]",
      "properties": {
        "publicIPAllocationMethod": "Dynamic"
      },
      "type": "Microsoft.Network/publicIPAddresses"
    },
{{end}}
    {
      "apiVersion": "[variables('apiVersionDefault')]",
      "location": "[variables('location')]",
      "name": "[variables('bootstrapWinNSGName')]",
      "properties": {
        "securityRules": [
            {
                "properties": {
                    "priority": 200,
                    "access": "Allow",
                    "direction": "Inbound",
                    "destinationPortRange": "3389",
                    "sourcePortRange": "*",
                    "destinationAddressPrefix": "*",
                    "protocol": "Tcp",
                    "description": "Allow RDP",
                    "sourceAddressPrefix": "*"
                },
                "name": "rdp"
            },
            {
                "properties": {
                    "priority": 201,
                    "access": "Allow",
                    "direction": "Inbound",
                    "destinationPortRange": "8086",
                    "sourcePortRange": "*",
                    "destinationAddressPrefix": "*",
                    "protocol": "Tcp",
                    "description": "Allow bootstrap service",
                    "sourceAddressPrefix": "*"
                },
                "name": "Port8086"
            }
        ]
      },
      "type": "Microsoft.Network/networkSecurityGroups"
    },
{{if HasWindowsCustomImage}}
    {"type": "Microsoft.Compute/images",
      "apiVersion": "2017-12-01",
      "name": "bootstrap-win-customImage",
      "location": "[variables('location')]",
      "properties": {
        "storageProfile": {
          "osDisk": {
            "osType": "Windows",
            "osState": "Generalized",
            "blobUri": "[parameters('agentWindowsSourceUrl')]",
            "storageAccountType": "Standard_LRS"
          }
        }
      }
    },
{{end}}
    {
      "apiVersion": "[variables('apiVersionDefault')]",
      "dependsOn": [
{{if not .MasterProfile.IsCustomVNET}}
        "[variables('vnetID')]",
{{end}}
{{if HasBootstrapPublicIP}}
        "[variables('bootstrapWinPublicIPAddressName')]",
{{end}}
        "[variables('bootstrapWinNSGID')]"
      ],
      "location": "[variables('location')]",
      "name": "[concat(variables('bootstrapWinVMName'), '-nic')]",
      "properties": {
        "ipConfigurations": [
          {
            "name": "ipConfigNode",
            "properties": {
              "privateIPAddress": "[variables('bootstrapWinStaticIP')]",
              "privateIPAllocationMethod": "Static",
{{if HasBootstrapPublicIP}}
              "publicIpAddress": {
                "id": "[resourceId('Microsoft.Network/publicIpAddresses', variables('bootstrapWinPublicIPAddressName'))]"
              },
{{end}}
              "subnet": {
                "id": "[variables('masterVnetSubnetID')]"
              }
            }
          }
        ],
        "networkSecurityGroup": {
          "id": "[variables('bootstrapWinNSGID')]"
        }
      },
      "type": "Microsoft.Network/networkInterfaces"
    },
    {
      "apiVersion": "[variables('apiVersionStorageManagedDisks')]",
      "dependsOn": [
        "[concat('Microsoft.Network/networkInterfaces/', variables('bootstrapWinVMName'), '-nic')]",
{{if .MasterProfile.IsStorageAccount}}
        "[variables('masterStorageAccountName')]",
{{end}}
        "[variables('masterStorageAccountExhibitorName')]"
      ],
      "tags":
      {
        "creationSource" : "[concat('dcos-engine-', variables('bootstrapWinVMName'))]",
        "orchestratorName": "dcos",
        "orchestratorVersion": "[variables('orchestratorVersion')]",
        "orchestratorNode": "bootstrap"
      },
      "location": "[variables('location')]",
      "name": "[variables('bootstrapWinVMName')]",
      "properties": {
        "hardwareProfile": {
          "vmSize": "[variables('bootstrapVMSize')]"
        },
        "networkProfile": {
          "networkInterfaces": [
            {
              "id": "[resourceId('Microsoft.Network/networkInterfaces',concat(variables('bootstrapWinVMName'), '-nic'))]"
            }
          ]
        },
        "osProfile": {
          "computername": "[concat('wbs', variables('nameSuffix'))]",
          "adminUsername": "[variables('windowsAdminUsername')]",
          "adminPassword": "[variables('windowsAdminPassword')]",
          {{GetDCOSBootstrapWindowsCustomData}}
        },
        "storageProfile": {
          "imageReference": {
{{if HasWindowsCustomImage}}
            "id": "[resourceId('Microsoft.Compute/images','bootstrap-win-customImage')]"
{{else}}
            "offer": "[variables('agentWindowsOffer')]",
            "publisher": "[variables('agentWindowsPublisher')]",
            "sku": "[variables('agentWindowsSKU')]",
            "version": "[variables('agentWindowsVersion')]"
{{end}}
          },
          "osDisk": {
            "caching": "ReadOnly"
            ,"createOption": "FromImage"
{{if .MasterProfile.IsStorageAccount}}
            ,"name": "[concat(variables('bootstrapWinVMName'), '-osdisk')]"
            ,"vhd": {
              "uri": "[concat(reference(concat('Microsoft.Storage/storageAccounts/',variables('masterStorageAccountName')),variables('apiVersionStorage')).primaryEndpoints.blob,'vhds/',variables('bootstrapWinVMName'),'-osdisk.vhd')]"
            }
{{end}}
{{if ne .OrchestratorProfile.DcosConfig.BootstrapProfile.OSDiskSizeGB 0}}
            ,"diskSizeGB": {{.OrchestratorProfile.DcosConfig.BootstrapProfile.OSDiskSizeGB}}
{{else}}
            ,"diskSizeGB": "120"
{{end}}
          }
        }
      },
      "type": "Microsoft.Compute/virtualMachines"
    },
    {
      "apiVersion": "[variables('apiVersionDefault')]",
      "dependsOn": [
        "[variables('bootstrapWinVMName')]"
      ],
      "location": "[variables('location')]",
      "type": "Microsoft.Compute/virtualMachines/extensions",
      "name": "[concat(variables('bootstrapWinVMName'),'/bootstrapready')]",
      "properties": {
        "publisher": "Microsoft.Compute",
        "type": "CustomScriptExtension",
        "typeHandlerVersion": "1.8",
        "autoUpgradeMinorVersion": true,
        "settings": {},
        "protectedSettings": {
          "commandToExecute": "[variables('bootstrapWinScript')]"
        }
      }
    }
