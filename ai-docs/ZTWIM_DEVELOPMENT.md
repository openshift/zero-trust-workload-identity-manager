# ZTWIM Development Guide

> For generic Go/operator practices, see the Platform Development Guide.

## Quick Start

### Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| Go | 1.25+ | See `go.mod` for exact version (1.25.7) |
| Docker or Podman | Latest | Set `CONTAINER_TOOL=podman` if using Podman |
| oc / kubectl | Latest | For cluster interaction |
| OpenShift | 4.18+ | E2E tests require a live cluster |
| operator-sdk | v1.39.0 | Auto-downloaded by Makefile |

### Build Commands

```bash
make build          # Full build: manifests + generate + fmt + vet + binary
make build-operator # Binary only (no codegen, fastest iteration)
make test           # Unit tests with coverage
make verify         # vet + fmt + lint
make lint           # golangci-lint (v1.59.1)
```

## Installation / Bootstrap

ZTWIM is installed via OLM. After the operator Deployment is running, **users must create all CRs manually** — the ZTWIM controller aggregates status but does **not** create operand CRs.

### Required CRs

All five ZTWIM-owned CRs are cluster-scoped singletons named `cluster`:

| Order | CR | Why This Order |
|-------|----|----------------|
| 1 | `ZeroTrustWorkloadIdentityManager` | Defines trust domain and cluster identity; parent for owner references |
| 2 | `SpireServer` | CA root and spire-controller-manager sidecar; required before agents can attest |
| 3 | `SpireAgent` | Per-node daemon; exposes Workload API socket |
| 4 | `SpiffeCSIDriver` | Bind-mounts agent socket into pods; [`agentSocketPath`](domain/spiffe-csi-driver.md#field-details) must match [SpireAgent `socketPath`](domain/spire-agent.md#spec-structure) |
| 5 | `SpireOIDCDiscoveryProvider` | Optional for X.509-only; required for JWT/OIDC. [`jwtIssuer`](domain/spire-oidc-discovery-provider.md#field-details) must match [SpireServer](domain/spire-server.md#spec-structure) |

Sample manifests live in `config/samples/`. Apply in order:

```bash
oc apply -f config/samples/operator.openshift.io_v1alpha1_zerotrustworkloadidentitymanager.yaml
oc apply -f config/samples/operator.openshift.io_v1alpha1_spireserver.yaml
oc apply -f config/samples/operator.openshift.io_v1alpha1_spireagent.yaml
oc apply -f config/samples/operator.openshift.io_v1alpha1_spiffecsidriver.yaml
oc apply -f config/samples/operator.openshift.io_v1alpha1_spireoidcdiscoveryprovider.yaml
```

Or apply the full sample set (includes upstream SPIFFE CR examples):

```bash
oc apply -k config/samples/
```

### Workload Identity (After Infrastructure Is Ready)

Once SpireServer and SpireAgent report Ready, create [user-managed ClusterSPIFFEIDs](domain/upstream-spiffe-crds.md#user-created-instances) to grant application workloads SVIDs. Workload pods must mount the CSI volume (`driver: csi.spiffe.io` — see [SpiffeCSIDriver](domain/spiffe-csi-driver.md#workload-volume-reference)).

The OIDC controller automatically creates two operator-managed ClusterSPIFFEIDs when SpireOIDCDiscoveryProvider is applied — do not create these manually.

### Verify Installation

```bash
oc get zerotrustworkloadidentitymanagers cluster -o yaml
oc get spireservers,spireagents,spiffecsidrivers,spireoidcdiscoveryproviders
oc get clusterspiffeids.spire.spiffe.io
```

Check `status.conditions` on the ZTWIM CR for aggregated operand health.

## Code Style

### Import Order

Imports follow this grouping convention (checked by `goimports` linter in CI):

1. Standard library
2. `k8s.io/*`, `sigs.k8s.io/*`
3. Third-party (`github.com/go-logr/logr`, `github.com/operator-framework/api`, etc.)
4. OpenShift (`github.com/openshift/api`, `github.com/openshift/client-go`)
5. This project (`github.com/openshift/zero-trust-workload-identity-manager/...`)

### Standard Import Aliases

| Alias | Package |
|---|---|
| `ctrl` | `sigs.k8s.io/controller-runtime` |
| `kerrors` | `k8s.io/apimachinery/pkg/api/errors` |
| `apimeta` | `k8s.io/apimachinery/pkg/api/meta` |
| `customClient` | `github.com/openshift/zero-trust-workload-identity-manager/pkg/client` |
| `routev1` | `github.com/openshift/api/route/v1` |

### File Headers

All `.go` files must include the Apache 2.0 license header from `hack/boilerplate.go.txt`.

## Development Workflow

### Local Build

```bash
export OPERATOR_NAMESPACE=zero-trust-workload-identity-manager
make build-operator
./bin/zero-trust-workload-identity-manager --v=5 --metrics-secure=false
```

The binary is FIPS-aware — `hack/go-fips.sh` sets `GOEXPERIMENT=strictfipsruntime` when the compiler supports it. Local builds without the FIPS-capable toolchain still succeed but emit a warning. Non-FIPS builds cannot be used in CI or production (per `hack/go-fips.sh`).

### Testing on Cluster

```bash
make docker-build IMG=<registry>/ztwim:dev
make docker-push  IMG=<registry>/ztwim:dev
make deploy       IMG=<registry>/ztwim:dev
```

Or the all-in-one:

```bash
make generate-deploy IMG=<registry>/ztwim:dev
```

### Debugging

Run locally against a remote cluster:

```bash
make run  # starts controller with --v=5 against current kubeconfig
```

Set `OPERATOR_NAMESPACE=zero-trust-workload-identity-manager` — the operator exits immediately if this is empty.

## Common Tasks

### Adding a New Operand

1. Define the CRD type in `api/v1alpha1/<operand>_types.go` with `+kubebuilder:resource:scope=Cluster`, singleton CEL validation, `ConditionalStatus` embedding, and `GetConditionalStatus()` method.
2. Create the controller package under `pkg/controller/<operand>/` following the reconciler structure: `Reconciler` struct, `New(mgr)`, `SetupWithManager(mgr)`, `Reconcile(ctx, req)`.
3. Register the scheme and wire the controller in `cmd/zero-trust-workload-identity-manager/main.go`.
4. Run `make manifests generate`.
5. Add the new CR type to `cacheResourceWithoutReqSelectors` and `informerResources` in `pkg/client/client.go`.
6. Add a `get<Operand>Status()` method to the ZTWIM controller and include it in `aggregateOperandStatus()`.
7. Add a `Watches` entry for the new operand CR in the ZTWIM controller's `SetupWithManager()` with `operandStatusChangedPredicate`.
8. Add RBAC markers on the ZTWIM controller for the new CRD, then `make manifests`.
9. Add component label constant and label generator function in `pkg/controller/utils/labels.go`.
10. Add image env var constant and getter in `pkg/controller/utils/constants.go` and `relatedImages.go`.
11. Add bindata YAML under `bindata/<operand>/`, add constants, and run `make update-bindata`.
12. Run `make manifests generate update-bindata && make verify && make test`.

### Adding a New Bindata Resource

1. Create the YAML manifest in `bindata/<component>/` with `app.kubernetes.io/managed-by: zero-trust-workload-identity-manager` label.
2. Add an asset path constant in `pkg/controller/utils/constants.go`.
3. Run `make update-bindata` to regenerate `pkg/operator/assets/bindata.go`.
4. Add a `reconcileNewResource()` method following the decode → mutate → SetControllerReference → Get → Create/Update pattern.
5. Call the new reconcile method in `Reconcile()` at the correct position in the ordering.
6. Add a status condition constant for the new resource.
7. Add a `Watches` entry in `SetupWithManager()` if it's a new GVK.
8. Register the resource type in `NewCacheBuilder()` under `cacheResources` in `pkg/client/client.go`.
9. Add the resource type to `ResourceNeedsUpdate()` in `pkg/controller/utils/` with field-level comparison.
10. Run `make manifests generate update-bindata && make verify`.

### Adding a New CRD Field

1. Add the field to the appropriate `*Spec` or `*Status` struct in `api/v1alpha1/`.
2. Add kubebuilder validation markers (Required/Optional, Enum, Pattern, Min/Max, Default).
3. If immutable, add a CEL `XValidation` rule with `self == oldSelf`.
4. Run `make generate` (deepcopy) then `make manifests` (CRD YAML).
5. Update controller logic to read/use the new field.
6. Update bindata templates if the field affects rendered YAML; run `make update-bindata`.
7. Add unit tests covering validation (valid values, invalid values, immutability).
8. Run `make verify` to confirm lint/fmt pass.

### Updating Dependencies

```bash
go get <package>@<version>
make vendor   # runs go mod tidy + go mod vendor
make verify   # ensure lint and vet still pass
```

## Build & Release

### Local Build

```bash
make docker-build IMG=<registry>/ztwim:latest
```

The `Dockerfile` uses a two-stage build: `registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.25-openshift-4.21` for compilation and `ubi9-minimal:9.4` as the runtime base.

### CI Build

`.ci-operator.yaml` defines the build root:

```yaml
build_root_image:
  name: builder
  namespace: ocp
  tag: rhel-9-golang-1.25-openshift-4.21
```

CI runs `make test` (unit) and `make test-e2e` (E2E on a live cluster).

### Release Process

Release images are managed in the separate `zero-trust-workload-identity-manager-release` repository. The OLM bundle is generated via:

```bash
make bundle VERSION=<version>
make bundle-build BUNDLE_IMG=<registry>/ztwim-bundle:v<version>
```

## Common Mistakes

1. **DO NOT** forget to set `OPERATOR_NAMESPACE` env var — the operator will exit(1) immediately.
2. **DO NOT** add bindata YAML without running `make update-bindata` — the embedded assets won't include your changes.
3. **DO NOT** run `make test` without `OPERATOR_NAMESPACE=zero-trust-workload-identity-manager` — tests rely on it for namespace resolution.
4. **DO NOT** skip `make manifests generate` after changing `api/v1alpha1/` types — CRDs and DeepCopy will be stale.
5. **DO NOT** use `controllerutil.SetControllerReference` without registering the owner type in the scheme — it will fail silently.
6. **DO NOT** return `nil` from a reconciler when a sub-function returns an API or transient error — always propagate these so controller-runtime can requeue. Validation errors are the exception: they set a status condition (e.g., `ConfigurationValid=False`) and return `(Result{}, nil)` to avoid requeue, waiting for a spec change instead.
7. **DO NOT** create resources without setting owner references — orphaned resources won't be garbage-collected.
8. **DO NOT** bypass the `FakeCustomCtrlClient` interface in tests — use counterfeiter fakes for consistent stubbing.

## Logging Conventions

| Scenario | Pattern |
|---|---|
| Error with requeue | `r.log.Error(err, "failed to <action>")` then `return (Result{}, err)` |
| Error without requeue (validation) | `r.log.Error(err, "descriptive message", "key", value)` then set condition |
| Informational not-found | `r.log.Info("resource not found, ignoring")` — never log at Error level |
| Debug-level detail | `r.log.V(1).Info("skipping update", "reason", "...")` |
| Status update failure in defer | `r.log.Error(err, "failed to update status")` — log only, do not propagate |

Never use `fmt.Printf` or `klog` directly.

## Event Recording

Events are reserved for **user-actionable warnings** — not routine reconciliation. Rules:
- Use `corev1.EventTypeWarning` for problems the user should address.
- Do not emit events for transient API errors or routine reconciliation.
- Event reason should be PascalCase (e.g., `TTLConfigurationWarning`).

## Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `OPERATOR_NAMESPACE` | Namespace for operator resources | (required) |
| `OPERATOR_CONDITION_NAME` | OLM OperatorCondition name for Upgradeable sync | (required) |
| `CREATE_ONLY_MODE` | Skip updates, only create resources | `""` (disabled) |
| `RELATED_IMAGE_SPIRE_SERVER` | Spire Server image override | Set by CSV |
| `RELATED_IMAGE_SPIRE_AGENT` | Spire Agent image override | Set by CSV |
| `RELATED_IMAGE_SPIFFE_CSI_DRIVER` | SPIFFE CSI Driver image override | Set by CSV |
| `RELATED_IMAGE_SPIRE_OIDC_DISCOVERY_PROVIDER` | OIDC Discovery Provider image override | Set by CSV |
| `RELATED_IMAGE_SPIRE_CONTROLLER_MANAGER` | Controller Manager image override | Set by CSV |
| `RELATED_IMAGE_NODE_DRIVER_REGISTRAR` | Node Driver Registrar image override | Set by CSV |
| `RELATED_IMAGE_SPIFFE_CSI_INIT_CONTAINER` | CSI init container image override | Set by CSV |
| `HTTP_PROXY` | HTTP proxy for operand pods | `""` |
| `HTTPS_PROXY` | HTTPS proxy for operand pods | `""` |
| `TRUSTED_CA_BUNDLE_CONFIGMAP` | CA bundle ConfigMap (required when proxy set) | `""` |

---
