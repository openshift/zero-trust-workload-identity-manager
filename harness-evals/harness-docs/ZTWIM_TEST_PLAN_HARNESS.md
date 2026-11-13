# ZTWIM Test Plan Harness

> **Use this doc when:** generating ADR-driven test *plans* (OpenSpec QE workflow) — requirements, scenarios, traceability.
>
> **Not this doc:** for writing test *code*, see [`ZTWIM_TEST_IMPLEMENTATION.md`](ZTWIM_TEST_IMPLEMENTATION.md).
>
> **Audience**: OpenSpec QE workflow agents generating ADR-driven test plans for Zero Trust Workload Identity Manager.
>
> **Purpose**: Supplement generic OpenSpec QE stages with ZTWIM-specific coverage surfaces, observables, and quality gates so generated test scenarios are comprehensive across all tiers and components.
>
> **Companion docs**: [`ZTWIM_TEST_IMPLEMENTATION.md`](ZTWIM_TEST_IMPLEMENTATION.md) (how to write tests) · [`architecture/components.md`](architecture/components.md) (reconcile flow) · [`domain/`](domain/) (CRD contracts)

## Retrieval Path (read before generating)

Execute this retrieval order **before** drafting test cases from an ADR:

1. **ADR** — full text (required input to OpenSpec QE workflow)
2. **`domain/`** — identify which CRDs, fields, immutability rules, and cross-field constraints the ADR touches
3. **`architecture/components.md`** — reconcile order, apply patterns, error classification, OpenShift integrations
4. **`domain/upstream-spiffe-crds.md`** — if ADR involves identity registration, federation, or SPIRE entries
5. **`ZTWIM_TEST_IMPLEMENTATION.md`** — existing test tiers, frameworks, and per-function unit test scenarios
6. **`test/e2e/e2e_test.go`** — existing E2E coverage to avoid duplication and identify regression gaps

---

## 1. Role and Objective

You are generating an **ADR-driven, traceable test plan** for ZTWIM — a controller-runtime operator that deploys upstream SPIFFE/SPIRE operands on OpenShift 4.18+.

**Output**: Markdown test plan with five tiers (UT, INT, E2E, MQE, NFT), stable requirement IDs, traceability matrix, and quality-gate compliance.

**Language quality**: Do not use vague verbs ("verify", "ensure", "check") unless each ties to a **named observable** — e.g. condition `Ready=True`, `OperatorCondition.Upgradeable=True`, SPIFFE ID `spiffe://example.org/ns/foo/sa/bar`, HTTP 200 on `/.well-known/openid-configuration`.

---

## 2. ADR Comprehension Protocol

Read and decompose the ADR before generating tests. ZTWIM ADRs follow this section order:

```text
What → Why → How → Alternatives → Risks
```

If an expected section is **missing**, note **Section absent** and continue.

### Step 1: "What" → scope, components, and resources

Identify the one-sentence scope and map it to one or more ZTWIM controllers:

| Controller | Primary CR | Workload |
|---|---|---|
| ZTWIM aggregator | `ZeroTrustWorkloadIdentityManager` | — (status only) |
| SpireServer | `SpireServer` | StatefulSet |
| SpireAgent | `SpireAgent` | DaemonSet |
| SpiffeCSIDriver | `SpiffeCSIDriver` | DaemonSet |
| SpireOIDCDiscoveryProvider | `SpireOIDCDiscoveryProvider` | Deployment |

Extract which of these are in scope:

- **Operator CRDs** (`operator.openshift.io/v1alpha1`) — all five are cluster-scoped singletons named `cluster`
- **Upstream CRDs** (`spire.spiffe.io/v1alpha1`) — `ClusterSPIFFEID`, `ClusterFederatedTrustDomain`, `ClusterStaticEntry` (NOT singletons; reconciled by spire-controller-manager sidecar)
- **Bindata resources** — ServiceAccount, Service, ClusterRole, ClusterRoleBinding, Role, RoleBinding, CSIDriver, ValidatingWebhookConfiguration, SCC
- **Programmatic resources** — StatefulSet, DaemonSet, Deployment, ConfigMap, Route, Secret
- **Status fields** — `metav1.Condition` types, per-resource conditions (`StatefulSetAvailable`, `ConfigMapAvailable`, etc.), `OperandStatus` aggregation
- **OLM integration** — `OperatorCondition.Upgradeable`, `RELATED_IMAGE_*` env vars

