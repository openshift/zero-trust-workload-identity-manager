# CI/CD Configuration

## CI System

The project uses **OpenShift CI (Prow)** for PR validation and merge gating, configured via
`.ci-operator.yaml` at the repo root.

### Build Root

```yaml
build_root_image:
  name: builder
  namespace: ocp
  tag: rhel-9-golang-1.25-openshift-4.21
```

This defines the base image used for CI builds — RHEL 9 with Go 1.25 targeting OCP 4.21.

## Required Checks Before Merge

| Check | What It Runs |
|-------|--------------|
| `make verify` | `go vet` + `gofmt` + golangci-lint |
| `make test` | Unit tests via envtest (K8s 1.31.0 binaries) |

Both must pass for a PR to be mergeable. Use `/retest` to re-trigger failed checks.

## E2E Testing

E2E tests (`make test-e2e`) run against a live OpenShift cluster with a 45-minute timeout.
These are triggered on specific Prow jobs, not on every PR push.

## Image Builds

### Operator Image

The `Dockerfile` uses a multi-stage build:
- **Builder stage**: `registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.25-openshift-4.21`
- **Runtime stage**: `registry.access.redhat.com/ubi9-minimal:9.4`
- Runs as non-root UID `65532:65532`
- FIPS-compliant: uses `CGO_ENABLED=1` for OpenSSL linkage

### Bundle Image

The `bundle.Dockerfile` packages OLM metadata (CSV, CRDs, scorecard tests) as a scratch-based
image for catalog inclusion.

## FIPS Requirement

Production and CI builds must use `hack/go-fips.sh` which sets:
- `GOEXPERIMENT=strictfipsruntime`
- Build tags: `strictfipsruntime,openssl`

A build that prints `WARN: building without FIPS support` is not shippable.

## OLM Bundle

Generated via `make bundle`, which produces:
- `bundle/manifests/` — ClusterServiceVersion + CRDs
- `bundle/metadata/` — annotations for OLM
- `bundle/tests/scorecard/` — OLM scorecard tests

Current version: `1.1.0` (set in `Makefile` `VERSION` variable).

## Release Branches

| Branch | Target | Go Version | OCP Version |
|--------|--------|------------|-------------|
| `main` | Development | 1.25 | 4.21 |
| `release-1.1` | Production release | 1.25 | 4.21 |
| `ai-staging-release-1.0.0` | Stage (Konflux) | 1.23 | 4.19 |
| `ai-staging-release-0.2.0` | Stage (Konflux, earlier) | 1.23 | 4.19 |

## Stage Builds (Konflux)

Stage builds publish to `registry.stage.redhat.io` via the Konflux/Tekton pipeline:

- **Registry path**: `registry.stage.redhat.io/zero-trust-workload-identity-manager/`
- **Components**: operator image + operator-bundle image
- **Pipeline checks**: `check-labels` (validates image `name` label matches registry path)
- **Branches**: `ai-staging-release-*` branches track stage releases

The bundle image requires a `LABEL name=` matching the registry path for the `check-labels`
pipeline task to pass.

## Deployment

| Method | Command |
|--------|---------|
| CRDs only | `make install` / `make uninstall` |
| Full operator | `make deploy IMG=<registry>/<image>:<tag>` |
| Local development | `make run` (runs against connected cluster) |
| Consolidated installer | `make build-installer` (generates `dist/install.yaml`) |
