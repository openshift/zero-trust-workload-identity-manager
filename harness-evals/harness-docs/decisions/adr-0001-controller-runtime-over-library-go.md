# ADR-0001: Use controller-runtime Instead of library-go

**Status**: Accepted
**Date**: 2025-01-01
**Deciders**: ZTWIM team
**Component**: ZTWIM

## Context

OpenShift operators historically use `openshift/library-go` for reconciliation, status reporting, and resource management. ZTWIM needed to choose between library-go and controller-runtime (Kubebuilder/Operator SDK) for its operator framework.

## Decision

Use `sigs.k8s.io/controller-runtime` with Kubebuilder v4 / Operator SDK conventions instead of `openshift/library-go`.

## Rationale

- ZTWIM is a Day-2 operator managing upstream SPIFFE/SPIRE components, not a core platform operator
- controller-runtime provides a cleaner separation for CRD-driven reconciliation with kubebuilder markers
- Upstream SPIRE controller-manager already uses controller-runtime, making API type integration natural
- Operator SDK tooling (bundle, scorecard, CSV generation) integrates directly with controller-runtime

## Consequences

### Positive
- Native kubebuilder marker support for CRD validation (CEL XValidation)
- Standard `client.Client` patterns with easy cache configuration
- Direct compatibility with SPIRE controller-manager API types

### Negative
- Cannot use library-go's `StaticResourceController`, `resourceapply`, or `OperatorStatus` patterns
- Must implement custom status management (`pkg/controller/status/`) and retry wrappers (`CustomCtrlClient`)
- Different patterns from most core OpenShift operators — contributors familiar with library-go must adapt

## Alternatives Considered

### library-go
**Description**: Use `openshift/library-go` operator framework with `StaticResourceController` and `resourceapply` patterns.
**Rejected because**: Adds complexity for a Day-2 operator; poor fit with upstream SPIRE API types and controller-runtime-based operand ecosystem.

## References

- `go.mod`: `sigs.k8s.io/controller-runtime v0.22.4`
- `cmd/zero-trust-workload-identity-manager/main.go`: controller-runtime manager setup
- `pkg/client/client.go`: custom wrapper over controller-runtime client

---
**SME Review Recommended**: Confirm original team rationale for this choice.