Also extract from **What**:

- **Scope boundaries** — what is explicitly excluded or out of scope (do not generate tests for these unless they are regression guards)
- **Present-state problems** — what existing behavior the change fixes or improves

### Step 2: "Why" → positive-path requirements

Each stated motivation or outcome in **Why** becomes at least one testable requirement. For ZTWIM, these often span:

- Identity issuance (X.509-SVID, JWT-SVID)
- Operator install/upgrade safety
- Configuration propagation (CommonConfig → operand pods)
- Federation or OIDC discovery exposure
- Platform integration (Routes, SCC, proxy, Service CA)

### Step 3: "How" → implementation branches (richest test source)

For each reconcile path described, identify:

| Pattern | Test implication |
|---|---|
| Bindata decode → mutate → create-or-update | UT for decode/mutate; INT for reconcile sub-function; E2E for cluster state |
| `ResourceNeedsUpdate()` drift detection | UT for comparison logic; E2E for no-op reconcile when spec unchanged |
| `UpdateWithRetry()` / `StatusUpdateWithRetry()` | INT for conflict handling; NFT recovery scenario |
| Config-hash annotation restart trigger | E2E: change ConfigMap → pod restart with new hash |
| `CREATE_ONLY_MODE` env var | E2E: pause/resume drift correction; OLM `Upgradeable=False` |
| `ReconcileError` classification | UT: irrecoverable (403/401/400) vs retry (409/conflict) |
| `HandleCreateConflict` / `ResourceConflict` condition | E2E: pre-existing unmanaged resource with same name |
| Owner reference via `NeedsOwnerReferenceUpdate()` | INT: ZTWIM CR missing → operand `Ready=False/Failed` |
| `ValidateAndUpdateStatus()` / CEL immutability | E2E: invalid spec rejected at admission or sets validation condition |
| spire-controller-manager `className` scoping | E2E: CR without matching className ignored |

Also extract from **How**:

- Open questions or known unknowns → Tier 4 exploratory tests
- Migration or upgrade paths → lifecycle E2E tests
- Error handling, retry logic, and failure modes

### Step 4: "Alternatives" → guard-rail tests

Rejected approaches may need tests ensuring the chosen path holds — e.g. bindata over Helm, controller-runtime over library-go, create-or-update over SSA.

### Step 5: "Risks" → negative, regression, and NFT scenarios

Map each risk row to test implications:

| Risk type | Typical ZTWIM test |
|---|---|
| Customer / behavior change | Tier 3 regression: existing attestation or config workflow unchanged |
| Operational / failure mode | Tier 5 recovery: operator pod crash, operand pod deletion, API server blip |
| Security / privilege | Tier 5 security: SCC enforcement, RBAC scope, secret handling |
| Upgrade safety | Tier 3/4: OLM upgrade with `Upgradeable=True`; operand rolling update |

### Step 6: ADR Decomposition Summary (required output block)

```markdown
## ADR Decomposition

**Feature:** <one-line scope from What>
**ADR status / version:** <Proposed / Accepted / date — or "Not stated">
**Controllers in scope:** <ZTWIM | SpireServer | SpireAgent | SpiffeCSIDriver | SpireOIDCDP>
**CRDs affected:** <operator CRDs and/or spire.spiffe.io CRDs>
**Kubernetes resources affected:** <StatefulSet, DaemonSet, bindata RBAC, Route, etc.>
**Cross-CR constraints:** <socket path, JWT issuer, CSI plugin name — or "none">
**Scope boundaries (from What):** <what is excluded — or "not stated">
**Positive-path requirements (from Why):** <numbered list>
**Implementation branches requiring coverage (from How):** <logic branch, dependency, migration path, error path>
**Risks requiring coverage (from Risks):** <risk → test implication>
**Rejected alternatives (from Alternatives):** <approach → guard-rail test implication>
**Open questions / exploratory areas (from How):** <unknowns → Tier 4 MQE>
```

