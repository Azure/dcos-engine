#!/bin/bash

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ]; do # resolve $SOURCE until the file is no longer a symlink
  DIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"
  SOURCE="$(readlink "$SOURCE")"
  [[ $SOURCE != /* ]] && SOURCE="$DIR/$SOURCE" # if $SOURCE was a relative symlink, we need to resolve it relative to the path where the symlink file was located
done
DIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"

set -e
set -u
set -o pipefail
set -x

source "${DIR}/common.sh"

# upgrade tests set EXPECTED_ORCHESTRATOR_VERSION in .env files
ENV_FILE="${CLUSTER_DEFINITION}.env"
if [ -e "${ENV_FILE}" ]; then
  source "${ENV_FILE}"
fi

[[ ! -z "${EXPECTED_ORCHESTRATOR_VERSION:-}" ]] || (echo "Must specify EXPECTED_ORCHESTRATOR_VERSION" && exit 1)

OUTPUT="${DIR}/../../../_output/${INSTANCE_NAME}"

CMD=""
if [ ! -z "${LINUX_BOOTSTRAP_URL:-}" ]; then
  CMD=" $CMD --linux-bootstrap-url ${LINUX_BOOTSTRAP_URL}"
fi
if [ ! -z "${WINDOWS_BOOTSTRAP_URL:-}" ]; then
  CMD=" $CMD --windows-bootstrap-url ${WINDOWS_BOOTSTRAP_URL}"
fi

validate_node_count

validate_node_health

${DIR}/../../../bin/dcos-engine upgrade \
  --subscription-id ${SUBSCRIPTION_ID} \
  --deployment-dir ${OUTPUT} \
  --location ${LOCATION} \
  --resource-group ${RESOURCE_GROUP} \
  --upgrade-version ${EXPECTED_ORCHESTRATOR_VERSION} \
  --ssh-private-key-path ${SSH_KEY} \
  $CMD
