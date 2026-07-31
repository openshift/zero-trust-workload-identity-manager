# Architecture: Components

> Zero Trust Workload Identity Manager (ZTWIM) — a **controller-runtime** operator managing SPIFFE/SPIRE on OpenShift 4.18+.

## 1. Repository Layout

```
cmd/zero-trust-workload-identity-manager/main.go   ← entry point, scheme registration, controller wiring
api/v1alpha1/                                       ← CRD types + conditions (kubebuilder markers)
  zz_generated.deepcopy.go                          ← DO NOT EDIT — controller-gen output
  conditions.go                                     ← shared condition constants (Ready, Degraded, Upgradeable)
  spire_server_config_types.go                      ← SpireServer CRD spec/status
  spire_agent_config_types.go                       ← SpireAgent CRD spec/status
  spiffe_csi_config_types.go                        ← SpiffeCSIDriver CRD spec/status
pkg/controller/
  zero-trust-workload-identity-manager/controller.go ← top-level aggregator, OLM OperatorCondition sync
  spire-server/controller.go                        ← StatefulSet, ConfigMaps, RBAC, Routes, Webhook
  spire-agent/controller.go                         ← DaemonSet, ConfigMap, SCC, RBAC
  spiffe-csi-driver/controller.go                   ← DaemonSet, CSIDriver, ServiceAccount, privileged SCC
  spire-oidc-discovery-provider/controller.go       ← Deployment, ConfigMap, Route, ClusterSPIFFEID
  utils/                                            ← shared utilities (see §6)
  status/status.go                                  ← condition aggregation, workload health checks
pkg/client/client.go                                ← CustomCtrlClient wrapper, unified cache builder
pkg/client/fakes/fake_custom_ctrl_client.go         ← counterfeiter-generated test double
pkg/operator/assets/bindata.go                      ← go-bindata generated (DO NOT EDIT)
pkg/version/version.go                              ← operator + component versions, ldflags injection
bindata/                                            ← raw YAML manifests decoded at runtime
  spire-server/         (6 files)  ← ClusterRole, ClusterRoleBinding, ServiceAccount, Service, external-cert RBAC
  spire-agent/          (4 files)  ← ClusterRole, ClusterRoleBinding, ServiceAccount, Service
  spiffe-csi/           (3 files)  ← CSIDriver, ServiceAccount, privileged RoleBinding
  spire-controller-manager/ (6 files) ← ClusterRole, ClusterRoleBinding, leader-election Role/RoleBinding, webhook Service, ValidatingWebhookConfig
  spire-bundle/         (2 files)  ← Role, RoleBinding
  spire-oidc-discovery-provider/ (4 files) ← ServiceAccount, Service, external-cert Role/RoleBinding
config/crd/bases/                                   ← controller-gen output CRDs (5 operator CRDs + 3 SPIFFE CRDs)
hack/go-fips.sh                                     ← FIPS build wrapper (GOEXPERIMENT=strictfipsruntime)
Makefile                                            ← build, test, lint, bindata, bundle targets
Dockerfile                                          ← multi-stage: rhel-9-golang-1.25 builder → ubi9-minimal
.golangci.yml                                       ← 21 linters enabled; dupl/lll excluded for pkg/
```

## 2. Controller Framework

**Framework:** `sigs.k8s.io/controller-runtime` v0.22.4 — NOT library-go.

Key characteristics:
- Reconcilers implement `reconcile.Reconciler` via `Reconcile(ctx, ctrl.Request)`
- Wiring via `ctrl.NewControllerManagedBy(mgr).For().Watches().Complete(r)`
- Leader election: `LeaderElectionID: "24a59323.operator.openshift.io"` (`main.go:221`)
- API rate limits: QPS=50, Burst=100 (`main.go:191-192`)

## 3. Controller Inventory

| Controller | Primary CR | Workload | Apply Method | Reconcile Order | Code Ref |
|---|---|---|---|---|---|
| ZTWIM | `ZeroTrustWorkloadIdentityManager` | — (aggregator) | status-only | aggregates 4 operands → sets Ready/Upgradeable | `pkg/controller/zero-trust-workload-identity-manager/controller.go` |
| SpireServer | `SpireServer` | StatefulSet | create-or-update | validate → SA → Service → RBAC → Webhook → ConfigMaps(×3) → StatefulSet → Route | `pkg/controller/spire-server/controller.go` |
| SpireAgent | `SpireAgent` | DaemonSet | create-or-update | validate → SA → Service → RBAC → SCC → ConfigMap → DaemonSet | `pkg/controller/spire-agent/controller.go` |
| SpiffeCSIDriver | `SpiffeCSIDriver` | DaemonSet | create-or-update | validate → SA → CSIDriver → privileged RoleBinding → DaemonSet | `pkg/controller/spiffe-csi-driver/controller.go` |
| SpireOIDCDP | `SpireOIDCDiscoveryProvider` | Deployment | create-or-update | validate → SA → Service → ClusterSPIFFEID → ConfigMap → Deployment → ext-cert RBAC → Route | `pkg/controller/spire-oidc-discovery-provider/controller.go` |

