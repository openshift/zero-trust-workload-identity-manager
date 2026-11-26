package spire_server

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	spiffev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

const (
	downstreamClusterLabel  = "ztwim.openshift.io/downstream-cluster"
	downstreamEntryPrefix   = "nested-spire-"
	downstreamClassName     = "zero-trust-workload-identity-manager-spire"
	downstreamKubeConfigKey = "kubeconfig"
	downstreamConfigPath    = "/run/spire/kubeconfigs"
)

type downstreamIssue struct {
	reason  string
	message string
}

func hasDownstreamClusters(spec *v1alpha1.SpireServerSpec) bool {
	return spec.NodeAttestor != nil &&
		spec.NodeAttestor.ExternalK8sPSAT != nil &&
		len(spec.NodeAttestor.ExternalK8sPSAT.Clusters) > 0
}

// sortedDownstreamClusters returns a sorted copy so that ConfigMap and
// StatefulSet generation is deterministic across reconcile loops.
func sortedDownstreamClusters(config *v1alpha1.SpireServerSpec) []v1alpha1.DownstreamCluster {
	if config.NodeAttestor == nil || config.NodeAttestor.ExternalK8sPSAT == nil {
		return nil
	}
	clusters := append([]v1alpha1.DownstreamCluster(nil), config.NodeAttestor.ExternalK8sPSAT.Clusters...)
	sort.Slice(clusters, func(i, j int) bool { return clusters[i].Name < clusters[j].Name })
	return clusters
}

// reconcileDownstreamClusters orchestrates downstream cluster reconciliation:
// validate Secrets, create/update ClusterStaticEntries for valid clusters,
// report status, and prune orphaned entries. When no downstream clusters are
// configured, remove the condition and prune entries for removed clusters.
func (r *SpireServerReconciler) reconcileDownstreamClusters(ctx context.Context, server *v1alpha1.SpireServer, ztwim *v1alpha1.ZeroTrustWorkloadIdentityManager, statusMgr *status.Manager, createOnlyMode bool) (*v1alpha1.SpireServer, []downstreamIssue, error) {
	if !hasDownstreamClusters(&server.Spec) {
		statusMgr.RemoveCondition(DownstreamClustersValid)
		if err := r.pruneDownstreamEntries(ctx, server, ztwim, createOnlyMode); err != nil {
			r.log.Error(err, "Failed to clean up obsolete downstream registrations")
			return nil, nil, err
		}
		return server, nil, nil
	}

	effectiveServer, issues, err := r.resolveDownstreamClusters(ctx, server, ztwim)
	if err != nil {
		statusMgr.AddCondition(DownstreamClustersValid, "KubeConfigReadFailed", err.Error(), metav1.ConditionFalse)
		return nil, nil, err
	}
	if hasDownstreamClusters(&effectiveServer.Spec) {
		if err := r.reconcileDownstreamEntries(ctx, effectiveServer, ztwim, statusMgr, createOnlyMode); err != nil {
			return nil, nil, err
		}
	}
	if len(issues) != 0 {
		statusMgr.AddCondition(DownstreamClustersValid, issues[0].reason, downstreamIssuesMessage(issues), metav1.ConditionFalse)
	}
	// Missing credentials do not offboard a configured cluster.
	if err := r.pruneDownstreamEntries(ctx, server, ztwim, createOnlyMode); err != nil {
		r.log.Error(err, "Failed to clean up obsolete downstream registrations")
		return nil, nil, err
	}
	return effectiveServer, issues, nil
}

// resolveDownstreamClusters validates each downstream cluster entry and returns
// an effectiveServer containing only the clusters with valid kubeconfig Secrets.
// Invalid clusters are excluded from config/volume generation and reported as issues.
// Transient API errors are returned so the reconciler retries rather than treating
// a temporarily unreachable Secret as absent.
func (r *SpireServerReconciler) resolveDownstreamClusters(ctx context.Context, server *v1alpha1.SpireServer, ztwim *v1alpha1.ZeroTrustWorkloadIdentityManager) (*v1alpha1.SpireServer, []downstreamIssue, error) {
	effective := server.DeepCopy()
	effective.Spec.NodeAttestor = nil
	clusters := sortedDownstreamClusters(&server.Spec)
	nameCounts := make(map[string]int, len(clusters))
	for _, cluster := range clusters {
		nameCounts[cluster.Name]++
	}
	var valid []v1alpha1.DownstreamCluster
	var issues []downstreamIssue
	for _, cluster := range clusters {
		if nameCounts[cluster.Name] != 1 || len(cluster.Name) > 63 || len(validation.IsDNS1123Subdomain(cluster.Name)) != 0 {
			issues = append(issues, downstreamIssue{"InvalidDownstreamCluster", fmt.Sprintf("Downstream %q requires a unique valid cluster name", cluster.Name)})
			continue
		}
		if cluster.Name == ztwim.Spec.ClusterName {
			issues = append(issues, downstreamIssue{"ClusterNameConflict", fmt.Sprintf("Downstream %q: Secret %s/%s cannot be used because the name matches the local cluster", cluster.Name, utils.GetOperatorNamespace(), downstreamSecretName(cluster))})
			continue
		}
		secret := &corev1.Secret{}
		key := types.NamespacedName{Namespace: utils.GetOperatorNamespace(), Name: downstreamSecretName(cluster)}
		if err := r.ctrlClient.Get(ctx, key, secret); err != nil {
			if !kerrors.IsNotFound(err) {
				return nil, nil, fmt.Errorf("get downstream credential Secret %s: %w", key, err)
			}
			issues = append(issues, downstreamIssue{"KubeConfigMissing", fmt.Sprintf("Downstream %q: credential Secret %s is missing", cluster.Name, key)})
			continue
		}
		if len(secret.Data[downstreamKubeConfigKey]) == 0 {
			issues = append(issues, downstreamIssue{"KubeConfigInvalid", fmt.Sprintf("Downstream %q: Secret %s must contain a non-empty %q key", cluster.Name, key, downstreamKubeConfigKey)})
			continue
		}
		valid = append(valid, cluster)
	}
	if len(valid) != 0 {
		effective.Spec.NodeAttestor = &v1alpha1.NodeAttestorConfig{ExternalK8sPSAT: &v1alpha1.ExternalK8sPSATConfig{Clusters: valid}}
	}
	return effective, issues, nil
}

