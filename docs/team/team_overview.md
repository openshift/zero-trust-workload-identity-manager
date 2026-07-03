# Team Overview

## Who We Are

The Zero Trust Workload Identity team operates within Hybrid Platforms, responsible for
bringing SPIFFE/SPIRE-based workload identity to OpenShift clusters through a Day-2 operator.

## Mission

Enable zero-trust security for OpenShift workloads by providing automated, policy-driven
workload identity management — issuing, rotating, and verifying cryptographic identities
without requiring application changes.

## What We Own

- **zero-trust-workload-identity-manager** operator — the controller binary, its CRDs, and
  all reconciliation logic for deploying and managing the SPIRE stack
- **Operator lifecycle** — OLM bundle, ClusterServiceVersion, catalog entries
- **Operand configuration** — SPIRE Server, SPIRE Agent, SPIFFE CSI Driver, SPIRE Controller
  Manager, and OIDC Discovery Provider as deployed on OpenShift
- **Release branches** — production (`release-1.1`) and staging (`ai-staging-release-*`)
- **CI configuration** — OpenShift CI (Prow) jobs and Konflux pipelines for this operator

## What We Do NOT Own

- **Upstream SPIRE development** — we consume upstream releases, we don't maintain the SPIRE
  codebase itself
- **OpenShift Service Mesh (OSSM)** — OSSM integrates with our SPIRE agent socket, but OSSM
  is owned by a separate team
- **Cluster security policy** — we follow org-wide security guidelines; we don't set them
- **OCP platform release process** — we participate in release cuts but don't own the process

## Team Members

Current reviewers and approvers (from OWNERS):

| GitHub Handle | Role |
|---------------|------|
| TrilokGeer | Reviewer, Approver |
| rausingh-rh | Reviewer, Approver |
| bharath-b-rh | Reviewer, Approver |
| swghosh | Reviewer, Approver |
| nhegde07 | Reviewer, Approver |

[TODO: team to fill in — Tech Lead, Engineering Manager, Product Manager roles]

## Org Placement

[TODO: team to fill in — which pillar/group this team sits under within Hybrid Platforms]

## Repositories

| Repository | Purpose |
|------------|---------|
| `openshift/zero-trust-workload-identity-manager` | Main operator repo |

[TODO: team to fill in — any other repos the team owns or contributes to]
