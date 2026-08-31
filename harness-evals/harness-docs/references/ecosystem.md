# Platform Ecosystem References

This document links to generic OpenShift/Kubernetes patterns in the Platform ecosystem hub. ZTWIM inherits these platform-wide patterns and practices.

## Operator Patterns

**Location**: [openshift/enhancements/ai-docs/platform/operator-patterns/](https://github.com/openshift/enhancements/tree/master/ai-docs)

- **Controller Runtime**: Reconciliation loops, event handling, client patterns
- **Status Conditions**: Available, Progressing, Degraded condition semantics
- **Finalizers**: Resource cleanup patterns
- **RBAC**: Service account and permissions

**Component Usage**:
- ZTWIM uses controller-runtime (not library-go) with a custom `CustomCtrlClient` wrapper
- Status conditions follow `metav1.Condition` with auto-derived `Ready` via `status.Manager`
- RBAC is generated from kubebuilder markers on controller files

## Testing Practices

**Location**: [openshift/enhancements/ai-docs/practices/testing/](https://github.com/openshift/enhancements/tree/master/ai-docs)

- **Test Pyramid**: Unit > Integration > E2E ratio
- **E2E Framework**: OpenShift E2E test patterns

**Component Usage**:
- See `ZTWIM_TEST_IMPLEMENTATION.md` for component-specific test suites; `ZTWIM_TEST_PLAN_HARNESS.md` for ADR-driven test planning
- Unit tests use counterfeiter-generated fakes with envtest for Kubernetes API server
- E2E tests use Ginkgo v2 + Gomega against live OpenShift clusters

## Security Practices

**Location**: [openshift/enhancements/ai-docs/practices/security/](https://github.com/openshift/enhancements/tree/master/ai-docs)

- **RBAC Guidelines**: Role and ClusterRole design

**Component Usage**:
- Operator runs as non-root (UID 65532) on UBI minimal
- FIPS-compliant builds via `hack/go-fips.sh` (strictfipsruntime)
- Metrics secured via TLS with OpenShift service CA
- Custom SCCs for SPIRE agent; privileged SCC RoleBinding for CSI driver

## Kubernetes Fundamentals

**Location**: [openshift/enhancements/ai-docs/domain/kubernetes/](https://github.com/openshift/enhancements/tree/master/ai-docs)

- **CRDs**: CustomResourceDefinition patterns

**Component Usage**:
- Five cluster-scoped singleton CRDs (operator.openshift.io/v1alpha1)
- Three upstream CRDs (spire.spiffe.io/v1alpha1) installed and managed

## OpenShift Fundamentals

**Location**: [openshift/enhancements/ai-docs/domain/openshift/](https://github.com/openshift/enhancements/tree/master/ai-docs)

- **OLM**: Operator Lifecycle Manager integration

**Component Usage**:
- OLM bundle with CSV, alpha channel
- `OperatorCondition` synced for Upgradeable status
- OpenShift Routes for SPIRE federation and OIDC discovery
- SecurityContextConstraints for privileged operand workloads

## Cross-Repository ADRs

**Location**: [openshift/enhancements/ai-docs/decisions/](https://github.com/openshift/enhancements/tree/master/ai-docs)

**Component-Specific ADRs**: See `ai-docs/decisions/` for ZTWIM-specific decisions.

---

**Note**: These links point to Platform (ecosystem hub) documentation. Component-specific patterns and decisions are documented in the `ai-docs/` directory of this repository.
