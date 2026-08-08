/*
Copyright 2026 Red Hat, Inc.
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

package tls

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	utiltls "github.com/openshift/controller-runtime-common/pkg/tls"
	libgocrypto "github.com/openshift/library-go/pkg/crypto"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OperandTLSConfig holds the TLS config spec for the SPIRE operand.
type OperandTLSConfig struct {
	CipherSuites     []string `json:"cipherSuites,omitempty"`     // IANA cipher suite names e.g. "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
	CurvePreferences []string `json:"curvePreferences,omitempty"` // IANA curve names e.g. "X25519, X25519MLKEM768,secp256r1"
	MinTLSVersion    string   `json:"minTLSVersion,omitempty"`    // Kubernetes TLS version e.g. "VersionTLS10"
}

// TLSConfigResult holds the resolved TLS configuration along with the cluster-wide TLS profile metadata needed by the SecurityProfileWatcher.
type TLSConfigResult struct {
	// TLSConfig is a function that applies TLS settings to a tls.Config. Used for operator metrics and webhook.
	TLSConfig func(*tls.Config)
	//OperandTLSConfig is the TLS config spec for the SPIRE operands.
	OperandTLSConfig *OperandTLSConfig
	// TLSAdherencePolicy is the cluster-wide TLS adherence policy.
	TLSAdherencePolicy configv1.TLSAdherencePolicy
	// TLSProfileSpec is the cluster-wide TLS profile spec.
	TLSProfileSpec configv1.TLSProfileSpec
}

// FetchAPIServerTLSConfig fetches operator TLS settings from apiservers/cluster.
func FetchAPIServerTLSConfig(ctx context.Context, restConfig *rest.Config, scheme *runtime.Scheme) (TLSConfigResult, error) {
	k8sClient, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return TLSConfigResult{}, fmt.Errorf("unable to create Kubernetes client: %w", err)
	}

	fetchCtx, fetchCancel := context.WithTimeout(ctx, 30*time.Second)
	defer fetchCancel()

	initialTLSAdherencePolicy, err := utiltls.FetchAPIServerTLSAdherencePolicy(fetchCtx, k8sClient)
	if err != nil {
		klog.Errorf("error while fetching TLS adherence policy from API server: %v. Continuing with default adherence value: %s", err, string(configv1.TLSAdherencePolicyNoOpinion))
		// Default to empty string if the API server is not available or the field is not set. We will still keep a watch on the API server for the field and trigger a restart if the value changes.
		initialTLSAdherencePolicy = configv1.TLSAdherencePolicyNoOpinion
	}

	initialTLSProfileSpec, err := utiltls.FetchAPIServerTLSProfile(fetchCtx, k8sClient)
	if err != nil {
		klog.Errorf("error while fetching TLS profile from API server: %v. Continuing with empty profile.", err)
		// Default to an empty profile if the API server is not available or the field is not set. We will still keep a watch on the API server for the field and trigger a restart if the value changes.
		initialTLSProfileSpec = configv1.TLSProfileSpec{}
	}

	operatorTLSConfig, operandTLSConfig := getOperatorAndOperandTLSConfig(initialTLSAdherencePolicy, initialTLSProfileSpec)

	return TLSConfigResult{
		TLSConfig:          operatorTLSConfig,
		OperandTLSConfig:   operandTLSConfig,
		TLSAdherencePolicy: initialTLSAdherencePolicy,
		TLSProfileSpec:     initialTLSProfileSpec,
	}, nil
}

// getOperatorAndOperandTLSConfig resolves operator TLS callbacks and operand TLS settings
// from the cluster adherence policy and profile spec.
func getOperatorAndOperandTLSConfig(tlsAdherencePolicy configv1.TLSAdherencePolicy, tlsProfileSpec configv1.TLSProfileSpec) (func(*tls.Config), *OperandTLSConfig) {
	var (
		operatorTLSConfig func(*tls.Config)
		operandTLSConfig  *OperandTLSConfig
	)

	// If the cluster-wide TLS adherence policy is set to honor the cluster-wide TLS profile,
	// use the cluster-wide TLS profile-based configuration.
	if libgocrypto.ShouldHonorClusterTLSProfile(tlsAdherencePolicy) {
		profileTLSConfig, unsupportedCiphers := utiltls.NewTLSConfigFromProfile(tlsProfileSpec)
		if len(unsupportedCiphers) > 0 {
			klog.Infof("TLS configuration contains unsupported ciphers that will be ignored: %v", unsupportedCiphers)
		}

		// Set the TLS configuration to the cluster-wide TLS profile-based configuration.
		operatorTLSConfig = profileTLSConfig
		operandTLSConfig = getOperandTLSConfig(tlsProfileSpec)
	} else {
		//Do nothing. Let The TLS Endpoints behave as they are.
		operatorTLSConfig = nil
		operandTLSConfig = nil
	}

	return operatorTLSConfig, operandTLSConfig
}

// getOperandTLSConfig converts a cluster TLS profile spec into operand-facing TLS settings.
func getOperandTLSConfig(tlsProfileSpec configv1.TLSProfileSpec) *OperandTLSConfig {
	profile := OperandTLSConfig{
		MinTLSVersion: string(tlsProfileSpec.MinTLSVersion),
		CipherSuites:  libgocrypto.OpenSSLToIANACipherSuites(tlsProfileSpec.Ciphers),
	}

	return &profile
}
