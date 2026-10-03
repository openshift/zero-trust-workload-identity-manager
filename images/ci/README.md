# CI operand images (cert-manager pattern)

These Dockerfiles build SPIRE-related operand images in OpenShift CI from
**OpenShift midstream** git pins, instead of pulling personal Quay tags.

| Dockerfile | Binary | Midstream source (default `RELEASE_BRANCH`) |
|---|---|---|
| `spire-server.Dockerfile` | spire-server | `openshift/spiffe-spire` @ `release/v1.15.3` |
| `spire-agent.Dockerfile` | spire-agent | same |
| `spire-oidc-discovery-provider.Dockerfile` | oidc-discovery-provider | same |
| `spire-controller-manager.Dockerfile` | spire-controller-manager | `openshift/spiffe-spire-controller-manager` @ `release/v0.7.0` |

## How CI uses them

`openshift/release` ci-operator config for this repo should:

1. List each file under `images:` → `pipeline:spire-server`, etc.
2. Add `operator.substitutions` so CSV / `RELATED_IMAGE_*` pullspecs
   (e.g. `ghcr.io/spiffe/spire-server:.*`) are rewritten to those pipeline images
   during e2e (same idea as cert-manager’s `quay.io/jetstack/...` → `pipeline:cert-manager`).

## Release bumps

When ZTWIM ships a new operand version:

1. Land code on the midstream release branch (or cut a new `release/vX.Y.Z`).
2. Update `ARG RELEASE_BRANCH=` in these Dockerfiles (and ztwim-release submodules).
3. Keep CSV placeholders in sync if you bump visible tags.

## Prerequisites

SPIRE-714 tls_config / tlsConfig drop commits must be present on the pinned
branches (see `openshift/spiffe-spire` PR #6 and
`openshift/spiffe-spire-controller-manager` PR #4) before CI images include
that functionality.

CSI driver and node-driver-registrar are not built here; they continue to use
the pullspecs already set in `config/manager/manager.yaml`.
