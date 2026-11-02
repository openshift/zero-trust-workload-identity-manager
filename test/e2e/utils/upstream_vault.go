/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/gomega"
	operatorv1alpha1 "github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	testdataVaultStack     = "testdata/upstream-vault/vault.yaml"
	testdataVaultBootstrap = "testdata/upstream-vault/bootstrap.yaml"
)

type vaultTestdataValues struct {
	Namespace string
	VaultRole string
}

// VaultPluginData is SPIRE vault UpstreamAuthority plugin_data.
type VaultPluginData struct {
	VaultAddr     string
	PKIMountPoint string
}

// InstallVaultUpstream deploys Vault and runs bootstrap Job.
func InstallVaultUpstream(ctx context.Context, k8sClient client.Client, clientset kubernetes.Interface, namespace string) {
	applyTestdataManifest(ctx, k8sClient, testdataVaultStack, datastoreTestdataValues{Namespace: namespace})
	WaitForDeploymentAvailable(ctx, clientset, VaultServiceName, namespace, DatastoreSetupTimeout)

	_ = clientset.BatchV1().Jobs(namespace).Delete(ctx, "e2e-vault-bootstrap", metav1.DeleteOptions{})
	applyTestdataManifest(ctx, k8sClient, testdataVaultBootstrap, vaultTestdataValues{
		Namespace: namespace,
		VaultRole: VaultK8sAuthRoleName,
	})
	WaitForJobComplete(ctx, clientset, namespace, "e2e-vault-bootstrap", DatastoreSetupTimeout)
}

// PatchSpireServerVaultUpstream enables vault UpstreamAuthority on SpireServer.
func PatchSpireServerVaultUpstream(ctx context.Context, k8sClient client.Client, namespace string) {
	addr := vaultServiceAddr(namespace)
	server := &operatorv1alpha1.SpireServer{}
	server.Name = OperandCRName
	err := UpdateCRWithRetry(ctx, k8sClient, server, func() {
		server.Spec.UpstreamAuthority = &operatorv1alpha1.UpstreamAuthorityConfig{
			Vault: &operatorv1alpha1.UpstreamAuthorityVault{
				VaultAddr:          addr,
				InsecureSkipVerify: true,
				K8sAuth: &operatorv1alpha1.VaultK8sAuthConfig{
					K8sAuthRoleName: VaultK8sAuthRoleName,
				},
			},
		}
	})
	Expect(err).NotTo(HaveOccurred(), "patch SpireServer vault upstreamAuthority")
}

// WaitForSpireServerReadyAfterVaultChange wipes PVC, waits for server, restarts agents.
func WaitForSpireServerReadyAfterVaultChange(ctx context.Context, k8sClient client.Client, clientset kubernetes.Interface, namespace string) {
	WipeSpireServerPVC(ctx, clientset, namespace)
	WaitForSpireServerConditions(ctx, k8sClient, OperandCRName, map[string]metav1.ConditionStatus{
		"Ready":                    metav1.ConditionTrue,
		"StatefulSetAvailable":     metav1.ConditionTrue,
		"ServerConfigMapAvailable": metav1.ConditionTrue,
	}, 2*DefaultTimeout)
	WaitForStatefulSetReady(ctx, clientset, SpireServerStatefulSetName, namespace, 2*DefaultTimeout)
	RestartSpireAgents(ctx, clientset, namespace)
	WaitForSpireAgentsSynced(ctx, clientset, namespace, 2*DefaultTimeout)
}

// GetVaultPluginData reads vault UpstreamAuthority plugin_data from server.conf.
func GetVaultPluginData(ctx context.Context, clientset kubernetes.Interface, namespace string) (*VaultPluginData, error) {
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, SpireServerConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	raw := cm.Data[SpireServerConfigKey]
	var conf map[string]any
	if err := json.Unmarshal([]byte(raw), &conf); err != nil {
		return nil, err
	}
	plugins, _ := conf["plugins"].(map[string]any)
	ua, _ := plugins["UpstreamAuthority"].([]any)
	if len(ua) == 0 {
		return nil, fmt.Errorf("UpstreamAuthority missing")
	}
	first, _ := ua[0].(map[string]any)
	vaultPlugin, _ := first["vault"].(map[string]any)
	pd, _ := vaultPlugin["plugin_data"].(map[string]any)
	if pd == nil {
		return nil, fmt.Errorf("vault plugin_data missing")
	}
	addr, _ := pd["vault_addr"].(string)
	mount, _ := pd["pki_mount_point"].(string)
	return &VaultPluginData{VaultAddr: addr, PKIMountPoint: mount}, nil
}

// CleanupVaultUpstream clears upstreamAuthority and removes vault test resources.
func CleanupVaultUpstream(ctx context.Context, k8sClient client.Client, clientset kubernetes.Interface, namespace string) {
	ClearSpireServerUpstreamAuthority(ctx, k8sClient)
	WaitForSpireServerReadyAfterVaultChange(ctx, k8sClient, clientset, namespace)
	deleteDatastoreE2EResources(ctx, k8sClient, clientset, namespace, "vault")
}

func WaitForJobComplete(ctx context.Context, clientset kubernetes.Interface, namespace, name string, timeout time.Duration) {
	Eventually(func() bool {
		job, err := clientset.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return job.Status.Succeeded > 0
	}).WithTimeout(timeout).WithPolling(ShortInterval).Should(BeTrue(), "Job %s should complete", name)
}

