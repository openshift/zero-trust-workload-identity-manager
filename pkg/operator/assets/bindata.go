// Code generated for package assets by go-bindata DO NOT EDIT. (@generated)
// sources:
// bindata/spiffe-csi/spiffe-csi-csi-driver.yaml
// bindata/spiffe-csi/spiffe-csi-service-account.yaml
// bindata/spire-agent/spire-agent-cluster-role-binding.yaml
// bindata/spire-agent/spire-agent-cluster-role.yaml
// bindata/spire-agent/spire-agent-service-account.yaml
// bindata/spire-agent/spire-agent-service.yaml
// bindata/spire-bundle/spire-bundle-role-binding.yaml
// bindata/spire-bundle/spire-bundle-role.yaml
// bindata/spire-controller-manager/spire-controller-manager-cluster-role-binding.yaml
// bindata/spire-controller-manager/spire-controller-manager-cluster-role.yaml
// bindata/spire-controller-manager/spire-controller-manager-leader-election-role-binding.yaml
// bindata/spire-controller-manager/spire-controller-manager-leader-election-role.yaml
// bindata/spire-controller-manager/spire-controller-manager-webhook-service.yaml
// bindata/spire-controller-manager/spire-controller-manager-webhook-validating-webhook.yaml
// bindata/spire-oidc-discovery-provider/spire-oidc-discovery-provider-service-account.yaml
// bindata/spire-oidc-discovery-provider/spire-oidc-discovery-provider-service.yaml
// bindata/spire-oidc-discovery-provider/spire-oidc-external-cert-role-binding.yaml
// bindata/spire-oidc-discovery-provider/spire-oidc-external-cert-role.yaml
// bindata/spire-server/spire-server-cluster-role-binding.yaml
// bindata/spire-server/spire-server-cluster-role.yaml
// bindata/spire-server/spire-server-external-cert-role-binding.yaml
// bindata/spire-server/spire-server-external-cert-role.yaml
// bindata/spire-server/spire-server-service-account.yaml
// bindata/spire-server/spire-server-service.yaml
package assets

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name return file name
func (fi bindataFileInfo) Name() string {
	return fi.name
}

// Size return file size
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}

// Mode return file mode
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}

// Mode return file modify time
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}

// IsDir return file whether a directory
func (fi bindataFileInfo) IsDir() bool {
	return fi.mode&os.ModeDir != 0
}

// Sys return file is sys mode
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _spiffeCsiSpiffeCsiCsiDriverYaml = []byte(`apiVersion: storage.k8s.io/v1
kind: CSIDriver
metadata:
  name: "csi.spiffe.io"
  labels:
    security.openshift.io/csi-ephemeral-volume-profile: restricted
    app.kubernetes.io/name: spiffe-csi-driver
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"

spec:
  # Only ephemeral, inline volumes are supported. There is no need for a
  # controller to provision and attach volumes.
  attachRequired: false

  # Request the pod information which the CSI driver uses to verify that an
  # ephemeral mount was requested.
  podInfoOnMount: true

  # Don't change ownership on the contents of the mount since the Workload API
  # Unix Domain Socket is typically open to all (i.e. 0777).
  fsGroupPolicy: None

  # Declare support for ephemeral volumes only.
  volumeLifecycleModes:
    - Ephemeral
`)

func spiffeCsiSpiffeCsiCsiDriverYamlBytes() ([]byte, error) {
	return _spiffeCsiSpiffeCsiCsiDriverYaml, nil
}