All operand controllers are **singleton** — they only reconcile `name: "cluster"`. The ZTWIM controller watches all four operand CRs via `Watches()` with `operandStatusChangedPredicate`.

## 4. Reconciliation Flow (8-step, verified from code)

Every operand controller follows this sequence (SpireServer shown; others are subsets):

1. **Fetch CR** — `Get(ctx, req.NamespacedName, &server)`, return nil on NotFound
2. **Initialize status manager** — `status.NewManager(r.ctrlClient)`, defer `ApplyStatus()`
3. **Fetch ZTWIM parent** — `Get(ctx, {Name: "cluster"}, &ztwim)`, fail if missing
4. **Set owner reference** — `controllerutil.SetControllerReference(&ztwim, &server, r.scheme)` only if `NeedsOwnerReferenceUpdate()` returns true; persist with `Update()`
5. **Check CREATE_ONLY_MODE** — `handleCreateOnlyMode()` reads env var, sets `CreateOnlyMode` condition
6. **Validate configuration** — affinity/tolerations/nodeSelector/resources/labels via `ValidateAndUpdateStatus()`; proxy via `ValidateProxyConfiguration()`; controller-specific (JWT URL, TTL, federation)
7. **Reconcile sub-resources** — ordered sequence of `reconcileServiceAccount()`, `reconcileService()`, `reconcileRBAC()`, etc. Each: decode bindata → mutate → `Exists()` check → `Create()` or `ResourceNeedsUpdate()` → `UpdateWithRetry()`
8. **Check workload health** — `CheckStatefulSetHealth()` / `CheckDaemonSetHealth()` / `CheckDeploymentHealth()` sets the workload-available condition; `SetReadyCondition()` auto-aggregates

### Reconcile Return Semantics

| Scenario | Return | Effect |
|---|---|---|
| Primary CR `IsNotFound` | `(Result{}, nil)` | Stop, no requeue |
| Parent ZTWIM `IsNotFound` | `(Result{}, nil)` | Stop, set `Ready=False/Failed` |
| Validation failure | `(Result{}, nil)` | Stop, set condition, wait for spec change |
| API transient error | `(Result{}, err)` | Requeue with exponential backoff |
| Sub-reconciler failure | `(Result{}, err)` | Requeue, condition already set by sub-reconciler |
| Successful reconciliation | `(Result{}, nil)` | Done |

### Status Condition Rules

1. Create `status.NewManager(r.ctrlClient)` early in `Reconcile`.
2. **Always** `defer statusMgr.ApplyStatus(...)` — status writes happen even if reconciliation returns early.
3. Each sub-reconciler calls `statusMgr.AddCondition(...)` for its component-specific condition type.
4. `ApplyStatus` auto-derives `Ready` from all other conditions unless `Ready` was explicitly set.
5. Status is only written when conditions actually changed (semantic equality check).
6. `ApplyStatus` errors are logged but never propagated — the reconciliation result is not affected.

Condition conventions:
- Types: `Ready`, `Degraded`, `Upgradeable` (global); per-resource types like `StatefulSetAvailable`, `ConfigMapAvailable`.
- Reasons: `Failed`, `Ready`, `Progressing`, `OperandsNotReady`, `ResourceConflict`.

### NotFound Handling: Primary vs Dependent

| Resource role | IsNotFound behavior |
|---|---|
| **Primary CR** (the CR this controller owns) | `return (Result{}, nil)` — CR deleted, nothing to do |
| **Parent ZTWIM CR** | Set `Ready=False/Failed`, `return (Result{}, nil)` — cannot proceed |
| **Dependent resource** (managed SA, ConfigMap, etc.) | Treat as "needs creation" — proceed to `Create` |
| **Operand CR** (in ZTWIM aggregator) | Record as `OperandMessageCRNotFound`, classify as progressing |
| **OperatorCondition** (OLM) | Log and continue — operator may run outside OLM |

## 5. Wiring & Registration (`main.go`)

```
main() → NewCacheBuilder() → ctrl.NewManager(config, opts{NewCache: cacheBuilder})
       → for each controller: New(mgr) → SetupWithManager(mgr)
       → mgr.Start(ctrl.SetupSignalHandler())
```

