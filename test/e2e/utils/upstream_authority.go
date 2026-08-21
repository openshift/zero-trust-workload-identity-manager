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
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	gvkIssuer = schema.GroupVersionKind{
		Group: "cert-manager.io", Version: "v1", Kind: "Issuer",
	}
	gvkClusterIssuer = schema.GroupVersionKind{
		Group: "cert-manager.io", Version: "v1", Kind: "ClusterIssuer",
	}
	gvkCertificate = schema.GroupVersionKind{
		Group: "cert-manager.io", Version: "v1", Kind: "Certificate",
	}
	gvkCertificateRequestList = schema.GroupVersionKind{
		Group: "cert-manager.io", Version: "v1", Kind: "CertificateRequestList",
	}

	// certManagerIssuedAfter, if set, ignores CertificateIssued events from earlier runs.
	certManagerIssuedAfter time.Time
)

const (
	testdataOperatorInstall  = "testdata/cert-manager-operator/operator-install.yaml"
	testdataSelfSignedIssuer = "testdata/upstream-authority/selfsigned-issuer.yaml"
	testdataCACertificate    = "testdata/upstream-authority/ca-certificate.yaml"
	testdataCAIssuer         = "testdata/upstream-authority/ca-issuer.yaml"
	testdataClusterIssuer    = "testdata/upstream-authority/cluster-issuer.yaml"
)

// upstreamAuthorityTestdataValues substitutes namespace-scoped cert-manager CRs from testdata.
type upstreamAuthorityTestdataValues struct {
	Namespace         string
	ClusterIssuerName string
}

type deploymentCheck struct {
	Name      string
	Namespace string
}

// NoteCertManagerIssuedAfter records when this UA setup started so stale events are ignored.
func NoteCertManagerIssuedAfter(t time.Time) {
	certManagerIssuedAfter = t
}

// InstallCertManager installs the cert-manager Operator via OLM when needed (idempotent).
func InstallCertManager(ctx context.Context, k8sClient client.Client, clientset kubernetes.Interface) {
	operandChecks := []deploymentCheck{
		{Name: "cert-manager", Namespace: CertManagerOperandNamespace},
		{Name: "cert-manager-webhook", Namespace: CertManagerOperandNamespace},
		{Name: "cert-manager-cainjector", Namespace: CertManagerOperandNamespace},
	}

	if certManagerOperandsPresent(ctx, clientset) {
		By(fmt.Sprintf("cert-manager operands already present in %s; waiting until Available", CertManagerOperandNamespace))
		waitForDeploymentsAvailable(ctx, clientset, operandChecks, DefaultTimeout)
		return
	}

	waitForPackageManifest(ctx, clientset, CertManagerPackageName, CertManagerCatalogSource, 2*time.Minute)

	By("Creating cert-manager Operator Namespace, OperatorGroup, and Subscription")
	applyTestdata(ctx, k8sClient, testdataOperatorInstall, nil)

	operatorChecks := []deploymentCheck{
		{Name: CertManagerOperatorDeployment, Namespace: CertManagerOperatorNamespace},
	}
	waitForDeploymentsAvailable(ctx, clientset, operatorChecks, DefaultTimeout)
	waitForDeploymentsAvailable(ctx, clientset, operandChecks, DefaultTimeout)
}

