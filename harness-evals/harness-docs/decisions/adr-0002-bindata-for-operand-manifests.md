# ADR-0002: Use go-bindata for Operand Manifest Embedding

**Status**: Accepted
**Date**: 2025-01-01
**Deciders**: ZTWIM team
**Component**: ZTWIM

## Context

The operator needs to deploy static Kubernetes resource manifests (ServiceAccounts, Services, RBAC, webhook configs) for each SPIRE operand. These manifests are mostly static YAML with runtime mutations (namespace, labels, owner refs).

**Scope**: This ADR is component-specific.

## Decision

Use `go-bindata` (via `openshift/build-machinery-go` make targets) to compile YAML templates from `bindata/` into `pkg/operator/assets/bindata.go`. Controllers decode at runtime via `assets.MustAsset(path)`.

## Rationale

- Consistent with OpenShift operator conventions (`build-machinery-go` bindata targets)
- Manifests are version-controlled YAML, easy to review in PRs
- Runtime decode + mutate pattern allows merging `CommonConfig` (labels, affinity, tolerations) from CR spec
- No Helm dependency, no template engine — pure Go decode and struct manipulation

## Consequences

### Positive
- YAML templates are readable and diffable in PRs
- No external templating dependency
- `MustAsset` panics on missing assets — catches build-time errors early

### Negative
- `bindata.go` is generated and large — must never be hand-edited
- Adding a new manifest requires: add YAML → add constant → `make update-bindata` → commit generated file
- No conditional logic in YAML itself — all conditions handled in Go controller code

## Alternatives Considered

### Go embed (//go:embed)
**Description**: Use Go 1.16+ embed directive instead of go-bindata.
**Rejected because**: build-machinery-go provides the bindata pipeline with OpenShift CI integration; switching adds migration cost with no functional benefit.

### Helm charts
**Description**: Use Helm to template operand manifests.
**Rejected because**: Adds Helm runtime dependency; operator already has full control via Go struct mutation.

## References

- `bindata/` — source YAML templates organized by component
- `pkg/operator/assets/bindata.go` — generated (NEVER hand-edit)
- `Makefile`: `$(call add-bindata,assets,...)` target
- `pkg/controller/utils/constants.go` — asset path constants