Do not proceed to test generation until this block is complete.

---

## 3. ZTWIM Coverage Surface Catalog

Use this catalog as a **mandatory checklist** when mapping ADR scope to test cases. For each surface touched by the ADR, generate at least one test per applicable tier.

### 3.1 Operator-wide surfaces

| Surface | Observable | Tiers |
|---|---|---|
| Singleton CR enforcement | Admission webhook / CEL rejects `metadata.name != cluster` | E2E, MQE |
| ZTWIM parent dependency | Operand `Ready=False`, reason `Failed` when ZTWIM CR missing | INT, E2E |
| Owner reference GC | Deleting ZTWIM CR cascades to operand CRs with owner refs | E2E |
| Operand status aggregation | `status.operands[]` reflects each operand `Ready` | E2E |
| `OperandsAvailable` condition | `True` only when all existing operands ready | E2E |
| `Upgradeable` → OLM sync | `OperatorCondition.spec.Upgradeable` mirrors ZTWIM condition | E2E |
| `CREATE_ONLY_MODE` | `CreateOnlyMode=True` condition; drift correction paused; `Upgradeable=False` | E2E |
| Label-filtered cache | Resources without `app.kubernetes.io/managed-by=zero-trust-workload-identity-manager` not reconciled | INT |
| `RELATED_IMAGE_*` resolution | Operand pods use images from CSV env vars, not hardcoded refs | E2E, NFT |
| Multi-arch image availability | All seven external images available on target arch (x86_64, arm64, s390x, ppc64le) | NFT |
| Metrics endpoint | `:8443` TLS, RBAC-protected `/metrics` | E2E |
| Proxy integration | `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY` propagated to operand pods | E2E |
| FIPS build | Operator binary built with `strictfipsruntime` tags | NFT |

### 3.2 Per-controller reconcile surfaces

Apply the **8-step reconcile flow** from `architecture/components.md` when the ADR touches a controller:

1. Fetch CR (NotFound → stop)
2. Initialize status manager (`defer ApplyStatus()`)
3. Fetch parent ZTWIM CR
4. Set owner reference
5. Check `CREATE_ONLY_MODE`
6. Validate configuration
7. Reconcile sub-resources (ordered)
8. Check workload health

**Unit test minimum per `reconcileX` sub-function** (from `ZTWIM_TEST_IMPLEMENTATION.md`):

1. Create success
2. Create error
3. Get error (non-NotFound)
4. Update success (drift detected)
5. Update error
6. Create-only mode skip
7. SetControllerReference error

### 3.3 SpireServer-specific surfaces

| Surface | Observable |
|---|---|
| StatefulSet health | Condition `StatefulSetAvailable=True`; pods `Ready` |
| PVC persistence | PVC bound; immutability of `persistence.*` fields |
| CA bootstrap / rotation | Valid CA in trust bundle ConfigMap; CA cert within `caValidity` TTL |
| JWT issuer | `jwtIssuer` URL reachable; matches OIDC provider config |
| Federation bundle endpoint | Route/Service on port 8443; remote trust domain bundle fetch |
| Upstream authority | cert-manager or Vault integration produces intermediate CA |
| spire-controller-manager sidecar | Sidecar pod running; webhook `ValidatingWebhookConfiguration` present |
| ConfigMap hash restart | `ztwim.openshift.io/spire-server-config-hash` annotation changes → StatefulSet rollout |
| Service CA TLS | `service.beta.openshift.io/serving-cert-secret-name` annotation → TLS secret mounted |
| Datastore backends | sqlite3 (default), postgres, mysql connection and migration |

### 3.4 SpireAgent-specific surfaces

