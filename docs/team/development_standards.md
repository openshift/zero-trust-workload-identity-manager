# Development Standards

## Language and Toolchain

- **Go 1.25** (specified in `go.mod`)
- **Vendored dependencies** — all dependencies are committed via `go mod vendor`; run
  `make vendor` after any dependency change
- **Scaffolding** — Kubebuilder v4 with Operator SDK v1.39.0 extensions

## Code Organization

- One controller per CRD kind, located in `pkg/controller/<crd-name>/`
- Static Kubernetes resources (RBAC, ServiceAccounts, Services) live as YAML in `bindata/`
  and are loaded via go-bindata at runtime
- Resources whose shape depends on CR spec fields are constructed programmatically in Go
- Shared utilities (constants, labels, validation, proxy) go in `pkg/controller/utils/`
- API type definitions live in `api/v1alpha1/`; run `make manifests` after any change

## Formatting and Style

- `gofmt` and `goimports` are enforced — run `make fmt` before committing
- Line length limit (`lll`) applies to `api/` files; relaxed for `pkg/` files
- Duplicate code detection (`dupl`) applies to `api/` files; relaxed for `pkg/`
- Use `fmt.Errorf("context: %w", err)` for error wrapping (never `%v`)

## Linting

golangci-lint is run via `make lint` with the following enabled linters:

`dupl`, `errcheck`, `exportloopref`, `ginkgolinter`, `goconst`, `gocyclo`, `gofmt`,
`goimports`, `gosimple`, `govet`, `ineffassign`, `lll`, `misspell`, `nakedret`, `prealloc`,
`revive`, `staticcheck`, `typecheck`, `unconvert`, `unparam`, `unused`

Configuration: `.golangci.yml` (5 minute timeout, parallel runners enabled).

## Build Commands

| Command | Purpose |
|---------|---------|
| `make build` | Full cycle: manifests + generate + fmt + vet + compile |
| `make manifests` | Regenerate CRDs, RBAC, webhook config from markers |
| `make generate` | Regenerate DeepCopy methods |
| `make verify` | vet + fmt + lint (CI gate) |
| `make vendor` | `go mod tidy` + `go mod vendor` |

## FIPS Compliance

Production builds must use `hack/go-fips.sh` which sets `GOEXPERIMENT=strictfipsruntime`.
`CGO_ENABLED=1` is required so the runtime links against OpenSSL for FIPS-validated crypto.
Never set `CGO_ENABLED=0` in CI or production builds.

## Detailed Guides

For in-depth patterns, see:

- [Error Handling Guidelines](../error-handling-guidelines.md)
- [API Contracts Guidelines](../api-contracts-guidelines.md)
- [Security Guidelines](../security-guidelines.md)
- [Performance Guidelines](../performance-guidelines.md)
- [Integration Guidelines](../integration-guidelines.md)
