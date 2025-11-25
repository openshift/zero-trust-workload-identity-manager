package spire_oidc_discovery_provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

// reconcileConfigMap reconciles the OIDC Discovery Provider ConfigMap
func (r *SpireOidcDiscoveryProviderReconciler) reconcileConfigMap(ctx context.Context, oidc *v1alpha1.SpireOIDCDiscoveryProvider, statusMgr *status.Manager, createOnlyMode bool) (string, error) {
	cm, err := generateOIDCConfigMapFromCR(oidc)
	if err != nil {
		r.log.Error(err, "failed to generate OIDC ConfigMap from CR")
		statusMgr.AddCondition(ConfigMapAvailable, "SpireOIDCConfigMapCreationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return "", err
	}

	if err = controllerutil.SetControllerReference(oidc, cm, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		statusMgr.AddCondition(ConfigMapAvailable, "SpireOIDCConfigMapCreationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return "", err
	}

	var existingOidcCm corev1.ConfigMap
	err = r.ctrlClient.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, &existingOidcCm)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, cm); err != nil {
			r.log.Error(err, "Failed to create ConfigMap")
			statusMgr.AddCondition(ConfigMapAvailable, "SpireOIDCConfigMapCreationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return "", err
		}
		r.log.Info("Created ConfigMap", "Namespace", cm.Namespace, "Name", cm.Name)
	} else if err == nil && (utils.GenerateMapHash(existingOidcCm.Data) != utils.GenerateMapHash(cm.Data) ||
		!equality.Semantic.DeepEqual(existingOidcCm.Labels, cm.Labels)) {
		if createOnlyMode {
			r.log.Info("Skipping ConfigMap update due to create-only mode", "Namespace", cm.Namespace, "Name", cm.Name)
		} else {
			cm.ResourceVersion = existingOidcCm.ResourceVersion
			if err = r.ctrlClient.Update(ctx, cm); err != nil {
				r.log.Error(err, "Failed to update ConfigMap", "Namespace", cm.Namespace, "Name", cm.Name)
				statusMgr.AddCondition(ConfigMapAvailable, "SpireOIDCConfigMapCreationFailed",
					err.Error(),
					metav1.ConditionFalse)
				return "", err
			}
			r.log.Info("Updated ConfigMap", "Namespace", cm.Namespace, "Name", cm.Name)
		}
	} else if err != nil {
		r.log.Error(err, "Failed to get ConfigMap")
		statusMgr.AddCondition(ConfigMapAvailable, "SpireOIDCConfigMapCreationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return "", err
	}

	statusMgr.AddCondition(ConfigMapAvailable, "SpireOIDCConfigMapCreationSucceeded",
		"Spire OIDC ConfigMap created",
		metav1.ConditionTrue)

	return utils.GenerateMapHash(cm.Data), nil
}

// generateOIDCConfigMapFromCR creates a ConfigMap for the spire oidc discovery provider from the CR spec
func generateOIDCConfigMapFromCR(dp *v1alpha1.SpireOIDCDiscoveryProvider) (*corev1.ConfigMap, error) {
	if dp == nil {
		return nil, errors.New("spire OIDC Discovery Provider Config is nil")
	}
	// Default to "spire-agent.sock" if not provided
	agentSocketName := dp.Spec.AgentSocketName
	if agentSocketName == "" {
		agentSocketName = "spire-agent.sock"
	}

	// Determine trust domain
	trustDomain := dp.Spec.TrustDomain

	// JWT Issuer validation and normalization
	jwtIssuer, err := utils.StripProtocolFromJWTIssuer(dp.Spec.JwtIssuer)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT issuer URL: %w", err)
	}
	// OIDC config map data
	oidcDefaultDomain := "spire-spiffe-oidc-discovery-provider." + utils.OperatorNamespace
	oidcSVCDomain := "spire-spiffe-oidc-discovery-provider." + utils.OperatorNamespace + ".svc.cluster.local"
	oidcConfig := map[string]interface{}{
		"domains": []string{
			"spire-spiffe-oidc-discovery-provider",
			oidcDefaultDomain,
			oidcSVCDomain,
			jwtIssuer,
		},
		"health_checks": map[string]string{
			"bind_port":  "8008",
			"live_path":  "/live",
			"ready_path": "/ready",
		},
		"log_level":  utils.GetLogLevelFromString(dp.Spec.LogLevel),
		"log_format": utils.GetLogFormatFromString(dp.Spec.LogFormat),
		"serving_cert_file": map[string]string{
			"addr":           ":8443",
			"cert_file_path": "/etc/oidc/tls/tls.crt",
			"key_file_path":  "/etc/oidc/tls/tls.key",
		},
		"workload_api": map[string]string{
			"socket_path":  "/spiffe-workload-api/" + agentSocketName,
			"trust_domain": trustDomain,
		},
	}

	oidcJSON, err := json.MarshalIndent(oidcConfig, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OIDC config: %w", err)
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-spiffe-oidc-discovery-provider",
			Namespace: utils.GetOperatorNamespace(),
			Labels:    utils.SpireOIDCDiscoveryProviderLabels(dp.Spec.Labels),
		},
		Data: map[string]string{
			"oidc-discovery-provider.conf": string(oidcJSON),
		},
	}

	return configMap, nil
}