func downstreamSecretName(cluster v1alpha1.DownstreamCluster) string {
	return "nested-spire-" + cluster.Name + "-kubeconfig"
}

// downstreamVolumeName hashes the cluster name to keep volume names within
// Kubernetes' 63-character limit ("downstream-" + 32 hex chars = 43 chars).
func downstreamVolumeName(cluster v1alpha1.DownstreamCluster) string {
	sum := sha256.Sum256([]byte(cluster.Name))
	return fmt.Sprintf("downstream-%x", sum[:16])
}

func downstreamKubeConfigDirectory(cluster v1alpha1.DownstreamCluster) string {
	return downstreamConfigPath + "/" + cluster.Name
}

// downstreamEntries builds two ClusterStaticEntries per downstream cluster:
//   - node-alias: maps the downstream sidecar agent to a stable SPIFFE ID
//     using k8s_psat attestation selectors (cluster, namespace, SA, pod name).
//   - downstream: grants the intermediate CA to the spire-server process
//     inside that pod via the unix:path workload attestor.
func downstreamEntries(server *v1alpha1.SpireServer, ztwim *v1alpha1.ZeroTrustWorkloadIdentityManager) []*spiffev1alpha1.ClusterStaticEntry {
	clusters := sortedDownstreamClusters(&server.Spec)
	entries := make([]*spiffev1alpha1.ClusterStaticEntry, 0, 2*len(clusters))
	for _, cluster := range clusters {
		aliasID := "spiffe://" + ztwim.Spec.TrustDomain + "/downstream/" + cluster.Name
		labels := utils.SpireServerLabels(server.Spec.Labels)
		labels[downstreamClusterLabel] = cluster.Name
		entries = append(entries,
			&spiffev1alpha1.ClusterStaticEntry{
				ObjectMeta: metav1.ObjectMeta{Name: downstreamEntryPrefix + cluster.Name + "-node-alias", Labels: labels},
				Spec: spiffev1alpha1.ClusterStaticEntrySpec{
					ClassName: downstreamClassName, ParentID: "spiffe://" + ztwim.Spec.TrustDomain + "/spire/server", SPIFFEID: aliasID,
					Selectors: []string{"k8s_psat:cluster:" + cluster.Name, "k8s_psat:agent_ns:" + utils.GetOperatorNamespace(), "k8s_psat:agent_sa:spire-server", "k8s_psat:agent_pod_name:spire-server-0"},
				},
			},
			&spiffev1alpha1.ClusterStaticEntry{
				ObjectMeta: metav1.ObjectMeta{Name: downstreamEntryPrefix + cluster.Name + "-downstream", Labels: labels},
				Spec: spiffev1alpha1.ClusterStaticEntrySpec{
					ClassName: downstreamClassName, ParentID: aliasID, SPIFFEID: aliasID + "/spire-server",
					Selectors: []string{"unix:path:/spire-server"}, Downstream: true,
				},
			},
		)
	}
	return entries
}

// isDownstreamEntryName identifies names generated for nested SPIRE registrations.
func isDownstreamEntryName(name string) bool {
	// Match on name prefix + suffix pattern.
	return strings.HasPrefix(name, downstreamEntryPrefix) &&
		(strings.HasSuffix(name, "-node-alias") || strings.HasSuffix(name, "-downstream"))
}

