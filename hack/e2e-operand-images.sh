#!/usr/bin/env bash
#
# Override operand images for an already-installed ZTWIM operator.
#
# Usage:
#   hack/e2e-operand-images.sh setup
#
# Set only the images you want to replace. When none of these are set, the
# script exits without changing the cluster and the operator keeps the
# RELATED_IMAGE_* values from the bundle.
#
#   E2E_OPERAND_IMAGE_SPIRE_SERVER
#   E2E_OPERAND_IMAGE_SPIRE_AGENT
#   E2E_OPERAND_IMAGE_SPIFFE_CSI_DRIVER
#   E2E_OPERAND_IMAGE_SPIRE_OIDC_DISCOVERY_PROVIDER
#   E2E_OPERAND_IMAGE_SPIRE_CONTROLLER_MANAGER
#   E2E_OPERAND_IMAGE_NODE_DRIVER_REGISTRAR
#   E2E_OPERAND_IMAGE_SPIFFE_CSI_INIT_CONTAINER
set -euo pipefail

NAMESPACE="${OPERATOR_NAMESPACE:-zero-trust-workload-identity-manager}"
DEPLOYMENT="zero-trust-workload-identity-manager-controller-manager"

# Operand workloads to restart when they already exist. The e2e suite creates
# these after this script runs; a missing workload is ignored.
OPERANDS=(
	"statefulset/spire-server"
	"daemonset/spire-agent"
	"daemonset/spire-spiffe-csi-driver"
	"deployment/spire-spiffe-oidc-discovery-provider"
)

setup() {
	local -a pairs=(
		"E2E_OPERAND_IMAGE_SPIRE_SERVER:RELATED_IMAGE_SPIRE_SERVER"
		"E2E_OPERAND_IMAGE_SPIRE_AGENT:RELATED_IMAGE_SPIRE_AGENT"
		"E2E_OPERAND_IMAGE_SPIFFE_CSI_DRIVER:RELATED_IMAGE_SPIFFE_CSI_DRIVER"
		"E2E_OPERAND_IMAGE_SPIRE_OIDC_DISCOVERY_PROVIDER:RELATED_IMAGE_SPIRE_OIDC_DISCOVERY_PROVIDER"
		"E2E_OPERAND_IMAGE_SPIRE_CONTROLLER_MANAGER:RELATED_IMAGE_SPIRE_CONTROLLER_MANAGER"
		"E2E_OPERAND_IMAGE_NODE_DRIVER_REGISTRAR:RELATED_IMAGE_NODE_DRIVER_REGISTRAR"
		"E2E_OPERAND_IMAGE_SPIFFE_CSI_INIT_CONTAINER:RELATED_IMAGE_SPIFFE_CSI_INIT_CONTAINER"
	)

	local env_json="["
	local first=1
	local count=0
	local pair src dest val

	for pair in "${pairs[@]}"; do
		src="${pair%%:*}"
		dest="${pair##*:}"
		val="${!src:-}"
		if [[ -z "${val}" ]]; then
			continue
		fi
		count=$((count + 1))
		if [[ "${first}" -eq 0 ]]; then
			env_json+=","
		fi
		first=0
		val="${val//\\/\\\\}"
		val="${val//\"/\\\"}"
		env_json+="{\"name\":\"${dest}\",\"value\":\"${val}\"}"
		echo "Override ${dest}=${val}"
	done
	env_json+="]"

	if [[ "${count}" -eq 0 ]]; then
		echo "No E2E_OPERAND_IMAGE_* variables set; leaving operand images unchanged"
		exit 0
	fi

	local sub
	sub=$(oc get subscription -n "${NAMESPACE}" -o jsonpath='{.items[0].metadata.name}')
	if [[ -z "${sub}" ]]; then
		echo "Error: no Subscription found in namespace ${NAMESPACE}" >&2
		exit 1
	fi
	echo "Patching Subscription ${sub}"

	oc patch subscription "${sub}" -n "${NAMESPACE}" --type=merge \
		-p "{\"spec\":{\"config\":{\"env\":${env_json}}}}"

	echo "Waiting for operator rollout..."
	oc rollout status "deployment/${DEPLOYMENT}" -n "${NAMESPACE}" --timeout=180s

	local operand
	for operand in "${OPERANDS[@]}"; do
		if oc get "${operand}" -n "${NAMESPACE}" >/dev/null 2>&1; then
			echo "Restarting ${operand}"
			oc rollout restart "${operand}" -n "${NAMESPACE}"
			oc rollout status "${operand}" -n "${NAMESPACE}" --timeout=180s
		fi
	done

	echo "Operand image override complete"
}

case "${1:-}" in
setup)
	setup
	;;
*)
	echo "Usage: $0 setup" >&2
	exit 1
	;;
esac
