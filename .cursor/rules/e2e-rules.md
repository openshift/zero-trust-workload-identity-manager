# ZTWIM E2E Test Generation Rules

Apply whenever you add, extend, or refactor tests in this repository, or when
a contributor asks you to generate e2e tests for a PR or Jira ticket. These
rules are always active -- even if the current file is not under `test/`.

---

## 1. Scope and context

This is the **Zero Trust Workload Identity Manager** (ZTWIM) operator. It
manages four operands on OpenShift via OLM:

| Operand | CR kind | Workload | Well-known name |
|---|---|---|---|
| SPIRE Server | `SpireServer` | StatefulSet | `spire-server` |
| SPIRE Agent | `SpireAgent` | DaemonSet | `spire-agent` |
| SPIFFE CSI Driver | `SpiffeCSIDriver` | DaemonSet | `spire-spiffe-csi-driver` |
| SPIRE OIDC Discovery Provider | `SpireOIDCDiscoveryProvider` | Deployment | `spire-spiffe-oidc-discovery-provider` |

The top-level CR is `ZeroTrustWorkloadIdentityManager` (cluster-scoped,
name: `cluster`). It aggregates status from all four operands.

**Framework:** controller-runtime / operator-sdk, Ginkgo v2 / Gomega, OLM.

---

## 2. Write-scope restriction (NON-NEGOTIABLE)

- **ONLY** create or modify files inside `test/` (e.g. `test/e2e/`) and
  `output/` (for generated test plans).
- **NEVER** touch `cmd/`, `pkg/`, `api/`, `config/`, `go.mod`, `go.sum`,
  `Makefile`, `Dockerfile`, or any file outside these two directories.
- Generated `test-cases.md` files go into `output/<key>/` (e.g.
  `output/pr-105/test-cases.md`), NOT into the test tree.
- If a non-test change is required for the test to work (e.g. a missing
  exported type), **report it as a suggestion** in the test-cases.md
  instead of making the edit.
- Before every commit, run:
  ```bash
  git diff --name-only | grep -v '^test/' | grep -v '^output/'
  ```
  If that command produces output, **abort the commit** and remove the
  offending files from staging.

---

## 3. Workflow -- generating tests from a PR or Jira ticket

When a contributor says "generate e2e tests for [PR URL or Jira key]",
follow these steps in order. Print progress after each step.

### Step 1: Identify source

- Extract the PR number or Jira key from the user's message.
- If the value contains `/pull/` -- treat as **GitHub PR**.
- If the value looks like a Jira key (`PROJ-123`) or contains `/browse/`
  -- treat as **Jira ticket**.
- If neither is provided, ask:
  `"Enter a Jira link or GitHub PR URL:"`

### Step 2: Fetch context

**If GitHub PR:**
```bash
gh pr view $PR_NUMBER --repo $OWNER/$REPO \
  --json title,body,files,headRefName,baseRefName,commits
gh pr diff $PR_NUMBER --repo $OWNER/$REPO
```

**If Jira:**
```bash
curl -s -u "$JIRA_EMAIL:$JIRA_PERSONAL_TOKEN" \
  "$JIRA_BASE_URL/rest/api/2/issue/$JIRA_KEY?fields=summary,description,issuetype,status,priority,labels,components"
```

If `gh` or Jira access fails, ask the user to provide the PR description,
diff, or Jira summary manually. Do not abort.

### Step 3: Discover existing tests

Since we are already inside the ZTWIM repo, read the existing test tree
directly:

```bash
rg 'Describe\(|Context\(|It\(' test/e2e/ --glob '*_test.go'
rg '^func ' test/e2e/utils/utils.go test/e2e/utils/*.go 2>/dev/null
rg '^\s*(const|var)\s' test/e2e/utils/constants.go 2>/dev/null
```

### Step 4: Analyze changes and map to test domains

Classify every changed file into one or more domains:

