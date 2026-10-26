# RHCOS10: UBI9 Compatibility Test (Baseline)

## Purpose

Validate that the existing UBI9-based container image in the
`zero-trust-workload-identity-manager` operator runs correctly on RHCOS10
cluster nodes **without any changes**.

RHCOS10 ships with RHEL10 as its host OS. This PR triggers CI against an
RHCOS10 cluster to confirm the UBI9 container remains compatible before
committing to a base image migration.

**No Dockerfile changes.** This is a baseline/smoke test only.

## Current Base Image

Product images use `registry.access.redhat.com/ubi9-minimal:9.4`; the build
root uses `rhel-9-golang-1.23-openshift-4.19`:

| Image | Registry | Current Base |
|-------|----------|-------------|
| `zero-trust-workload-identity-manager` | `registry.access.redhat.com` | `ubi9-minimal:9.4` |
| build root (CI) | `registry.ci.openshift.org/ocp/builder` | `rhel-9-golang-1.23-openshift-4.18` |

## Test Scope

- Run the full e2e suite (`e2e`, `e2e-fips`) against an RHCOS10 cluster
- Confirm operator pods schedule and reach `Available` state
- Confirm no functional regressions in SPIRE server, SPIRE agent, SPIFFE CSI
  driver, OIDC discovery provider, and SPIRE controller manager components

## Expected Outcome

- All e2e tests pass on RHCOS10 nodes
- No image pull or runtime failures related to UBI9 on RHEL10 hosts

## Follow-up

If CI passes → **PR2** (`rhcos10-ubi10-migration`) migrates the base image
from UBI9 to UBI10 and the build root from RHEL9 to RHEL10.
