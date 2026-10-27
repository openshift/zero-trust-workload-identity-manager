# ZTWIM 1.2 GA Operand Version Proposal (OCP 5.0)

**Target release:** ZTWIM 1.2.0 GA  
**Target platform:** OpenShift Container Platform 5.0 (Kubernetes 1.36.2)  
**Baseline:** ZTWIM 1.1.0 GA operand versions  
**Date:** 2026-10-16 (updated for Konflux builder compatibility)

## Summary

For ZTWIM 1.2 GA on OCP 5.0, this proposal recommends **minor-line** operand bumps only (no major version jumps), staying within Go 1.26 and aligning with controller-runtime v0.24.1 and Kubernetes 1.36.x.

| Operand | ZTWIM 1.1 GA | **Proposed ZTWIM 1.2 GA** | Bump type |
|---------|--------------|---------------------------|-----------|
| spire-server | 1.14.7 | **1.15.3** | Minor (1.14 → 1.15) |
| spire-agent | 1.14.7 | **1.15.3** | Minor (1.14 → 1.15) |
| spire-oidc-discovery-provider | 1.14.7 | **1.15.3** | Minor (1.14 → 1.15) |
| spiffe-csi-driver | 0.2.8 | **0.2.13** | Patch (within 0.2.x) |
| spire-controller-manager | 0.6.4 | **0.7.0** | Minor (0.6 → 0.7) |
| csi-node-driver-registrar *(sidecar)* | v2.15.0 | **v2.17.0** | Minor (2.15 → 2.17) |

All proposed versions satisfy the OCP 5.0 constraints:

