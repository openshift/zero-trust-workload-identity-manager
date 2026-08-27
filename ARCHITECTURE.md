# Architecture

This document describes the high-level architecture of the Zero Trust Workload Identity Manager.
If you want to familiarize yourself with the code base, you are in the right place.

## Bird's Eye View

The Zero Trust Workload Identity Manager is an OpenShift Day-2 operator that deploys, configures,
and lifecycle-manages the complete SPIFFE/SPIRE workload identity stack on OpenShift clusters.

On the highest level, the operator accepts cluster-scoped singleton Custom Resources as input
and produces a fully reconciled SPIRE infrastructure as output — StatefulSets, DaemonSets,
Deployments, RBAC, ConfigMaps, Services, and Routes.

The operator is a single binary running multiple controllers within one controller-runtime
manager process. Each controller owns exactly one CRD kind and is responsible for the full
set of Kubernetes resources that CRD implies.

## Upstream Components (SPIFFE/SPIRE)

This operator deploys and manages the following upstream components:

**SPIFFE** is the specification (Secure Production Identity Framework for Everyone). It defines
the SPIFFE ID format (`spiffe://trust-domain/path`) and the Workload API for identity delivery
to workloads without requiring application changes.

**SPIRE Server** is the central identity authority. It issues X.509-SVIDs and JWT-SVIDs to
attested workloads, stores registration entries (which workloads get which identities), and
manages the trust domain's CA. Deployed as a StatefulSet for data persistence.

**SPIRE Agent** runs on every node as a DaemonSet. It performs node attestation with the server,
then performs local workload attestation. It exposes the Workload API via a Unix domain socket
that workloads connect to for receiving their SVIDs.

**SPIFFE CSI Driver** is a CSI ephemeral volume plugin. It mounts the agent's Workload API
socket directly into workload pods, eliminating the need for hostPath mounts and reducing the
security surface.

**SPIRE Controller Manager** is a Kubernetes controller running as a sidecar to spire-server.
It watches pods and auto-registers workloads based on ClusterSPIFFEID CRs, removing the need
for manual registration entry management.

**OIDC Discovery Provider** exposes a JWKS endpoint so external systems (AWS IAM, GCP, Vault)
can validate JWT-SVIDs issued by this trust domain.

How they interact:

- Agent contacts Server for node attestation and SVID issuance
- CSI Driver mounts the Agent's Workload API socket into application pods
- Controller Manager watches Kubernetes pods and auto-creates registration entries on the Server
- OIDC Provider reads the Server's JWT signing keys and serves them over HTTPS via an OpenShift Route

## Code Map

This section describes the major directories and important types.
Use symbol search to find the mentioned entities by name.

### `cmd/zero-trust-workload-identity-manager/`

Single entry point. The `main.go` file wires up the controller-runtime manager, registers all
controllers, and starts the process. All image references and feature flags come in through
environment variables.

### `api/v1alpha1/`

CRD type definitions for all five custom resources: `ZeroTrustWorkloadIdentityManager`,
`SpireServer`, `SpireAgent`, `SpiffeCSIDriver`, `SpireOIDCDiscoveryProvider`.

All types embed `CommonConfig` for shared fields (labels, resources, affinity, tolerations,
nodeSelector). Status is tracked through `ConditionalStatus` with standard condition types.

**Architecture Invariant:** all CRs are cluster-scoped singletons that must be named `"cluster"`.
There is exactly one instance of each kind per cluster. This is validated in the controllers.

### `pkg/controller/<name>/`

One controller per CRD kind. Each follows the same structural pattern:

- `New(mgr)` — creates the reconciler with a custom client, logger, and event recorder
- `SetupWithManager(mgr)` — registers watches with label-filtered predicates
- `Reconcile()` — fetches the CR, reconciles sub-resources, updates status conditions