// CreateSpireUpstreamCAIssuer creates a self-signed root, a CA Certificate, and a CA Issuer
// that SPIRE's cert-manager UpstreamAuthority plugin can request intermediates from.
func CreateSpireUpstreamCAIssuer(ctx context.Context, k8sClient client.Client, namespace string) {
	values := upstreamAuthorityTestdataValues{Namespace: namespace}

	By("Creating self-signed Issuer for upstream CA bootstrap")
	applyTestdata(ctx, k8sClient, testdataSelfSignedIssuer, values)

	DeferCleanup(func(cleanupCtx context.Context) {
		deleteUnstructured(cleanupCtx, k8sClient, gvkIssuer, CertManagerIssuerName, namespace)
		deleteUnstructured(cleanupCtx, k8sClient, gvkCertificate, CertManagerCACertificateName, namespace)
		deleteUnstructured(cleanupCtx, k8sClient, gvkIssuer, CertManagerSelfSignedIssuer, namespace)
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: CertManagerCASecretName, Namespace: namespace}}
		if err := k8sClient.Delete(cleanupCtx, secret); err != nil && !apierrors.IsNotFound(err) {
			fmt.Fprintf(GinkgoWriter, "cleanup: failed to delete Secret %s/%s: %v\n", namespace, CertManagerCASecretName, err)
		}
	})

	By("Creating upstream CA Certificate")
	applyTestdata(ctx, k8sClient, testdataCACertificate, values)

	By("Waiting for upstream CA Certificate to become Ready")
	Eventually(func() bool {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvkCertificate)
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: CertManagerCACertificateName, Namespace: namespace}, obj); err != nil {
			fmt.Fprintf(GinkgoWriter, "get Certificate: %v\n", err)
			return false
		}
		return unstructuredConditionReady(obj)
	}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(BeTrue(),
		"Certificate %s/%s should become Ready", namespace, CertManagerCACertificateName)

	By("Creating CA Issuer for SPIRE UpstreamAuthority")
	applyTestdata(ctx, k8sClient, testdataCAIssuer, values)

	By("Waiting for CA Issuer to become Ready")
	Eventually(func() bool {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvkIssuer)
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: CertManagerIssuerName, Namespace: namespace}, obj); err != nil {
			fmt.Fprintf(GinkgoWriter, "get Issuer: %v\n", err)
			return false
		}
		return unstructuredConditionReady(obj)
	}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(BeTrue(),
		"Issuer %s/%s should become Ready", namespace, CertManagerIssuerName)
}

// CreateSelfSignedClusterIssuer creates a cluster-scoped self-signed ClusterIssuer.
func CreateSelfSignedClusterIssuer(ctx context.Context, k8sClient client.Client, name string) {
	applyTestdata(ctx, k8sClient, testdataClusterIssuer, upstreamAuthorityTestdataValues{
		ClusterIssuerName: name,
	})
	DeferCleanup(func(cleanupCtx context.Context) {
		deleteUnstructured(cleanupCtx, k8sClient, gvkClusterIssuer, name, "")
	})
}

// WipeSpireServerPVC deletes the SPIRE server PVC (and its pod) so a new CA can be generated.
// The StatefulSet immediately recreates a claim with the same name, so success is the old
// UID disappearing (NotFound or a replacement PVC), not the name staying absent.
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

	By("Waiting for the old PVC volume to be replaced")
	Eventually(func() bool {
		pvc, err := pvcClient.Get(ctx, SpireServerPVCName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true
		}
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "get PVC: %v\n", err)
			return false
		}
		if oldUID != "" && string(pvc.UID) != oldUID {
			fmt.Fprintf(GinkgoWriter, "PVC %s/%s replaced (old uid %s, new uid %s)\n",
				namespace, SpireServerPVCName, oldUID, pvc.UID)
			return true
		}
		return false
	}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(BeTrue(),
		"old PVC %s/%s should be deleted or replaced", namespace, SpireServerPVCName)
}

// RestartSpireAgents deletes agent pods so emptyDir persistence is cleared and they
// rebootstrap against the server's new CA. DaemonSet Available is not enough after a CA switch:
// pods stay Ready while mTLS to the server is failing.
func RestartSpireAgents(ctx context.Context, clientset kubernetes.Interface, namespace string) {
	podClient := clientset.CoreV1().Pods(namespace)
	old, err := podClient.List(ctx, metav1.ListOptions{LabelSelector: SpireAgentPodLabel})
	Expect(err).NotTo(HaveOccurred(), "failed to list SPIRE agent pods")
	oldNames := map[string]struct{}{}
	for i := range old.Items {
		oldNames[old.Items[i].Name] = struct{}{}
	}

	By("Deleting SPIRE agent pods to rebootstrap against the new CA")
	err = podClient.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: SpireAgentPodLabel})
	if err != nil && !apierrors.IsNotFound(err) {
		Expect(err).NotTo(HaveOccurred(), "failed to delete SPIRE agent pods")
	}

	By("Waiting for replacement SPIRE agent pods")
	Eventually(func() bool {
		pods, err := podClient.List(ctx, metav1.ListOptions{LabelSelector: SpireAgentPodLabel})
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "list agent pods: %v\n", err)
			return false
		}
		if len(pods.Items) == 0 {
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
	}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(BeTrue(),
		"SPIRE agent pods should be replaced and Running")

	WaitForDaemonSetAvailable(ctx, clientset, SpireAgentDaemonSetName, namespace, DefaultTimeout)
}

