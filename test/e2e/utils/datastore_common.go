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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	operatorv1alpha1 "github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultSQLiteConnectionString = "/run/spire/data/datastore.sqlite3"
	E2EDatastoreLabelKey          = "ztwim.e2e/datastore"
)

type datastoreTestdataValues struct {
	Namespace string
}

// SQLDatastorePluginData is the SPIRE sql DataStore plugin_data block.
type SQLDatastorePluginData struct {
	DatabaseType     string
	ConnectionString string
}

// RestoreSpireServerSQLiteDatastore reverts SpireServer to the default sqlite datastore.
func RestoreSpireServerSQLiteDatastore(ctx context.Context, k8sClient client.Client) {
	server := &operatorv1alpha1.SpireServer{}
	server.Name = OperandCRName
	err := UpdateCRWithRetry(ctx, k8sClient, server, func() {
		server.Spec.Datastore.DatabaseType = "sqlite3"
		server.Spec.Datastore.ConnectionString = DefaultSQLiteConnectionString
	})
	if err != nil && !apierrors.IsNotFound(err) {
		Expect(err).NotTo(HaveOccurred(), "failed to restore sqlite datastore on SpireServer")
	}
}

// RestoreSpireServerSQLiteAndWait reverts to sqlite and re-syncs SPIRE server and agents (journey AfterAll).
func RestoreSpireServerSQLiteAndWait(ctx context.Context, k8sClient client.Client, clientset kubernetes.Interface, namespace string) {
	RestoreSpireServerSQLiteDatastore(ctx, k8sClient)
	WaitForSpireServerReadyAfterDatastoreChange(ctx, k8sClient, clientset, namespace)
}

// ClearSpireServerUpstreamAuthority removes upstreamAuthority from SpireServer.
func ClearSpireServerUpstreamAuthority(ctx context.Context, k8sClient client.Client) {
	server := &operatorv1alpha1.SpireServer{}
	server.Name = OperandCRName
	err := UpdateCRWithRetry(ctx, k8sClient, server, func() {
		server.Spec.UpstreamAuthority = nil
	})
	if err != nil && !apierrors.IsNotFound(err) {
		Expect(err).NotTo(HaveOccurred(), "failed to clear upstreamAuthority on SpireServer")
	}
}

// WipeSpireServerPVC deletes the SPIRE server PVC (and pod) so a new datastore/CA can bootstrap.
func WipeSpireServerPVC(ctx context.Context, clientset kubernetes.Interface, namespace string) {
	pvcClient := clientset.CoreV1().PersistentVolumeClaims(namespace)
	old, err := pvcClient.Get(ctx, SpireServerPVCName, metav1.GetOptions{})
	oldUID := ""
	if err == nil {
		oldUID = string(old.UID)
	} else if !apierrors.IsNotFound(err) {
		Expect(err).NotTo(HaveOccurred(), "failed to get SPIRE server PVC")
	}

	By(fmt.Sprintf("Deleting PVC %s/%s", namespace, SpireServerPVCName))
	err = pvcClient.Delete(ctx, SpireServerPVCName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		Expect(err).NotTo(HaveOccurred(), "failed to delete SPIRE server PVC")
	}

	By(fmt.Sprintf("Deleting pod %s/%s to release the PVC", namespace, SpireServerPodName))
	err = clientset.CoreV1().Pods(namespace).Delete(ctx, SpireServerPodName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		Expect(err).NotTo(HaveOccurred(), "failed to delete SPIRE server pod")
	}

	Eventually(func() bool {
		pvc, err := pvcClient.Get(ctx, SpireServerPVCName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true
		}
		if err != nil {
			return false
		}
		if oldUID != "" && string(pvc.UID) != oldUID {
			return true
		}
		return false
	}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(BeTrue(),
		"SPIRE server PVC should be replaced")
}

// WaitForSpireServerReadyAfterDatastoreChange wipes server PVC, waits for Ready, and re-syncs agents after CA rotation.
func WaitForSpireServerReadyAfterDatastoreChange(ctx context.Context, k8sClient client.Client, clientset kubernetes.Interface, namespace string) {
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

// RestartSpireAgents deletes agent pods so they rebootstrap against a new server CA.
func RestartSpireAgents(ctx context.Context, clientset kubernetes.Interface, namespace string) {
	podClient := clientset.CoreV1().Pods(namespace)
	old, err := podClient.List(ctx, metav1.ListOptions{LabelSelector: SpireAgentPodLabel})
	Expect(err).NotTo(HaveOccurred())
	oldNames := map[string]struct{}{}
	for i := range old.Items {
		oldNames[old.Items[i].Name] = struct{}{}
	}
	_ = podClient.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: SpireAgentPodLabel})
	Eventually(func() bool {
		pods, err := podClient.List(ctx, metav1.ListOptions{LabelSelector: SpireAgentPodLabel})
		if err != nil || len(pods.Items) == 0 {
			return false
		}
		for i := range pods.Items {
			if _, exists := oldNames[pods.Items[i].Name]; exists {
				return false
			}
			if pods.Items[i].Status.Phase != corev1.PodRunning {
				return false
			}
		}
		return true
	}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(BeTrue())
	WaitForDaemonSetAvailable(ctx, clientset, SpireAgentDaemonSetName, namespace, DefaultTimeout)
}

// WaitForSpireAgentsSynced waits until agent logs show successful attestation without CA mismatch errors.
func WaitForSpireAgentsSynced(ctx context.Context, clientset kubernetes.Interface, namespace string, timeout time.Duration) {
	tail := int64(80)
	Eventually(func() bool {
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: SpireAgentPodLabel})
		if err != nil || len(pods.Items) == 0 {
			return false
		}
		for i := range pods.Items {
			pod := &pods.Items[i]
			stream, err := clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
				Container: "spire-agent",
				TailLines: &tail,
			}).Stream(ctx)
			if err != nil {
				return false
			}
			b, err := io.ReadAll(stream)
			_ = stream.Close()
			if err != nil {
				return false
			}
			logs := string(b)
			if strings.Contains(logs, "unknown authority") || strings.Contains(logs, "Failed to fetch authorized entries") {
				return false
			}
		}
		return true
	}).WithTimeout(timeout).WithPolling(DefaultInterval).Should(BeTrue())
}

