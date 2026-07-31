# Shared Types and Conditions

| Source Files | `api/v1alpha1/meta.go`, `api/v1alpha1/conditions.go` |
|--------------|------------------------------------------------------|

## Purpose

Defines the shared type vocabulary used across all ZTWIM CRDs — the condition model for status reporting, the common pod-level configuration, and the object reference type.

## ConditionalStatus

```go
// meta.go
type ConditionalStatus struct {
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

Embedded inline in every operand's status struct. Provides the standard Kubernetes conditions model with `type`, `status`, `reason`, `message`, `lastTransitionTime`, and `observedGeneration`.

Every CRD status (`SpireServerStatus`, `SpireAgentStatus`, `SpiffeCSIDriverStatus`, `SpireOIDCDiscoveryProviderStatus`) embeds `ConditionalStatus` via JSON inline.

All operand types implement `GetConditionalStatus() ConditionalStatus` for generic condition access.

## Condition Constants

```go
// conditions.go

// Condition types
Degraded    = "Degraded"      // Irrecoverable error (e.g., permission issues)
Ready       = "Ready"         // Operand deployed and functioning
Upgradeable = "Upgradeable"   // Safe to upgrade

// Condition reasons
ReasonFailed           = "Failed"            // Reconciliation failed
ReasonReady            = "Ready"             // Successfully deployed and ready
ReasonInProgress       = "Progressing"       // Reconciliation in progress
ReasonOperandsNotReady = "OperandsNotReady"  // Some operands not yet ready
ReasonResourceConflict = "ResourceConflict"  // Resource ownership conflict
```

### Condition Semantics

| Type        | Status=True                              | Status=False                        |
|-------------|------------------------------------------|-------------------------------------|
| Ready       | Operand is fully deployed and healthy    | Operand is progressing or failed    |
| Degraded    | Irrecoverable failure state              | Normal operation                    |
| Upgradeable | All existing operand CRs are ready       | Unsafe to upgrade (not-ready or CreateOnlyMode) |

**Upgradeable** has a nuance: CRs that don't exist yet are considered OK. Only *existing* operand CRs must be Ready for Upgradeable to be True.

## CommonConfig

```go
// zero_trust_workload_identity_manager_types.go
type CommonConfig struct {
    Labels       map[string]string                // Max 64 labels. Applied to all managed resources.
    Resources    *corev1.ResourceRequirements      // CPU/memory requests and limits.
    Affinity     *corev1.Affinity                  // Pod scheduling affinity rules.
    Tolerations  []*corev1.Toleration              // Max 50 tolerations.
    NodeSelector map[string]string                 // Max 50 selectors.
}
```

Embedded inline in `SpireServerSpec`, `SpireAgentSpec`, `SpiffeCSIDriverSpec`, and `SpireOIDCDiscoveryProviderSpec`. Provides uniform pod scheduling and resource management across all operands.

## OperandStatus

```go
type OperandStatus struct {
    Name       string               // Resource name (typically "cluster")
    Kind       string               // Enum: SpireServer | SpireAgent | SpiffeCSIDriver | SpireOIDCDiscoveryProvider
    Ready      string               // "true" | "false"
    Message    string               // Human-readable detail (max 32768 chars)
    Conditions []metav1.Condition   // Per-operand conditions
}
```

Used only in `ZeroTrustWorkloadIdentityManagerStatus.Operands[]`. Provides a per-operand health summary keyed by `kind` (since all operands are named `cluster`).

## ObjectReference

```go
type ObjectReference struct {
    Name  string   // Resource name
    Kind  string   // Resource kind (optional)
    Group string   // API group (optional)
}
```

Generic reference type for pointing to other Kubernetes resources. Used internally by the operator for cross-resource references.

## Singleton Pattern

All five CRDs enforce `metadata.name == 'cluster'` via CEL validation rules:

```
+kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'"
```

This means there can only ever be one instance of each CRD. The operator relies on this invariant for reconciliation — it always looks up the `cluster` resource.

## Immutability Pattern

Several fields use CEL immutability rules:

```
+kubebuilder:validation:XValidation:rule="self == oldSelf",message="field is immutable"
```

Applied to: `trustDomain`, `clusterName`, `bundleConfigMap` (on ZeroTrustWorkloadIdentityManager), `persistence.size/accessMode/storageClass` (on SpireServer), and federation `profile` (on SpireServer).
```
All six domain concept documents are above, delimited by `
