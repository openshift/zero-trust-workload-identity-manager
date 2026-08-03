# Upstream SPIFFE CRDs

| API Group | Version | Scope | Source |
|-----------|---------|-------|--------|
| `spire.spiffe.io` | `v1alpha1` | Cluster | [spire-controller-manager](https://github.com/spiffe/spire-controller-manager) |

## Purpose

Three upstream CRDs extend SPIRE identity registration beyond what the ZTWIM operand CRs configure. They are **not** ZTWIM-owned singletons — each can have multiple instances with arbitrary names.

ZTWIM installs the CRD definitions and deploys **spire-controller-manager** as a sidecar with the SPIRE Server. The controller-manager reconciles these CRs into SPIRE registration entries on the server.

## CRD Inventory

| Kind | Plural | Managed By | ZTWIM Creates? |
|------|--------|------------|----------------|
| `ClusterSPIFFEID` | `clusterspiffeids` | spire-controller-manager (reconcile) + ZTWIM OIDC controller (two instances) | Yes — two operator-managed instances |
| `ClusterFederatedTrustDomain` | `clusterfederatedtrustdomains` | spire-controller-manager | No — user-created |
| `ClusterStaticEntry` | `clusterstaticentries` | spire-controller-manager | No — user-created |

## Ownership Boundaries

```text
User / Platform                    ZTWIM Operator                    spire-controller-manager
─────────────────                  ──────────────                    ────────────────────────
SpireServer CR  ──────────────►    StatefulSet + CM config    ──►  sidecar watches CRs
SpireAgent CR   ──────────────►    DaemonSet
ClusterSPIFFEID (workload)  ─────────────────────────────────────►  creates SPIRE entries
ClusterSPIFFEID (operator)  ◄──  OIDC controller creates 2   ──►  creates SPIRE entries
ClusterFederatedTrustDomain ─────────────────────────────────────►  federation bundle sync
ClusterStaticEntry          ─────────────────────────────────────►  static SPIRE entries
```

**Key distinction**: ZTWIM operand controllers manage Kubernetes resources (StatefulSet, DaemonSet, etc.). spire-controller-manager translates `spire.spiffe.io` CRs into SPIRE server registration entries. Do not conflate the two reconciliation loops.

## spire-controller-manager Configuration

ZTWIM configures the sidecar via the SPIRE Server ConfigMap (`pkg/controller/spire-server/configmap.go`):

| Setting | Value | Effect |
|---------|-------|--------|
| `ClassName` | `zero-trust-workload-identity-manager-spire` | Only CRs with matching `spec.className` are reconciled |
| `WatchClassless` | `false` | CRs without a className are ignored |
| `EntryIDPrefix` | ZTWIM `spec.clusterName` | Prefixes SPIRE entry IDs |
| `Reconcile.ClusterSPIFFEIDs` | `true` | Watches ClusterSPIFFEID |
| `Reconcile.ClusterFederatedTrustDomains` | `true` | Watches ClusterFederatedTrustDomain |
| `Reconcile.ClusterStaticEntries` | `true` | Watches ClusterStaticEntry |

Ignored namespaces include `kube-system`, `kube-public`, `local-path-storage`, and `openshift-*`.

## ClusterSPIFFEID

Maps Kubernetes workload selectors to SPIFFE ID templates. This is the primary mechanism for granting workloads SVIDs.

### Operator-Managed Instances

The SpireOIDCDiscoveryProvider controller creates two fixed-name ClusterSPIFFEIDs (`pkg/controller/spire-oidc-discovery-provider/clusterspiffeid.go`):

| Name | Purpose |
|------|---------|
| `zero-trust-workload-identity-manager-spire-oidc-discovery-provider` | SVID for OIDC discovery provider pods in the operator namespace |
| `zero-trust-workload-identity-manager-spire-default` | Fallback ID template (`spec.fallback: true`) for workloads outside the operator namespace |

Both use `spec.className: zero-trust-workload-identity-manager-spire`. Do not hand-edit these — the OIDC controller reconciles them.

### User-Created Instances

Users create additional ClusterSPIFFEIDs for application workloads. Example (schema-valid; the sample in `config/samples/` uses deprecated field shapes):

```yaml
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterSPIFFEID
metadata:
  name: example-workload
spec:
  className: zero-trust-workload-identity-manager-spire  # required for this ZTWIM installation
  spiffeIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}"
  namespaceSelector:
    matchLabels:
      kubernetes.io/metadata.name: example-ns
  podSelector:
    matchLabels:
      app: example-workload
  dnsNameTemplates:
  - "*.example.com"
  ttl: 3600s
```

**Important**: User-created ClusterSPIFFEIDs must set `spec.className` to `zero-trust-workload-identity-manager-spire` or spire-controller-manager will ignore them.

## ClusterFederatedTrustDomain

Declares a remote trust domain whose bundle SPIRE should fetch and trust. Used for SPIRE federation beyond what `SpireServer.spec.federation` configures at the server level.

Example (schema-valid; includes required `bundleEndpointURL`):

```yaml
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterFederatedTrustDomain
metadata:
  name: example.com
spec:
  trustDomain: example.com
  bundleEndpointURL: https://spire.example.com/bundle
  bundleEndpointProfile:
    type: https_web
```

ZTWIM does not create these — users or platform teams manage them. spire-controller-manager syncs bundles to the SPIRE Server.

## ClusterStaticEntry

Creates a SPIRE registration entry with explicit selectors, bypassing the dynamic Kubernetes workload selector model. Useful for non-Kubernetes workloads or legacy integrations.

Example (schema-valid):

```yaml
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterStaticEntry
metadata:
  name: example-static-entry
spec:
  parentID: "spiffe://example.com/spire/server"
  spiffeID: "spiffe://example.com/example-workload"
  selectors:
  - "unix:uid:1000"
  x509SVIDTTL: 3600s
```

## Relationship to ZTWIM Operand CRs

| Concern | Use This |
|---------|----------|
| Deploy SPIRE Server / Agent / CSI / OIDC | ZTWIM operand CRs (`operator.openshift.io/v1alpha1`) |
| Grant a K8s workload a SPIFFE ID | User-created `ClusterSPIFFEID` |
| Trust a remote trust domain's bundle | `ClusterFederatedTrustDomain` |
| Register a non-K8s workload | `ClusterStaticEntry` |
| Configure federation endpoint on this cluster | `SpireServer.spec.federation` |
| OIDC provider's own identity | Operator-managed `ClusterSPIFFEID` (automatic) |

## Common Mistakes

- **Creating ClusterSPIFFEID without `className`** — spire-controller-manager ignores classless CRs when `WatchClassless: false`.
- **Editing operator-managed ClusterSPIFFEIDs** — the OIDC controller reconciles `zero-trust-workload-identity-manager-spire-*` instances; manual changes are overwritten.
- **Expecting ZTWIM to create workload ClusterSPIFFEIDs** — only the two OIDC-related instances are operator-managed; application IDs are user-created.
- **Confusing SpireServer federation config with ClusterFederatedTrustDomain** — `SpireServer.spec.federation` configures this cluster's bundle endpoint; `ClusterFederatedTrustDomain` declares remote domains to trust.

See also: [SpireServer](spire-server.md) for federation and CA config; [SpireOIDCDiscoveryProvider](spire-oidc-discovery-provider.md) for OIDC-managed ClusterSPIFFEIDs; [Installation / Bootstrap](../ZTWIM_DEVELOPMENT.md#installation--bootstrap).
