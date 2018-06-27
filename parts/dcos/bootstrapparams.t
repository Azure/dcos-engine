{{if IsHostedBootstrap}}
    "bootstrapSubnet": {
      "defaultValue": "{{.HostedBootstrapProfile.Subnet}}",
      "metadata": {
        "description": "Sets the subnet for the VMs in the cluster."
      },
      "type": "string"
    },
    "bootstrapEndpoint": {
      "defaultValue": "{{.HostedBootstrapProfile.FQDN}}",
      "metadata": {
        "description": "Sets the static IP of the first bootstrap"
      },
      "type": "string"
    },
{{else}}
    "bootstrapStaticIP": {
      "metadata": {
        "description": "Sets the static IP of the first bootstrap"
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