// GetSQLDatastorePluginData reads plugins.DataStore sql plugin_data from server.conf.
func GetSQLDatastorePluginData(ctx context.Context, clientset kubernetes.Interface, namespace string) (*SQLDatastorePluginData, error) {
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, SpireServerConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get ConfigMap: %w", err)
	}
	raw, ok := cm.Data[SpireServerConfigKey]
	if !ok {
		return nil, fmt.Errorf("key %s missing", SpireServerConfigKey)
	}
	var conf map[string]any
	if err := json.Unmarshal([]byte(raw), &conf); err != nil {
		return nil, fmt.Errorf("parse server.conf: %w", err)
	}
	plugins, _ := conf["plugins"].(map[string]any)
	if plugins == nil {
		return nil, fmt.Errorf("plugins missing")
	}
	ds, ok := plugins["DataStore"].([]any)
	if !ok || len(ds) == 0 {
		return nil, fmt.Errorf("DataStore plugin missing")
	}
	first, _ := ds[0].(map[string]any)
	sqlPlugin, _ := first["sql"].(map[string]any)
	if sqlPlugin == nil {
		return nil, fmt.Errorf("sql plugin missing")
	}
	pd, _ := sqlPlugin["plugin_data"].(map[string]any)
	if pd == nil {
		return nil, fmt.Errorf("plugin_data missing")
	}
	dbType, _ := pd["database_type"].(string)
	conn, _ := pd["connection_string"].(string)
	return &SQLDatastorePluginData{DatabaseType: dbType, ConnectionString: conn}, nil
}

// SpireServerHasNetworkPoliciesFeature returns true when NetworkPoliciesAvailable is reported.
func SpireServerHasNetworkPoliciesFeature(ctx context.Context, k8sClient client.Client) bool {
	server := &operatorv1alpha1.SpireServer{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: OperandCRName}, server); err != nil {
		return false
	}
	for _, c := range server.Status.Conditions {
		if c.Type == operatorv1alpha1.NetworkPoliciesAvailable {
			return true
		}
	}
	return false
}

func applyDatastoreTestdata(ctx context.Context, k8sClient client.Client, path string, namespace string) {
	applyTestdataManifest(ctx, k8sClient, path, datastoreTestdataValues{Namespace: namespace})
}