| Surface | Observable |
|---|---|
| DaemonSet per-node | Agent pod `Ready` on every schedulable node |
| Custom SCC | `spire-agent` SCC applied; agent pod admitted |
| Socket path | Unix socket at `spec.socketPath`; matches CSI driver `agentSocketPath` |
| Node attestation (PSAT) | Agent registers with server; node SVID issued |
| Workload attestation | Workload connecting to socket receives SVID matching `ClusterSPIFFEID` template |
| Kubelet verification modes | `auto` / `hostCert` / `skip` — correct CA path or skip flag in agent config |
| SVID rotation | New `svid.pem` before expiry; fresh SVID after pod recreation |
| ConfigMap hash restart | `ztwim.openshift.io/spire-agent-config-hash` → DaemonSet rollout |

### 3.5 SpiffeCSIDriver-specific surfaces

| Surface | Observable |
|---|---|
| CSIDriver registration | `CSIDriver` object with correct `spec` |
| Privileged SCC binding | RoleBinding to `privileged` SCC; CSI pod admitted |
| Socket delivery | Workload pod mounts SPIFFE volume; `svid.pem` and `bundle.pem` present |
| Init container | UBI init container copies socket to shared mount |
| Socket path consistency | `agentSocketPath` matches SpireAgent `socketPath` |
| CSI plugin name | Matches SpireOIDCDiscoveryProvider `csiPluginName` when OIDC in scope |

### 3.6 SpireOIDCDiscoveryProvider-specific surfaces

| Surface | Observable |
|---|---|
| Deployment health | Condition `DeploymentAvailable=True` |
| OIDC discovery endpoint | `GET /.well-known/openid-configuration` returns 200 with `issuer` matching `jwtIssuer` |
| Route exposure | OpenShift Route serves discovery and JWKS endpoints |
| Operator-managed ClusterSPIFFEIDs | Two fixed-name instances with `className: zero-trust-workload-identity-manager-spire` |
| JWT-SVID validation | External system validates JWT-SVID against discovery document |
| ConfigMap hash restart | `ztwim.openshift.io/spire-oidc-discovery-provider-config-hash` → Deployment rollout |

### 3.7 Upstream SPIFFE CRD surfaces

Only generate these when the ADR involves identity registration, federation, or SPIRE entries:

| Surface | Observable |
|---|---|
| `ClusterSPIFFEID` className scoping | Entries created only for `className: zero-trust-workload-identity-manager-spire` |
| SPIFFE ID template rendering | SVID URI matches template with trust domain, namespace, service account |
| Template update propagation | Changing `spiffeIDTemplate` → workload receives new SPIFFE ID |
| `ClusterFederatedTrustDomain` | Remote bundle synced to SPIRE server |
| `ClusterStaticEntry` | Static SPIRE entry created with specified SPIFFE ID |
| Ignored namespaces | CRs in `openshift-*`, `kube-system` not reconciled |

### 3.8 Cross-CR consistency (always check)

| Field | CRs | Test when ADR touches either |
|---|---|---|
| Socket path | SpireAgent ↔ SpiffeCSIDriver | Mismatch → CSI cannot deliver SVIDs |
| JWT issuer | SpireServer ↔ SpireOIDCDiscoveryProvider | Mismatch → OIDC discovery invalid |
| CSI plugin name | SpiffeCSIDriver ↔ SpireOIDCDiscoveryProvider | Mismatch → volume mount fails |
| Trust domain | ZeroTrustWorkloadIdentityManager → all SPIFFE IDs | Immutable; all SVIDs use `spiffe://<trustDomain>/...` |

### 3.9 CommonConfig propagation

When the ADR modifies or depends on pod scheduling or labeling, cover **each affected operand**:

| CommonConfig field | Observable |
|---|---|
| `labels` | Labels on operand pod template match CR spec |
| `resources` | Container `resources.requests/limits` match CR spec |
| `nodeSelector` | Pods scheduled only on matching nodes |
| `tolerations` | Pods tolerate specified taints |
| `affinity` | Pods respect affinity/anti-affinity rules |

