# Claude Instructions for Zero Trust Workload Identity Manager

## Agents

Specialized agents are available in `.claude/agents/`:
- **solve.md** - For solving bugs, issues, and problems
- **develop.md** - For implementing new features and enhancements
- **review.md** - For code review and quality checks
- **test.md** - For writing and maintaining tests

## Project Overview

Zero Trust Workload Identity Manager (ZTWIM) is an OpenShift Day-2 operator written in Go that automates the deployment and lifecycle management of SPIFFE/SPIRE components. It enables zero-trust security by dynamically issuing and rotating workload identities.

## Technology Stack

- **Language**: Go 1.23+
- **Framework**: controller-runtime (Kubernetes operator SDK)
- **Build System**: Make + Go modules (vendored dependencies)
- **Target Platform**: OpenShift 4.19+
- **Module Path**: `github.com/openshift/zero-trust-workload-identity-manager`

## Repository Structure

```
├── api/v1alpha1/           # CRD type definitions
│   ├── conditions.go       # Condition type/reason constants
│   ├── meta.go             # Shared status types
│   └── *_types.go          # Individual CR type definitions
├── bindata/                # Static Kubernetes manifests (embedded at build)
├── cmd/                    # Main entry point
├── config/                 # Kustomize manifests, CRD bases, samples
├── pkg/
│   ├── client/             # Custom controller-runtime client
│   ├── controller/         # All controller implementations
│   │   ├── zero-trust-workload-identity-manager/  # Main aggregating controller
│   │   ├── spire-server/   # SpireServer controller
│   │   ├── spire-agent/    # SpireAgent controller
│   │   ├── spiffe-csi-driver/  # SpiffeCSIDriver controller
│   │   ├── spire-oidc-discovery-provider/  # OIDC provider controller
│   │   ├── status/         # Status management utilities
│   │   └── utils/          # Constants, validation, shared utilities
│   ├── operator/           # Bindata asset loading
│   └── version/            # Build version info
├── test/e2e/               # End-to-end tests
└── vendor/                 # Vendored dependencies
```

## Key Components Managed

| Component | Kind | Description |
|-----------|------|-------------|
| SPIRE Server | StatefulSet | Identity server with embedded controller-manager |
| SPIRE Agent | DaemonSet | Node-level workload attestation |
| SPIFFE CSI Driver | DaemonSet | Mounts SVID credentials to pods |
| OIDC Discovery Provider | Deployment | JWT-SVID validation endpoint |

## Essential Commands

```bash
# Build
make build-operator    # Fast build (binary only)
make build             # Full build with code generation
make all               # Build + verify

# Test
make test              # Unit tests with envtest
make test-e2e          # E2E tests (requires cluster)
make lint              # Run golangci-lint

# Code Generation (after API changes)
make generate          # Regenerate DeepCopy methods
make manifests         # Regenerate CRDs and RBAC

# Deploy
make install           # Install CRDs to cluster
make deploy            # Full deployment
make run               # Run locally against cluster
```

## Key Patterns

### Singleton Resources

All CRs are singletons named "cluster":
- `ZeroTrustWorkloadIdentityManager/cluster`
- `SpireServer/cluster`
- `SpireAgent/cluster`
- `SpiffeCSIDriver/cluster`
- `SpireOIDCDiscoveryProvider/cluster`

### Controller Reconciliation Pattern

```go
func (r *MyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. Get the CR
    var cr v1alpha1.MyCR
    if err := r.ctrlClient.Get(ctx, req.NamespacedName, &cr); err != nil {
        if kerrors.IsNotFound(err) {
            return ctrl.Result{}, nil
        }
        return ctrl.Result{}, err
    }

    // 2. Set initial status (Ready=False, Progressing)
    status.SetInitialReconciliationStatus(ctx, r.ctrlClient, &cr,
        func() *v1alpha1.ConditionalStatus { return &cr.Status.ConditionalStatus },
        "MyCR")

    // 3. Create status manager with deferred apply
    statusMgr := status.NewManager(r.ctrlClient)
    defer func() {
        statusMgr.ApplyStatus(ctx, &cr,
            func() *v1alpha1.ConditionalStatus { return &cr.Status.ConditionalStatus })
    }()

    // 4. Get parent ZTWIM CR and set owner reference
    // 5. Check create-only mode
    // 6. Validate configuration
    // 7. Reconcile child resources
    
    return ctrl.Result{}, nil
}
```

### Status Conditions

Use `status.Manager` for condition updates:
```go
statusMgr.AddCondition(conditionType, reason, message, metav1.ConditionTrue/False)
```

Standard reasons: `v1alpha1.ReasonReady`, `ReasonFailed`, `ReasonInProgress`

### Create-Only Mode

Check before updating existing resources:
```go
if utils.IsInCreateOnlyMode() {
    // Skip updates, only create if not exists
}
```

## Kubebuilder Markers

When adding API fields:
```go
// +kubebuilder:validation:Required
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:Pattern=`^[a-z0-9-]+$`
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="field is immutable"
```

After changes: `make generate manifests bundle`

## Testing

- Unit tests use envtest (simulated K8s API)
- Test files colocated: `foo.go` → `foo_test.go`
- Run: `make test`

## Important Notes

1. **Namespace**: Operator runs in `zero-trust-workload-identity-manager`
2. **FIPS**: Build uses FIPS-compliant Go via `hack/go-fips.sh`
3. **Vendored deps**: Use `go mod vendor` after dependency changes
4. **Owner references**: All operand CRs are owned by ZeroTrustWorkloadIdentityManager
5. **Immutable fields**: `trustDomain`, `clusterName`, `bundleConfigMap`

## Terminology

| Term | Meaning |
|------|---------|
| ZTWIM | Zero Trust Workload Identity Manager |
| SPIRE | SPIFFE Runtime Environment |
| SPIFFE | Secure Production Identity Framework for Everyone |
| SVID | SPIFFE Verifiable Identity Document |
| Trust Domain | Security boundary for SPIFFE identities |
| Operand | Component managed by the operator |

