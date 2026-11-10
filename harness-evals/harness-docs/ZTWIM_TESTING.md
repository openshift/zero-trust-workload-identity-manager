# ZTWIM Testing Guide

> For generic testing practices, see the Platform Testing Guide.

## Test Architecture

ZTWIM uses a **two-tier** test architecture:

| Tier | Location | Framework | Target |
|------|----------|-----------|--------|
| Unit | `pkg/controller/*/` | Go `testing` + testify | Logic in isolation with fakes |
| E2E | `test/e2e/` | Ginkgo v2 + Gomega | Live OpenShift cluster |

The `make test` target uses controller-runtime's envtest (`setup-envtest` / `KUBEBUILDER_ASSETS`) to provide a real API server and etcd for tests, but controller logic is exercised through the counterfeiter-generated `FakeCustomCtrlClient`, not by running full reconciliation loops against envtest.

## Unit Tests

### Location & Execution

```bash
# Run all unit tests (excludes e2e)
OPERATOR_NAMESPACE=zero-trust-workload-identity-manager make test

# Run a specific package
OPERATOR_NAMESPACE=zero-trust-workload-identity-manager \
  go test ./pkg/controller/spire-server/... -v
```

### Patterns from the Codebase

**Table-driven tests with `t.Run`:**

```go
tests := []struct {
    name        string
    setupClient func(*fakes.FakeCustomCtrlClient)
    expectError bool
}{...}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        fakeClient := &fakes.FakeCustomCtrlClient{}
        tt.setupClient(fakeClient)
        // ...
    })
}
```

**`newTestReconciler` factory:**

```go
func newTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpireServerReconciler {
    return &SpireServerReconciler{
        ctrlClient:    fakeClient,
        ctx:           context.Background(),
        log:           logr.Discard(),
        scheme:        runtime.NewScheme(),
        eventRecorder: record.NewFakeRecorder(100),
    }
}
```

**FakeCustomCtrlClient (counterfeiter-generated):**

- Stub per-call behavior: `fc.GetStub = func(...) error { ... }`
- Stub return values: `fc.GetReturns(err)`, `fc.CreateReturns(nil)`
- Assert call counts: `fakeClient.CreateCallCount()`
- Assert call args: `fakeClient.CreateArgsForCall(0)`

**Standard test utilities:**

- `logr.Discard()` — silent logger for tests
- `record.NewFakeRecorder(100)` — buffered event recorder
- `kerrors.NewNotFound(...)` / `kerrors.NewAlreadyExists(...)` — typed API errors
- `t.Setenv("ENV_VAR", "value")` — scoped env vars (auto-cleaned)

### Per-Reconciler Test File Organization

Each reconciler sub-function gets its own test file:

```text
pkg/controller/spire-server/
├── controller_test.go          # Reconcile loop, full flow, error scenarios
├── service_account_test.go     # reconcileServiceAccount tests
├── configmaps_test.go          # reconcileSpireServerConfigMap tests
├── statefulset_test.go         # reconcileStatefulSet tests
└── ...
```

### Status Manager Testing

The `status.NewManager(fakeClient)` is passed to each reconcile sub-function. Tests verify that conditions are set correctly by inspecting the status manager's behavior (no direct status assertions needed since the manager uses the fake client).

### Required Test Scenarios Per Sub-Function

Every `reconcileX` function must cover these scenarios:

1. **Create success** — `Get` returns `NotFound`, `Create` returns nil.
2. **Create error** — `Get` returns `NotFound`, `Create` returns error.
3. **Get error** — `Get` returns non-`NotFound` error.
4. **Update success** — resource exists with drift, `Update` returns nil.
5. **Update error** — resource exists with drift, `Update` returns error.
6. **Create-only mode skip** — resource exists, `createOnlyMode=true`, no Update.
7. **SetControllerReference error** — empty scheme causes ref failure.

## E2E Tests

### Location & Execution

```bash
# Run E2E (requires KUBECONFIG pointing to a live cluster)
OPERATOR_NAMESPACE=zero-trust-workload-identity-manager make test-e2e

# With custom timeout
E2E_TIMEOUT=60m make test-e2e
```

