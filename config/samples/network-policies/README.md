# Optional operand NetworkPolicy examples (reference only)

These manifests are **not** reconciled by the operator. They document port-scoped egress
policies that were considered during SPIRE-220 design. The shipped operand model reconciles
ingress plus a single **`ztwim-sys-*-egress-allow-all`** policy per server and agent when
`spec.managedNetworkPolicy` is `"true"` (default).

## Why operator-managed egress is allow-all

SPIRE server and agent pods must reach optional backends (external or in-cluster Postgres/MySQL,
Vault, HTTP proxies, Kubernetes API, DNS, federation peers) that vary by deployment. We explored
deriving egress rules from the datastore connection string in the `SpireServer` CR.

That breaks when the database is reached through an in-cluster `Service`: the DSN port is usually
the Service **port** (`servicePort`), not the Pod **targetPort**, and those values are not paired
predictably. Resolving Service/Endpoints at reconcile time would also require watching objects
outside the operand CRs and still miss external hosts, Routes, and org-specific policy models.

After discussion with the OpenShift NetworkPolicy team (**@bbennett**), the agreed GA approach is:

- **Allow-all egress** in operator-managed `ztwim-sys-*-egress-allow-all` policies, with a
  **tight `podSelector`** (server or agent only).
- **This directory** for admins who want narrower egress (copy/adapt; not auto-applied).
- **`managedNetworkPolicy: "false"`** when the customer owns all operand NetworkPolicies.
- **No `networkPolicyRef` API** — managed vs fully customer-managed keeps the contract simple;
  cluster **AdminNetworkPolicy** can override where the platform requires it.

## Using the samples

The `*-egress-baseline.yaml` and related files are optional stricter egress examples only.

Use them if you set `managedNetworkPolicy: "false"`, remove allow-all, or layer additional rules.
Adjust namespaces, ports, and selectors for your environment before applying with `oc apply`.