Existing E2E coverage exists per operand in `test/e2e/e2e_test.go` — reference for regression, extend for new fields.

### 3.10 Security surfaces

| Surface | Observable |
|---|---|
| Operator non-root | Operator pod `securityContext.runAsUser: 65532` |
| Operand `readOnlyRootFilesystem` | Operand containers run read-only with EmptyDir writable paths |
| RBAC least privilege | Operator ClusterRole uses `resourceNames: [cluster]` for operand CRs |
| SCC: spire-agent | Custom SCC with required capabilities |
| SCC: spiffe-csi | Privileged SCC RoleBinding |
| Secret handling | TLS certs, DB credentials not in logs or events |
| Webhook admission | `ValidatingWebhookConfiguration` rejects malformed spire-controller-manager requests |
| Workload attestor `skip` mode | Only in dev/test contexts; document risk in MQE |

---

## 4. Requirement Extraction

Transform ADR decomposition into numbered requirements: `REQ-001`, `REQ-002`, … or `<ADR_SLUG>-REQ-001`.

Merge overlapping candidates. Categorize:

| Category | ZTWIM examples |
|---|---|
| Functional | SVID issued matching ClusterSPIFFEID template |
| Negative-path | Invalid immutable field rejected; missing ZTWIM parent fails reconcile |
| Regression | Existing attestation E2E still passes; CommonConfig propagation unchanged |
| Performance | Reconcile latency under N operand CR updates; SVID rotation within TTL |
| Security | RBAC denied → irrecoverable error; SCC prevents privilege escalation |
| Operational | Operator pod restart → operands reconverge; `Upgradeable` recovers after operand failure |

---

## 5. Test Generation Rules by Tier

### Tier 1: Unit Tests (UT)

**Framework**: Go `testing` + testify; `FakeCustomCtrlClient` (counterfeiter).

**Derive from**: ADR "How" — logic branches, helpers, validation, drift detection.

**ZTWIM conventions**:

- Table-driven `t.Run` with `setupClient func(*fakes.FakeCustomCtrlClient)`
- One test file per reconcile sub-function (`foo_test.go`)
- Test all seven scenarios per `reconcileX` (see §3.2)
- Use `logr.Discard()`, `record.NewFakeRecorder(100)`
- Assert `CreateCallCount()`, `UpdateCallCount()`, stub return values

**Do NOT include**: Cluster behavior, real API server (except envtest for INT).

### Tier 2: Integration Tests (INT)

**Framework**: envtest or fake API server; exercise reconcile loop.

**Derive from**: Component interactions, webhook cycles, status propagation.

**ZTWIM minimum per ADR interaction**:

- Primary reconcile success path
- Reconcile error / requeue path
- Status condition set on failure (`Ready=False`, component-specific reason)
- Parent ZTWIM CR missing → operand stops with `Failed`
- `CREATE_ONLY_MODE` skips update but not create

### Tier 3: E2E Automated Tests (E2E)

**Framework**: Ginkgo v2 + Gomega; live OpenShift cluster; `Ordered` specs.

**Derive from**: ADR "What", "Why", "Risks".

**ZTWIM minimum per ADR** (in addition to OpenSpec generic minimums):

| Minimum | ZTWIM observable |
|---|---|
| Smoke | Operator Deployment ready; CRDs established; operand CR reaches `Ready=True` |
| Negative input | Invalid spec rejected (CEL) or `ConfigurationValid=False` |
| Regression | Existing workflow in `test/e2e/e2e_test.go` for touched area still valid |
| Lifecycle | Create → update spec → delete operand CR; finalizer/cleanup if applicable |
| Cross-CR consistency | If ADR touches coordinated fields, assert end-to-end identity flow |

**Ginkgo conventions**:

- `Eventually` / `Consistently` with `DefaultTimeout` (5min) / `DefaultInterval` (10s)
- `DeferCleanup` for teardown
- At least one `Label(...)` per `It` block
- Use helpers from `test/e2e/utils/`: `WaitForDeploymentAvailable`, `SetupAttestationTest`, `UpdateCRWithRetry`