### Framework

- **Ginkgo v2** with `Ordered` spec containers
- **Gomega** matchers with `Eventually`/`Consistently` for async assertions
- **Live OpenShift cluster** — no mocks at the E2E level
- JUnit XML report output to `ARTIFACT_DIR` (CI) or `/tmp` (local)

### Timeouts (from `test/e2e/utils/constants.go`)

| Constant | Value | Purpose |
|----------|-------|---------|
| `DefaultInterval` | 10s | Polling interval for `Eventually` |
| `ShortInterval` | 5s | Fast-polling for CRD/condition checks |
| `DefaultTimeout` | 5min | Standard wait for resources |
| `ShortTimeout` | 2min | CRD establishment, deployment available |
| `TestContextTimeout` | 10min | Per-test context deadline |
| `E2E_TIMEOUT` (Makefile) | 45min | Suite-level go test timeout |

### Test Structure

```go
var _ = Describe("Zero Trust Workload Identity Manager", Ordered, func() {
    BeforeAll(func() { /* discover cluster, find subscription */ })
    BeforeEach(func() { testCtx, cancel = context.WithTimeout(..., TestContextTimeout) })

    Context("Installation", func() { /* CRD, Deployment checks */ })
    Context("OperatorCondition", func() { /* OLM condition checks */ })
    Context("SpireAgent attestation", func() { /* SVID issuance end-to-end */ })
    Context("Common configurations", func() { /* Labels, resources, tolerations */ })
    Context("CreateOnlyMode", func() { /* Create-only mode behavior */ })
})
```

### Artifacts

When `OPENSHIFT_CI=true`, results go to `$ARTIFACT_DIR`:
- `junit.xml` — Ginkgo JUnit report
- Pod logs collected on failure

## How to Add a New Unit Test

1. Identify the reconciler sub-function to test (e.g., `reconcileFoo`).
2. Create or open the corresponding test file (`foo_test.go` in the same package).
3. Write a `newFooTestReconciler` factory if the existing one doesn't fit.
4. Define a table-driven test struct with `name`, `setupClient`, and expected outcomes.
5. In each case, create `&fakes.FakeCustomCtrlClient{}`, configure stubs, call the function, assert.
6. Run: `OPERATOR_NAMESPACE=zero-trust-workload-identity-manager go test ./pkg/controller/<pkg>/... -v -run TestYourFunction`

## How to Add a New E2E Test

1. Add a new `It(...)` or `Context(...)` block in `test/e2e/e2e_test.go`.
2. Use `testCtx` (10-min deadline) for all API calls.
3. Use helpers from `test/e2e/utils/`:
   - `WaitForDeploymentAvailable`, `WaitForStatefulSetReady`, `WaitForDaemonSetAvailable`
   - `WaitForSpireServerConditions`, `WaitForSpireAgentConditions`
   - `SetupAttestationTest` for full attestation fixtures
   - `UpdateCRWithRetry` for safe spec modifications
4. Register cleanup with `DeferCleanup(func(ctx context.Context) { ... })`.
5. Write assertions using `Eventually(...).WithTimeout(DefaultTimeout).WithPolling(DefaultInterval)`.
6. Run locally: `KUBECONFIG=~/.kube/config OPERATOR_NAMESPACE=zero-trust-workload-identity-manager go test ./test/e2e/ -v -timeout 45m -run "TestE2E/YourTest"`

## CI/CD Testing Configuration

| CI Component | Value |
|--------------|-------|
| Build image | `ocp/builder:rhel-9-golang-1.25-openshift-4.21` |
| Lint tool | golangci-lint v1.59.1 |
| Lint timeout | 5 minutes |
| Unit coverage | `cover.out` (via `make test`) |
| E2E coverage | `Dockerfile.coverage` + `hack/e2e-coverage.sh` |
| envtest k8s version | 1.31.0 |

### Linters Enabled (`.golangci.yml`)

dupl, errcheck, exportloopref, ginkgolinter, goconst, gocyclo, gofmt, goimports, gosimple, govet, ineffassign, lll, misspell, nakedret, prealloc, revive, staticcheck, typecheck, unconvert, unparam, unused
