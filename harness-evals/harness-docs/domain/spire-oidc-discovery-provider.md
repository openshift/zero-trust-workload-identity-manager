# SpireOIDCDiscoveryProvider

| Field     | Value                                                 |
|-----------|-------------------------------------------------------|
| API Group | `operator.openshift.io`                               |
| Version   | `v1alpha1`                                            |
| Kind      | `SpireOIDCDiscoveryProvider`                          |
| Scope     | `Cluster`                                             |
| Source    | `api/v1alpha1/spire_oidc_discovery_provider_types.go`  |

## Purpose

Configures the SPIRE OIDC Discovery Provider operand — a component that publishes OIDC discovery documents (`/.well-known/openid-configuration` and JWKS) so that external systems (cloud IAMs, service meshes, API gateways) can validate JWT-SVIDs issued by the SPIRE Server using standard OIDC token verification.

## Key Principle

**SPIFFE meets OIDC.** This bridges the SPIFFE identity world with systems that only understand OIDC/JWT. The provider serves the public keys (JWKS) corresponding to the SPIRE Server's JWT signing key, allowing any OIDC-aware relying party to verify JWT-SVIDs without SPIFFE-specific tooling.

## Spec Structure

```go
type SpireOIDCDiscoveryProviderSpec struct {
    LogLevel          string       // Enum: debug|info|warn|error. Default: "info"
    LogFormat         string       // Enum: text|json. Default: "text"
    CSIDriverName     string       // Default: "csi.spiffe.io". Must match SpiffeCSIDriver.spec.pluginName.
    JwtIssuer         string       // Required. Must match SpireServer.spec.jwtIssuer.
    ReplicaCount      int          // 1–5. Default: 1.
    ManagedRoute      string       // "true" (default) | "false". Auto-create OpenShift Route.
    ExternalSecretRef string       // Optional. Secret with TLS cert for the Route.
    CommonConfig                   // Inline: labels, resources, affinity, tolerations, nodeSelector.
}
```

### Field Details

- `csiDriverName`: must match [`SpiffeCSIDriver.spec.pluginName`](spiffe-csi-driver.md#field-details) so the OIDC provider pod can mount the Workload API socket via CSI.
- `jwtIssuer`: must match [`SpireServer.spec.jwtIssuer`](spire-server.md#spec-structure). The OIDC discovery document's `issuer` field is set to this value.
- `replicaCount`: controls the Deployment replica count (1–5). Scale up for HA in production.
- `managedRoute`: when `"true"`, operator creates an OpenShift Route pointing to the OIDC provider Service (typically `*.apps.<cluster>`).
- `externalSecretRef`: reference to a Secret containing a TLS certificate for the Route. Used when the default wildcard cert is insufficient (e.g., custom domain).

## Key Concepts

- **OIDC Discovery**: The provider serves `/.well-known/openid-configuration` and `/keys` (JWKS) endpoints. Cloud providers (AWS, GCP, Azure) can be configured to trust this issuer URL for workload identity federation.
- **JWT Issuer Consistency**: The `jwtIssuer` here, in the SpireServer, and the actual URL the OIDC provider is reachable at must all agree. Mismatches cause token validation failures.
- **CSI Driver Dependency**: The OIDC provider itself is a SPIFFE workload — it gets its own SVID via the CSI driver to authenticate to the SPIRE Server's Workload API.
- **Deployment (not DaemonSet)**: Unlike the Agent and CSI Driver, the OIDC provider runs as a Deployment with configurable replicas. It doesn't need to be on every node.
- **Route Management**: With `managedRoute: "true"`, the operator creates an OpenShift Route with edge TLS termination. Set to `"false"` to manage ingress manually (e.g., custom Ingress, external LB).

## Lifecycle

1. **Create**: Operator creates a Deployment, Service, and (optionally) an OpenShift Route.
2. **CSI Mount**: Each OIDC provider pod gets the SPIRE agent socket mounted via the SPIFFE CSI driver.
3. **Key Fetch**: The provider connects to the SPIRE Server's Workload API to fetch the current JWT signing keys.
4. **Serve**: Serves OIDC discovery and JWKS endpoints over HTTPS.
5. **Key Rotation**: When the SPIRE Server rotates JWT keys, the provider picks up new keys automatically.

## Example YAML

```yaml
apiVersion: operator.openshift.io/v1alpha1
kind: SpireOIDCDiscoveryProvider
metadata:
  name: cluster
spec:
  logLevel: info
  csiDriverName: csi.spiffe.io
  jwtIssuer: https://oidc-discovery.apps.prod.example.com
  replicaCount: 2
  managedRoute: "true"
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
```

## Component-Specific Behavior

- **Cloud IAM Integration**: Once the OIDC provider is exposed, configure cloud IAM (e.g., AWS IAM OIDC Provider, GCP Workload Identity Federation) to trust the `jwtIssuer` URL. Workloads can then exchange JWT-SVIDs for cloud credentials.
- **External TLS**: When using a custom domain (not `*.apps`), set `externalSecretRef` to a Secret with `tls.crt` and `tls.key` matching the custom domain.
- **Health Monitoring**: The provider's health is reported via `ConditionalStatus` conditions (Ready, Degraded) and aggregated into the parent ZeroTrustWorkloadIdentityManager's `status.operands[]`.

## Common Mistakes

- **`jwtIssuer` mismatch between SpireServer and SpireOIDCDiscoveryProvider** — the most critical consistency requirement. If they differ, JWT-SVIDs will fail validation at relying parties. See [SpireServer.spec.jwtIssuer](spire-server.md#spec-structure).
- **`csiDriverName` mismatch with SpiffeCSIDriver** — OIDC provider pods fail to mount the Workload API socket and can't fetch signing keys. See [SpiffeCSIDriver.spec.pluginName](spiffe-csi-driver.md#field-details).
- **Running only 1 replica in production** — the OIDC endpoint is on the critical path for cloud credential exchange. Use `replicaCount: 2+` for HA.
- **Forgetting DNS for the Route** — `managedRoute: "true"` creates the Route, but the `jwtIssuer` URL must resolve to the cluster's ingress. Ensure DNS is configured.
- **Not exposing the provider externally** — cloud IAM providers need to reach the OIDC endpoints from the internet (or via VPC peering). A cluster-internal-only Route won't work.

See also: [SpireServer](spire-server.md) for `jwtIssuer`; [SpiffeCSIDriver](spiffe-csi-driver.md) for `pluginName`; [upstream SPIFFE CRDs](upstream-spiffe-crds.md) for operator-managed `ClusterSPIFFEID` resources.

---
