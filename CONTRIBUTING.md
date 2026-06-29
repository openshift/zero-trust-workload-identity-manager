# Contributing

We welcome contributions to the Zero Trust Workload Identity Manager. This guide covers
how to set up your development environment, build and test the project, and submit changes.

## Prerequisites

- Go 1.25+
- Docker 17.03+
- `kubectl` or `oc` CLI
- Access to an OpenShift 4.19+ cluster (for E2E testing)
- `golangci-lint` (installed automatically by `make lint` if missing)

## Getting Started

1. Fork the repository on GitHub
2. Clone your fork:
   ```sh
   git clone git@github.com:<your-username>/zero-trust-workload-identity-manager.git
   cd zero-trust-workload-identity-manager
   ```
3. Add the upstream remote:
   ```sh
   git remote add upstream git@github.com:openshift/zero-trust-workload-identity-manager.git
   ```
4. Verify the vendored dependencies are in sync:
   ```sh
   make vendor
   ```

## Development Workflow

The Makefile is the primary interface for all development tasks:

| Command | Purpose |
|---------|---------|
| `make manifests` | Regenerate CRDs, RBAC, and webhook config from code markers |
| `make generate` | Regenerate DeepCopy methods |
| `make fmt` | Run `gofmt` on all Go files |
| `make vet` | Run `go vet` |
| `make build` | Full build cycle: manifests + generate + fmt + vet + compile |
| `make lint` | Run golangci-lint |
| `make lint-fix` | Run golangci-lint with auto-fix |
| `make test` | Run unit tests with envtest |
| `make test-e2e` | Run E2E tests against a live cluster (45m timeout) |
| `make run` | Run the operator locally against a connected cluster |
| `make verify` | Run vet + fmt + lint (used by CI) |

Run `make help` to see all available targets.

## Code Organization

When contributing code, follow these conventions:

- **One controller per CRD** lives in `pkg/controller/<crd-name>/`
- **Static RBAC and ServiceAccount manifests** go in `bindata/` as YAML
- **Shared utilities** (constants, labels, validation) go in `pkg/controller/utils/`
- **API type changes** go in `api/v1alpha1/` — run `make manifests` afterward
- **New images** are referenced via environment variables in `pkg/controller/utils/constants.go`

## Linting

The project uses golangci-lint with a curated set of linters defined in `.golangci.yml`.
Key enabled linters include: `errcheck`, `govet`, `staticcheck`, `gofmt`, `goimports`,
`revive`, `misspell`, `ineffassign`, and `unused`.

Run `make lint` before submitting a PR. CI will reject PRs that fail lint.

## Testing

**Unit tests** use controller-runtime's envtest framework, which spins up a local API server
and etcd. Run with:
```sh
make test
```

**E2E tests** require a live OpenShift cluster. They create actual CRs and verify the full
SPIRE stack deploys correctly:
```sh
make test-e2e
```

## Pull Request Process

1. Create a branch from an up-to-date `main`:
   ```sh
   git fetch upstream
   git checkout -b my-feature upstream/main
   ```
2. Make your changes and ensure the following pass:
   ```sh
   make verify
   make test
   ```
3. Commit with a clear, descriptive message explaining the *why* behind the change.
4. Push to your fork and open a PR against `openshift/zero-trust-workload-identity-manager:main`.
5. PRs require approval from at least one approver listed in the `OWNERS` file.

## Code Review

- Reviewers and approvers are defined in the `OWNERS` file at the repo root.
- OpenShift CI (Prow) runs automatically on all PRs via `.ci-operator.yaml`.
- Use Prow commands for the review workflow: `/lgtm`, `/approve`, `/hold`, `/retest`.
- Address review feedback by pushing additional commits (do not force-push during review).

## Reporting Issues

Open an issue on the GitHub repository with:
- A clear description of the problem or feature request
- Steps to reproduce (for bugs)
- Expected vs. actual behavior
- Relevant logs or error messages