// WaitForSpireAgentsSynced waits until agent logs no longer show a trust-bundle mismatch
// with the server (unknown authority / failed authorized entries).
func WaitForSpireAgentsSynced(ctx context.Context, clientset kubernetes.Interface, namespace string, timeout time.Duration) {
	By("Waiting for SPIRE agents to trust the server CA")
	tail := int64(80)
	Eventually(func() bool {
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: SpireAgentPodLabel})
		if err != nil || len(pods.Items) == 0 {
			return false
		}
		for i := range pods.Items {
			pod := &pods.Items[i]
			req := clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
				Container: "spire-agent",
				TailLines: &tail,
			})
			stream, err := req.Stream(ctx)
			if err != nil {
				fmt.Fprintf(GinkgoWriter, "agent %s logs: %v\n", pod.Name, err)
				return false
			}
			b, err := io.ReadAll(stream)
			_ = stream.Close()
			if err != nil {
				return false
			}
			logs := string(b)
			if strings.Contains(logs, "unknown authority") || strings.Contains(logs, "Trust Bandle and Server dont agree") {
				fmt.Fprintf(GinkgoWriter, "agent %s still does not trust server CA\n", pod.Name)
				return false
			}
			if strings.Contains(logs, "Failed to fetch authorized entries") {
				fmt.Fprintf(GinkgoWriter, "agent %s still failing to fetch entries\n", pod.Name)
				return false
			}
			if !strings.Contains(logs, "Workload API") && !strings.Contains(logs, "Node attestation") {
				fmt.Fprintf(GinkgoWriter, "agent %s has not finished attestation yet\n", pod.Name)
				return false
			}
		}
		return true
	}).WithTimeout(timeout).WithPolling(DefaultInterval).Should(BeTrue(),
		"SPIRE agents should sync with the server after CA switch")
}

// CertManagerPluginData is the SPIRE cert-manager UpstreamAuthority plugin_data block.
type CertManagerPluginData struct {
	IssuerName  string
	IssuerKind  string
	IssuerGroup string
	Namespace   string
}

// GetCertManagerPluginData reads plugins.UpstreamAuthority cert-manager plugin_data from server.conf.
func GetCertManagerPluginData(ctx context.Context, clientset kubernetes.Interface, namespace string) (*CertManagerPluginData, error) {
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, SpireServerConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap: %w", err)
	}
	raw, ok := cm.Data[SpireServerConfigKey]
	if !ok {
		return nil, fmt.Errorf("key %s not found in ConfigMap", SpireServerConfigKey)
	}

	var conf map[string]any
	if err := json.Unmarshal([]byte(raw), &conf); err != nil {
		return nil, fmt.Errorf("failed to parse server.conf: %w", err)
	}
	plugins, _ := conf["plugins"].(map[string]any)
	if plugins == nil {
		return nil, fmt.Errorf("plugins section missing")
	}
	ua, ok := plugins["UpstreamAuthority"].([]any)
	if !ok || len(ua) == 0 {
		return nil, fmt.Errorf("UpstreamAuthority plugin missing")
	}
	first, _ := ua[0].(map[string]any)
	cmPlugin, _ := first["cert-manager"].(map[string]any)
	if cmPlugin == nil {
		return nil, fmt.Errorf("cert-manager plugin missing")
	}
	pd, _ := cmPlugin["plugin_data"].(map[string]any)
	if pd == nil {
		return nil, fmt.Errorf("plugin_data missing")
	}

	str := func(key string) string {
		v, _ := pd[key].(string)
		return v
	}
	return &CertManagerPluginData{
		IssuerName:  str("issuer_name"),
		IssuerKind:  str("issuer_kind"),
		IssuerGroup: str("issuer_group"),
		Namespace:   str("namespace"),
	}, nil
}

