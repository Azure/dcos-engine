#!/bin/bash

workdir=$(pwd)

# Download preprovision scripts from https://github.com/Microsoft/mesos-jenkins

tmpdir=$(mktemp -d)
trap "rm -rf $tmpdir" EXIT
cd $tmpdir
git init mesos-jenkins
cd mesos-jenkins
git remote add origin https://github.com/Microsoft/mesos-jenkins.git
git config core.sparsecheckout true
echo "DCOS/preprovision/extensions" >> .git/info/sparse-checkout
git pull origin master

# Upload preprovision scripts to a storage account
storage_account=dcosenginetest
key=$(az storage account keys list -g osct-test-infra -n ${storage_account} --output tsv | grep key1 | cut -f 3)

cd DCOS/preprovision
for f in \
  extensions/preprovision-master-linux/v1/preprovision-master-linux.sh \
  extensions/preprovision-agent-linux-public/v1/preprovision-agent-linux-public.sh \
  extensions/preprovision-agent-linux-private/v1/preprovision-agent-linux-private.sh \
  extensions/preprovision-agent-windows/v1/preprovision-agent-windows-ci-setup.ps1 \
  extensions/preprovision-agent-windows/v1/preprovision-agent-windows.ps1 \
  extensions/preprovision-agent-windows/v1/preprovision-agent-windows-fluentd.ps1 \
  extensions/preprovision-agent-windows/v1/preprovision-agent-windows-mesos-credentials.ps1; do

  az storage blob upload --container-name preprovision --file $f --name $f --account-name ${storage_account} --account-key $key
done

cd $workdir
