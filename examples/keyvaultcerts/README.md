# Microsoft DC/OS Engine - Key vault certificate deployment

## Overview

DCOS-Engine enables you to create customized Docker enabled cluster on Microsoft Azure with certs installed from key vault during deployment.

**dcos.json** - installing a cert from keyvault. These certs are assumed to be in the secrets portion of your keyvault.

On windows machines certificates will be installed under the machine in the specified store.
On linux machines the certificates will be installed in the folder /var/lib/waagent/. There will be two files
1. {thumbprint}.prv - this will be the private key pem formatted
2. {thumbprint}.crt - this will be the full cert chain pem formatted