// reconcileDownstreamEntries creates or updates the desired ClusterStaticEntries.
// Reconciliation status is recorded after processing all entries; API errors are returned
// to the reconcile loop for retry.
func (r *SpireServerReconciler) reconcileDownstreamEntries(ctx context.Context, server *v1alpha1.SpireServer, ztwim *v1alpha1.ZeroTrustWorkloadIdentityManager, statusMgr *status.Manager, createOnly bool) error {
	var reconciliationIssue *downstreamIssue
	for _, desired := range downstreamEntries(server, ztwim) {
		issue, err := r.reconcileDownstreamEntry(ctx, server, desired, statusMgr, createOnly)
		if err != nil {
			if !statusMgr.HasCondition(DownstreamClustersValid) {
				statusMgr.AddCondition(DownstreamClustersValid, "EntryReconcileFailed", err.Error(), metav1.ConditionFalse)
			}
			return err
		}
		// Retain the first skipped update while reconciling the remaining entries.
		if reconciliationIssue == nil {
			reconciliationIssue = issue
		}
	}
	if reconciliationIssue != nil {
		statusMgr.AddCondition(DownstreamClustersValid, reconciliationIssue.reason, reconciliationIssue.message, metav1.ConditionFalse)
	} else {
		statusMgr.AddCondition(DownstreamClustersValid, "Ready", "All configured downstream ClusterStaticEntry resources are reconciled", metav1.ConditionTrue)
	}
	return nil
}

// pruneDownstreamEntries removes obsolete owned entries. Cleanup is separate
// from the validity of registrations for currently configured clusters.
func (r *SpireServerReconciler) pruneDownstreamEntries(ctx context.Context, server *v1alpha1.SpireServer, ztwim *v1alpha1.ZeroTrustWorkloadIdentityManager, createOnly bool) error {
	desiredNames := map[string]bool{}
	for _, desired := range downstreamEntries(server, ztwim) {
		desiredNames[desired.Name] = true
	}
	var entries spiffev1alpha1.ClusterStaticEntryList
	if err := r.ctrlClient.List(ctx, &entries, client.MatchingLabels{utils.AppManagedByLabelKey: utils.AppManagedByLabelValue}); err != nil {
		return fmt.Errorf("list downstream entries for cleanup: %w", err)
	}
	for i := range entries.Items {
		entry := &entries.Items[i]
		// Only prune nested entries controlled by the current SpireServer.
		if desiredNames[entry.Name] || !isDownstreamEntryName(entry.Name) || !metav1.IsControlledBy(entry, server) {
			continue
		}
		if createOnly {
			r.log.Info("Skipping obsolete downstream entry deletion due to create-only mode", "entry", entry.Name)
			continue
		}
		// UID prevents deleting a replacement object; ResourceVersion prevents
		// deleting an entry changed since we inspected it.
		options := client.Preconditions{UID: &entry.UID, ResourceVersion: &entry.ResourceVersion}
		if err := r.ctrlClient.Delete(ctx, entry, options); err != nil && !kerrors.IsNotFound(err) {
			return fmt.Errorf("delete obsolete downstream entry %s: %w", entry.Name, err)
		}
	}
	return nil
}

// reconcileDownstreamEntry reads each entry once and reports skipped updates
// or API errors. It does not inspect spire-controller-manager's status fields.
func (r *SpireServerReconciler) reconcileDownstreamEntry(ctx context.Context, server *v1alpha1.SpireServer, desired *spiffev1alpha1.ClusterStaticEntry, statusMgr *status.Manager, createOnly bool) (*downstreamIssue, error) {
	if err := controllerutil.SetControllerReference(server, desired, r.scheme); err != nil {
		return nil, err
	}
	existing := &spiffev1alpha1.ClusterStaticEntry{}
	err := r.ctrlClient.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if kerrors.IsNotFound(err) {
		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			if conflict := utils.HandleCreateConflict(err, desired, r.log, statusMgr, DownstreamClustersValid); conflict != nil {
				return nil, conflict
			}
			return nil, err
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !existing.DeletionTimestamp.IsZero() {
		return nil, fmt.Errorf("downstream entry %s is terminating", existing.Name)
	}
	owner := metav1.GetControllerOf(existing)
	if owner != nil && !metav1.IsControlledBy(existing, server) {
		return nil, fmt.Errorf("ClusterStaticEntry %q is controlled by another resource", existing.Name)
	}
	if !utils.ResourceNeedsUpdate(existing, desired) && metav1.IsControlledBy(existing, server) {
		return nil, nil
	}
	if createOnly {
		r.log.Info("Skipping downstream entry update due to create-only mode", "entry", existing.Name)
		description := fmt.Sprintf("Downstream %q, ClusterStaticEntry %q", desired.Labels[downstreamClusterLabel], desired.Name)
		return &downstreamIssue{
			reason:  utils.CreateOnlyModeStatusType,
			message: description + " differs from the desired configuration or ownership; updates are skipped because create-only mode is enabled",
		}, nil
	}
	desired.ResourceVersion = existing.ResourceVersion
	return nil, r.ctrlClient.Update(ctx, desired)
}

func downstreamIssuesMessage(issues []downstreamIssue) string {
	messages := make([]string, 0, len(issues))
	for _, issue := range issues {
		messages = append(messages, issue.message)
	}
	return strings.Join(messages, "; ")
}
