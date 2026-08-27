# Code Review Standards

## Reviewers and Approvers

All reviewers and approvers are listed in the `OWNERS` file at the repo root:

| GitHub Handle | Roles |
|---------------|-------|
| TrilokGeer | Reviewer, Approver |
| rausingh-rh | Reviewer, Approver |
| bharath-b-rh | Reviewer, Approver |
| swghosh | Reviewer, Approver |
| nhegde07 | Reviewer, Approver |

## Approval Requirements

A PR requires both:

- At least one `/lgtm` from a reviewer (signals "code looks good")
- At least one `/approve` from an approver (signals "change is appropriate for the project")

Both can come from the same person if they hold both roles.

## Prow Commands

| Command | Effect |
|---------|--------|
| `/lgtm` | Add LGTM label (reviewer approval) |
| `/lgtm cancel` | Remove LGTM label |
| `/approve` | Add Approved label (approver approval) |
| `/approve cancel` | Remove Approved label |
| `/hold` | Block merge (adds `do-not-merge/hold`) |
| `/hold cancel` | Release hold |
| `/retest` | Re-run failed CI checks |
| `/cc @username` | Request review from a specific person |
| `/assign @username` | Assign PR to someone |

## Turnaround Expectations

[TODO: team to fill in — target review turnaround time, e.g., "initial review within 1 business day"]

## What Reviewers Should Check

- [ ] Tests are present for new/changed logic
- [ ] `make verify` passes (lint, fmt, vet)
- [ ] No hardcoded image references (images must come from environment variables)
- [ ] Status conditions are updated when reconciliation behavior changes
- [ ] Static resources use bindata YAML; only spec-dependent resources are built in Go
- [ ] API changes include updated CRD markers and `make manifests` was run
- [ ] Error messages use `%w` wrapping with actionable context
- [ ] No unrelated changes bundled into the PR

## Merge Criteria

A PR is eligible to merge when:

1. CI is green (`make verify` + `make test` pass)
2. At least one `/lgtm` and one `/approve` are present
3. No `/hold` label is active
4. No unresolved review comments marked as "changes requested"

## PR Best Practices

- Keep PRs focused — one logical change per PR
- Write a clear description explaining the *why*, not just the *what*
- If a PR touches multiple controllers, explain the cross-cutting reason
- Address review feedback with additional commits (do not force-push during review)
- Use draft PRs for work-in-progress that needs early feedback