| File path pattern | Domain(s) |
|---|---|
| `api/*_types.go` | reconciliation, negative-input-validation |
| `pkg/controller/*_controller.go` | reconciliation, controller-manager |
| `pkg/controller/*/scc.go` | openshift-scc, security-context |
| `pkg/controller/*/daemonset.go` or `*/statefulset.go` or `*/deployment.go` | controller-manager, security-context |
| `pkg/controller/*/configmap.go` | configmap, reconciliation |
| `pkg/controller/*/rbac.go` or `*/role.go` | rbac, openshift-rbac-scoping |
| `config/rbac/` | rbac, openshift-rbac-scoping |
| `config/crd/` | negative-input-validation, csv-versioning |
| `config/webhook/` | negative-input-validation, webhook |
| `config/manager/` | install-health, controller-manager |
| `config/manifests/` | olm-lifecycle-install, csv-versioning |
| `test/` | (informational only -- existing test coverage) |

Always append the Red Hat mandatory domains regardless of diff:
`install-health`, `security-context` / `openshift-scc`, `rbac` /
`openshift-rbac-scoping`, `openshift-monitoring` (if applicable),
`olm-lifecycle-install`.

### Step 5: Dedup

Run dedup queries against `test/e2e/` for each domain:

```bash
rg 'Describe\(|Context\(|It\(' test/e2e/ --glob '*_test.go'

# Per domain
rg 'CRD|Established' test/e2e/                           # install-health
rg 'SecurityContext|RunAsNonRoot|Capabilities' test/e2e/  # security-context
rg 'ClusterRole|Permission|RBAC' test/e2e/                # rbac
rg 'ConfigMap|Data\[' test/e2e/                           # configmap
rg 'Deployment|StatefulSet|DaemonSet' test/e2e/           # controller-manager
rg 'Create\(|Update\(|Delete\(' test/e2e/                 # reconciliation
rg 'invalid|reject|deny|forbidden' test/e2e/ -i          # negative-input-validation
rg 'Subscription|InstallPlan|CSV' test/e2e/               # olm-lifecycle-install
rg 'upgrade|channel|replaces' test/e2e/ -i                # olm-upgrade-path
rg 'uninstall|cleanup|finalizer' test/e2e/ -i             # olm-uninstall
rg 'SCC|SecurityContextConstraint' test/e2e/              # openshift-scc
rg 'ServiceMonitor|Prometheus|/metrics' test/e2e/         # openshift-monitoring
rg 'WaitFor.*Conditions' test/e2e/                        # condition-based waits
rg 'ResourceRequirements|Limits|Requests' test/e2e/       # resource limits
rg 'NodeSelector|Toleration|Affinity' test/e2e/           # scheduling
rg 'DeferCleanup' test/e2e/                               # cleanup patterns
```

For each scenario, decide:

| Hits | Decision |
|---|---|
| Exact spec match with same assertions | `skip` -- already covered |
| Partial match (same Context, different assertions) | `extend` -- add assertions to existing `It` or new `It` in same `Context` |
| Related file exists but different scenario | `new-in-file` -- add new `Context`/`It` in the existing file |
| No match at all | `new-file` -- create new `*_test.go` (only if truly distinct) |

### Step 6: Generate Top 10 Most Impactful Missing Tests

Select the **10 most impactful** test scenarios **NOT already covered** by
existing specs (confirmed via Step 5 dedup). Rank using these categories:

