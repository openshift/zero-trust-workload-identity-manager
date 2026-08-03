# ZTWIM Agent Documentation

Retrieval-first documentation for AI agents and contributors working on the Zero Trust Workload Identity Manager operator.

> **Start here**: Read [`../AGENTS.md`](../AGENTS.md) for critical patterns and build commands before generating code.

## Agent Path

Follow this order when working on a task:

1. **`domain/`** — CRD types, validation rules, field relationships
2. **`architecture/components.md`** — controller patterns, reconcile flow, shared utilities
3. **`ZTWIM_DEVELOPMENT.md`** — build workflow, bootstrap, common tasks
4. **`decisions/`** — architectural decisions (controller-runtime, bindata)
5. **`ZTWIM_TESTING.md`** — unit and E2E test patterns (when writing tests)

## Documentation Map

| Document | Purpose |
|----------|---------|
| [domain/zero-trust-workload-identity-manager.md](domain/zero-trust-workload-identity-manager.md) | Top-level singleton CR — trust domain, status aggregation |
| [domain/spire-server.md](domain/spire-server.md) | SPIRE Server CR — CA, persistence, federation, JWT issuer |
| [domain/spire-agent.md](domain/spire-agent.md) | SPIRE Agent CR — node/workload attestation, socket path |
| [domain/spiffe-csi-driver.md](domain/spiffe-csi-driver.md) | CSI Driver CR — Workload API socket delivery to pods |
| [domain/spire-oidc-discovery-provider.md](domain/spire-oidc-discovery-provider.md) | OIDC Discovery Provider CR — JWT-SVID validation for external systems |
| [domain/shared-types.md](domain/shared-types.md) | CommonConfig, conditions, singleton/immutability patterns |
| [domain/upstream-spiffe-crds.md](domain/upstream-spiffe-crds.md) | Upstream `spire.spiffe.io` CRDs and ZTWIM vs controller-manager ownership |
| [architecture/components.md](architecture/components.md) | Repository layout, controllers, reconcile flow, anti-patterns |
| [decisions/adr-0001-controller-runtime-over-library-go.md](decisions/adr-0001-controller-runtime-over-library-go.md) | Why controller-runtime instead of library-go |
| [decisions/adr-0002-bindata-for-operand-manifests.md](decisions/adr-0002-bindata-for-operand-manifests.md) | Why go-bindata for static operand manifests |
| [ZTWIM_DEVELOPMENT.md](ZTWIM_DEVELOPMENT.md) | Build, bootstrap, adding operands/bindata/CRD fields |
| [ZTWIM_TESTING.md](ZTWIM_TESTING.md) | Unit (FakeCustomCtrlClient) and E2E (Ginkgo) patterns |
| [references/ecosystem.md](references/ecosystem.md) | Links to Platform operator/testing/security docs |
| [references/enhancements.md](references/enhancements.md) | OpenShift enhancement proposals and upstream refs |
| [exec-plans/README.md](exec-plans/README.md) | Multi-step feature execution plans |

## Cross-Field Consistency

These fields must agree across CRs — see linked domain docs for details:

| Field | CRs |
|-------|-----|
| Socket path | [SpireAgent](domain/spire-agent.md) ↔ [SpiffeCSIDriver](domain/spiffe-csi-driver.md) |
| JWT issuer | [SpireServer](domain/spire-server.md) ↔ [SpireOIDCDiscoveryProvider](domain/spire-oidc-discovery-provider.md) |
| CSI plugin name | [SpiffeCSIDriver](domain/spiffe-csi-driver.md) ↔ [SpireOIDCDiscoveryProvider](domain/spire-oidc-discovery-provider.md) |

## Platform Documentation

Generic OpenShift operator patterns live in [openshift/enhancements/ai-docs/](https://github.com/openshift/enhancements/tree/master/ai-docs). ZTWIM-specific patterns stay in this directory.
