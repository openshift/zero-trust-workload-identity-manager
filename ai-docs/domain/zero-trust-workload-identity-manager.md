# ZeroTrustWorkloadIdentityManager

| Field     | Value                                              |
|-----------|----------------------------------------------------|
| API Group | `operator.openshift.io`                            |
| Version   | `v1alpha1`                                         |
| Kind      | `ZeroTrustWorkloadIdentityManager`                 |
| Scope     | `Cluster`                                          |
| Source    | `api/v1alpha1/zero_trust_workload_identity_manager_types.go` |

## Purpose

Top-level singleton CRD that defines the trust domain and cluster identity for the ZTWIM operator. It acts as the global configuration knob — not for low-level SPIRE tuning, but for the identity parameters that all operands inherit. The operator reconciles this CR, watches the four operand CRs (SpireServer, SpireAgent, SpiffeCSIDriver, SpireOIDCDiscoveryProvider), and aggregates their status. It does NOT create operand CRs — those must be created separately.

## Key Principle

**One cluster, one identity root.** The CR is enforced as a singleton (`metadata.name` must be `cluster`). The `trustDomain` and `clusterName` fields are immutable after creation — changing them would invalidate every SVID in the cluster.

## Spec Structure

```go
type ZeroTrustWorkloadIdentityManagerSpec struct {
    TrustDomain     string  // Immutable. SPIFFE trust domain (e.g., "example.org").
    ClusterName     string  // Immutable. DNS-1123 subdomain identifying this cluster.
    BundleConfigMap string  // Immutable. ConfigMap name for the SPIRE trust bundle. Default: "spire-bundle".
}
```

### Validation Rules

- `trustDomain`: required, 1–255 chars, pattern `^[a-z0-9]([a-z0-9\-\.]*[a-z0-9])?$`, immutable.
- `clusterName`: required, 1–63 chars, DNS-1123 subdomain, immutable.
- `bundleConfigMap`: optional (default `spire-bundle`), 1–253 chars, immutable.

## Status Structure

```go
type ZeroTrustWorkloadIdentityManagerStatus struct {
    ConditionalStatus               // inline — []metav1.Condition
    Operands []OperandStatus        // list-map keyed by `kind`
}

type OperandStatus struct {
    Name       string               // Typically "cluster"
    Kind       string               // Enum: SpireServer | SpireAgent | SpiffeCSIDriver | SpireOIDCDiscoveryProvider
    Ready      string               // "true" or "false"
    Message    string               // Human-readable detail
    Conditions []metav1.Condition   // Per-operand conditions
}
```

Status aggregates health from all four operand CRs into a single view. The `Operands` list is keyed by `kind` (since all operands are named `cluster`).

## Key Concepts

- **Trust Domain**: The SPIFFE trust domain root (e.g., `example.org`). All SPIFFE IDs in the cluster start with `spiffe://<trustDomain>/`. Immutable — changing it requires a full re-bootstrap.
- **Bundle ConfigMap**: The operator creates and maintains a ConfigMap containing root CA certificates for the trust domain. Workloads and federated peers consume this bundle.
- **Operand Aggregation**: The status acts as a dashboard — if any operand's `Ready` is `"false"`, the top-level `Ready` condition reflects that.

## Lifecycle

1. **Create**: User applies the CR with `metadata.name: cluster`. Operator validates trust domain format and immutability constraints.
2. **Reconcile**: Operator watches existing operand CRs and aggregates their status into `status.operands[]` and top-level conditions.
3. **Status Roll-up**: Operator reads each operand CR's Ready condition and aggregates into OperandsAvailable, Ready, and syncs Upgradeable to OLM OperatorCondition.
4. **Update**: None of the spec fields can be changed after creation — all three (`trustDomain`, `clusterName`, `bundleConfigMap`) are immutable via CEL validation.
5. **Delete**: Operand CRs with owner references are garbage-collected when this CR is deleted.

## Conditions

| Type        | Reasons                                 | Meaning                                        |
|-------------|----------------------------------------|-------------------------------------------------|
| Ready       | Ready, Progressing, Failed             | All operands deployed and healthy               |
| Degraded    | Failed                                 | Irrecoverable error (e.g., permission issue)    |
| Upgradeable | Ready, OperandsNotReady                | Safe to upgrade when all existing operands ready |

## Example YAML

```yaml
apiVersion: operator.openshift.io/v1alpha1
kind: ZeroTrustWorkloadIdentityManager
metadata:
  name: cluster
spec:
  trustDomain: example.org
  clusterName: prod-east-1
  bundleConfigMap: spire-bundle
```

## Common Mistakes

- **Naming it anything other than `cluster`** — CEL validation rejects it at admission.
- **Trying to change `trustDomain` after creation** — immutable field; you must delete and recreate.
- **Assuming this CR configures SPIRE internals** — it does not. Low-level SPIRE config lives in SpireServer and SpireAgent CRs.
- **Ignoring `status.operands`** — the top-level `Ready` condition alone doesn't tell you *which* component is unhealthy.
```

---