Scheme registration order:
1. `clientgoscheme` (init)
2. `operatoropenshiftiov1alpha1` (init)
3. `securityv1`, `ctrlmgr` (spire-controller-manager), `routev1`, `operatorv1` (main)

The unified `NewCacheBuilder()` (`pkg/client/client.go:224`) configures label-filtered informers for managed resources using `app.kubernetes.io/managed-by=zero-trust-workload-identity-manager`, preventing cache races between manager and reconciler. The cache sets `ReaderFailOnMissingInformer: true` — if a controller reads a type not registered in the informer list, the read fails immediately instead of silently starting a cluster-wide informer. Always add new types to `informerResources` before reading them.

## 6. Shared Utilities (`pkg/controller/utils/`)

| File | Key Exports | Purpose |
|---|---|---|
| `constants.go` | `ZeroTrustWorkloadIdentityManager*ControllerName`, `RELATED_IMAGE_*` env names, `ServiceCAAnnotationKey`, asset path constants | Centralized constants |
| `utils.go` | `Decode*ObjBytes()` (×8 types), `IsInCreateOnlyMode()`, `GenerateConfigHash()`, `SetLabel()`, `ZTWIMSpecChangedPredicate`, `OwnerReferenceChangedPredicate`, `GenerationOrOwnerReferenceChangedPredicate`, `NeedsOwnerReferenceUpdate()` | Bindata decoders, predicates, helpers |
| `resource_comparison.go` | `ResourceNeedsUpdate()`, `*NeedsUpdate()` (13 type-specific: Service, SA, ClusterRole, ClusterRoleBinding, Role, RoleBinding, CSIDriver, ValidatingWebhookConfig, SCC, ClusterSPIFFEID, StatefulSet, Deployment, DaemonSet) | Drift detection — compares desired vs. existing, ignoring K8s-managed fields |
| `errors.go` | `ReconcileError{Reason, Message, Err}`, `NewIrrecoverableError()`, `NewRetryRequiredError()`, `NewMultipleInstanceError()`, `FromClientError()`, `IsIrrecoverableError()` | Error classification: irrecoverable (403/401/400) vs retry-required |
| `relatedImages.go` | `GetSpireServerImage()`, `GetSpireAgentImage()`, `GetSpiffeCSIDriverImage()`, `GetSpireOIDCDiscoveryProviderImage()`, `GetSpireControllerManagerImage()`, `GetNodeDriverRegistrarImage()`, `GetSpiffeCsiInitContainerImage()` | `RELATED_IMAGE_*` env var accessors |
| `labels.go` | `StandardizedLabels()`, `Spire*Labels()` (5 component helpers), `ControllerManagedResourcesForComponent()`, component constants (`ComponentCSI`, `ComponentControlPlane`, `ComponentNodeAgent`, `ComponentDiscovery`) | K8s recommended labels + component-scoped watch predicates |
| `validation.go` | `ValidateAndUpdateStatus()`, `ValidateCommonConfig*()` (Affinity, Tolerations, NodeSelector, Resources, Labels) | Uses k8s.io/kubernetes internal validation APIs |
| `proxy.go` | `GetProxyEnvVars()`, `IsProxyEnabled()`, `ValidateProxyConfiguration()`, `AddProxyConfigToPod()`, `GetTrustedCABundleVolume()` | Cluster-wide proxy support (OLM-injected HTTP_PROXY/HTTPS_PROXY) |
| `resource_ownership.go` | `CheckResourceConflict()`, `HandleCreateConflict()`, `IsResourceConflictOnCreate()` | Detects naming conflicts with pre-existing unmanaged resources |
| `url_validation.go` | `IsValidURL()`, `NormalizeURL()`, `StripProtocolFromJWTIssuer()` | JWT issuer URL validation |

### Error Classification (`ReconcileError`)

| Reason | Meaning | When to use |
|---|---|---|
| `IrrecoverableError` | Retrying will not help | RBAC denied, bad request, invalid spec, service unavailable |
| `RetryRequiredError` | Transient failure, retry likely to succeed | Conflict, timeout, not-found on dependent resources |
| `MultipleInstanceError` | Singleton constraint violated | Multiple CRs exist where only `cluster` is allowed |

`FromClientError(err, msg, args...)` auto-classifies Kubernetes API errors:

| API error type | Classification |
|---|---|
| `Unauthorized`, `Forbidden`, `Invalid`, `BadRequest`, `ServiceUnavailable` | Irrecoverable |
| `NotFound`, `Conflict`, `Timeout`, `ServerTimeout`, all others | Retry required |