// ClusterRoleHasCertManagerCertificateRequests returns true when the spire-server ClusterRole
// allows certificaterequests in the cert-manager.io API group.
func ClusterRoleHasCertManagerCertificateRequests(ctx context.Context, clientset kubernetes.Interface) (bool, error) {
	cr, err := clientset.RbacV1().ClusterRoles().Get(ctx, SpireServerClusterRoleName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	for _, rule := range cr.Rules {
		if !hasString(rule.APIGroups, "cert-manager.io") {
			continue
		}
		if hasString(rule.Resources, "certificaterequests") {
			return true, nil
		}
	}
	return false, nil
}

// HasIssuedCertificateRequest reports whether SPIRE obtained a signed intermediate from issuerName.
// The cert-manager UpstreamAuthority plugin creates CertificateRequests named spiffe-ca-* and
// deletes them after they are issued, so a live object may already be gone. A CertificateIssued
// event on that object is enough.
func HasIssuedCertificateRequest(ctx context.Context, k8sClient client.Client, clientset kubernetes.Interface, namespace, issuerName string) bool {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvkCertificateRequestList)
	if err := k8sClient.List(ctx, list, client.InNamespace(namespace)); err != nil {
		fmt.Fprintf(GinkgoWriter, "list CertificateRequests: %v\n", err)
	} else {
		for i := range list.Items {
			item := &list.Items[i]
			name, _, _ := unstructured.NestedString(item.Object, "spec", "issuerRef", "name")
			if name != issuerName {
				continue
			}
			if unstructuredConditionReady(item) {
				fmt.Fprintf(GinkgoWriter, "CertificateRequest %s is Ready (issuer %s)\n", item.GetName(), issuerName)
				return true
			}
			fmt.Fprintf(GinkgoWriter, "CertificateRequest %s for issuer %s not Ready yet\n", item.GetName(), issuerName)
		}
	}

	evs, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(GinkgoWriter, "list Events: %v\n", err)
		return false
	}
	for i := range evs.Items {
		ev := &evs.Items[i]
		if ev.InvolvedObject.Kind != "CertificateRequest" {
			continue
		}
		if ev.Reason != "CertificateIssued" {
			continue
		}
		if !strings.HasPrefix(ev.InvolvedObject.Name, "spiffe-ca-") {
			continue
		}
		ts := ev.LastTimestamp.Time
		if ts.IsZero() {
			ts = ev.EventTime.Time
		}
		if !certManagerIssuedAfter.IsZero() && ts.Before(certManagerIssuedAfter) {
			continue
		}
		fmt.Fprintf(GinkgoWriter, "CertificateRequest %s was issued (event; object may already be deleted)\n", ev.InvolvedObject.Name)
		return true
	}
	return false
}

// IssuerCACert parses the CA certificate from the upstream CA Secret.
func IssuerCACert(ctx context.Context, clientset kubernetes.Interface, namespace string) (*x509.Certificate, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, CertManagerCASecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	pemBytes := secret.Data[corev1.TLSCertKey]
	if len(pemBytes) == 0 {
		pemBytes = secret.Data["ca.crt"]
	}
	if len(pemBytes) == 0 {
		return nil, fmt.Errorf("secret %s/%s has no ca.crt or tls.crt", namespace, CertManagerCASecretName)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("secret %s/%s is not PEM", namespace, CertManagerCASecretName)
	}
	return x509.ParseCertificate(block.Bytes)
}

func createIgnoreAlreadyExists(ctx context.Context, k8sClient client.Client, obj client.Object) {
	err := k8sClient.Create(ctx, obj)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred(), "failed to create %s %s", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
	}
}

