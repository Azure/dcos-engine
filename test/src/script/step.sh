#!/usr/bin/env bash

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ]; do # resolve $SOURCE until the file is no longer a symlink
  DIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"
  SOURCE="$(readlink "$SOURCE")"
  [[ $SOURCE != /* ]] && SOURCE="$DIR/$SOURCE" # if $SOURCE was a relative symlink, we need to resolve it relative to the path where the symlink file was located
done
DIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"

###############################################################################

set -e
set -u
set -o pipefail

source "${DIR}/common.sh"

ROOT="${DIR}/.."

case $1 in

set_azure_account)
  set_azure_account
;;

get_secrets)
  get_secrets
;;

create_resource_group)
  create_resource_group
;;

predeploy)
  DCOS_PREDEPLOY=${DCOS_PREDEPLOY:-}
  if [ ! -z "${DCOS_PREDEPLOY}" ] && [ -x "${DCOS_PREDEPLOY}" ]; then
    "${DCOS_PREDEPLOY}"
  fi
;;

postdeploy)
  DCOS_POSTDEPLOY=${DCOS_POSTDEPLOY:-}
  if [ ! -z "${DCOS_POSTDEPLOY}" ] && [ -x "${DCOS_POSTDEPLOY}" ]; then
    export OUTPUT=${OUTPUT:-"${ROOT}/_output/${INSTANCE_NAME}"}
    "${DCOS_POSTDEPLOY}"
  fi
;;

generate_template)
  export OUTPUT="${ROOT}/_output/${INSTANCE_NAME}"
  generate_template
;;

deploy_template)
  export OUTPUT="${ROOT}/_output/${INSTANCE_NAME}"
  deploy_template
;;

get_node_count)
  export OUTPUT="${ROOT}/_output/${INSTANCE_NAME}"
  get_node_count
;;

get_orchestrator_type)
  get_orchestrator_type
;;

get_orchestrator_version)
  export OUTPUT="${ROOT}/_output/${INSTANCE_NAME}"
  get_orchestrator_version
;;

get_name_suffix)
  export OUTPUT="${ROOT}/_output/${INSTANCE_NAME}"
  get_name_suffix
;;

validate)
  export OUTPUT=${OUTPUT:-"${ROOT}/_output/${INSTANCE_NAME}"}
  set +e
  validate
;;

cleanup)
  export CLEANUP="${CLEANUP:-true}"
  cleanup
;;

*)
  echo "unsupported command $1"
  exit 1
;;
esac