func deleteDatastoreE2EResources(ctx context.Context, k8sClient client.Client, clientset kubernetes.Interface, namespace, labelValue string) {
	selector := fmt.Sprintf("%s=%s", E2EDatastoreLabelKey, labelValue)
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(map[string]string{E2EDatastoreLabelKey: labelValue}),
	}

	for _, obj := range []struct {
		gvk  schema.GroupVersionKind
		kind string
	}{
		{schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}, "Job"},
		{schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, "Deployment"},
		{schema.GroupVersionKind{Version: "v1", Kind: "Service"}, "Service"},
		{schema.GroupVersionKind{Version: "v1", Kind: "Secret"}, "Secret"},
		{schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, "ConfigMap"},
	} {
		uList := &unstructured.UnstructuredList{}
		uList.SetGroupVersionKind(obj.gvk.GroupVersion().WithKind(obj.kind + "List"))
		if err := k8sClient.List(ctx, uList, listOpts...); err != nil {
			continue
		}
		for _, item := range uList.Items {
			_ = k8sClient.Delete(ctx, &item)
		}
	}

	// Best-effort pod cleanup for deployments
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err == nil {
		for _, p := range pods.Items {
			_ = clientset.CoreV1().Pods(namespace).Delete(ctx, p.Name, metav1.DeleteOptions{})
		}
	}
}

func applyTestdataManifest(ctx context.Context, k8sClient client.Client, path string, values any) {
	manifest, err := readE2ETestdata(path, values)
	Expect(err).NotTo(HaveOccurred(), "read testdata %s", path)
	applyManifestDocuments(ctx, k8sClient, manifest)
}

func readE2ETestdata(path string, values any) ([]byte, error) {
	raw, err := e2eTestdata.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if values == nil {
		return raw, nil
	}
	tmpl, err := template.New(path).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return nil, err
	}
	var doc bytes.Buffer
	if err := tmpl.Execute(&doc, values); err != nil {
		return nil, err
	}
	return doc.Bytes(), nil
}

func applyManifestDocuments(ctx context.Context, k8sClient client.Client, manifest []byte) {
	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 4096)
	for {
		var doc map[string]any
		err := decoder.Decode(&doc)
		if err == io.EOF {
			break
		}
		Expect(err).NotTo(HaveOccurred(), "decode testdata manifest")
		if len(doc) == 0 {
			continue
		}
		u := &unstructured.Unstructured{Object: doc}
		apiVersion, _, _ := unstructured.NestedString(doc, "apiVersion")
		kind, _, _ := unstructured.NestedString(doc, "kind")
		if apiVersion != "" && kind != "" {
			gv, err := schema.ParseGroupVersion(apiVersion)
			Expect(err).NotTo(HaveOccurred())
			u.SetGroupVersionKind(gv.WithKind(kind))
		}
		createOrUpdateUnstructured(ctx, k8sClient, u)
	}
}

func createOrUpdateUnstructured(ctx context.Context, k8sClient client.Client, obj *unstructured.Unstructured) {
	err := k8sClient.Create(ctx, obj)
	if err == nil {
		return
	}
	if apierrors.IsAlreadyExists(err) {
		existing := &unstructured.Unstructured{}
		existing.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), existing)).To(Succeed())
		obj.SetResourceVersion(existing.GetResourceVersion())
		Expect(k8sClient.Update(ctx, obj)).To(Succeed())
		return
	}
	Expect(err).NotTo(HaveOccurred())
}

func postgresConnectionString(namespace string) string {
	host := fmt.Sprintf("%s.%s.svc", PostgresServiceName, namespace)
	return fmt.Sprintf("dbname=spire user=spire host=%s port=%d sslmode=disable password=%s",
		host, PostgresServicePort, PostgresPassword)
}

func mysqlConnectionString(namespace string) string {
	host := fmt.Sprintf("%s.%s.svc", MySQLServiceName, namespace)
	// SPIRE mysql driver uses DSN-style connection string
	return fmt.Sprintf("spire:spire-e2e@tcp(%s:%d)/spire?parseTime=true", host, MySQLServicePort)
}

func vaultServiceAddr(namespace string) string {
	return fmt.Sprintf("http://%s.%s.svc:%d", VaultServiceName, namespace, VaultServicePort)
}

func serverConfContains(ctx context.Context, clientset kubernetes.Interface, namespace, substr string) bool {
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, SpireServerConfigMapName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return strings.Contains(cm.Data[SpireServerConfigKey], substr)
}
