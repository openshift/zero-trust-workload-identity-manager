# Enhancement Proposals & Design Docs

Catalog of design documentation relevant to ZTWIM.

## OpenShift Enhancement Proposals

All three proposals are merged to `master` in `openshift/enhancements` under `enhancements/workload-identity-management/`:

| Title | File | Tracking | Created |
|---|---|---|---|
| Zero Trust Workload Identity Manager | `zero-trust-workload-identity-manager.md` | OCPSTRAT-1691 | 2025-07-02 |
| SPIRE Federation Support | `spire-federation-support.md` | SPIRE-211 | 2025-10-16 |
| OIDC Routes Integration | `oidc-routes-integration.md` | OCPSTRAT-1691 | 2025-08-08 |

## Local Design Docs

No local design docs in the component repo. Feature designs follow the OpenShift enhancement proposal process.

## Upstream References

| Project | Documentation |
|---|---|
| SPIFFE Specification | [github.com/spiffe/spiffe](https://github.com/spiffe/spiffe) |
| SPIRE Documentation | [spiffe.io/docs/latest/spire-about/](https://spiffe.io/docs/latest/spire-about/) |
| SPIRE Controller Manager | [github.com/spiffe/spire-controller-manager](https://github.com/spiffe/spire-controller-manager) |
| SPIFFE CSI Driver | [github.com/spiffe/spiffe-csi](https://github.com/spiffe/spiffe-csi) |

## Enhancement vs ADR

- **Enhancement proposals** are feature designs (often cross-component), tracked in `openshift/enhancements`
- **ADRs** are component architectural decisions, tracked in `ai-docs/decisions/`
- Don't conflate them
