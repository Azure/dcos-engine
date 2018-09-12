#!/bin/bash

set -e
set -x

# Upload bootstrap binary to a storage account

ShowUsage()
{
  cat << EOF
  Error: $1

  Usage: $0 -v <version> [-l <file>] [-w <file>]

    -v <DC/OS version>
    -l <Linux bootstrap package filepath>
    -w <Windows bootstrap package filepath>
EOF
    exit 1
}

UploadFile()
{
  local path=$1
  local account=$2
  local key=$3
  local container=$4
  local name=$5
  echo "Uploading $path to ${account}/${container}/${name}"
  az storage blob upload --container-name $container --file $path --name $name --account-name $account --account-key $key
}

while (( "$#" )); do
  if [ "$1" == "-v" ]; then
    shift
    VERSION=$1
  elif [ "$1" == "-l" ] ; then
    shift
    LINUX_BSTRAP_PATH=$1
  elif [ "$1" == "-w" ] ; then
    shift
    WINDOWS_BSTRAP_PATH=$1
  else
    ShowUsage "unsupported argument '$1'"
  fi
  shift
done

if [ -z "$VERSION" ]; then
  ShowUsage "Missing version"
fi

dashedVersion=$(echo $VERSION | tr "." "-")

storage_account=dcosprodcdn
key=$(az storage account keys list -g dcos-prod-cdn -n ${storage_account} --output tsv | grep key1 | cut -f 3)

if [ ! -z "$LINUX_BSTRAP_PATH" ]; then
  container="dcos"
  name="${dashedVersion}/dcos_generate_config.sh"
  UploadFile $LINUX_BSTRAP_PATH $storage_account $key $container $name

  sha1file=$(mktemp)
  sha1sum -b $LINUX_BSTRAP_PATH | cut -f 1 -d " " | cat > $sha1file
  UploadFile $sha1file $storage_account $key $container "${name}.sha1sum"
  rm $sha1file
fi

if [ ! -z "$WINDOWS_BSTRAP_PATH" ]; then
  container="dcos"
  name="${dashedVersion}/dcos_generate_config.windows.tar.xz"
  UploadFile $WINDOWS_BSTRAP_PATH $storage_account $key $container $name

  sha1file=$(mktemp)
  sha1sum -b $WINDOWS_BSTRAP_PATH | cut -f 1 -d " " | cat > $sha1file
  UploadFile $sha1file $storage_account $key $container "${name}.sha1sum"
  rm $sha1file
fi