The five controllers are: `spire-server`, `spire-agent`, `spiffe-csi-driver`,
`spire-oidc-discovery-provider`, and `zero-trust-workload-identity-manager`.

**Architecture Invariant:** controllers never communicate with each other directly. The
`zero-trust-workload-identity-manager` controller aggregates operand health by watching all
other operand CRs and computing a top-level status. There is no shared mutable state between
controllers beyond what the Kubernetes API server provides.

### `pkg/client/`

`CustomCtrlClient` wraps the controller-runtime client with retry logic and convenience methods
like `CreateOrUpdateObject` and `Exists`. `NewCacheBuilder` configures label-selector-scoped
informers to limit the operator's watch scope to only resources it manages.

**Architecture Invariant:** all controllers use `CustomCtrlClient`, never the raw
controller-runtime client directly. This ensures consistent retry behavior and scoped watches.

### `pkg/controller/status/`

The status `Manager` accumulates conditions during reconciliation and computes aggregate
`Ready` and `Degraded` states for any operand CR.

**Architecture Invariant:** status is computed from conditions alone, never from directly
inspecting the runtime state of child resources (pod readiness, etc.). Conditions are the
single source of truth for health reporting.

### `pkg/controller/utils/`

Shared utilities consumed by all controllers: constants (controller names, image environment
variable names, asset paths), label generators following the `app.kubernetes.io/*` convention,
validation helpers, proxy configuration, resource comparison, and ownership utilities.

### `bindata/`

Embedded YAML manifests for static resources — RBAC ClusterRoles, ClusterRoleBindings,
ServiceAccounts, Services, CSIDriver objects. Loaded via go-bindata at runtime and applied
by controllers.

**Architecture Invariant:** resources that have a fixed, configuration-independent shape live
in bindata as YAML. Only resources whose shape depends on CR spec fields are constructed
programmatically in Go code.

### `pkg/version/`

Operator and component version strings. Injected at build time via ldflags.

### `config/`

Kustomize-based deployment configuration:

- `config/crd/bases/` — generated CRD manifests
- `config/rbac/` — operator's own RBAC
- `config/manager/` — operator Deployment
- `config/samples/` — example CRs for each kind
- `config/manifests/` — OLM ClusterServiceVersion base
- `config/prometheus/` — ServiceMonitor for metrics

### `test/e2e/`

End-to-end tests that run against a real OpenShift cluster. These create CRs and verify the
full stack comes up healthy.

## Cross-Cutting Concerns

### Reconciliation Pattern

Every controller follows the same flow: fetch the singleton CR, reconcile each sub-resource
in sequence using `CreateOrUpdateObject` (which handles create-vs-update idempotently), then
set status conditions reflecting success or failure of each step.

### Status Conditions

All operand CRs use standard condition types: `Ready`, `Degraded`, `Upgradeable`. The top-level
`ZeroTrustWorkloadIdentityManager` controller aggregates conditions from all operand CRs into
a single status, which is also reflected in the OLM `OperatorCondition`.

### Image Configuration

All container images are specified via environment variables (`SPIRE_SERVER_IMAGE`,
`SPIRE_AGENT_IMAGE`, `SPIFFE_CSI_DRIVER_IMAGE`, etc.). The operator never hardcodes image
references. This allows the images to be overridden by the cluster version operator or by
CI without rebuilding.

### Build System

The project uses Kubebuilder v4 scaffolding with Operator SDK extensions. Key build facts:

- `make manifests` regenerates CRDs and RBAC from code markers
- `make generate` regenerates DeepCopy methods
- `make build` runs the full cycle (manifests + generate + fmt + vet + compile)
- FIPS-compliant builds are supported via `hack/build-fips.sh`
- OLM bundle is generated with `make bundle`

### Testing

- Unit tests use controller-runtime's envtest (`make test`)
- E2E tests run against a live cluster with a 45-minute timeout (`make test-e2e`)
- Coverage-instrumented builds are available for E2E coverage collection