| # | Category | What to test |
|---|---|---|
| 1 | **Core Functionality** | Primary use cases: workload attestation, SVID issuance, trust bundle distribution, CSI volume mount, ClusterSPIFFEID, operand CR lifecycle |
| 2 | **Configuration Edge Cases** | Invalid/boundary configurations: missing required fields, out-of-range TTL, invalid persistence size, webhook rejection, min/max boundary values |
| 3 | **Dynamic Behavior** | Runtime changes not tested: log-level reload, resource limit changes, CreateOnlyMode toggle, ConfigMap drift correction, rolling update tracking |
| 4 | **Integration Gaps** | Component interactions not validated: cross-operand dependency (Agent needs Server), ZeroTrustWorkloadIdentityManager aggregate status, OLM Subscription propagation |
| 5 | **Multi-tenant / Namespace** | Cross-namespace scenarios: ClusterSPIFFEID with namespace selectors, workload attestation across namespaces, RBAC scoping per namespace |
| 6 | **Error Handling** | Failure modes not covered: permission denial, missing CRDs, operator behavior when parent CR is absent, pod crash recovery, cascading failures |
| 7 | **Upgrade / Compatibility** | Version compatibility gaps: OLM upgrade path (channel switching), CSV replacement chain, uninstall cleanup, OperatorCondition.Upgradeable transitions |
| 8 | **Performance** | Load/scale testing if missing: large number of ClusterSPIFFEIDs, concurrent attestation, DaemonSet rollout on many-node clusters |
| 9 | **Security** | Permission/isolation tests: SCC field validation, pod SecurityContext (runAsNonRoot, drop ALL, readOnlyRootFilesystem), restricted-v2 SCC compliance, RBAC least-privilege |
| 10 | **Real Customer Scenarios** | Use cases from the RFE/Jira not tested: end-to-end workflows described in the ticket, production-like topologies, day-2 operational patterns |

**Prioritization rules:**

- For each PR/Jira, identify which categories are impacted by the diff.
- Generate test cases for impacted categories **first**, then fill gaps in
  un-covered categories if the change is broad.
- Assign a priority to each test case:
  - **Critical** -- blocks core functionality or security; must pass before merge.
  - **High** -- important gap in coverage; should be addressed in the same release.
  - **Medium** -- nice-to-have hardening; can be deferred if time-constrained.

**Test case ID format:**

Use `<TICKET>-TC-NNN` where `<TICKET>` is the source identifier:

| Source | ID example |
|---|---|
| Jira `SPIRE-439` | `SPIRE-439-TC-001` |
| Jira `OCPSTRAT-1234` | `OCPSTRAT-1234-TC-001` |
| GitHub PR #105 | `PR-105-TC-001` |

Number sequentially (`TC-001`, `TC-002`, ...) within a single test plan.

### Step 7: Generate test-cases.md

Write the test plan to a local output directory. Use the Jira key when
available, otherwise use the PR number:

```bash
# Jira source
mkdir -p output/${JIRA_KEY}
# GitHub PR source
mkdir -p output/pr-${PR_NUMBER}
```

File name: `test-cases.md` (e.g. `output/SPIRE-439/test-cases.md` or
`output/pr-105/test-cases.md`).

Use this template:

```markdown
# Test Plan: <title>

<!-- Source: <URL> -->
<!-- Repo: openshift/zero-trust-workload-identity-manager -->
<!-- Framework: controller-runtime (operator-sdk) | Ginkgo v2 -->

## Summary
<1-3 sentences: what changed and what must be tested.>

## Test Cases

### <TICKET>-TC-001: <Title>
**Priority:** Critical | High | Medium
**Domain:** <domain-key(s) from Section 6>
**Category:** <one of the 10 categories from Step 6>
**OpenShift-specific:** yes / no
**Coverage Gap:** <What existing tests do NOT cover that this test fills.>
**Prerequisites:** <cluster state, CRDs, namespaces, operator version>
**Steps:**
1. <Concrete action with actual config/commands>
   **Expected:** <What should happen after this step>
2. <Next action>
   **Expected:** <Outcome>
3. ...
**Stop condition:** <which later TCs are blocked if this fails>

(repeat for <TICKET>-TC-002, <TICKET>-TC-003, ... up to 10)

## Coverage Map

| Scenario | Existing spec | Domain | Decision |
|---|---|---|---|
| <keyword> | <path:line or "(none)"> | <domain> | extend / new / skip |

## OLM Coverage
- Subscription install: covered / not covered
- Channel switching: covered / not covered
- Upgrade path: covered / not covered
- Dependency management: covered / not covered
- Uninstall cleanup: covered / not covered

## OpenShift Coverage
- SCC validation: covered / not covered
- RBAC scoping: covered / not covered
- Image scanning: covered / not covered
- Prometheus metrics: covered / not covered
- Audit logging: covered / not covered
- Version compatibility: covered / not covered

## Red Hat Certification Checklist
- [ ] OLM install
- [ ] SCC validation
- [ ] RBAC least-privilege
- [ ] Image scanning / signing
- [ ] Prometheus metrics
- [ ] Audit logging
- [ ] Version compatibility
- [ ] Uninstall cleanup
- [ ] Security context
```

