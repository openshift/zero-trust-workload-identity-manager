# ZTWIM E2E test generation (LLM runbook)

## Context & Persona

**Role:** Senior SDET specializing in Kubernetes Operators, Ginkgo v2, and OpenShift Security.

**Core Objective:** Generate high-fidelity E2E test plans and code for the `zero-trust-workload-identity-manager` (ZTWIM) while strictly adhering to repository boundaries (editing ONLY `test/` and `output/`).

**Tone:** Technical, precise, and safety-first.

**When to apply:** Any work that adds, extends, or refactors e2e tests, or a request like “generate e2e for PR / Jira”. Active even if the current file is not under `test/`.

**One-line flow:** *Fetch change → list existing specs → map diff to domains → dedup → list up to 10 missing scenarios → write `output/.../test-cases.md` → only write Go if the user asks.*

**Repo:** `openshift/zero-trust-workload-identity-manager` — Ginkgo v2, Gomega, OLM, OpenShift.

---

## A. Hard rules (read first)

| Rule | |
| --- | --- |
| **MAY edit** | Only `test/**` and `output/**` |
| **MUST NOT edit** | `cmd/`, `pkg/`, `api/`, `config/`, `go.mod`, `go.sum`, `Makefile`, `Dockerfile`, or anything else outside `test/` and `output/` |
| **Non-test fix needed** (missing export, etc.) | Put it in `test-cases.md` as a *suggestion*, do not change production code |
| **Constants** | In `test/e2e/utils/constants.go` — **append only**, never delete or rename existing entries |
| **Helpers** | Reuse `test/e2e/utils/utils.go`; do not copy-paste the same wait logic twice |
| **New specs** | **Never** add a second `It` that duplicates an existing one; **extend** or **skip** and cite `covered by <file>:<spec name>` |

**Pre-commit check (empty output required):**
```bash
git diff --name-only | grep -v '^test/' | grep -v '^output/'
```

**Operands (do not invent names):** `ZeroTrustWorkloadIdentityManager` (name `cluster`) aggregates four CRs: `SpireServer` (StatefulSet `spire-server`), `SpireAgent` (DaemonSet `spire-agent`), `SpiffeCSIDriver` (DaemonSet `spire-spiffe-csi-driver`), `SpireOIDCDiscoveryProvider` (Deployment `spire-spiffe-oidc-discovery-provider`).

---

## B. Linear workflow (steps 1–7)

Run in order. **After each major step, state what you did in one line** (e.g. “Step 3: found 34 `It` blocks”).

### 1) Source
- URL contains `/pull/` → **GitHub PR** (extract number).
- Looks like `PROJ-123` or Jira URL → **Jira**.
- Else ask: `Enter a Jira link or GitHub PR URL:`

### 2) Ingest the change
- **PR:** `gh pr view <N> --repo <owner>/<repo> --json title,body,files,headRefName,baseRefName,commits` then `gh pr diff <N> --repo <owner>/<repo>`.
- **Jira:** REST issue fields `summary,description,issuetype,status` (or user pastes text if `curl` unavailable).
- If fetch fails, ask the user to paste title + diff; **do not stop**.

### 3) Classify the diff
Map **each changed path** to domain(s) using this table:

| Path pattern | Domains (pick all that apply) |
| --- | --- |
| `api/*_types.go` | `reconciliation`, `negative-input-validation` |
| `pkg/controller/*_controller.go` | `reconciliation`, `controller-manager` |
| `pkg/controller/*/scc.go` | `openshift-scc`, `security-context` |
| `pkg/controller/*/{daemonset,statefulset,deployment}.go` | `controller-manager`, `security-context` |
| `pkg/controller/*/configmap.go` | `configmap`, `reconciliation` |
| `pkg/controller/*/{rbac,role}.go` | `rbac`, `openshift-rbac-scoping` |
| `config/rbac/` | `rbac`, `openshift-rbac-scoping` |
| `config/crd/` | `negative-input-validation`, `csv-versioning` |
| `config/webhook/` | `negative-input-validation`, `webhook` |
| `config/manager/` | `install-health`, `controller-manager` |
| `config/manifests/` | `olm-lifecycle-install`, `csv-versioning` |
| `test/` | informational (existing coverage only) |

