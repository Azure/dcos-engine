{{if IsHostedBootstrap}}
    "bootstrapEndpoint": {
      "defaultValue": "{{.LinuxBootstrapProfile.BootstrapURL}}",
      "metadata": {
        "description": "Sets the FQDN of hosted Linux bootstrap node"
      },
      "type": "string"
    }
{{else}}
    "bootstrapStaticIP": {
      "metadata": {
        "description": "Sets the static IP of Linux bootstrap node"
      },
      "type": "string"
    },
    "bootstrapVMSize": {
      {{GetMasterAllowedSizes}}
      "metadata": {
        "description": "The size of the Virtual Machine."
      },
      "type": "string"
    }
{{end}}
{{if .HasWindows}}
{{if IsHostedWindowsBootstrap}}
    ,"windowsBootstrapEndpoint": {
      "defaultValue": "{{.WindowsBootstrapProfile.BootstrapURL}}",
      "metadata": {
        "description": "Sets the FQDN of hosted Windows bootstrap node"
      },
      "type": "string"
    }
{{else}}
    ,"windowsBootstrapStaticIP": {
      "metadata": {
        "description": "Sets the static IP of Windows bootstrap node"
      },
      "type": "string"
    },
    "windowsBootstrapVMSize": {
      {{GetMasterAllowedSizes}}
      "metadata": {
        "description": "The size of the Virtual Machine."
      },
      "type": "string"
    }
{{end}}
{{end}}
