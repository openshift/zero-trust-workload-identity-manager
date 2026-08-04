# SpireServer

| Field     | Value                                         |
|-----------|-----------------------------------------------|
| API Group | `operator.openshift.io`                       |
| Version   | `v1alpha1`                                    |
| Kind      | `SpireServer`                                 |
| Scope     | `Cluster`                                     |
| Source    | `api/v1alpha1/spire_server_config_types.go`    |

## Purpose

Configures the SPIRE Server operand — the central authority that manages SPIFFE identity registration entries, issues X.509 and JWT SVIDs, manages CA key material, and serves as the trust anchor for the cluster. This is the most complex CRD in the ZTWIM stack.

## Key Principle

**The CA root of the trust domain.** The SPIRE Server owns the CA lifecycle — key type, validity periods, upstream authority chain, and data persistence. Several fields are immutable after creation (persistence settings, federation profile) because changing them would break the identity infrastructure.

## Spec Structure

```go
type SpireServerSpec struct {
    LogLevel            string                   // Enum: debug|info|warn|error. Default: "info"
    LogFormat           string                   // Enum: text|json. Default: "text"
    JwtIssuer           string                   // Required. HTTPS/HTTP URL for JWT issuer.
    CAValidity          metav1.Duration          // Default: 24h. TTL of CA certificate.
    DefaultX509Validity metav1.Duration          // Default: 1h. TTL of X.509 SVIDs.
    DefaultJWTValidity  metav1.Duration          // Default: 5m. TTL of JWT SVIDs.
    CAKeyType           string                   // Enum: rsa-2048|rsa-4096|ec-p256|ec-p384. Default: rsa-2048.
    JWTKeyType          string                   // Optional. Same enum as CAKeyType.
    KeyManager          *KeyManager              // Disk and/or memory key manager toggles.
    CASubject           CASubject                // Required. Country, Org, CommonName for CA cert.
    Persistence         Persistence              // Required, immutable. PVC size/accessMode/storageClass.
    Datastore           DataStore                // Required. SQL backend config (sqlite3, postgres, mysql, etc.).
    Federation          *FederationConfig        // Optional. Bundle endpoint + federated trust domains.
    UpstreamAuthority   *UpstreamAuthorityConfig // Optional. cert-manager or Vault upstream CA.
    CommonConfig                                 // Inline: labels, resources, affinity, tolerations, nodeSelector.
}
```

### Sub-types

**Persistence** (immutable once set):
- `size`: PVC size (default `1Gi`, pattern `^[1-9][0-9]*Gi$`)
- `accessMode`: `ReadWriteOnce` | `ReadWriteOncePod` | `ReadWriteMany`
- `storageClass`: optional

**DataStore**:
- `databaseType`: `sqlite3` (default) | `postgres` | `mysql` | `sql` | `aws_postgresql` | `aws_mysql`
- `connectionString`: default `/run/spire/data/datastore.sqlite3`
- `tlsSecretName`: optional Secret with `ca.crt`, `tls.crt`, `tls.key` mounted at `/run/spire/db/certs`
- `maxOpenConns`, `maxIdleConns`, `connMaxLifetime`, `disableMigration`

**FederationConfig**:
- `bundleEndpoint`: profile (`https_spiffe` | `https_web`), refreshHint (60–3600s, default 300)
- `federatesWith[]`: remote trust domains with endpoint URL and auth profile
- `managedRoute`: `"true"` | `"false"` — auto-create OpenShift Route

**UpstreamAuthorityConfig** (exactly one of):
- `certManager`: namespace, issuerName, issuerKind, issuerGroup
- `vault`: vaultAddr, pkiMountPoint, caCertSecretRef, k8sAuth (role, mount, audience)

**KeyManager**:
- `diskEnabled`: `"true"` (default) | `"false"`
- `memoryEnabled`: `"false"` (default) | `"true"`

## Key Concepts

- **CA Validity Chain**: `CAValidity` (CA cert TTL) must be longer than `DefaultX509Validity` (workload SVID TTL). The server rotates the CA before expiry.
- **Upstream Authority**: When absent, SPIRE self-signs the root CA. When configured (cert-manager or Vault), SPIRE acts as an intermediate CA.
- **Federation**: Exposes a bundle endpoint (port 8443) so remote trust domains can fetch this cluster's trust bundle. Supports SPIFFE-authenticated (`https_spiffe`) or Web PKI (`https_web`) profiles.
- **Persistence Immutability**: `size`, `accessMode`, and `storageClass` are validated as immutable at the top-level CR via CEL rules — changing them would corrupt the StatefulSet's PVC.
- **Federation Irreversibility**: Once `federation` is set, it cannot be removed (CEL: `oldSelf == null || !has(oldSelf.spec.federation) || has(self.spec.federation)`).

## Lifecycle

1. **Create**: Operator creates StatefulSet with PVC, configures SPIRE server config file, mounts secrets.
2. **CA Bootstrap**: Server generates (or receives from upstream) a root/intermediate CA based on `caKeyType` and `caSubject`.
3. **Steady State**: Server issues SVIDs, rotates CA, serves registration API to controller-manager.
4. **Federation**: If configured, opens bundle endpoint on 8443, creates OpenShift Route (if `managedRoute: "true"`).
5. **Database Migration**: Runs automatically unless `disableMigration: "true"`.

## Example YAML

```yaml
apiVersion: operator.openshift.io/v1alpha1
kind: SpireServer
metadata:
  name: cluster
spec:
  logLevel: info
  jwtIssuer: https://oidc-discovery.apps.prod.example.com
  caValidity: 24h
  defaultX509Validity: 1h
  defaultJWTValidity: 5m
  caKeyType: ec-p256
  caSubject:
    country: US
    organization: Example Corp
    commonName: SPIRE CA
  persistence:
    size: 5Gi
    accessMode: ReadWriteOnce
  datastore:
    databaseType: sqlite3
    connectionString: /run/spire/data/datastore.sqlite3
  resources:
    requests:
      cpu: 200m
      memory: 512Mi
```

## Component-Specific Behavior

- **HTTPS Web Federation with ACME**: When `federation.bundleEndpoint.profile` is `https_web` and `httpsWeb.acme` is set, the server auto-provisions TLS certs via Let's Encrypt. `tosAccepted` must be `"true"`.
- **Vault Upstream Authority**: Uses Kubernetes auth method — a projected SA token is mounted into the pod. Zero static credentials.
- **TLS Database Connections**: Set `tlsSecretName` and reference cert paths in `connectionString` (e.g., `sslmode=verify-full sslrootcert=/run/spire/db/certs/ca.crt`).

## Common Mistakes

- **Setting `persistence.size` too small** — once created, it's immutable. Plan for growth.
- **Mismatching `jwtIssuer` with the OIDC Discovery Provider's `jwtIssuer`** — they must agree. See [SpireOIDCDiscoveryProvider](spire-oidc-discovery-provider.md#field-details).
- **Forgetting `endpointSpiffeId` for `https_spiffe` federation** — CEL validation requires it when profile is `https_spiffe`.
- **Switching between ACME and servingCert** — the `httpsWeb` config is immutable once the choice is made.
- **Setting `CAValidity` shorter than `DefaultX509Validity`** — issued SVIDs would outlive their signing CA.

See also: [`jwtIssuer` must match SpireOIDCDiscoveryProvider](spire-oidc-discovery-provider.md#field-details); [upstream SPIFFE CRDs](upstream-spiffe-crds.md) for workload identity registration.

---
