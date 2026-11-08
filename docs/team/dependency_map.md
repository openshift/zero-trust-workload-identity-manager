# Dependency Map

## Upstream Dependencies (What We Consume)

### SPIFFE/SPIRE Project

We consume upstream container images — we do not build SPIRE from source.

| Component | Version | Source |
|-----------|---------|--------|
| SPIRE Server | 1.14.7 | `ghcr.io/spiffe/spire-server` |
| SPIRE Agent | 1.14.7 | `ghcr.io/spiffe/spire-agent` |
| SPIFFE CSI Driver | 0.2.8 | `ghcr.io/spiffe/spiffe-csi-driver` |
| SPIRE Controller Manager | 0.6.4 | `ghcr.io/spiffe/spire-controller-manager` |
| SPIRE OIDC Discovery Provider | 1.14.7 | `ghcr.io/spiffe/oidc-discovery-provider` |

### Go Libraries

| Dependency | Version | Purpose |
|------------|---------|---------|
| `sigs.k8s.io/controller-runtime` | v0.22.4 | Controller framework |
| `k8s.io/api` | v1.35.3 | Kubernetes API types |
| `k8s.io/apimachinery` | v1.35.3 | API machinery utilities |
| `k8s.io/client-go` | v1.35.3 | Kubernetes client |
| `github.com/openshift/api` | latest | OpenShift API types (Routes, SCCs) |
| `github.com/openshift/build-machinery-go` | latest | OpenShift build helpers (bindata) |
| `github.com/spiffe/spire-controller-manager` | v0.6.4 | ClusterSPIFFEID CRD types |
| `github.com/spiffe/go-spiffe/v2` | v2.6.0 | SPIFFE ID parsing (indirect) |

### Build Infrastructure

| Dependency | Purpose |
|------------|---------|
| OpenShift CI (Prow) | PR validation, merge gating |
| Konflux/Tekton | Stage and production image pipelines |
| Operator SDK v1.39.0 | Bundle generation, scorecard |
| OLM | Operator lifecycle on-cluster |

## Downstream Consumers (Who Depends On Us)

### OpenShift Service Mesh (OSSM)

OSSM consumes the SPIRE Agent's Workload API socket for mTLS identity in the service mesh
data plane. The integration point is the agent socket path
(`/run/spire/agent-sockets/spire-agent.sock`) mounted via the SPIFFE CSI Driver.

**Coordination required when**: changing the default socket path, agent DaemonSet labels,
or SDS configuration.

### Cluster Administrators

Cluster admins deploy and configure the operator via OLM. They create the singleton CRs
(`ZeroTrustWorkloadIdentityManager`, `SpireServer`, `SpireAgent`, etc.) to configure the
SPIRE stack for their cluster.

### OLM Catalog

The operator bundle is published to the OLM catalog via Konflux pipelines. Downstream
catalog builds consume our bundle image from `registry.stage.redhat.io` (stage) or
`registry.redhat.io` (production).

## Cross-Team Coordination

| Team | Integration Point | When to Coordinate |
|------|-------------------|-------------------|
| **OSSM** | SPIRE agent socket path, SDS config | Socket path changes, agent config changes |
| **OCP Release** | Release branch cuts, image mirroring | New release branches, version bumps |
| **Konflux/CPaaS** | Stage and production image pipelines | New components, registry path changes, label fixes |
| **Security/Compliance** | FIPS requirements, CVE response | Dependency updates, security patches |

## Impact Analysis Quick Reference

**If we change something, who might be affected?**

| Change Type | Affected Parties |
|-------------|-----------------|
| Agent socket path | OSSM, any workload using CSI driver volumes |
| CRD API fields | Cluster admins, documentation, OLM CSV |
| Operator image | Konflux pipeline, OLM catalog |
| Bundle metadata | OLM catalog, Konflux check-labels |
| Go dependency bump | CI build, FIPS compliance verification |
| SPIRE version bump | All operands, E2E tests, OSSM compatibility |

**If someone else changes something, are we affected?**

| External Change | Impact on Us |
|----------------|--------------|
| Upstream SPIRE release | Need to bump operand images, test compatibility |
| OpenShift API changes | May need to update Route/SCC handling |
| controller-runtime release | May need reconciler updates |
| Konflux pipeline changes | May affect stage/prod image builds |
| OCP release branch cut | Need corresponding release branch |
