# SpireAgent

| Field     | Value                                        |
|-----------|----------------------------------------------|
| API Group | `operator.openshift.io`                      |
| Version   | `v1alpha1`                                   |
| Kind      | `SpireAgent`                                 |
| Scope     | `Cluster`                                    |
| Source    | `api/v1alpha1/spire_agent_config_types.go`    |

## Purpose

Configures the SPIRE Agent operand — a per-node daemon that performs node attestation against the SPIRE Server, caches SVIDs, and exposes the Workload API socket to local workloads. Deployed as a DaemonSet.

## Key Principle

**Every node gets an agent; every agent proves its identity.** The agent bridges the trust gap between the SPIRE Server and individual workloads. It attests the node (via K8s PSAT), then attests each workload requesting an SVID (via K8s pod metadata).

## Spec Structure

```go
type SpireAgentSpec struct {
    SocketPath         string              // Default: "/run/spire/agent-sockets". Workload API socket dir.
    LogLevel           string              // Enum: debug|info|warn|error. Default: "info"
    LogFormat          string              // Enum: text|json. Default: "text"
    NodeAttestor       *NodeAttestor       // K8s PSAT node attestation config.
    WorkloadAttestors  *WorkloadAttestors  // K8s workload attestor config.
    CommonConfig                           // Inline: labels, resources, affinity, tolerations, nodeSelector.
}
```

### Sub-types

**NodeAttestor**:
- `k8sPSATEnabled`: `"true"` (default) | `"false"` — enable K8s Projected Service Account Token node attestation.

**WorkloadAttestors**:
- `k8sEnabled`: `"true"` (default) | `"false"` — enable K8s workload attestor.
- `disableContainerSelectors`: `"false"` (default) | `"true"` — disable container selectors (needed for Istio `holdApplicationUntilProxyStarts`).
- `useNewContainerLocator`: `"true"` (default) | `"false"` — use cgroups v2–compatible container locator.
- `workloadAttestorsVerification`: kubelet TLS verification config.

**WorkloadAttestorsVerification**:
- `type`: `auto` (default) | `hostCert` | `skip`
  - `auto`: uses OpenShift default path (`/etc/kubernetes/kubelet-ca.crt`)
  - `hostCert`: requires explicit `hostCertBasePath` + `hostCertFileName`
  - `skip`: disables kubelet TLS verification entirely
- `hostCertBasePath`: directory containing kubelet CA cert (required for `hostCert`)
- `hostCertFileName`: cert file name (required for `hostCert`, max 256 chars)

## Key Concepts

- **Socket Path Coordination**: `socketPath` must match `SpiffeCSIDriver.spec.agentSocketPath`. The agent creates a Unix socket in this directory; the CSI driver bind-mounts it into workload pods.
- **Node Attestation (PSAT)**: The agent presents a Kubernetes Projected Service Account Token to the SPIRE Server to prove which node it's running on. Enabled by default.
- **Workload Attestation**: When a workload connects to the Workload API socket, the agent queries the kubelet to verify the caller's pod identity (namespace, service account, labels, etc.).
- **Kubelet Verification Modes**: In OpenShift, `auto` mode finds the kubelet CA at `/etc/kubernetes/kubelet-ca.crt`. For custom setups or non-standard node images, use `hostCert` with explicit paths.
- **Container Locator**: The `useNewContainerLocator` flag enables cgroups v2 support, which is required on modern Linux kernels and OpenShift 4.12+.

## Lifecycle

1. **Create**: Operator creates a DaemonSet. Each agent pod mounts the host socket directory.
2. **Node Attestation**: On startup, agent presents PSAT to SPIRE Server and receives a node SVID.
3. **Workload API**: Agent listens on the Unix socket; workloads connect to request SVIDs.
4. **SVID Caching**: Agent caches and rotates SVIDs locally, reducing server load.
5. **Rolling Update**: DaemonSet updates are rolling by default. During update, workloads on a node lose socket access briefly.

## Example YAML

```yaml
apiVersion: operator.openshift.io/v1alpha1
kind: SpireAgent
metadata:
  name: cluster
spec:
  socketPath: /run/spire/agent-sockets
  logLevel: info
  nodeAttestor:
    k8sPSATEnabled: "true"
  workloadAttestors:
    k8sEnabled: "true"
    disableContainerSelectors: "false"
    useNewContainerLocator: "true"
    workloadAttestorsVerification:
      type: auto
  resources:
    requests:
      cpu: 100m
      memory: 256Mi
  tolerations:
    - operator: Exists
```

## Component-Specific Behavior

- **DaemonSet Tolerations**: Agents typically need `- operator: Exists` to run on all nodes including masters and infra nodes.
- **Istio Compatibility**: Set `disableContainerSelectors: "true"` when using Istio's `holdApplicationUntilProxyStarts` to avoid attestation race conditions.
- **cgroups v2**: `useNewContainerLocator: "true"` is required on OpenShift 4.12+ and any system using cgroups v2. Only set to `"false"` for legacy cgroups v1 environments.

## Common Mistakes

- **Mismatched `socketPath`** — if this doesn't match `SpiffeCSIDriver.spec.agentSocketPath`, workloads get "connection refused" when requesting SVIDs.
- **Missing tolerations** — without appropriate tolerations, agents won't schedule on tainted nodes (masters, infra), leaving those nodes without identity infrastructure.
- **Setting `type: skip` in production** — disabling kubelet TLS verification defeats node identity assurance. Use only for debugging.
- **Forgetting to set `hostCertBasePath` when `type: hostCert`** — CEL validation rejects the CR.
```

---