All constructors return `nil` when passed a `nil` error — safe to call without nil-checking.

### AlreadyExists / Resource Conflict Pattern

When `Create` returns `IsAlreadyExists`, the resource exists outside the operator's label-filtered cache (a pre-existing resource with the same name). Use `HandleCreateConflict` to set a `ResourceConflict` condition and return the error for requeue.

## 7. Resource Management: Bindata Decode → Mutate → Apply

Static resources (RBAC, ServiceAccounts, CSIDriver, Services, Webhooks) follow:

```
bindata YAML → Decode*ObjBytes() → set namespace/labels/ownerRef → Exists() check
  → if !exists: Create()
  → if exists && !createOnlyMode && ResourceNeedsUpdate(): UpdateWithRetry()
```

Dynamic resources (StatefulSet, DaemonSet, Deployment, ConfigMap) are built programmatically in Go, not from bindata. ConfigMap changes trigger workload restarts via config-hash annotations:
- `ztwim.openshift.io/spire-server-config-hash`
- `ztwim.openshift.io/spire-controller-manager-config-hash`
- `ztwim.openshift.io/spire-agent-config-hash`
- `ztwim.openshift.io/spire-oidc-discovery-provider-config-hash`

## 8. Image Resolution

Images are resolved from `RELATED_IMAGE_*` environment variables set by OLM from the CSV (`pkg/controller/utils/relatedImages.go`, `constants.go:57-63`):

| Env Var | Component |
|---|---|
| `RELATED_IMAGE_SPIRE_SERVER` | spire-server container |
| `RELATED_IMAGE_SPIRE_AGENT` | spire-agent container |
| `RELATED_IMAGE_SPIFFE_CSI_DRIVER` | spiffe-csi-driver container |
| `RELATED_IMAGE_SPIRE_OIDC_DISCOVERY_PROVIDER` | OIDC discovery provider container |
| `RELATED_IMAGE_SPIRE_CONTROLLER_MANAGER` | spire-controller-manager sidecar |
| `RELATED_IMAGE_NODE_DRIVER_REGISTRAR` | node-driver-registrar sidecar |
| `RELATED_IMAGE_SPIFFE_CSI_INIT_CONTAINER` | CSI init container (default: `registry.access.redhat.com/ubi9:latest`) |

## 9. Feature Toggles

**`CREATE_ONLY_MODE`** (env var, `utils.go:237-255`): When `"true"` (case-insensitive), controllers create resources that don't exist but skip updates to existing resources. Each controller checks via `handleCreateOnlyMode()` and sets a `CreateOnlyMode` status condition. The ZTWIM aggregator reflects this to the OLM `OperatorCondition.Upgradeable` — setting `Upgradeable=False` when create-only mode is active.

## 10. OpenShift Integrations