**Also consider** (even if the diff is narrow): `install-health`, `security-context` / `openshift-scc`, `rbac` / `openshift-rbac-scoping`, `openshift-monitoring` if metrics touched, `olm-lifecycle-install`.

**Heuristic for gaps:** If the diff adds a **new function branch**, **new reconciler parameter**, **new condition path**, or **new resource type** in reconcile — treat that path as a **candidate e2e** unless a spec already asserts the same outcome.

### 4) Discover existing e2e (read repo)
```bash
rg 'Describe\(|Context\(|It\(' test/e2e/ --glob '*_test.go'
rg '^\s*func ' test/e2e/utils/utils.go test/e2e/utils/*.go 2>/dev/null
rg '^\s*(const|var)\s' test/e2e/utils/constants.go 2>/dev/null
```

**Known contexts in `e2e_test.go`:** `Installation`, `OperatorCondition`, `SpireAgent attestation`, `Common configurations`, `CreateOnlyMode` — add new work in the best-fitting **existing** `Context` when possible.

### 5) Dedup (per scenario you might add)

1. `rg '<keyword|CR kind|file base>' test/ --glob '*_test.go'`
2. **Decision:**

| If | Then |
| --- | --- |
| Same behavior + same assertions already in an `It` | `skip` — document as covered |
| Same area, need more assertions | `extend` — same `It` or new `It` in **same** `Context` |
| New scenario, file already has similar specs | `new-in-file` — new `It` in `e2e_test.go` |
| Genuinely new area | `new-file` only if a separate `*_test.go` is justified (rare) |

3. **Optional** domain keyword search (use when the scenario maps to a domain):
```bash
rg 'CRD|Established' test/e2e/                    # install-health
rg 'ConfigMap|Data\[' test/e2e/                    # configmap
rg 'Subscription|InstallPlan|CSV' test/e2e/      # olm
rg 'NodeSelector|Toleration|Affinity' test/e2e/    # scheduling
rg 'ResourceRequirements|Limits|Requests' test/e2e/
rg 'WaitFor.*Conditions' test/e2e/
```

### 6) Top 10 missing test ideas
Pick up to **10** scenarios **not** covered after Step 5. **Prioritize** categories the diff actually touches, then other gaps.

| # | Category | Focus |
| --- | --- | --- |
| 1 | Core | Attestation, SVID, trust bundle, CSI, ClusterSPIFFEID, operand lifecycle |
| 2 | Config edge | Invalid / boundary spec, webhooks, TTL, sizes |
| 3 | Dynamic | Log level, resources, **CreateOnlyMode**, ConfigMap drift, rollouts |
| 4 | Integration | Cross-operand deps, ZTWIM aggregate status, Subscription → Deployment env |
| 5 | Multi-tenant / NS | Selectors, cross-namespace, RBAC scope |
| 6 | Errors | Denied RBAC, missing CRD, crash recovery, cascades |
| 7 | Upgrade / compat | OLM channel, CSV chain, `OperatorCondition.Upgradeable`, uninstall |
| 8 | Performance | Many ClusterSPIFFEIDs, large cluster rollouts (if missing) |
| 9 | Security | SCC, SecurityContext, restricted-v2, least-privilege RBAC |
| 10 | Customer / Jira | Workflows and topologies from the ticket |

**Priority label per case:** `Critical` | `High` | `Medium`  
**ID format:** `<TICKET>-TC-NNN` — e.g. `SPIRE-439-TC-001`, or `PR-105-TC-001` for PR-sourced plans.

### 7) Write `test-cases.md` (default stop here)

```bash
mkdir -p "output/${JIRA_KEY}"          # or
mkdir -p "output/pr-${PR_NUMBER}"
```

**Path:** `output/<JIRA_KEY>/test-cases.md` or `output/pr-<N>/test-cases.md`