**Attestation E2E pattern** (when ADR affects identity):

1. Create test namespace + ServiceAccount + ClusterSPIFFEID
2. Deploy workload with SPIFFE CSI volume mount
3. Assert `svid.pem` is valid X.509, chains to `bundle.pem`
4. Assert SPIFFE ID matches template
5. Assert SVID rotates before expiry

### Tier 4: Manual QE Tests (MQE)

**Derive from**: Why, Risks, open questions from How, usability.

**ZTWIM-specific MQE scenarios to consider**:

- Admin installs via OLM, creates CRs in documented order, reads `status.operands` to diagnose failure
- Error messages actionable without reading source (condition `message`, Kubernetes events)
- Upgrade from N-1 to N with `Upgradeable=True` throughout
- Node drain during DaemonSet rollout — workloads on remaining nodes retain SVID access
- Manual deletion of managed ConfigMap/ServiceAccount — operator recreates or reports `ResourceConflict`
- Documentation accuracy for new ADR feature

### Tier 5: Non-Functional Tests (NFT)

**Derive from**: Risks, security design, operational concerns.

**ZTWIM NFT sub-types**:

| Sub-type | When to generate | ZTWIM example |
|---|---|---|
| 5a Performance | ADR affects reconcile hot path or SVID TTL | Measure reconcile duration for N spec updates |
| 5b Regression | Broad release validation | Full E2E suite pass on upgraded cluster |
| 5c Security | ADR touches RBAC, SCC, secrets, webhooks | Attempt privilege escalation; audit RBAC verbs |
| 5d Recovery | ADR affects availability | Kill operator pod mid-reconcile; delete operand pod; API blip |
| 5e Scalability | ADR affects multi-node or many namespaces | N nodes × M namespaces with ClusterSPIFFEIDs |
| 5f Compliance | Platform baseline | FIPS mode; multi-arch image pull on all supported arches |

**Avoid duplication**: If Tier 3 E2E already covers the same observable as a regression scenario, assign to Tier 3 only.

---

## 6. Output Template

Structure the complete test plan as:

```markdown
# Test Plan: <Feature Name>

**Sources:** ADR: <link or path>; Jira: <key or "none">
**Date:** <YYYY-MM-DD>
**Scope:** <one-line>
**Controllers:** <list>

## Source conflicts
<ADR vs Jira inconsistencies, or "None">

## ADR Decomposition
<from §2 Step 6>

## Testable Requirements
| ID | Requirement | Category | ADR Source | ZTWIM Surface |
| REQ-001 | ... | Functional | How §... | SpireAgent socket path |

## Test Cases
### Tier 1: Unit Tests
#### UT-001: <Title>
**Priority:** Critical / High / Medium
**Controller:** SpireServer | ...
**Relevant Requirement(s):** REQ-NNN
**Traceability:** <ADR section>
**Preconditions:** ...
**Steps:** ...
**Cleanup:** ...
**Failure Impact:** ...

(repeat for INT, E2E, MQE, NFT tiers)

## Traceability Matrix
| Requirement | UT | INT | E2E | MQE | NFT | Coverage Status |

## Uncovered Requirements
<REQ-NNN: reason>

## Coverage Summary
| Tier | Count | Critical | High | Medium |
```

---

## 7. Quality Gates

Revise the plan until **all** gates pass.

### Generic gates

| Gate | Requirement |
|---|---|
| ADR fully read | Decomposition complete; missing sections noted |
| Requirements extracted | Every Why outcome and Risks row maps to ≥1 requirement |
| Tier 1–5 minimums | Per §5 minimum coverage per tier |
| Traceability complete | Every REQ has tests or explicit NOT COVERED justification |
| No vague steps | Every step has concrete action + named observable |
| Cleanup specified | Every test creating resources specifies teardown |
| Priority assigned | Every test has Critical / High / Medium |
| Scope respected | No tests for out-of-scope items stated in What unless regression guard |

### ZTWIM-specific gates