- **Routes** — `routev1.Route` for SPIRE Server federation and OIDC discovery provider (`spire-server/routes.go`, `spire-oidc-discovery-provider/routes.go`)
- **SecurityContextConstraints** — custom `spire-agent` SCC (`spire-agent/scc.go`); `privileged` SCC for spiffe-csi-driver via RoleBinding (`spiffe-csi-driver/rbac.go`)
- **Service CA** — annotation `service.beta.openshift.io/serving-cert-secret-name` on spire-server Service for automatic TLS cert (`constants.go:53-54`)
- **OLM OperatorCondition** — syncs `Upgradeable` condition to `operatorv1.OperatorCondition` (`controller.go:573-636`); reads `OPERATOR_CONDITION_NAME` env var. **OLMv1 caveat**: `OPERATOR_NAMESPACE` can be replaced with [Kubernetes downward API](https://kubernetes.io/docs/tasks/inject-data-application/environment-variable-expose-pod-information/) (`fieldRef: metadata.namespace`). `OperatorCondition` for blocking upgrades has no OLMv1 equivalent yet — a different API is expected in a future OLMv1 phase
- **Proxy** — inherits cluster-wide proxy config (HTTP_PROXY, HTTPS_PROXY, NO_PROXY) injected by OLM; mounts trusted CA bundle ConfigMap (`proxy.go`)

## 11. Naming Conventions

- **Controller names:** `zero-trust-workload-identity-manager-{component}-controller` (`constants.go:6-10`)
- **K8s labels:** `app.kubernetes.io/{name,instance,part-of,component,managed-by,version}` (`labels.go:27-43`)
- **Component values:** `control-plane`, `node-agent`, `csi`, `discovery`
- **CR names:** always `cluster` (singleton pattern)
- **Config hash annotations:** `ztwim.openshift.io/{component}-config-hash`
- **Condition types:** PascalCase, e.g., `StatefulSetAvailable`, `ConfigurationValid`, `CreateOnlyMode`

## 12. Generated Code Inventory

| Generator | Output | Trigger |
|---|---|---|
| `controller-gen object` | `api/v1alpha1/zz_generated.deepcopy.go` | `make generate` |
| `controller-gen rbac crd webhook` | `config/crd/bases/*.yaml` | `make manifests` |
| `go-bindata` | `pkg/operator/assets/bindata.go` | `make update-bindata-assets` |
| `counterfeiter` | `pkg/client/fakes/fake_custom_ctrl_client.go` | `go generate` on `client.go:92-93` |

## 13. FIPS Compliance

`hack/go-fips.sh` enables FIPS mode when the Go compiler supports it:
- Sets `GOEXPERIMENT=strictfipsruntime`
- Adds build tags: `-tags=strictfipsruntime,openssl`
- Build target `build-operator` sources this script (`Makefile:145`)
- Dockerfile uses `CGO_ENABLED=1` (required for OpenSSL/FIPS)

## 14. Anti-Patterns

1. **DO NOT** use library-go `staticresourcecontroller` — this operator uses controller-runtime exclusively
2. **DO NOT** edit `zz_generated.deepcopy.go` or `bindata.go` — they are generated; run `make generate` / `make update-bindata-assets`
3. **DO NOT** create resources without `app.kubernetes.io/managed-by=zero-trust-workload-identity-manager` labels — the cache builder filters on this label (`client.go:225-229`)
4. **DO NOT** hardcode image references — always use `RELATED_IMAGE_*` env var accessors from `relatedImages.go`
5. **DO NOT** skip owner reference checks — use `NeedsOwnerReferenceUpdate()` before `SetControllerReference()` to avoid unnecessary updates
6. **DO NOT** compare full K8s objects for drift — use `ResourceNeedsUpdate()` which ignores server-set fields (ClusterIP, healthCheckNodePort, etc.)
7. **DO NOT** use `Update()` for status subresource — use `StatusUpdateWithRetry()` which handles resource version conflicts
8. **DO NOT** create multiple CR instances — all CRs are singletons named `cluster`; `MultipleInstanceError` is a classified error type
9. **DO NOT** add watches without component-scoped predicates — use `ControllerManagedResourcesForComponent()` to prevent cross-controller reconciliation noise
10. **DO NOT** bypass `CREATE_ONLY_MODE` — every reconcile function that modifies resources must check the `createOnlyMode` flag

## 15. Security Patterns

**Container security**: Operator runs as non-root UID 65532:65532 on `ubi9-minimal`. Operand containers set `ReadOnlyRootFilesystem: true` and use `EmptyDir` for writable paths.

**Metrics**: Binds to `:8443` with TLS; `--enable-http2=false` mitigates HTTP/2 rapid-reset. `filters.WithAuthenticationAndAuthorization` enforces RBAC on `/metrics`.

**RBAC scoping**: Operator RBAC uses `resourceNames` restrictions wherever possible. All operand CRs are restricted to the singleton name `cluster` via `resourceNames`.

**Workload attestor verification** (`SpireAgent.spec.workloadAttestorsVerification.type`):

| Type | Behavior | Use |
|---|---|---|
| `skip` | `skip_kubelet_verification: true` | Development / testing only |
| `auto` | Default CA at `/etc/kubernetes/kubelet-ca.crt` | Standard OpenShift clusters |
| `hostCert` | Requires explicit `hostCertBasePath` + `hostCertFileName` (CEL-enforced) | Custom PKI |

## 16. API Call Reduction Layers

| Layer | Mechanism | Avoids |
|---|---|---|
| Cache label selector | `managed-by` filter on informer | Watching unrelated resources cluster-wide |
| Component predicate | `ControllerManagedResourcesForComponent` | Cross-controller reconcile storms |
| Generation predicate | `GenerationChangedPredicate` | Reconciling on status-only writes |
| Status predicate | `operandStatusChangedPredicate` | ZTWIM reconciling on operand spec changes |
| `ResourceNeedsUpdate` | Type-specific field comparison | No-op Update API calls |
| Status semantic equality | `equality.Semantic.DeepEqual` on status | No-op Status Update API calls |
| Retry wrappers | `RetryOnConflict` | Failed writes due to stale resourceVersion |
| Create-only mode | Skip all Updates | All update writes post-creation |

---

**SME Review Recommended**: Implementation recipes for adding new components, anti-patterns from institutional knowledge, and rationale behind pattern choices may be incomplete from automated discovery
