# Testing Standards

## Philosophy

Testing follows a two-tier architecture:

1. **Unit tests** — fast, isolated, run on every PR via envtest
2. **E2E tests** — integration-level, run against a live OpenShift cluster

Unit tests are always required for new or changed logic. E2E tests validate the full
reconciliation stack and are required for changes that affect resource creation or
cross-component interaction.

## What's Expected Per PR

- New reconciler logic or bug fixes must include unit tests
- Changes to resource templates (ConfigMaps, StatefulSets, DaemonSets) must have tests
  verifying the generated output
- E2E tests are required when adding new CRD fields that affect deployed resources
- Test-only PRs (adding coverage for existing code) are encouraged

## Frameworks

| Tier | Framework | Location |
|------|-----------|----------|
| Unit | Standard Go `testing` + controller-runtime envtest | `pkg/controller/<name>/*_test.go` |
| E2E | Ginkgo/Gomega | `test/e2e/` |

Unit tests use `setup-envtest` with Kubernetes 1.31.0 binaries (downloaded to `bin/` on
first run). The envtest framework spins up a local API server and etcd — no real cluster
needed.

## Coverage

[TODO: team to fill in — coverage thresholds or "no regression" policy]

Coverage-instrumented builds are available via `make docker-build-coverage` for E2E
coverage collection.

## Running Tests

| Command | What It Does |
|---------|--------------|
| `make test` | Unit tests via envtest. Excludes `test/e2e/`. |
| `make test-e2e` | E2E tests against a live cluster. Requires `KUBECONFIG`. 45 min timeout. |
| `make verify` | vet + fmt + lint (includes `ginkgolinter` for test style). |

## Test File Layout

Each reconciler sub-function gets its own test file:

```
pkg/controller/spire-server/
├── controller_test.go
├── service_account_test.go
├── service_test.go
├── rbac_test.go
├── configmaps_test.go
├── statefulset_test.go
└── suite_test.go
```

## Detailed Guide

For in-depth test patterns, fixtures, helper utilities, and assertion conventions, see:
[Testing Guidelines](../testing-guidelines.md)