Mark each checklist item `[x]` if a TC covers it. Append a warning for
missing items.

**Writing good test steps:**

- Each step must be **concrete**: include the actual API call, kubectl
  command, CR YAML snippet, or Go assertion -- not vague prose.
- Every step must have its own **Expected** line describing the observable
  outcome (HTTP status, condition value, field value, pod state, etc.).
- If a step creates a resource, note the cleanup requirement
  (`DeferCleanup` in the eventual e2e code).
- Group related assertions into one step when they all verify the same
  object (e.g. "Verify SCC fields" with multiple Expected sub-bullets).

**Default behavior stops here.** Steps 8-9 below run only if the user
explicitly asks to generate e2e code or raise a PR.

### Step 8: Generate e2e code (only if user asks)

Follow the code generation guardrails in Section 9 below. Only modify
files under `test/`.

### Step 9: Branch and PR (only if user asks)

Create a feature branch (e.g. `qa/e2e-<key>-<short-title>`), commit only
files under `test/` and `output/`, push, and open a PR against `main`.
Include the test-cases.md summary in the PR body.

---

## 4. No duplicate e2e coverage

Before adding any new `It` / `Describe` / file:

1. Search the test tree for specs that already exercise the same feature:
   ```bash
   rg '<keyword>' test/ --glob '*_test.go'
   ```
2. If existing e2e already covers the behavior (same assertions and setup,
   or a small extension suffices): **do not add a parallel spec**. Extend
   the existing `Context`/`It`, or report `covered by <path>:<spec>` in the
   coverage map.
3. Add new specs **only** when no current test reasonably covers the
   scenario.

---

## 5. ZTWIM test structure (baked-in)

The repo's e2e layout. Do not invent a new structure.

| File | Role |
|---|---|
| `test/e2e/e2e_suite_test.go` | Suite entrypoint, `BeforeSuite` (k8sClient, clientset, apiextClient, configClient), `TestE2E`. Keep thin. |
| `test/e2e/e2e_test.go` | All test specs. Single `Ordered` top-level `Describe("Zero Trust Workload Identity Manager")` with `BeforeAll` for cluster discovery and `BeforeEach` for per-test context timeout. |
| `test/e2e/utils/constants.go` | All fixed values. **Never modify or delete existing constants.** Only append. |
| `test/e2e/utils/utils.go` | All reusable helpers. **Never duplicate existing helpers.** |

### Existing Contexts in `e2e_test.go`

| Context | What it covers |
|---|---|
| `Installation` | CRD Established, operator Deployment available, ZTWIM CR creation, pod recovery, all 4 operand CR creation + condition checks, aggregate status |
| `OperatorCondition` | Upgradeable True/False transitions on pod deletion, concurrent pod failures |
| `SpireAgent attestation` | ClusterSPIFFEID, test pod with CSI volume, SVID file verification |
| `Common configurations` | Log level, resource limits, nodeSelector, tolerations, affinity, custom labels -- for all 4 operands |
| `CreateOnlyMode` | Subscription env toggle, ConfigMap drift detection |

### Constants (`test/e2e/utils/constants.go`)

