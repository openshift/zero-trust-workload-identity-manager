# Optional operand NetworkPolicy examples (reference only)

These manifests are **not** reconciled by the operator. They document port-scoped egress
policies that were considered during SPIRE-220 design. The shipped operand model reconciles
ingress plus a single **`ztwim-sys-*-egress-allow-all`** policy per server and agent.

The `*-egress-baseline.yaml` files are optional stricter egress examples only.

Use these files as starting points if you stop operator egress management or remove
allow-all and want stricter egress (database, Vault, federation peers, or cluster HTTP proxy).
Adjust namespaces, ports, and selectors for your environment before applying with `oc apply`.