func spiffeCsiSpiffeCsiCsiDriverYaml() (*asset, error) {
	bytes, err := spiffeCsiSpiffeCsiCsiDriverYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spiffe-csi/spiffe-csi-csi-driver.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spiffeCsiSpiffeCsiServiceAccountYaml = []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: spire-spiffe-csi-driver
  namespace: zero-trust-workload-identity-manager
  labels:
    app.kubernetes.io/name: spiffe-csi-driver
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"

`)

func spiffeCsiSpiffeCsiServiceAccountYamlBytes() ([]byte, error) {
	return _spiffeCsiSpiffeCsiServiceAccountYaml, nil
}

func spiffeCsiSpiffeCsiServiceAccountYaml() (*asset, error) {
	bytes, err := spiffeCsiSpiffeCsiServiceAccountYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spiffe-csi/spiffe-csi-service-account.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireAgentSpireAgentClusterRoleBindingYaml = []byte(`kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: "spire-agent"
  labels:
    app.kubernetes.io/name: "agent"
    app.kubernetes.io/instance: "spire"
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"
subjects:
  - kind: ServiceAccount
    name: "spire-agent"
    namespace: "zero-trust-workload-identity-manager"
roleRef:
  kind: ClusterRole
  name: "spire-agent"
  apiGroup: rbac.authorization.k8s.io
`)

func spireAgentSpireAgentClusterRoleBindingYamlBytes() ([]byte, error) {
	return _spireAgentSpireAgentClusterRoleBindingYaml, nil
}

func spireAgentSpireAgentClusterRoleBindingYaml() (*asset, error) {
	bytes, err := spireAgentSpireAgentClusterRoleBindingYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-agent/spire-agent-cluster-role-binding.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireAgentSpireAgentClusterRoleYaml = []byte(`kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: "spire-agent"
  labels:
    app.kubernetes.io/name: "agent"
    app.kubernetes.io/instance: "spire"
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"
rules:
  - apiGroups: [""]
    resources:
      - pods
      - nodes
      - nodes/proxy
    verbs: ["get"]
`)

func spireAgentSpireAgentClusterRoleYamlBytes() ([]byte, error) {
	return _spireAgentSpireAgentClusterRoleYaml, nil
}

func spireAgentSpireAgentClusterRoleYaml() (*asset, error) {
	bytes, err := spireAgentSpireAgentClusterRoleYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-agent/spire-agent-cluster-role.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireAgentSpireAgentServiceAccountYaml = []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: "spire-agent"
  namespace: "zero-trust-workload-identity-manager"
  labels:
    app.kubernetes.io/name: "agent"
    app.kubernetes.io/instance: "spire"
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"`)

func spireAgentSpireAgentServiceAccountYamlBytes() ([]byte, error) {
	return _spireAgentSpireAgentServiceAccountYaml, nil
}