```go
OperatorNamespace                         = "zero-trust-workload-identity-manager"
OperatorDeploymentName                    = "zero-trust-workload-identity-manager-controller-manager"
OperatorLabelSelector                     = "name=zero-trust-workload-identity-manager"
OperatorSubscriptionNameFragment          = "zero-trust-workload-identity-manager"
OperatorLogLevelEnvVar                    = "OPERATOR_LOG_LEVEL"
CreateOnlyModeEnvVar                      = "CREATE_ONLY_MODE"
SpireServerStatefulSetName                = "spire-server"
SpireServerPodLabel                       = "app.kubernetes.io/name=spire-server"
SpireServerConfigMapName                  = "spire-server"
SpireServerConfigKey                      = "server.conf"
SpireAgentDaemonSetName                   = "spire-agent"
SpireAgentPodLabel                        = "app.kubernetes.io/name=spire-agent"
SpireAgentConfigMapName                   = "spire-agent"
SpireAgentConfigKey                       = "agent.conf"
SpiffeCSIDriverDaemonSetName              = "spire-spiffe-csi-driver"
SpiffeCSIDriverPodLabel                   = "app.kubernetes.io/name=spiffe-csi-driver"
SpireOIDCDiscoveryProviderDeploymentName  = "spire-spiffe-oidc-discovery-provider"
SpireOIDCDiscoveryProviderPodLabel        = "app.kubernetes.io/name=spiffe-oidc-discovery-provider"
SpireOIDCDiscoveryProviderConfigMapName   = "spire-spiffe-oidc-discovery-provider"
SpireOIDCDiscoveryProviderConfigKey       = "oidc-discovery-provider.conf"
SpiffeHelperConfigMapName                 = "spiffe-helper-config"
SpiffeHelperContainerName                 = "spiffe-helper"
SpiffeHelperImage                         = "ghcr.io/spiffe/spiffe-helper:0.11.0"
DefaultInterval                           = 10 * time.Second
ShortInterval                             = 5 * time.Second
DefaultTimeout                            = 5 * time.Minute
ShortTimeout                              = 2 * time.Minute
TestContextTimeout                        = 10 * time.Minute
```

### Helpers (`test/e2e/utils/utils.go`)

Reuse these -- never write inline wait loops when a helper exists:

**Cluster:**
`GetKubeConfig`, `GetClusterBaseDomain`, `InferControlPlaneRoleKey`, `GetTestDir`

**CRD:**
`IsCRDEstablished`, `WaitForCRDEstablished`

**Pod:**
`IsPodRunning`, `IsPodReady`, `FilterActivePods`, `WaitForPodRunning`, `WaitForPodReady`, `ExecInPod`

**Deployment:**
`IsDeploymentAvailable`, `IsDeploymentRolloutComplete`, `WaitForDeploymentAvailable`, `WaitForDeploymentRollingUpdate`

**StatefulSet:**
`IsStatefulSetReady`, `WaitForStatefulSetReady`, `WaitForStatefulSetRollingUpdate`

**DaemonSet:**
`IsDaemonSetAvailable`, `WaitForDaemonSetAvailable`, `WaitForDaemonSetRollingUpdate`

**Operand conditions:**
`WaitForSpireServerConditions`, `WaitForSpireAgentConditions`, `WaitForSpiffeCSIDriverConditions`, `WaitForSpireOIDCDiscoveryProviderConditions`, `WaitForZeroTrustWorkloadIdentityManagerConditions`

**Verification:**
`VerifyContainerResources`, `VerifyPodLabels`, `VerifyPodScheduling`, `VerifyPodTolerations`

**OLM / Subscription:**
`FindOperatorSubscription`, `PatchSubscriptionEnv`, `WaitForDeploymentEnvVar`, `GetDeploymentEnvVar`

**ConfigMap:**
`GetNestedStringFromConfigMapJSON`

**OperatorCondition:**
`FindOperatorConditionName`, `GetUpgradeableCondition`, `WaitForUpgradeableStatus`

**CR update:**
`UpdateCRWithRetry`

**Attestation:**
`DefaultAttestationSpiffeHelperConfig`, `SpiffeHelperConfig.String`

---

## 6. Domain tagging (mandatory)