| Gate | Requirement |
|---|---|
| Controller mapping | Every test case names the controller(s) it exercises |
| Singleton awareness | Tests use `metadata.name: cluster`; negative test for wrong name if CR creation involved |
| Parent ZTWIM dependency | Operand tests acknowledge ZTWIM CR prerequisite or test its absence |
| Cross-CR consistency | If ADR touches coordinated fields (§3.8), ≥1 E2E validates end-to-end identity flow |
| Reconcile sub-function coverage | UT covers all seven scenarios for each new/modified `reconcileX` |
| Condition observables | Status assertions use named condition `type`, `status`, `reason` — not generic "healthy" |
| CREATE_ONLY_MODE | If ADR affects resource updates, include create-only mode test |
| Upgradeable sync | If ADR affects operand readiness, include OLM `OperatorCondition` test |
| spire-controller-manager boundary | If ADR involves SPIRE entries, distinguish ZTWIM operand reconcile from controller-manager reconcile |
| Bindata vs programmatic | Tests target correct resource creation path (bindata decode vs Go-built objects) |
| Config-hash restart | If ADR modifies operand ConfigMap, include restart propagation test |
| Security posture | If ADR changes security config, NFT asserts runtime UID/GID/capabilities/SCC — not just spec correctness |
| Multi-arch | If ADR adds/changes `RELATED_IMAGE_*`, NFT validates image availability on all target architectures |
| Existing E2E regression | Identify which `test/e2e/e2e_test.go` contexts are regression guards for this ADR |
| Anti-pattern guards | No tests assuming library-go, hand-edited bindata, or non-singleton CRs |

---

## 8. Common Generation Gaps (avoid these)

The OpenSpec QE workflow often under-generates in these ZTWIM areas. Explicitly check each:

1. **Only testing happy path** — missing negative input, missing parent CR, API conflict, irrecoverable RBAC error
2. **Ignoring CREATE_ONLY_MODE** — drift correction pause and `Upgradeable=False` not tested
3. **Skipping OLM integration** — `OperatorCondition` sync not in plan
4. **Treating all CRDs as singletons** — upstream `spire.spiffe.io` CRDs allow multiple instances
5. **Conflating reconcile loops** — ZTWIM operand controller vs spire-controller-manager entry reconciliation
6. **Missing cross-CR tests** — socket path, JWT issuer, CSI plugin name consistency
7. **Generic "operator is ready"** — not checking operand-specific conditions (`StatefulSetAvailable`, etc.)
8. **No attestation coverage** — ADR affects identity but no SVID/bundle/rotation tests
9. **Config change without restart test** — ConfigMap update doesn't trigger operand rollout
10. **Security spec-only** — checking CR fields but not runtime pod `securityContext` or SCC
11. **No CommonConfig matrix** — testing one operand but ADR applies to all four
12. **Duplicate tier regression** — same observable in both E2E and NFT 5b

---

## 9. Reference: Existing E2E Coverage Map

Use to identify regression guards and avoid duplication:

| E2E Context | Covers |
|---|---|
| Installation | Operator deploy, all four operand CRs, ZTWIM aggregation, metrics |
| OperatorCondition | `Upgradeable` sync, operand failure/recovery |
| SpireAgent attestation | SVID validation, rotation, pod recreation, ClusterSPIFFEID template update |
| Common configurations | CommonConfig per operand (resources, nodeSelector, tolerations, affinity, labels, log level) |
| CreateOnlyMode | Env var toggle, ConfigMap reconciliation pause/resume |

---

## 10. Methodology Definitions

| Methodology | ZTWIM example |
|---|---|
| Black box | `oc apply` SpireAgent CR → DaemonSet pods Ready on all nodes |
| White box | Unit test `ResourceNeedsUpdate()` ignoring ClusterIP drift |
| Grey box | Create CR, watch controller logs for reconcile step, assert condition transition |

---

**Maintainers**: Update this harness when new controllers, CRDs, conditions, or E2E contexts are added. Cross-reference changes in `ZTWIM_TEST_IMPLEMENTATION.md` and `architecture/components.md`.