| Constraint | Status |
|------------|--------|
| Operand minor bump only (no major) | ✅ All bumps are minor or patch within same minor line |
| Go ≤ 1.26 (not 1.27) | ✅ All operands use Go 1.26.x |
| Kubernetes ≤ 1.36.2 | ✅ See [Kubernetes alignment notes](#kubernetes-alignment-notes) |
| controller-runtime ≤ v0.24.1 | ✅ SPIRE 1.15.3 and spire-controller-manager 0.7.0 use v0.24.1 |
| Konflux/RHEL builder buildable | ✅ See [Konflux builder compatibility](#konflux--rhel-builder-compatibility) |

---

## Constraint Context

### OCP 5.0 platform limits

- **Kubernetes:** 1.36.2 (max)
- **controller-runtime:** v0.24.1 (max; maps to k8s.io/* v0.36.x per [controller-runtime VERSIONING](https://github.com/kubernetes-sigs/controller-runtime/blob/main/VERSIONING.md))
- **Go toolchain:** max 1.26 (operands must not require Go 1.27)

### ZTWIM minor release policy

ZTWIM 1.2 is a minor release. Operand versions may advance by **one minor line** (e.g., SPIRE 1.14 → 1.15, spire-controller-manager 0.6 → 0.7) but must not jump major versions (e.g., SPIRE 1.x → 2.x is out of scope).

---

## Konflux / RHEL Builder Compatibility

Operand container images for ZTWIM are built in Konflux using the Red Hat OpenShift golang builder. Konflux builds typically set `GOTOOLCHAIN=local`, which means the builder's bundled Go version must be **≥ the `go` directive** in each operand's `go.mod`.

### Builder image in use

| Registry | Image tag | Go version (verified) |
|----------|-----------|----------------------|
| Brew (Konflux/product builds) | `brew.registry.redhat.io/rh-osbs/openshift-golang-builder:rhel_9_golang_1.26` | **1.26.7** (Red Hat 1.26.7-1.el9_8) |
| OCP CI (ZTWIM operator `Dockerfile`) | `registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.26-openshift-5.0` | Same Go 1.26.x line |

Red Hat publishes **minor-line floating tags** (e.g. `rhel_9_golang_1.26`), not patch-level tags. The tag is rebuilt when RHEL rebases the `golang` RPM; there is no separate `rhel_9_golang_1.26.9` tag. As of 2026-10-16, RHEL go-toolset is at **1.26.7**; upstream Go **1.26.9** has not yet been packaged into the builder.

### Build compatibility matrix

| Component | Proposed version | `go.mod` requires | Builder (1.26.7) | Buildable? |
|-----------|------------------|-------------------|------------------|------------|
| spire-server/agent/oidc | **1.15.3** | 1.26.6 | 1.26.7 | ✅ Yes |
| spire-server/agent/oidc | 1.15.4 *(deferred)* | 1.26.9 | 1.26.7 | ❌ No |
| spiffe-csi-driver | 0.2.13 | 1.26.5 | 1.26.7 | ✅ Yes |
| spire-controller-manager | 0.7.0 | 1.26.5 | 1.26.7 | ✅ Yes |
| csi-node-driver-registrar | v2.17.0 | 1.26.0 | 1.26.7 | ✅ Yes |
| ZTWIM operator | *(repo `go.mod`)* | 1.26.5 | 1.26.7 | ✅ Yes |

**Only SPIRE 1.15.4 is blocked** by the current builder. All other proposed operand versions build cleanly with `GOTOOLCHAIN=local` on Go 1.26.7.

### Why SPIRE 1.15.4 is deferred

SPIRE **1.15.4** (released 2026-10-15) requires `go 1.26.9` in its `go.mod`. The current RHEL/Konflux builder ships **Go 1.26.7**, so builds fail with:

```
go: go.mod requires go >= 1.26.9 (running go 1.26.7; GOTOOLCHAIN=local)
```

1.15.4 adds TLS listener policy configuration and two security fixes (PROXY protocol header validation, SDS v3 API crash on malformed `DiscoveryRequest`). These are valuable but not blocking for the 1.2 GA platform alignment goals. **SPIRE 1.15.3** (`go 1.26.6`) is proposed instead; it includes the same k8s/controller-runtime alignment (v0.36.3 / v0.24.1) and all 1.15.0–1.15.3 features/fixes.

**Future path:** When RHEL rebases `rhel_9_golang_1.26` to include golang 1.26.9+, SPIRE can be bumped to 1.15.4 in a subsequent ZTWIM release without a minor-line change.

---

## Proposed Operand Details

### 1. SPIRE (spire-server / spire-agent / spire-oidc-discovery-provider)

**Proposed version:** `1.15.3` (from `1.14.7`)

| Attribute | 1.14.7 (baseline) | 1.15.3 (proposed) |
|-----------|-------------------|-------------------|
| Go | 1.26.3 | 1.26.6 |
| k8s.io/* | v0.35.2 | v0.36.3 |
| controller-runtime | v0.23.3 | v0.24.1 |
| Latest in 1.15 line | — | 1.15.4 exists but is [deferred](#why-spire-1154-is-deferred) |

**Why 1.15.3 and not 1.15.4?**

- **1.15.4 is blocked** by the current Konflux builder (requires Go 1.26.9; builder has 1.26.7). See [Konflux builder compatibility](#konflux--rhel-builder-compatibility).
- 1.15.3 is the latest SPIRE version that builds with the available RHEL golang builder.
- Same k8s/controller-runtime alignment as 1.15.4 (v0.36.3 / v0.24.1).

**Why 1.15.x and not staying on 1.14.x?**

- 1.15 is the only available minor bump from 1.14.7.
- Upgrading brings controller-runtime to v0.24.1, required for OCP 5.0 / k8s 1.36 alignment.
- SPIRE 1.14.7 and 1.15.1 both contain the same `azure_imds` security fix; 1.15.3 includes all subsequent fixes and features through 1.15.3.
- **Konflux buildable:** `go 1.26.6` ≤ builder Go 1.26.7 ✅

#### Release notes: 1.14.7 → 1.15.3

##### [1.15.0] — 2026-05-19

**Added**
- `account_id` selector for `aws_iid` node attestor
- TLS support for Prometheus metrics sink
- X509-SVID prefetch control per registration entry
- Rootless Podman support in docker workload attestor
- PROXY protocol support for rate limiting behind load balancers
- Agent X509-SVID fetch for SPIFFE attestation mode from Workload API socket
- `iss` claim support for WIT-SVIDs
- Instance flag support for `spire-server` and `spire-agent` CLI
- Experimental `spiffe_id` node selector for node aliasing
- HashiCorp Vault Key Manager plugin

**Changed**
- Metric label renamed: `bootstraped` → `bootstrapped` (dashboards/alerts may need update)
- cosign updated to v3 major release
- Events-based cache authorized entry lookup performance improved
- `sigstore` support in k8s and docker attestors promoted out of experimental
- Azure SDK libraries updated to recent major versions
- Go upgraded to 1.26.3
- **Breaking (CLI JSON output):** Stop wrapping objects in slices when printing

**Fixed**
- Nil panic in `spire` upstream authority plugin
- Nil panic in `azure_imds` for instances without NSG
- `azure_key_vault` key manager now supports Azure Managed HSM
- Agent Debug service "unrecognized service" errors
- `aws_kms` plugin alias rotation issue

##### [1.15.1] — 2026-05-28

**Security**
- `azure_imds` node attestor PKCS7 validation fix (forged attested document acceptance)

**Changed**
- `golang.org/x/net` → v0.55.0, `golang.org/x/crypto` → v0.52.0

##### [1.15.2] — 2026-07-09

**Added**
- JTI claim inclusion configuration for JWT-SVIDs at entry level
- Experimental per-caller rate limiting for agent Workload API and Envoy SDS
- TLS for Prometheus metrics endpoint using SPIRE SVID with SPIFFE ID allowlist
- Optional client certificate IP verification in `x509pop` node attestor
- SPIFFE Broker endpoint, API, and documentation
- `disable_group_name_selectors` for Windows workload attestor
- `log_selectors` agent configuration
- Two additional PQC curves
- Tag-based key discovery in `aws_kms` Key Manager
- Logger service for `spire-agent`
- Configurable batch size for pruning attested nodes

**Security**
- Migrated `github.com/docker/docker` → `github.com/moby/moby` (CVE resolution)

**Changed**
- Deprecation warnings use dedicated log markers
- OIDC Discovery Provider warns on `allow_insecure_scheme`
- PQC policy applied to bundle endpoint and Prometheus server
- Bulk attested node fetch (reduced DB load)
- Pod/container IDs preferably from cgroup file
- MySQL list entries query optimized

**Fixed**
- `azure_imds` standalone VM attestation and signature validation
- Join-token alias cascade deletion on node delete/evict/prune
- CA journal survives transient datastore save failures
- Delegated API no longer serves JWT-SVIDs for admin/downstream entries
- Event cache reload preserves first/last event info
- Multiple agent/server plugin and Windows fixes

##### [1.15.3] — 2026-08-21

**Added**
- Slurm workload attestor
- `azure_blob` BundlePublisher plugin
- `trust_bundle_spiffe_workload_api` agent option (nested agent trust bundle bootstrap)
- `disable_workload_api` / `disable_sds_api` agent options
- `disable_kubelet_client` for `k8s` workload attestor
- `use_pod_uid_for_agent_id` for `k8s_psat` node attestor
- `enable_namespace_labels` for `k8s` workload attestor (`ns-label` selectors)
- `account_list_file` for `aws_iid` node attestor
- `debug getinfo` CLI command
- Incremental WIT-SVID support in `svid.v1` API client

**Changed**
- Broker API endpoint included in health checks when enabled
- Improved kubelet pod list coordination in `k8s` workload attestor
- `gcp_kms` no longer requires 36-char `key_identifier_value`
- Agent warns on `memory` Key Manager without re-attestation-capable node attestor
- AWS CA certificates updated in `aws_iid`
- Go upgraded to 1.26.6

**Fixed**
- Events-based cache fetches events ordered by ID
- Agent telemetry sinks start before node attestation
- Graceful shutdown during startup (no crash message / non-zero exit)
- `hashicorp_vault` Key Manager audit log warnings
- Grafana dashboard empty query targets

**References:** [SPIRE CHANGELOG](https://github.com/spiffe/spire/blob/main/CHANGELOG.md), [v1.15.3 tag](https://github.com/spiffe/spire/releases/tag/v1.15.3)

> **Deferred — SPIRE 1.15.4** (2026-10-15): TLS listener policy config, PROXY protocol security fix, SDS v3 crash fix, Go 1.26.9. Blocked by Konflux builder; revisit when `rhel_9_golang_1.26` is rebased to golang ≥ 1.26.9.

---

### 2. spiffe-csi-driver

**Proposed version:** `0.2.13` (from `0.2.8`)

| Attribute | 0.2.8 (baseline) | 0.2.13 (proposed) |
|-----------|------------------|-------------------|
| Go | (deps only) | 1.26.5 |
| k8s dependency | None (CSI-only) | None |
| Latest in 0.2 line | — | 0.2.13 (no 0.3.x GA as of 2026-10-16) |

**Why 0.2.13?**

- Stays within the 0.2.x minor line (no 0.3.x release exists).
- Includes a critical CSI idempotency fix (0.2.12) for kubelet restart scenarios.
- Latest security patches (Go 1.26.5, golang.org/x/net CVE fixes in 0.2.13).
- **Konflux buildable:** `go 1.26.5` ≤ builder Go 1.26.7 ✅

#### Release notes: 0.2.8 → 0.2.13

##### [0.2.9] — 2025-11-17
- Dependency updates

##### [0.2.10] — 2026-04-01
- Dependency updates

##### [0.2.11] — 2026-05-04
- Reduced per-call memory usage when checking CSI volume mounts (notable on nodes with many mounts)

##### [0.2.12] — 2026-06-19

**Fixed**
- `NodePublishVolume` returns success when target path is already mounted (prevents duplicate bind-mount accumulation on kubelet restart) — **important for OpenShift node stability**

**Changed**
- Go upgraded to 1.26.4

##### [0.2.13] — 2026-07-09

**Security**
- Go upgraded to 1.26.5 (CVE-2026-42505)
- `golang.org/x/net` → v0.55.0 (CVE-2026-33814 and related)

**Changed**
- Dependency updates

**References:** [spiffe-csi CHANGELOG](https://github.com/spiffe/spiffe-csi/blob/main/CHANGELOG.md), [v0.2.13 release](https://github.com/spiffe/spiffe-csi/releases/tag/v0.2.13)

---

### 3. spire-controller-manager

**Proposed version:** `0.7.0` (from `0.6.4`)

| Attribute | 0.6.4 (baseline) | 0.7.0 (proposed) |
|-----------|------------------|------------------|
| Go | 1.25.7 | 1.26.5 |
| k8s.io/* | v0.35.2 | v0.36.3 |
| controller-runtime | v0.22.4 | v0.24.1 |
| spire-api-sdk | v1.14.1 | v1.15.2 |

**Why 0.7.0 and not 0.6.6?**

- 0.7.0 is the latest minor release (0.6 → 0.7 is allowed for ZTWIM 1.2).
- Includes controller-runtime v0.24.1 migration (introduced in 0.6.5), which is **required** for OCP 5.0 alignment and ZTWIM operator compatibility.
- 0.6.4 is incompatible with controller-runtime ≥ v0.23.0 when used as a Go module dependency ([issue #665](https://github.com/spiffe/spire-controller-manager/issues/665)); ZTWIM must consume ≥ 0.6.5.
- 0.7.0 adds informer cache filtering for large-cluster memory optimization.
- **Konflux buildable:** `go 1.26.5` ≤ builder Go 1.26.7 ✅

> **Note:** The ZTWIM repo `go.mod` already references `spire-controller-manager v0.6.5`. The operand image should be bumped to `0.7.0` for 1.2 GA.

#### Release notes: 0.6.4 → 0.7.0

##### [0.6.5] — 2026-05-30

**Updated**
- Migrated controller-runtime to **v0.24.1** (#670) — critical for k8s 1.36 / OCP 5.0

**Security**
- Dependency CVE migrations (#680, #681)

##### [0.6.6] — 2026-06-04

**Fixed**
- Log configuration from config file not applied at startup (#667)

**Security**
- Go upgraded to 1.26.4 (`crypto/x509` security fixes) (#685)

##### [0.7.0] — 2026-07-25

**Added**
- `clusterSPIFFEIDLabelSelector` — restrict ClusterSPIFFEID informer cache to labeled objects
- `filterByClassName` — restrict informer cache to controller's own className (reduces memory in large clusters)

**Fixed**
- `gcInterval` of zero caused garbage collection loop to consume 100% CPU (#699)

**Updated**
- Go upgraded to 1.26.5 (#723)

**References:** [spire-controller-manager CHANGELOG](https://github.com/spiffe/spire-controller-manager/blob/main/CHANGELOG.md), [v0.7.0 release](https://github.com/spiffe/spire-controller-manager/releases/tag/v0.7.0)

---

### 4. csi-node-driver-registrar (sidecar)

**Proposed version:** `v2.17.0` (from `v2.15.0`)

| Attribute | v2.15.0 (baseline) | v2.17.0 (proposed) |
|-----------|--------------------|--------------------|
| Go | — | 1.26.0 |
| k8s.io/* | v0.34.x era | v0.36.1 |

**Why v2.17.0?**

- v2.17.0 explicitly bumps k8s dependencies to v0.36.1, aligning with OCP 5.0 Kubernetes 1.36.2.
- v2.15.0 predates k8s 1.36 and uses older client libraries.
- **Do not use v2.18.0** — it targets k8s v0.37.0, which exceeds the OCP 5.0 max of 1.36.2.
- **Konflux buildable:** `go 1.26.0` ≤ builder Go 1.26.7 ✅

#### Release notes: v2.15.0 → v2.17.0

##### v2.16.0 — 2026-02-13
- General dependency updates (k8s v0.35.x era)

##### v2.17.0 — 2026-05-21
- Bump k8s dependencies to **v1.36.1** / v0.36.1 (#577)
- gRPC and protobuf dependency updates

**References:** [node-driver-registrar CHANGELOG 2.17](https://github.com/kubernetes-csi/node-driver-registrar/blob/release-2.17/CHANGELOG/CHANGELOG-2.17.md)

---

## Kubernetes Alignment Notes

OCP 5.0 ships Kubernetes **1.36.2**. Several proposed operands compile against k8s.io/* **v0.36.3** (SPIRE 1.15.3, spire-controller-manager 0.7.0). This is a **patch-level** client library difference within the same Kubernetes minor version and is standard practice:

- controller-runtime v0.24.1 is tested against k8s.io/* v0.36.x
- Kubernetes client-go maintains backward compatibility within a minor release line
- v0.36.3 clients operate against a 1.36.2 API server without requiring 1.36.3 cluster features

Operands using k8s v0.36.1 (spire-controller-manager 0.6.6, csi-node-driver-registrar v2.17.0) are even closer to the OCP 5.0 target.

**Rejected:** csi-node-driver-registrar v2.18.0 (k8s v0.37.0) exceeds the 1.36.2 ceiling.

---

## Go Toolchain Verification

| Operand | Proposed version | `go.mod` requires | Go 1.27? | Konflux builder (1.26.7) |
|---------|------------------|-------------------|----------|--------------------------|
| spire-server/agent/oidc | 1.15.3 | 1.26.6 | No ✅ | ✅ Buildable |
| spiffe-csi-driver | 0.2.13 | 1.26.5 | No ✅ | ✅ Buildable |
| spire-controller-manager | 0.7.0 | 1.26.5 | No ✅ | ✅ Buildable |
| csi-node-driver-registrar | v2.17.0 | 1.26.0 | No ✅ | ✅ Buildable |
| ZTWIM operator | — | 1.26.5 | No ✅ | ✅ Buildable |

None of the proposed operands require Go 1.27. SPIRE 1.15.4 (`go 1.26.9`) is the only evaluated version blocked by the current builder; see [Konflux builder compatibility](#konflux--rhel-builder-compatibility).

---

## controller-runtime Verification

| Component | controller-runtime | Within v0.24.1 max? |
|-----------|-------------------|----------------------|
| SPIRE 1.15.3 | v0.24.1 | ✅ |
| spire-controller-manager 0.7.0 | v0.24.1 | ✅ |
| ZTWIM operator (target) | v0.24.1 | ✅ |

spire-controller-manager 0.6.4 (baseline) uses v0.22.4 and must be upgraded for OCP 5.0.

---

## Versions Considered but Not Recommended

| Component | Version | Reason excluded |
|-----------|---------|-----------------|
| SPIRE 1.15.4 | Latest 1.15 patch | Requires `go 1.26.9`; current Konflux builder has Go 1.26.7 — [deferred](#why-spire-1154-is-deferred) |
| SPIRE 1.14.8 | Backport on 1.14 line | Staying on 1.14.x misses 1.15 features and controller-runtime v0.24.1 |
| SPIRE 2.x | — | Major version bump; out of scope for ZTWIM 1.2 |
| spiffe-csi 0.3.x | — | Does not exist; latest is 0.2.13 |
| spire-controller-manager 0.6.5/0.6.6 | Patch only | Valid intermediate steps, but 0.7.0 supersedes with additional features/fixes within allowed minor bump |
| csi-node-driver-registrar v2.18.0 | k8s v0.37.0 | Exceeds OCP 5.0 k8s 1.36.2 maximum |

---

## Upgrade Impact Summary for ZTWIM 1.2

### Operator / build changes required

1. Update `pkg/version/version.go` operand version constants
2. Update `RELATED_IMAGE_*` values in `config/manager/manager.yaml` and bundle CSV
3. Bump `github.com/spiffe/spire-controller-manager` in `go.mod` to `v0.7.0` (currently `v0.6.5`)
4. Run `make vendor`, `make update-bindata`, regenerate bundle
5. Rebuild and publish operand container images at proposed versions via Konflux/release repo

### Runtime / operational notes

| Area | Impact |
|------|--------|
| SPIRE metric dashboards | `bootstraped` → `bootstrapped` label rename (1.15.0) |
| SPIRE CLI automation | JSON output no longer wraps objects in slices (1.15.0) |
| sigstore selectors | Promoted from experimental in k8s/docker attestors (1.15.0) |
| CSI driver | Idempotent mount fix improves kubelet restart behavior (0.2.12+) |
| Controller manager | New optional `clusterSPIFFEIDLabelSelector` / `filterByClassName` config for large clusters (0.7.0) |
| CRD changes | None required between 0.6.4 and 0.7.0 (no CRD schema changes since 0.6.0) |

### Testing recommendations

- [ ] SPIRE server/agent rollout and federation on OCP 5.0 (k8s 1.36.2)
- [ ] Workload registration via ClusterSPIFFEID with spire-controller-manager 0.7.0
- [ ] SPIFFE CSI volume mount/unmount across node reboot and kubelet restart
- [ ] OIDC discovery provider Route and JWT-SVID fetch
- [ ] Operand image builds succeed in Konflux with `rhel_9_golang_1.26` builder (Go 1.26.7)
- [ ] ZTWIM operator upgrade path from 1.1 → 1.2 on existing cluster
- [ ] Verify ZTWIM `Upgradeable` condition with new operand versions

---

## Source References

| Upstream | Changelog | Latest release |
|----------|-----------|----------------|
| SPIRE | https://github.com/spiffe/spire/blob/main/CHANGELOG.md | https://github.com/spiffe/spire/releases/tag/v1.15.3 |
| spiffe-csi | https://github.com/spiffe/spiffe-csi/blob/main/CHANGELOG.md | https://github.com/spiffe/spiffe-csi/releases/tag/v0.2.13 |
| spire-controller-manager | https://github.com/spiffe/spire-controller-manager/blob/main/CHANGELOG.md | https://github.com/spiffe/spire-controller-manager/releases/tag/v0.7.0 |
| csi-node-driver-registrar | https://github.com/kubernetes-csi/node-driver-registrar/tree/master/CHANGELOG | https://github.com/kubernetes-csi/node-driver-registrar/releases/tag/v2.17.0 |
| controller-runtime | https://github.com/kubernetes-sigs/controller-runtime/blob/main/VERSIONING.md | v0.24.1 |