Every generated `It` block must carry at least one Ginkgo v2 `Label`:

```text
install-health, security-context, rbac, configmap, controller-manager,
reconciliation, negative-input-validation, negative-permission-validation,
upgrade, olm-lifecycle-install, olm-upgrade-path, olm-uninstall,
olm-dependency-management, csv-versioning, openshift-scc,
openshift-rbac-scoping, openshift-network-policy, openshift-image-scanning,
openshift-monitoring, openshift-logging, openshift-audit,
openshift-version-compat, openshift-fips-mode
```

Example:
```go
It("should reject invalid bundle format", Label("negative-input-validation", "webhook"), func() { ... })
```

---

## 7. OLM, OpenShift, and Red Hat certification

### OLM-aware testing

- Tests that install the operator **must** use the OLM path
  (Subscription -> InstallPlan -> CSV), not raw `kubectl apply`.
- Include channel-switching tests when the operator supports multiple
  channels.
- Use `Eventually` with timeouts >= 60s for OLM operations (CSV
  activation can take 30-90s).
- Verify `OperatorCondition.Upgradeable` transitions correctly when
  operand pods are unhealthy.

### OpenShift-aware testing

- Assume tests run on a genuine OpenShift cluster, not vanilla Kubernetes.
- Verify the operator works under the `restricted-v2` SCC (default on
  OpenShift 4.11+).
- For every operand pod, validate the full `securityContext`:
  ```yaml
  runAsNonRoot: true
  allowPrivilegeEscalation: false
  capabilities.drop: [ALL]
  seccompProfile.type: RuntimeDefault
  ```
- Check the audit log for sensitive operations where relevant.
- SPIRE Agent has a custom SCC (`spire-agent`) -- validate its fields
  explicitly (AllowHostPID, AllowHostDirVolumePlugin, etc.).

### Red Hat certification mindset

Every test should answer: *"Does this operator meet Red Hat certification
requirements?"*

Mandatory checklist (warn if any are missing):

- [ ] OLM install (Subscription-based)
- [ ] SCC validation (restricted-v2 + custom spire-agent SCC)
- [ ] RBAC least-privilege
- [ ] Image scanning / signing (if applicable)
- [ ] Prometheus metrics export (if applicable)
- [ ] Audit logging
- [ ] Operator version compatibility (min 2 OCP versions)
- [ ] Uninstall cleanup
- [ ] Security context (runAsNonRoot, readOnly filesystem)

---

## 8. Ginkgo v2 / Gomega patterns

- `Describe` -> `Context` -> `It`, with `By()` for each logical step.
- `Eventually(...).WithTimeout(t).WithPolling(p).Should(...)` for async
  waits.
- `Consistently(...).WithTimeout(t).WithPolling(p).Should(...)` to prove
  something does NOT change (e.g. drift not corrected in CreateOnlyMode).
- `Ordered` on the top-level `Describe` when tests share state
  (installation -> configuration -> uninstall).
- `DeferCleanup(func(ctx context.Context) { ... })` for resource teardown.
- `BeforeAll` for expensive one-time setup (cluster discovery, subscription
  lookup).
- `BeforeEach` for per-test context isolation:
  ```go
  BeforeEach(func() {
      var cancel context.CancelFunc
      testCtx, cancel = context.WithTimeout(context.Background(), utils.TestContextTimeout)
      DeferCleanup(cancel)
  })
  ```

### ZTWIM-specific proven patterns

**Condition polling:**
```go
utils.WaitForSpireAgentConditions(testCtx, k8sClient, "cluster", map[string]metav1.ConditionStatus{
    "ServiceAccountAvailable":             metav1.ConditionTrue,
    "SecurityContextConstraintsAvailable": metav1.ConditionTrue,
    "DaemonSetAvailable":                  metav1.ConditionTrue,
    "Ready":                               metav1.ConditionTrue,
}, utils.DefaultTimeout)
```