func spireAgentSpireAgentServiceAccountYaml() (*asset, error) {
	bytes, err := spireAgentSpireAgentServiceAccountYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-agent/spire-agent-service-account.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireAgentSpireAgentServiceYaml = []byte(`apiVersion: v1
kind: Service
metadata:
  name: spire-agent
  namespace: zero-trust-workload-identity-manager
  labels:
    app.kubernetes.io/name: agent
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"
spec:
  type: ClusterIP
  ports:
    - name: metrics
      port: 9402
      targetPort: 9402
  selector:
    app.kubernetes.io/name: agent
    app.kubernetes.io/instance: spire
`)

func spireAgentSpireAgentServiceYamlBytes() ([]byte, error) {
	return _spireAgentSpireAgentServiceYaml, nil
}

func spireAgentSpireAgentServiceYaml() (*asset, error) {
	bytes, err := spireAgentSpireAgentServiceYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-agent/spire-agent-service.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireBundleSpireBundleRoleBindingYaml = []byte(`kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: spire-bundle
  namespace: zero-trust-workload-identity-manager
  labels:
    app.kubernetes.io/name: server
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"
subjects:
  - kind: ServiceAccount
    name: spire-server
    namespace: zero-trust-workload-identity-manager
roleRef:
  kind: Role
  name: spire-bundle
  apiGroup: rbac.authorization.k8s.io
`)

func spireBundleSpireBundleRoleBindingYamlBytes() ([]byte, error) {
	return _spireBundleSpireBundleRoleBindingYaml, nil
}

func spireBundleSpireBundleRoleBindingYaml() (*asset, error) {
	bytes, err := spireBundleSpireBundleRoleBindingYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-bundle/spire-bundle-role-binding.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireBundleSpireBundleRoleYaml = []byte(`kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: spire-bundle
  namespace: zero-trust-workload-identity-manager
  labels:
    app.kubernetes.io/name: server
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"
rules:
  - apiGroups: [""]
    resources: [configmaps]
    resourceNames: [spire-bundle]
    verbs:
      - get
      - patch
`)

func spireBundleSpireBundleRoleYamlBytes() ([]byte, error) {
	return _spireBundleSpireBundleRoleYaml, nil
}

func spireBundleSpireBundleRoleYaml() (*asset, error) {
	bytes, err := spireBundleSpireBundleRoleYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-bundle/spire-bundle-role.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireControllerManagerSpireControllerManagerClusterRoleBindingYaml = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: spire-controller-manager
  labels:
    app.kubernetes.io/name: server
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: spire-controller-manager

subjects:
  - kind: ServiceAccount
    name: spire-server
    namespace: zero-trust-workload-identity-manager
`)

func spireControllerManagerSpireControllerManagerClusterRoleBindingYamlBytes() ([]byte, error) {
	return _spireControllerManagerSpireControllerManagerClusterRoleBindingYaml, nil
}

func spireControllerManagerSpireControllerManagerClusterRoleBindingYaml() (*asset, error) {
	bytes, err := spireControllerManagerSpireControllerManagerClusterRoleBindingYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-controller-manager/spire-controller-manager-cluster-role-binding.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireControllerManagerSpireControllerManagerClusterRoleYaml = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: spire-controller-manager
  labels:
    app.kubernetes.io/name: server
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"
rules:
  - apiGroups: [""]
    resources: ["namespaces"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["admissionregistration.k8s.io"]
    resources: ["validatingwebhookconfigurations"]
    verbs: ["get", "list", "patch", "watch"]
  - apiGroups: [""]
    resources: ["nodes"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["endpoints"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["spire.spiffe.io"]
    resources: ["clusterfederatedtrustdomains"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["spire.spiffe.io"]
    resources: ["clusterfederatedtrustdomains/finalizers"]
    verbs: ["update"]
  - apiGroups: ["spire.spiffe.io"]
    resources: ["clusterfederatedtrustdomains/status"]
    verbs: ["get", "patch", "update"]
  - apiGroups: ["spire.spiffe.io"]
    resources: ["clusterspiffeids"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["spire.spiffe.io"]
    resources: ["clusterspiffeids/finalizers"]
    verbs: ["update"]
  - apiGroups: ["spire.spiffe.io"]
    resources: ["clusterspiffeids/status"]
    verbs: ["get", "patch", "update"]
  - apiGroups: ["spire.spiffe.io"]
    resources: ["clusterstaticentries"]
    verbs: ["create", "delete", "get", "list", "patch", "update", "watch"]
  - apiGroups: ["spire.spiffe.io"]
    resources: ["clusterstaticentries/finalizers"]
    verbs: ["update"]
  - apiGroups: ["spire.spiffe.io"]
    resources: ["clusterstaticentries/status"]
    verbs: ["get", "patch", "update"]
`)

func spireControllerManagerSpireControllerManagerClusterRoleYamlBytes() ([]byte, error) {
	return _spireControllerManagerSpireControllerManagerClusterRoleYaml, nil
}

func spireControllerManagerSpireControllerManagerClusterRoleYaml() (*asset, error) {
	bytes, err := spireControllerManagerSpireControllerManagerClusterRoleYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-controller-manager/spire-controller-manager-cluster-role.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireControllerManagerSpireControllerManagerLeaderElectionRoleBindingYaml = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: spire-controller-manager-leader-election
  namespace: zero-trust-workload-identity-manager
  labels:
    app.kubernetes.io/name: server
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: spire-controller-manager-leader-election

subjects:
  - kind: ServiceAccount
    name: spire-server
    namespace: zero-trust-workload-identity-manager
`)

func spireControllerManagerSpireControllerManagerLeaderElectionRoleBindingYamlBytes() ([]byte, error) {
	return _spireControllerManagerSpireControllerManagerLeaderElectionRoleBindingYaml, nil
}

func spireControllerManagerSpireControllerManagerLeaderElectionRoleBindingYaml() (*asset, error) {
	bytes, err := spireControllerManagerSpireControllerManagerLeaderElectionRoleBindingYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-controller-manager/spire-controller-manager-leader-election-role-binding.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireControllerManagerSpireControllerManagerLeaderElectionRoleYaml = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: spire-controller-manager-leader-election
  namespace: zero-trust-workload-identity-manager
  labels:
    app.kubernetes.io/name: server
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["coordination.k8s.io"]
    resources: ["leases"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "patch"]
`)

func spireControllerManagerSpireControllerManagerLeaderElectionRoleYamlBytes() ([]byte, error) {
	return _spireControllerManagerSpireControllerManagerLeaderElectionRoleYaml, nil
}

func spireControllerManagerSpireControllerManagerLeaderElectionRoleYaml() (*asset, error) {
	bytes, err := spireControllerManagerSpireControllerManagerLeaderElectionRoleYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-controller-manager/spire-controller-manager-leader-election-role.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireControllerManagerSpireControllerManagerWebhookServiceYaml = []byte(`apiVersion: v1
kind: Service
metadata:
  name: spire-controller-manager-webhook
  namespace: zero-trust-workload-identity-manager
  labels:
    app.kubernetes.io/name: server
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"
spec:
  type: ClusterIP
  ports:
    - name: https
      port: 443
      targetPort: https
      protocol: TCP
  selector:
    app.kubernetes.io/name: server
    app.kubernetes.io/instance: spire
`)

func spireControllerManagerSpireControllerManagerWebhookServiceYamlBytes() ([]byte, error) {
	return _spireControllerManagerSpireControllerManagerWebhookServiceYaml, nil
}

func spireControllerManagerSpireControllerManagerWebhookServiceYaml() (*asset, error) {
	bytes, err := spireControllerManagerSpireControllerManagerWebhookServiceYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-controller-manager/spire-controller-manager-webhook-service.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireControllerManagerSpireControllerManagerWebhookValidatingWebhookYaml = []byte(`apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: spire-controller-manager-webhook
  labels:
    app.kubernetes.io/name: server
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"
webhooks:
  - admissionReviewVersions: ["v1"]
    clientConfig:
      service:
        name: spire-controller-manager-webhook
        namespace: zero-trust-workload-identity-manager
        path: /validate-spire-spiffe-io-v1alpha1-clusterfederatedtrustdomain
    failurePolicy: Ignore # Actual value to be set by post install/upgrade hooks
    name: vclusterfederatedtrustdomain.kb.io
    rules:
      - apiGroups: ["spire.spiffe.io"]
        apiVersions: ["v1alpha1"]
        operations: ["CREATE", "UPDATE"]
        resources: ["clusterfederatedtrustdomains"]
    sideEffects: None
  - admissionReviewVersions: ["v1"]
    clientConfig:
      service:
        name: spire-controller-manager-webhook
        namespace: zero-trust-workload-identity-manager
        path: /validate-spire-spiffe-io-v1alpha1-clusterspiffeid
    failurePolicy: Ignore # Actual value to be set by post install/upgrade hooks
    name: vclusterspiffeid.kb.io
    rules:
      - apiGroups: ["spire.spiffe.io"]
        apiVersions: ["v1alpha1"]
        operations: ["CREATE", "UPDATE"]
        resources: ["clusterspiffeids"]
    sideEffects: None
`)

func spireControllerManagerSpireControllerManagerWebhookValidatingWebhookYamlBytes() ([]byte, error) {
	return _spireControllerManagerSpireControllerManagerWebhookValidatingWebhookYaml, nil
}

func spireControllerManagerSpireControllerManagerWebhookValidatingWebhookYaml() (*asset, error) {
	bytes, err := spireControllerManagerSpireControllerManagerWebhookValidatingWebhookYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-controller-manager/spire-controller-manager-webhook-validating-webhook.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireOidcDiscoveryProviderSpireOidcDiscoveryProviderServiceAccountYaml = []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: spire-spiffe-oidc-discovery-provider
  namespace: zero-trust-workload-identity-manager
  labels:
    app.kubernetes.io/name: spiffe-oidc-discovery-provider
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"

`)

func spireOidcDiscoveryProviderSpireOidcDiscoveryProviderServiceAccountYamlBytes() ([]byte, error) {
	return _spireOidcDiscoveryProviderSpireOidcDiscoveryProviderServiceAccountYaml, nil
}

func spireOidcDiscoveryProviderSpireOidcDiscoveryProviderServiceAccountYaml() (*asset, error) {
	bytes, err := spireOidcDiscoveryProviderSpireOidcDiscoveryProviderServiceAccountYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-oidc-discovery-provider/spire-oidc-discovery-provider-service-account.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireOidcDiscoveryProviderSpireOidcDiscoveryProviderServiceYaml = []byte(`apiVersion: v1
kind: Service
metadata:
  name: spire-spiffe-oidc-discovery-provider
  namespace: zero-trust-workload-identity-manager
  annotations:
    service.beta.openshift.io/serving-cert-secret-name: oidc-serving-cert
  labels:
    app.kubernetes.io/name: spiffe-oidc-discovery-provider
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"
spec:
  type: ClusterIP
  ports:
    - name: https
      port: 443
      targetPort: https
      protocol: TCP
  selector:
    app.kubernetes.io/name: spiffe-oidc-discovery-provider
    app.kubernetes.io/instance: spire
`)

func spireOidcDiscoveryProviderSpireOidcDiscoveryProviderServiceYamlBytes() ([]byte, error) {
	return _spireOidcDiscoveryProviderSpireOidcDiscoveryProviderServiceYaml, nil
}

func spireOidcDiscoveryProviderSpireOidcDiscoveryProviderServiceYaml() (*asset, error) {
	bytes, err := spireOidcDiscoveryProviderSpireOidcDiscoveryProviderServiceYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-oidc-discovery-provider/spire-oidc-discovery-provider-service.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireOidcDiscoveryProviderSpireOidcExternalCertRoleBindingYaml = []byte(`kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: spire-oidc-external-cert-reader
  namespace: zero-trust-workload-identity-manager
  labels:
    app.kubernetes.io/name: spiffe-oidc-discovery-provider
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"
subjects:
  - kind: ServiceAccount
    name: router
    namespace: openshift-ingress
roleRef:
  kind: Role
  name: spire-oidc-external-cert-reader
  apiGroup: rbac.authorization.k8s.io

`)

func spireOidcDiscoveryProviderSpireOidcExternalCertRoleBindingYamlBytes() ([]byte, error) {
	return _spireOidcDiscoveryProviderSpireOidcExternalCertRoleBindingYaml, nil
}

func spireOidcDiscoveryProviderSpireOidcExternalCertRoleBindingYaml() (*asset, error) {
	bytes, err := spireOidcDiscoveryProviderSpireOidcExternalCertRoleBindingYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-oidc-discovery-provider/spire-oidc-external-cert-role-binding.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireOidcDiscoveryProviderSpireOidcExternalCertRoleYaml = []byte(`kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: spire-oidc-external-cert-reader
  namespace: zero-trust-workload-identity-manager
  labels:
    app.kubernetes.io/name: spiffe-oidc-discovery-provider
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    resourceNames: []  # Will be populated dynamically from externalSecretRef
    verbs: ["get"]
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["list", "watch"]

`)

func spireOidcDiscoveryProviderSpireOidcExternalCertRoleYamlBytes() ([]byte, error) {
	return _spireOidcDiscoveryProviderSpireOidcExternalCertRoleYaml, nil
}

func spireOidcDiscoveryProviderSpireOidcExternalCertRoleYaml() (*asset, error) {
	bytes, err := spireOidcDiscoveryProviderSpireOidcExternalCertRoleYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-oidc-discovery-provider/spire-oidc-external-cert-role.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireServerSpireServerClusterRoleBindingYaml = []byte(`kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: spire-server
  labels:
    app.kubernetes.io/name: server
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"
subjects:
  - kind: ServiceAccount
    name: spire-server
    namespace: zero-trust-workload-identity-manager
roleRef:
  kind: ClusterRole
  name: spire-server
  apiGroup: rbac.authorization.k8s.io
`)

func spireServerSpireServerClusterRoleBindingYamlBytes() ([]byte, error) {
	return _spireServerSpireServerClusterRoleBindingYaml, nil
}

func spireServerSpireServerClusterRoleBindingYaml() (*asset, error) {
	bytes, err := spireServerSpireServerClusterRoleBindingYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-server/spire-server-cluster-role-binding.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireServerSpireServerClusterRoleYaml = []byte(`kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: spire-server
  labels:
    app.kubernetes.io/name: server
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"
rules:
  - apiGroups: [authentication.k8s.io]
    resources: [tokenreviews]
    verbs:
      - get
      - watch
      - list
      - create
  - apiGroups: [""]
    resources: [nodes, pods]
    verbs:
      - get
      - list
`)

func spireServerSpireServerClusterRoleYamlBytes() ([]byte, error) {
	return _spireServerSpireServerClusterRoleYaml, nil
}

func spireServerSpireServerClusterRoleYaml() (*asset, error) {
	bytes, err := spireServerSpireServerClusterRoleYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-server/spire-server-cluster-role.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireServerSpireServerExternalCertRoleBindingYaml = []byte(`kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: spire-server-external-cert-reader
  namespace: zero-trust-workload-identity-manager
  labels:
    app.kubernetes.io/name: server
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"
subjects:
  - kind: ServiceAccount
    name: router
    namespace: openshift-ingress
roleRef:
  kind: Role
  name: spire-server-external-cert-reader
  apiGroup: rbac.authorization.k8s.io

`)

func spireServerSpireServerExternalCertRoleBindingYamlBytes() ([]byte, error) {
	return _spireServerSpireServerExternalCertRoleBindingYaml, nil
}

func spireServerSpireServerExternalCertRoleBindingYaml() (*asset, error) {
	bytes, err := spireServerSpireServerExternalCertRoleBindingYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-server/spire-server-external-cert-role-binding.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireServerSpireServerExternalCertRoleYaml = []byte(`kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: spire-server-external-cert-reader
  namespace: zero-trust-workload-identity-manager
  labels:
    app.kubernetes.io/name: server
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    resourceNames: []  # Will be populated dynamically from externalSecretRef
    verbs: ["get"]
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["list", "watch"]

`)

func spireServerSpireServerExternalCertRoleYamlBytes() ([]byte, error) {
	return _spireServerSpireServerExternalCertRoleYaml, nil
}

func spireServerSpireServerExternalCertRoleYaml() (*asset, error) {
	bytes, err := spireServerSpireServerExternalCertRoleYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-server/spire-server-external-cert-role.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireServerSpireServerServiceAccountYaml = []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: spire-server
  namespace: zero-trust-workload-identity-manager
  labels:
    app.kubernetes.io/name: server
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"

`)

func spireServerSpireServerServiceAccountYamlBytes() ([]byte, error) {
	return _spireServerSpireServerServiceAccountYaml, nil
}

func spireServerSpireServerServiceAccountYaml() (*asset, error) {
	bytes, err := spireServerSpireServerServiceAccountYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-server/spire-server-service-account.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _spireServerSpireServerServiceYaml = []byte(`apiVersion: v1
kind: Service
metadata:
  name: spire-server
  namespace: zero-trust-workload-identity-manager
  labels:
    app.kubernetes.io/name: server
    app.kubernetes.io/instance: spire
    app.kubernetes.io/managed-by: "zero-trust-workload-identity-manager"
    app.kubernetes.io/part-of: "zero-trust-workload-identity-manager"
spec:
  type: ClusterIP
  ports:
    - name: grpc
      port: 443
      targetPort: grpc
      protocol: TCP
    - name: metrics
      port: 9402
      targetPort: 9402
  selector:
    app.kubernetes.io/name: server
    app.kubernetes.io/instance: spire
`)

func spireServerSpireServerServiceYamlBytes() ([]byte, error) {
	return _spireServerSpireServerServiceYaml, nil
}

func spireServerSpireServerServiceYaml() (*asset, error) {
	bytes, err := spireServerSpireServerServiceYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "spire-server/spire-server-service.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"spiffe-csi/spiffe-csi-csi-driver.yaml":                                               spiffeCsiSpiffeCsiCsiDriverYaml,
	"spiffe-csi/spiffe-csi-service-account.yaml":                                          spiffeCsiSpiffeCsiServiceAccountYaml,
	"spire-agent/spire-agent-cluster-role-binding.yaml":                                   spireAgentSpireAgentClusterRoleBindingYaml,
	"spire-agent/spire-agent-cluster-role.yaml":                                           spireAgentSpireAgentClusterRoleYaml,
	"spire-agent/spire-agent-service-account.yaml":                                        spireAgentSpireAgentServiceAccountYaml,
	"spire-agent/spire-agent-service.yaml":                                                spireAgentSpireAgentServiceYaml,
	"spire-bundle/spire-bundle-role-binding.yaml":                                         spireBundleSpireBundleRoleBindingYaml,
	"spire-bundle/spire-bundle-role.yaml":                                                 spireBundleSpireBundleRoleYaml,
	"spire-controller-manager/spire-controller-manager-cluster-role-binding.yaml":         spireControllerManagerSpireControllerManagerClusterRoleBindingYaml,
	"spire-controller-manager/spire-controller-manager-cluster-role.yaml":                 spireControllerManagerSpireControllerManagerClusterRoleYaml,
	"spire-controller-manager/spire-controller-manager-leader-election-role-binding.yaml": spireControllerManagerSpireControllerManagerLeaderElectionRoleBindingYaml,
	"spire-controller-manager/spire-controller-manager-leader-election-role.yaml":         spireControllerManagerSpireControllerManagerLeaderElectionRoleYaml,
	"spire-controller-manager/spire-controller-manager-webhook-service.yaml":              spireControllerManagerSpireControllerManagerWebhookServiceYaml,
	"spire-controller-manager/spire-controller-manager-webhook-validating-webhook.yaml":   spireControllerManagerSpireControllerManagerWebhookValidatingWebhookYaml,
	"spire-oidc-discovery-provider/spire-oidc-discovery-provider-service-account.yaml":    spireOidcDiscoveryProviderSpireOidcDiscoveryProviderServiceAccountYaml,
	"spire-oidc-discovery-provider/spire-oidc-discovery-provider-service.yaml":            spireOidcDiscoveryProviderSpireOidcDiscoveryProviderServiceYaml,
	"spire-oidc-discovery-provider/spire-oidc-external-cert-role-binding.yaml":            spireOidcDiscoveryProviderSpireOidcExternalCertRoleBindingYaml,
	"spire-oidc-discovery-provider/spire-oidc-external-cert-role.yaml":                    spireOidcDiscoveryProviderSpireOidcExternalCertRoleYaml,
	"spire-server/spire-server-cluster-role-binding.yaml":                                 spireServerSpireServerClusterRoleBindingYaml,
	"spire-server/spire-server-cluster-role.yaml":                                         spireServerSpireServerClusterRoleYaml,
	"spire-server/spire-server-external-cert-role-binding.yaml":                           spireServerSpireServerExternalCertRoleBindingYaml,
	"spire-server/spire-server-external-cert-role.yaml":                                   spireServerSpireServerExternalCertRoleYaml,
	"spire-server/spire-server-service-account.yaml":                                      spireServerSpireServerServiceAccountYaml,
	"spire-server/spire-server-service.yaml":                                              spireServerSpireServerServiceYaml,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//
//	data/
//	  foo.txt
//	  img/
//	    a.png
//	    b.png
//
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"spiffe-csi": {nil, map[string]*bintree{
		"spiffe-csi-csi-driver.yaml":      {spiffeCsiSpiffeCsiCsiDriverYaml, map[string]*bintree{}},
		"spiffe-csi-service-account.yaml": {spiffeCsiSpiffeCsiServiceAccountYaml, map[string]*bintree{}},
	}},
	"spire-agent": {nil, map[string]*bintree{
		"spire-agent-cluster-role-binding.yaml": {spireAgentSpireAgentClusterRoleBindingYaml, map[string]*bintree{}},
		"spire-agent-cluster-role.yaml":         {spireAgentSpireAgentClusterRoleYaml, map[string]*bintree{}},
		"spire-agent-service-account.yaml":      {spireAgentSpireAgentServiceAccountYaml, map[string]*bintree{}},
		"spire-agent-service.yaml":              {spireAgentSpireAgentServiceYaml, map[string]*bintree{}},
	}},
	"spire-bundle": {nil, map[string]*bintree{
		"spire-bundle-role-binding.yaml": {spireBundleSpireBundleRoleBindingYaml, map[string]*bintree{}},
		"spire-bundle-role.yaml":         {spireBundleSpireBundleRoleYaml, map[string]*bintree{}},
	}},
	"spire-controller-manager": {nil, map[string]*bintree{
		"spire-controller-manager-cluster-role-binding.yaml":         {spireControllerManagerSpireControllerManagerClusterRoleBindingYaml, map[string]*bintree{}},
		"spire-controller-manager-cluster-role.yaml":                 {spireControllerManagerSpireControllerManagerClusterRoleYaml, map[string]*bintree{}},
		"spire-controller-manager-leader-election-role-binding.yaml": {spireControllerManagerSpireControllerManagerLeaderElectionRoleBindingYaml, map[string]*bintree{}},
		"spire-controller-manager-leader-election-role.yaml":         {spireControllerManagerSpireControllerManagerLeaderElectionRoleYaml, map[string]*bintree{}},
		"spire-controller-manager-webhook-service.yaml":              {spireControllerManagerSpireControllerManagerWebhookServiceYaml, map[string]*bintree{}},
		"spire-controller-manager-webhook-validating-webhook.yaml":   {spireControllerManagerSpireControllerManagerWebhookValidatingWebhookYaml, map[string]*bintree{}},
	}},
	"spire-oidc-discovery-provider": {nil, map[string]*bintree{
		"spire-oidc-discovery-provider-service-account.yaml": {spireOidcDiscoveryProviderSpireOidcDiscoveryProviderServiceAccountYaml, map[string]*bintree{}},
		"spire-oidc-discovery-provider-service.yaml":         {spireOidcDiscoveryProviderSpireOidcDiscoveryProviderServiceYaml, map[string]*bintree{}},
		"spire-oidc-external-cert-role-binding.yaml":         {spireOidcDiscoveryProviderSpireOidcExternalCertRoleBindingYaml, map[string]*bintree{}},
		"spire-oidc-external-cert-role.yaml":                 {spireOidcDiscoveryProviderSpireOidcExternalCertRoleYaml, map[string]*bintree{}},
	}},
	"spire-server": {nil, map[string]*bintree{
		"spire-server-cluster-role-binding.yaml":       {spireServerSpireServerClusterRoleBindingYaml, map[string]*bintree{}},
		"spire-server-cluster-role.yaml":               {spireServerSpireServerClusterRoleYaml, map[string]*bintree{}},
		"spire-server-external-cert-role-binding.yaml": {spireServerSpireServerExternalCertRoleBindingYaml, map[string]*bintree{}},
		"spire-server-external-cert-role.yaml":         {spireServerSpireServerExternalCertRoleYaml, map[string]*bintree{}},
		"spire-server-service-account.yaml":            {spireServerSpireServerServiceAccountYaml, map[string]*bintree{}},
		"spire-server-service.yaml":                    {spireServerSpireServerServiceYaml, map[string]*bintree{}},
	}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
