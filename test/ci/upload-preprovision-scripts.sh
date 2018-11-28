#!/bin/bash

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ]; do # resolve $SOURCE until the file is no longer a symlink
  DIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"
  SOURCE="$(readlink "$SOURCE")"
  [[ $SOURCE != /* ]] && SOURCE="$DIR/$SOURCE" # if $SOURCE was a relative symlink, we need to resolve it relative to the path where the symlink file was located
done
DIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"

# Upload preprovision scripts to a storage account
resource_group="dcos-engine-CI"
storage_account="dcosenginetest"
key=$(az storage account keys list -g ${resource_group} -n ${storage_account} --output tsv | grep key1 | cut -f 3)

cd $DIR/preprovision

for f in \
  extensions/preprovision-master-linux/v1/preprovision-master-linux.sh \
  extensions/preprovision-agent-linux-public/v1/preprovision-agent-linux-public.sh \
  extensions/preprovision-agent-linux-private/v1/preprovision-agent-linux-private.sh \
  extensions/postinstall-agent-linux/v1/postinstall-agent-linux.sh \
  extensions/preprovision-agent-windows/v1/preprovision-agent-windows.ps1 \
  extensions/postinstall-agent-windows/v1/postinstall-agent-windows.ps1; do
  az storage blob upload --container-name preprovision --file $f --name $f --account-name ${storage_account} --account-key $key
done