**Rolling update tracking:**
```go
initialGen := daemonset.Generation
// ... apply CR change ...
utils.WaitForDaemonSetRollingUpdate(testCtx, clientset, utils.SpireAgentDaemonSetName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)
utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpireAgentDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)
```

**Pod lifecycle verification:**
```go
oldPodNames := make(map[string]struct{})
for _, pod := range pods.Items { oldPodNames[pod.Name] = struct{}{} }
// ... delete pods ...
Eventually(func() bool {
    newPods, _ := clientset.CoreV1().Pods(ns).List(ctx, opts)
    for _, p := range newPods.Items {
        if _, old := oldPodNames[p.Name]; old { return false }
        if p.Status.Phase != corev1.PodRunning { return false }
    }
    return true
}).WithTimeout(timeout).WithPolling(interval).Should(BeTrue())
```

**SecurityContext verification:**
```go
for _, pod := range pods.Items {
    for _, c := range pod.Spec.Containers {
        Expect(c.SecurityContext.RunAsNonRoot).To(Equal(ptr.To(true)))
        Expect(c.SecurityContext.AllowPrivilegeEscalation).To(Equal(ptr.To(false)))
        Expect(c.SecurityContext.Capabilities.Drop).To(ContainElement(corev1.Capability("ALL")))
    }
}
```

**CR update with retry:**
```go
err = utils.UpdateCRWithRetry(testCtx, k8sClient, spireAgent, func() {
    spireAgent.Spec.Resources = expectedResources
})
Expect(err).NotTo(HaveOccurred())
DeferCleanup(func(ctx context.Context) {
    agent := &operatorv1alpha1.SpireAgent{}
    if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, agent); err == nil {
        agent.Spec.Resources = nil
        k8sClient.Update(ctx, agent)
    }
})
```

**Namespace isolation for test data:**
```go
testNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "e2e-<domain>-test"}}
Expect(k8sClient.Create(testCtx, testNS)).To(Succeed())
DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, testNS) })
```

---

## 9. Code generation guardrails

### Phase E2E-0 -- Pre-generation analysis

```bash
find test/e2e -name '*_test.go' | sort
rg 'Describe\(|Context\(|It\(' test/e2e/ --glob '*_test.go'
rg '^func ' test/e2e/utils/utils.go test/e2e/utils/*.go 2>/dev/null
rg '^\s*(const|var)\s' test/e2e/utils/constants.go 2>/dev/null
```

### Phase E2E-1 -- Dedup with reusability check

1. Search existing test files for the scenario keyword.
2. If found in an existing `It` block with the same assertions -> mark
   `extend`, do NOT add a new spec.
3. If found in the same file but different `Context`/`It` -> add a new
   `It` in the **same file**, reusing helpers.
4. If truly new -> proceed to Phase E2E-2.

### Phase E2E-2 -- Helper and constant extraction

- **NEVER modify or delete existing constants** in
  `test/e2e/utils/constants.go`. Only **append** new constants.
- Immutable values (timeouts, names, labels, ports) go into
  `test/e2e/utils/constants.go`.
- Reusable functions (used in >= 2 tests) go into
  `test/e2e/utils/utils.go`.
- One-off helpers stay private inside the test file.

### Phase E2E-3 -- Test code generation

- Follow the repo's `Describe` -> `Context` -> `It` structure.
- Use constants from `utils/constants.go` -- no hardcoded values.
- Call helpers -- no inline wait loops.
- Tag every `It` with domain labels.
- Use `DeferCleanup` for every created resource.
- Use `BeforeEach` with `context.WithTimeout` for test isolation.

### Phase E2E-4 -- Scope check before commit

```bash
git diff --name-only | grep -v '^test/' | grep -v '^output/'
# Must produce no output.
```

---

## 10. Style and reviewability

- Go: idiomatic naming, small functions, explicit error handling.
- Prefer code a reviewer can follow quickly: linear flow inside `It`
  blocks, named helpers for non-obvious steps, avoid deep nesting.
- Do not add comments that simply narrate what the code does. Comments
  explain *why*, not *what*.