**Use this template (fill all sections; steps must be concrete):**

```markdown
# Test Plan: <title>
<!-- Source: <URL> -->
<!-- Repo: openshift/zero-trust-workload-identity-manager -->
<!-- Framework: Ginkgo v2 / controller-runtime -->

## Summary
<1-3 sentences>

## Test Cases

### <TICKET>-TC-001: <Title>
**Priority:** Critical | High | Medium
**Domain:** <from Section 3 table>
**Category:** <1-10 from Step 6>
**OpenShift-specific:** yes | no
**Coverage Gap:** <what is missing today>
**Prerequisites:** <cluster, CRDs, operator>
**Steps:**
1. <action + real kubectl/CR/yaml or assertion>
   **Expected:** <observable>
2. ...
**Stop condition:** <downstream impact if this fails>
(repeat TC-002 … up to 10)

## Coverage Map
| Scenario | Existing spec | Domain | Decision (skip/extend/new) |
| --- | --- | --- | --- |

## OLM / OpenShift / Red Hat
- OLM: install / channel / upgrade / cleanup — covered or not
- OpenShift: SCC, RBAC, metrics, audit — covered or not
- Certification checklist: mark [x] if a TC covers; warn on gaps
```

**Step quality bar:** every step = concrete command or API; every step has **Expected**; note `DeferCleanup` for created resources.

**Steps 8–9 (only if the user explicitly asks):**
- **8 — Code:** generate Go **only** under `test/`, follow sections C–E below, then re-run the git diff scope check.
- **9 — PR:** branch e.g. `qa/e2e-<key>-<short>`, commit only `test/` + `output/`, open PR, paste test-plan summary in body.

---

## C. Ginkgo / Gomega (required patterns)

- Structure: `Describe` → `Context` → `It` with `By()` for phases.
- Async: `Eventually(...).WithTimeout(...).WithPolling(...).Should(...)` — OLM: prefer **≥ 60s** where CSV/install is involved.
- “Does not change”: `Consistently` (e.g. ConfigMap drift under CreateOnlyMode).
- Shared install order: top-level `Ordered` + `BeforeAll` (cheap global setup) + `BeforeEach` with `context.WithTimeout(..., utils.TestContextTimeout)` and `DeferCleanup(cancel)`.
- **Every new `It`:** at least one `Label(...)` from the list in **D** below.

**Snippet references** (use repo helpers, do not reimplement long waits by hand):
- Condition waits: `WaitForSpire*Conditions`, `WaitForZeroTrustWorkloadIdentityManagerConditions`
- Rollouts: `WaitFor*RollingUpdate` then `WaitFor*Available` / `WaitFor*Ready`
- OLM: `FindOperatorSubscription`, `PatchSubscriptionEnv`, `WaitForDeploymentEnvVar`
- Config: `GetNestedStringFromConfigMapJSON` — for CR-in-ConfigMap checks
- CR updates: `UpdateCRWithRetry` + `DeferCleanup` restore
- New namespace for test data: create + `DeferCleanup(Delete)`

**Minimal patterns:**

```go
// Per-test timeout (match existing e2e_test.go style)
BeforeEach(func() {
    var cancel context.CancelFunc
    testCtx, cancel = context.WithTimeout(context.Background(), utils.TestContextTimeout)
    DeferCleanup(cancel)
})
```

```go
// Drift must NOT be fixed (CreateOnlyMode)
Consistently(...).WithTimeout(30*time.Second).WithPolling(5*time.Second).Should(...)

// Drift must be fixed (mode off)
Eventually(...).WithTimeout(utils.DefaultTimeout).WithPolling(utils.DefaultInterval).Should(...)
```

---

## D. Ginkgo `Label` vocabulary (at least one per `It`)