func deleteUnstructured(ctx context.Context, k8sClient client.Client, gvk schema.GroupVersionKind, name, namespace string) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName(name)
	if namespace != "" {
		obj.SetNamespace(namespace)
	}
	if err := k8sClient.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
		key := name
		if namespace != "" {
			key = namespace + "/" + name
		}
		fmt.Fprintf(GinkgoWriter, "cleanup: failed to delete %s %s: %v\n", gvk.Kind, key, err)
	}
}

func unstructuredConditionReady(obj *unstructured.Unstructured) bool {
	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}
	for _, c := range conditions {
		m, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if fmt.Sprint(m["type"]) == "Ready" && fmt.Sprint(m["status"]) == "True" {
			return true
		}
	}
	return false
}

func hasString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func readTestdata(path string, values any) ([]byte, error) {
	raw, err := e2eTestdata.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read testdata %s: %w", path, err)
	}
	if values == nil {
		return raw, nil
	}
	tmpl, err := template.New(path).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parse testdata template %s: %w", path, err)
	}
	var doc bytes.Buffer
	if err := tmpl.Execute(&doc, values); err != nil {
		return nil, fmt.Errorf("render testdata template %s: %w", path, err)
	}
	return doc.Bytes(), nil
}

func applyManifests(ctx context.Context, k8sClient client.Client, manifest []byte) {
	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 4096)
	for {
		var doc map[string]any
		err := decoder.Decode(&doc)
		if err == io.EOF {
			break
		}
		Expect(err).NotTo(HaveOccurred(), "failed to decode testdata manifest")

		if len(doc) == 0 {
			continue
		}

		u := &unstructured.Unstructured{Object: doc}
		apiVersion, _, _ := unstructured.NestedString(doc, "apiVersion")
		kind, _, _ := unstructured.NestedString(doc, "kind")
		if apiVersion != "" && kind != "" {
			gv, err := schema.ParseGroupVersion(apiVersion)
			Expect(err).NotTo(HaveOccurred(), "failed to parse apiVersion %q", apiVersion)
			u.SetGroupVersionKind(gv.WithKind(kind))
		}
		createIgnoreAlreadyExists(ctx, k8sClient, u)
	}
}

func applyTestdata(ctx context.Context, k8sClient client.Client, path string, values any) {
	manifest, err := readTestdata(path, values)
	Expect(err).NotTo(HaveOccurred(), "failed to read testdata %s", path)
	applyManifests(ctx, k8sClient, manifest)
}

func waitForPackageManifest(ctx context.Context, clientset kubernetes.Interface, name, catalog string, timeout time.Duration) {
	By(fmt.Sprintf("Waiting for PackageManifest %s in catalog %s", name, catalog))
	Eventually(func() bool {
		list, err := clientset.CoreV1().RESTClient().Get().
			AbsPath("/apis/packages.operators.coreos.com/v1/namespaces/openshift-marketplace/packagemanifests").
			Param("labelSelector", "catalog="+catalog).
			Param("fieldSelector", "metadata.name="+name).
			DoRaw(ctx)
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "list PackageManifests: %v\n", err)
			return false
		}
		return bytes.Contains(list, []byte(name))
	}).WithTimeout(timeout).WithPolling(ShortInterval).Should(BeTrue(),
		"PackageManifest %s should appear in catalog %s", name, catalog)
}

func certManagerOperandsPresent(ctx context.Context, clientset kubernetes.Interface) bool {
	for _, name := range []string{
		"cert-manager",
		"cert-manager-webhook",
		"cert-manager-cainjector",
	} {
		_, err := clientset.AppsV1().Deployments(CertManagerOperandNamespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false
		}
	}
	return true
}

func waitForDeploymentsAvailable(ctx context.Context, clientset kubernetes.Interface, checks []deploymentCheck, timeout time.Duration) {
	for _, check := range checks {
		WaitForDeploymentAvailable(ctx, clientset, check.Name, check.Namespace, timeout)
	}
}
