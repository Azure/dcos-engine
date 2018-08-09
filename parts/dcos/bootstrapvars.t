{{if .OrchestratorProfile.DcosConfig.BootstrapProfile}}
    ,
    "dcosBootstrapURL": "[parameters('dcosBootstrapURL')]",
    "bootstrapVMSize": "[parameters('bootstrapVMSize')]",
    "bootstrapNSGID": "[resourceId('Microsoft.Network/networkSecurityGroups',variables('bootstrapNSGName'))]",
    "bootstrapNSGName": "[concat('bootstrap-nsg-', variables('nameSuffix'))]",
    "bootstrapVMName": "[concat('dcos-bootstrap-', variables('nameSuffix'))]",
    "bootstrapStaticIP": "[parameters('bootstrapStaticIP')]"
{{if .HasWindows}}
    ,
    "bootstrapWinPublicIPAddressName": "[concat('bootstrap-win-ip-', variables('nameSuffix'))]",
    "bootstrapWinNSGName": "[concat('bootstrap-win-nsg-', variables('nameSuffix'))]",
    "bootstrapWinNSGID": "[resourceId('Microsoft.Network/networkSecurityGroups',variables('bootstrapWinNSGName'))]",
    "bootstrapWinVMName": "[concat('dcos-b', variables('nameSuffix'))]",
    "bootstrapAddrOctets": "[split(parameters('bootstrapStaticIP'),'.')]",
    "bootstrapAddrPrefix": "[concat(variables('bootstrapAddrOctets')[0],'.',variables('bootstrapAddrOctets')[1],'.',variables('bootstrapAddrOctets')[2],'.')]",
    "bootstrapAddrOctet4": "[variables('bootstrapAddrOctets')[3]]",
    "bootstrapWinStaticIP": "[concat(variables('bootstrapAddrPrefix'), add(int(variables('bootstrapAddrOctet4')),1))]",
    "bootstrapWinScriptSuffix": " $inputFile = '%SYSTEMDRIVE%\\AzureData\\CustomData.bin' ; $outputFile = '%SYSTEMDRIVE%\\AzureData\\bootstrapWindowsProvision.ps1' ; $inputStream = New-Object System.IO.FileStream $inputFile, ([IO.FileMode]::Open), ([IO.FileAccess]::Read), ([IO.FileShare]::Read) ; $sr = New-Object System.IO.StreamReader(New-Object System.IO.Compression.GZipStream($inputStream, [System.IO.Compression.CompressionMode]::Decompress)) ; $sr.ReadToEnd() | Out-File($outputFile) ; Invoke-Expression('{0} {1}' -f $outputFile, $arguments) ; ",
    "bootstrapWinScriptArguments": "[concat('$arguments = ', variables('singleQuote'),'-BootstrapURL ',variables('dcosWindowsBootstrapURL'),variables('singleQuote'), ' ; ')]",
    "bootstrapWinScript": "[concat('powershell.exe -ExecutionPolicy Unrestricted -command \"', variables('bootstrapWinScriptArguments'), variables('bootstrapWinScriptSuffix'), '\" > %SYSTEMDRIVE%\\AzureData\\bootstrapScript.log 2>&1; exit $LASTEXITCODE')]",
    "winAgentCustomSuffix": " $inputFile = '%SYSTEMDRIVE%\\AzureData\\CustomData.bin' ; $outputFile = '%SYSTEMDRIVE%\\AzureData\\dcosWindowsProvision.ps1' ; $inputStream = New-Object System.IO.FileStream $inputFile, ([IO.FileMode]::Open), ([IO.FileAccess]::Read), ([IO.FileShare]::Read) ; $sr = New-Object System.IO.StreamReader(New-Object System.IO.Compression.GZipStream($inputStream, [System.IO.Compression.CompressionMode]::Decompress)) ; $sr.ReadToEnd() | Out-File($outputFile) ; Invoke-Expression('{0} {1}' -f $outputFile, $arguments) ; ",
    "winAgentCustomArguments": "[concat('$arguments = ', variables('singleQuote'),'-AdminUser ', variables('windowsAdminUsername'), ' -BootstrapIP ',variables('bootstrapWinStaticIP'),variables('singleQuote'), ' ; ')]",
    "windowsAgentCustomScript": "[concat('powershell.exe -ExecutionPolicy Unrestricted -command \"', variables('winAgentCustomArguments'), variables('winAgentCustomSuffix'), '\" > %SYSTEMDRIVE%\\AzureData\\windowsAgent.log 2>&1; exit $LASTEXITCODE')]"
{{end}}
{{end}}