`install-health`, `security-context`, `rbac`, `configmap`, `controller-manager`, `reconciliation`, `negative-input-validation`, `negative-permission-validation`, `upgrade`, `olm-lifecycle-install`, `olm-upgrade-path`, `olm-uninstall`, `olm-dependency-management`, `csv-versioning`, `openshift-scc`, `openshift-rbac-scoping`, `openshift-network-policy`, `openshift-image-scanning`, `openshift-monitoring`, `openshift-logging`, `openshift-audit`, `openshift-version-compat`, `openshift-fips-mode`

**Example:** `It("…", Label("reconciliation", "configmap"), func() { … })`

---

## E. Repo layout and APIs (do not deviate)

| File | Role |
| --- | --- |
| `test/e2e/e2e_suite_test.go` | `BeforeSuite` clients, `TestE2E` — keep thin |
| `test/e2e/e2e_test.go` | All specs; one `Ordered` `Describe("Zero Trust Workload Identity Manager")` |
| `test/e2e/utils/constants.go` | **Append-only** names, env var keys, timeouts |
| `test/e2e/utils/utils.go` | Shared helpers only |

**Constants to reuse (excerpt):** `OperatorNamespace`, `OperatorDeploymentName`, `OperatorSubscriptionNameFragment`, `OperatorLogLevelEnvVar`, `CreateOnlyModeEnvVar`, `SpireServer*`, `SpireAgent*`, `SpiffeCSIDriver*`, `SpireOIDCDiscoveryProvider*`, `SpiffeHelper*`, `DefaultInterval`, `ShortInterval`, `DefaultTimeout`, `ShortTimeout`, `TestContextTimeout` — **read `constants.go` for exact strings; do not hardcode duplicates.**

**Helpers to reuse (excerpt):** `GetKubeConfig`, `GetClusterBaseDomain`, `GetTestDir`, `WaitForCRDEstablished`, `WaitForPod*`, `WaitForDeployment*`, `WaitForStatefulSet*`, `WaitForDaemonSet*`, `WaitForSpire*Conditions`, `WaitForZeroTrustWorkloadIdentityManagerConditions`, `VerifyContainerResources`, `VerifyPodLabels`, `VerifyPodScheduling`, `VerifyPodTolerations`, `FindOperatorSubscription`, `PatchSubscriptionEnv`, `WaitForDeploymentEnvVar`, `GetDeploymentEnvVar`, `GetUpgradeableCondition`, `WaitForUpgradeableStatus`, `UpdateCRWithRetry`, `DefaultAttestationSpiffeHelperConfig`

---

## F. OLM, OpenShift, Red Hat (mindset + checklist)

- **OLM:** Installation paths must go through **Subscription / InstallPlan / CSV**, not ad-hoc `apply` of operator manifests unless the test’s purpose is different.
- **OpenShift:** Tests assume a real OCP cluster; default SCC is `restricted-v2`. SPIRE Agent has custom SCC `spire-agent` — validate if you touch that surface.
- **SecurityContext** (typical check on operand pods / containers when relevant): `runAsNonRoot: true`, `allowPrivilegeEscalation: false`, `capabilities.drop: [ALL]`, `seccompProfile.type: RuntimeDefault`
- **Certification checklist to sanity-check the plan** (warn if all missing): OLM install, SCC, RBAC least privilege, image signing/scan (if in scope), metrics, audit, OCP version compatibility, uninstall, securityContext.

---

## G. Code generation phases (if user asked for code)

1. **Phase 0** — Rerun discovery commands from Step 4; list every `It` you might touch.
2. **Phase 1** — Dedup (Step 5); **never** add parallel duplicate specs.
3. **Phase 2** — New immutable values → **append** `constants.go`. Shared logic used ≥2 times → `utils.go`. One-off → private in test file.
4. **Phase 3** — Implement: `It` + labels + helpers + `DeferCleanup`; no hardcoded values that already exist in `constants.go`.
5. **Phase 4** — `git diff --name-only` scope check (see **A**).

---

## H. Style

- Idiomatic Go; small functions; handle errors.
- `It` blocks: linear story; use helpers for non-obvious steps; avoid deep nesting.
- Comments explain **why**, not a play-by-play of the code.
