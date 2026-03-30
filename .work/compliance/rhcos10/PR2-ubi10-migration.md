# RHCOS10: UBI10 Migration

## Summary

Migrate container base images and operand images from UBI9/RHEL9 and upstream
`ghcr.io/spiffe` mirrors to UBI10/RHEL10 and Red Hat internal
`quay.io/rh-ee-rausingh` mirrors for native RHCOS10 compatibility.

```text
registry.access.redhat.com/ubi9-minimal:9.4  →  registry.redhat.io/ubi10:10.1          (Dockerfile runtime)
registry.access.redhat.com/ubi9:latest        →  registry.redhat.io/ubi10/ubi:10.1     (init container)
ghcr.io/spiffe/*                              →  quay.io/rh-ee-rausingh/zero-trust-workload-identity-manager-*
```

## Operator Image Changes

| File | Before | After |
|------|--------|-------|
| `Dockerfile` (builder stage) | `registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.23-openshift-4.18` | `registry.redhat.io/ubi10/go-toolset:10.1` |
| `Dockerfile` (runtime stage) | `registry.access.redhat.com/ubi9-minimal:9.4` | `registry.redhat.io/ubi10:10.1` |
| `vendor/…/build-machinery-go/.ci-operator.yaml` | `rhel-9-release-golang-1.23-openshift-4.19` | `rhel-10-release-golang-1.24-openshift-4.20` |

## Operand / Related Image Changes

| Env Var | Before | After |
|---------|--------|-------|
| `RELATED_IMAGE_SPIRE_SERVER` | `ghcr.io/spiffe/spire-server:1.13.3` | `quay.io/rh-ee-rausingh/…-spire-server:v1.13.3` |
| `RELATED_IMAGE_SPIRE_AGENT` | `ghcr.io/spiffe/spire-agent:1.13.3` | `quay.io/rh-ee-rausingh/…-spire-agent:v1.13.3` |
| `RELATED_IMAGE_SPIFFE_CSI_DRIVER` | `ghcr.io/spiffe/spiffe-csi-driver:0.2.8` | `quay.io/rh-ee-rausingh/…-spiffe-spiffe-csi:v0.2.8` |
| `RELATED_IMAGE_SPIRE_OIDC_DISCOVERY_PROVIDER` | `ghcr.io/spiffe/oidc-discovery-provider:1.13.3` | `quay.io/rh-ee-rausingh/…-spire-oidc-discovery-provider:v1.13.3` |
| `RELATED_IMAGE_SPIRE_CONTROLLER_MANAGER` | `ghcr.io/spiffe/spire-controller-manager:0.6.3` | `quay.io/rh-ee-rausingh/…-spiffe-spire-controller-manager:v0.6.3` |
| `RELATED_IMAGE_SPIFFE_CSI_INIT_CONTAINER` | `registry.access.redhat.com/ubi9:latest` | `registry.redhat.io/ubi10/ubi:10.1` |
| `RELATED_IMAGE_SPIFFE_HELPER` _(new)_ | `ghcr.io/spiffe/spiffe-helper:0.11.0` | `quay.io/rh-ee-rausingh/…-spiffe-spiffe-helper:v0.10.0` |

## Files Changed

- `Dockerfile`
- `config/manager/manager.yaml`
- `bundle/manifests/zero-trust-workload-identity-manager.clusterserviceversion.yaml`
- `pkg/controller/utils/relatedImages.go`
- `pkg/controller/spiffe-csi-driver/daemonset_test.go`
- `test/e2e/utils/constants.go`
- `vendor/github.com/openshift/build-machinery-go/.ci-operator.yaml`

## Prerequisite

PR1 (`rhcos10-ubi9-compat-test`) must pass CI on RHCOS10 nodes before merging this.

## Test Checklist

- [ ] `e2e` passes on RHCOS10 nodes
- [ ] `e2e-fips` passes on RHCOS10 nodes
- [ ] Operator image pulls successfully from RHCOS10 nodes
- [ ] SPIRE server, agent, CSI driver, OIDC provider all reach `Available`
- [ ] No regressions against existing RHEL9 CI jobs

## Exclusions

- `bundle.Dockerfile` — uses `FROM scratch`, no base image change needed
- `.ci-operator.yaml` (root) — build root remains on RHEL9 for this cycle
