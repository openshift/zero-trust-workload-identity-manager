# SpiffeCSIDriver

| Field     | Value                                        |
|-----------|----------------------------------------------|
| API Group | `operator.openshift.io`                      |
| Version   | `v1alpha1`                                   |
| Kind      | `SpiffeCSIDriver`                            |
| Scope     | `Cluster`                                    |
| Source    | `api/v1alpha1/spiffe_csi_config_types.go`     |

## Purpose

Configures the SPIFFE CSI Driver operand — a Kubernetes CSI ephemeral volume driver that bind-mounts the SPIRE Agent's Workload API socket into workload pods. This is the bridge that lets workloads transparently access SPIFFE identities without knowing where the agent socket lives on the host.

## Key Principle

**No hostPath in workload specs.** Instead of requiring workloads to declare `hostPath` volumes (which need elevated privileges), the CSI driver presents the agent socket as a CSI ephemeral volume. Workloads reference `csi.spiffe.io` in their volume spec and get the socket injected.

## Spec Structure

```go
type SpiffeCSIDriverSpec struct {
    AgentSocketPath string       // Default: "/run/spire/agent-sockets". Must match SpireAgent.spec.socketPath.
    PluginName      string       // Default: "csi.spiffe.io". CSI driver name registered with kubelet.
    CommonConfig                 // Inline: labels, resources, affinity, tolerations, nodeSelector.
}
```

### Field Details

- `agentSocketPath`: absolute path (1–256 chars, pattern `^/[a-zA-Z0-9._/\-]+$`). Default: `/run/spire/agent-sockets`. The CSI driver reads the SPIRE agent socket from this host directory.
- `pluginName`: domain-name format (max 127 chars). Default: `csi.spiffe.io`. This is the `driver` name workloads use in `csi` volume definitions. Must match [`SpireOIDCDiscoveryProvider.spec.csiDriverName`](spire-oidc-discovery-provider.md#field-details).

## Key Concepts

- **Socket Path Pair**: [`agentSocketPath`](spiffe-csi-driver.md#field-details) here and [`SpireAgent.spec.socketPath`](spire-agent.md#spec-structure) must match — the agent writes to the path and the CSI driver reads from it. The workload's `volumeMounts[].mountPath` does **not** need to match; the CSI driver bind-mounts the agent socket directory into whatever path the workload declares.
- **CSI Ephemeral Volumes**: The driver registers as a CSI plugin with the kubelet. When a pod with a `csi.spiffe.io` volume starts, kubelet calls the CSI driver's `NodePublishVolume` to bind-mount the socket into the pod.
- **Plugin Name**: Changing `pluginName` from the default requires updating all workload pod specs to reference the new driver name.
- **DaemonSet Deployment**: The CSI driver runs as a DaemonSet (one per node), co-located with the SPIRE Agent.

## Lifecycle

1. **Create**: Operator creates a DaemonSet for the CSI driver and registers the `CSIDriver` object with Kubernetes.
2. **Registration**: kubelet discovers the CSI plugin via the registration socket.
3. **Pod Scheduling**: When a pod with a `csi.spiffe.io` volume is scheduled, kubelet calls `NodePublishVolume` on the CSI driver.
4. **Mount**: CSI driver bind-mounts the agent socket directory into the pod's volume mount path.
5. **Unmount**: On pod termination, kubelet calls `NodeUnpublishVolume` to clean up the mount.

## Example YAML

```yaml
apiVersion: operator.openshift.io/v1alpha1
kind: SpiffeCSIDriver
metadata:
  name: cluster
spec:
  agentSocketPath: /run/spire/agent-sockets
  pluginName: csi.spiffe.io
  resources:
    requests:
      cpu: 50m
      memory: 64Mi
  tolerations:
    - operator: Exists
```

### Workload Volume Reference

```yaml
volumes:
  - name: spiffe-workload-api
    csi:
      driver: csi.spiffe.io
      readOnly: true
containers:
  - name: app
    volumeMounts:
      - name: spiffe-workload-api
        mountPath: /spiffe-workload-api
        readOnly: true
```

## Component-Specific Behavior

- **No Storage**: This is a *nodeless*, *ephemeral-only* CSI driver. It does not provision PersistentVolumes. It only does bind-mounts.
- **Security**: The CSI driver itself needs host access to the agent socket directory. Workloads do not — they just see the socket via the CSI volume.
- **Singleton Plugin Name**: Only one CSI driver instance per `pluginName` can exist in a cluster. Multiple ZTWIM installations would conflict.

## Common Mistakes

- **`agentSocketPath` mismatch with SpireAgent** — the most common issue. If these paths diverge, the CSI driver mounts an empty directory and workloads fail to connect to the Workload API. See [SpireAgent.spec.socketPath](spire-agent.md#spec-structure).
- **Changing `pluginName` without updating workloads** — existing pods reference the old driver name and will fail to schedule.
- **Missing tolerations** — like the agent, the CSI driver must run on all nodes where workloads need SPIFFE identities.
- **Forgetting `readOnly: true` on workload volume mounts** — while functional without it, the socket should always be mounted read-only for security.

See also: [SpireAgent](spire-agent.md) for socket path pairing; [SpireOIDCDiscoveryProvider](spire-oidc-discovery-provider.md) for `pluginName`/`csiDriverName` consistency.

---
