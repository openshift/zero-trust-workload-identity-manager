# Zero Trust Workload Identity Manager (ZTWIM) - Agentic Documentation

**Component**: Zero Trust Workload Identity Manager
**Repository**: openshift/zero-trust-workload-identity-manager

> **Retrieval-first**: Before generating code, read the relevant `ai-docs/` files. Start with `domain/` for API types, then `architecture/components.md` for patterns, then `ZTWIM_DEVELOPMENT.md` for workflow.
>
> **Generic Platform Patterns**: See Platform documentation (openshift/enhancements/ai-docs/) for operator patterns, testing practices, security guidelines, and cross-repo ADRs.

## What is ZTWIM?

A controller-runtime operator (not library-go) that deploys and manages upstream SPIFFE/SPIRE components on OpenShift 4.18+ so workloads get dynamic, rotatable identities (JWT-SVID / X.509-SVID). GA since v1.0.0; published to `stable-v1` channel via `redhat-operators` CatalogSource.

**Key Principle**: The operator does not embed upstream code — it manages upstream resources as static YAML manifests (bindata) applied imperatively, deploying upstream container images via `RELATED_IMAGE_*` env vars.

## Core Components

| Controller | Purpose | Key Resources |
|---|---|---|
| **ZTWIM** | Status aggregator | Watches all operand CRs + OperatorCondition, syncs Upgradeable to OLM |
| **SpireServer** | SPIRE server lifecycle | StatefulSet, ConfigMaps, Routes, webhooks, federation, SPIFFE CRs |
| **SpireAgent** | SPIRE agent lifecycle | DaemonSet, custom SCC, ConfigMap, ClusterRole |
| **SpiffeCSIDriver** | CSI driver lifecycle | DaemonSet, CSIDriver, privileged SCC RoleBinding |
| **SpireOIDCDiscoveryProvider** | OIDC discovery | Deployment, Route, ClusterSPIFFEID, ConfigMap |

**Quick Start**: `oc get zerotrustworkloadidentitymanagers cluster -o yaml` | `oc get spireservers,spireagents,spiffecsidrivers,spireoidcdiscoveryproviders`

## Critical Patterns

**1. Never hand-edit generated files** — `zz_generated.deepcopy.go`, `config/crd/bases/*.yaml`, `pkg/operator/assets/bindata.go`. Use `make generate`, `make manifests`, `make update-bindata`. Run `make verify` before pushing.

**2. All five ZTWIM-owned CRDs are cluster-scoped singletons named `cluster`** — enforced by CEL XValidation. The three upstream `spire.spiffe.io` CRDs (ClusterSPIFFEID, ClusterFederatedTrustDomain, ClusterStaticEntry) are NOT singletons — see [`ai-docs/domain/upstream-spiffe-crds.md`](ai-docs/domain/upstream-spiffe-crds.md). Controller names use the full prefix: `zero-trust-workload-identity-manager-<component>-controller` (constants in `pkg/controller/utils/constants.go`).

**3. Operand controllers watch the parent ZTWIM CR** — all four operand controllers watch `ZeroTrustWorkloadIdentityManager` with `ZTWIMSpecChangedPredicate`, and managed resources with component-specific label predicates (`ControllerManagedResourcesForComponent`). The ZTWIM controller does NOT create operand CRs — it only aggregates status.

**4. Vendor is tracked** — after dependency changes, always `make vendor` and commit the vendor directory. The build uses `-mod=vendor`.

## Key Build Commands

| Command | Purpose |
|---|---|
| `make build` | Full build (manifests + generate + fmt + vet + binary) |
| `make test` | Unit tests with envtest |
| `make verify` | vet + fmt + golangci-lint |
| `make manifests generate update-bindata` | Regenerate all generated files |

## Documentation Structure

```text
ai-docs/
├── domain/                    # CRD type docs (ZTWIM, SpireServer, SpireAgent, etc.)
│   └── upstream-spiffe-crds.md
├── architecture/              # Repo layout, controller table, reconciliation flow
│   └── components.md
├── decisions/                 # Component ADRs (controller-runtime choice, bindata)
│   ├── adr-0001-*.md
│   └── adr-template.md
├── exec-plans/
│   └── README.md              # Pointer to Platform guidance
├── references/
│   ├── ecosystem.md           # Links to Platform
│   └── enhancements.md        # Enhancement proposals & upstream refs
├── ZTWIM_DEVELOPMENT.md       # Build, workflow, common tasks, env vars
└── ZTWIM_TESTING.md           # Unit (FakeCustomCtrlClient) + E2E (Ginkgo)
```

**AI Agent Path**: [`ai-docs/README.md`](ai-docs/README.md) → domain/ → architecture/ → ZTWIM_DEVELOPMENT.md → decisions/

**Platform Patterns**: [Operator](https://github.com/openshift/enhancements/tree/master/ai-docs) | [Testing](https://github.com/openshift/enhancements/tree/master/ai-docs) | [Security](https://github.com/openshift/enhancements/tree/master/ai-docs)

## Upstream Operands

| Upstream | Integration |
|---|---|
| [spire](https://github.com/spiffe/spire) | Server (StatefulSet), agent (DaemonSet), OIDC (Deployment) images |
| [spire-controller-manager](https://github.com/spiffe/spire-controller-manager) | Go module dep for API types; sidecar with SPIRE server |
| [spiffe-csi](https://github.com/spiffe/spiffe-csi) | CSI driver DaemonSet image |

**Release**: Managed via [zero-trust-workload-identity-manager-release](https://github.com/openshift/zero-trust-workload-identity-manager-release) (Konflux, dual-branch model: `release-*` for images, `main` for FBC catalogs).

## External References

- [SPIFFE Specification](https://github.com/spiffe/spiffe) | [SPIRE Docs](https://spiffe.io/docs/latest/spire-about/) | [OpenShift Docs](https://docs.openshift.com/)

---

**Platform Documentation**: openshift/enhancements/ai-docs/
