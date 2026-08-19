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

	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	utiltls "github.com/openshift/controller-runtime-common/pkg/tls"
	libgocrypto "github.com/openshift/library-go/pkg/crypto"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OperandTLSConfig holds the TLS config spec for the SPIRE operand.
type OperandTLSConfig struct {
	CipherSuites     []string                    `json:"cipherSuites,omitempty"`     // IANA cipher suite names e.g. "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
	CurvePreferences []string                    `json:"curvePreferences,omitempty"` // IANA curve names e.g. "X25519, X25519MLKEM768,secp256r1"
	MinTLSVersion    configv1.TLSProtocolVersion `json:"minTLSVersion,omitempty"`    // Kubernetes TLS version e.g. "VersionTLS10"
}

type Resolved struct {
	// OperatorTLSConfig is a function that applies TLS settings to a tls.Config. Used for operator metrics and webhook.
	OperatorTLSConfig func(*tls.Config)
	//OperandTLSConfig is the TLS config spec for the SPIRE operands.
	OperandTLSConfig *OperandTLSConfig
}

// TLSConfig holds the resolved TLS configuration along with the cluster-wide TLS profile metadata needed by the SecurityProfileWatcher.
type TLSConfig struct {
	Resolved *Resolved
	// InitialTLSAdherencePolicy is the cluster-wide TLS adherence policy fetched at the startup.
	InitialTLSAdherencePolicy configv1.TLSAdherencePolicy
	// InitialTLSProfileSpec is the cluster-wide TLS profile spec fetched at the startup.
	InitialTLSProfileSpec configv1.TLSProfileSpec
}

// FetchAPIServerTLSConfig fetches operator TLS settings from apiservers/cluster.
func FetchAPIServerTLSConfig(ctx context.Context, restConfig *rest.Config, scheme *runtime.Scheme, setupLog logr.Logger) (*TLSConfig, error) {
	k8sClient, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("unable to create Kubernetes client: %w", err)
	}

	tlsConfig := &TLSConfig{
		Resolved: &Resolved{
			OperatorTLSConfig: nil,
			OperandTLSConfig:  nil,
		},
		InitialTLSAdherencePolicy: configv1.TLSAdherencePolicyNoOpinion, // Default value.
		InitialTLSProfileSpec:     configv1.TLSProfileSpec{},
	}

	apiServer, err := fetchConfigV1APIServer(ctx, k8sClient)
	if err != nil {
		if errors.IsNotFound(err) {
			// 404 error: continue with default profile spec.
			setupLog.Info(utiltls.APIServerName, "not found. Continuing with default profile spec.")
			// Assign default spec. Error is not read since it is never returned as of today for this code path. Following call will return the default profile spec from initialized map.
			tlsConfig.InitialTLSProfileSpec, _ = utiltls.GetTLSProfileSpec(nil)
			tlsConfig.Resolved.OperatorTLSConfig = getOperatorTLSConfig(tlsConfig.InitialTLSProfileSpec, setupLog)
			tlsConfig.Resolved.OperandTLSConfig = getOperandTLSConfig(tlsConfig.InitialTLSProfileSpec, setupLog)
		} else {
			// Non-404 error: return error, do not proceed. Never start with unkonwn tlsProfile.
			return nil, fmt.Errorf("error while fetching %q: %w", utiltls.APIServerName, err)
		}
	} else {
		tlsConfig, err = getOperatorAndOperandTLSConfig(apiServer, setupLog)
		if err != nil {
			// Error while parsing TLS profile spec: return error, do not proceed. Never start with unkonwn tlsProfile.
			return nil, err
		}
	}

	return tlsConfig, nil
}

// fetchConfigV1APIServer fetches the config.v1.APIServer object.
func fetchConfigV1APIServer(ctx context.Context, k8sClient client.Client) (*configv1.APIServer, error) {
	apiServer := &configv1.APIServer{}
	key := client.ObjectKey{Name: utiltls.APIServerName}

	if err := k8sClient.Get(ctx, key, apiServer); err != nil {
		return nil, fmt.Errorf("failed to get APIServer %q: %w", key.String(), err)
	}

	return apiServer, nil
}

// getOperatorAndOperandTLSConfig resolves operator TLS callbacks and operand TLS settings
// from the cluster adherence policy and profile spec.
func getOperatorAndOperandTLSConfig(apiServer *configv1.APIServer, setupLog logr.Logger) (*TLSConfig, error) {

	tlsProfileSpec, err := utiltls.GetTLSProfileSpec(apiServer.Spec.TLSSecurityProfile)
	if err != nil {
		// Error can only happen if profile type is set to custom and then the custom profile is nil.
		return nil, fmt.Errorf("failed to parse TLS profile spec: %w", err)
	}

	tlsConfig := &TLSConfig{
		Resolved: &Resolved{
			OperatorTLSConfig: nil,
			OperandTLSConfig:  nil,
		},
		InitialTLSAdherencePolicy: apiServer.Spec.TLSAdherence,
		InitialTLSProfileSpec:     tlsProfileSpec,
	}

	// If the TLS adherence policy is set to honor the TLS profile,
	// use the cluster-wide TLS profile-based configuration.
	if libgocrypto.ShouldHonorClusterTLSProfile(apiServer.Spec.TLSAdherence) {
		tlsConfig.Resolved.OperatorTLSConfig = getOperatorTLSConfig(tlsProfileSpec, setupLog)
		tlsConfig.Resolved.OperandTLSConfig = getOperandTLSConfig(tlsProfileSpec, setupLog)
	} else {
		// Do nothing. Go defaults.
		tlsConfig.Resolved.OperatorTLSConfig = nil
		tlsConfig.Resolved.OperandTLSConfig = nil
	}

	return tlsConfig, nil
}

func getOperatorTLSConfig(tlsProfileSpec configv1.TLSProfileSpec, setupLog logr.Logger) func(*tls.Config) {
	profileTLSConfig, unsupportedCiphers := utiltls.NewTLSConfigFromProfile(tlsProfileSpec)
	if len(unsupportedCiphers) > 0 {
		setupLog.Info("TLS configuration contains unsupported ciphers that will be ignored", "unsupportedCiphers", unsupportedCiphers)
	}

	return profileTLSConfig
}

// getOperandTLSConfig converts a cluster TLS profile spec into operand-facing TLS settings.
func getOperandTLSConfig(tlsProfileSpec configv1.TLSProfileSpec, setupLog logr.Logger) *OperandTLSConfig {
	// If the minimum TLS version is less than 1.2, return nil. SPIRE does not support TLS 1.0 and TLS 1.1.
	if tlsProfileSpec.MinTLSVersion == configv1.VersionTLS10 || tlsProfileSpec.MinTLSVersion == configv1.VersionTLS11 {
		setupLog.Info("TLS profile specifies a minimum TLS version that is less than 1.2. Continuing with empty operand TLS config. Operands will run with Go default TLS config.")

		return nil
	}

	config := OperandTLSConfig{
		MinTLSVersion: tlsProfileSpec.MinTLSVersion,
		CipherSuites:  libgocrypto.OpenSSLToIANACipherSuites(tlsProfileSpec.Ciphers),
	}

	if config.MinTLSVersion == "" && len(config.CipherSuites) == 0 && len(config.CurvePreferences) == 0 {
		return nil
	}

	if len(config.CipherSuites) == 0 {
		config.CipherSuites = nil
	}
	if len(config.CurvePreferences) == 0 {
		config.CurvePreferences = nil
	}

	return &config
}

// GetInjectableTLSConfigForOperand returns a map of TLS config that can be injected into the Operand configmap.
// Only non-empty fields are included. Returns nil when there is nothing to inject.
func GetInjectableTLSConfigForOperand(tlsConfig *OperandTLSConfig) map[string]interface{} {
	if tlsConfig == nil {
		return nil
	}

	injectable := map[string]interface{}{}
	if tlsConfig.MinTLSVersion != "" {
		injectable["min_tls_version"] = tlsConfig.MinTLSVersion
	}
	if len(tlsConfig.CipherSuites) > 0 {
		injectable["cipher_suites"] = tlsConfig.CipherSuites
	}
	if len(tlsConfig.CurvePreferences) > 0 {
		injectable["curve_preferences"] = tlsConfig.CurvePreferences
	}
	if len(injectable) == 0 {
		return nil
	}

	return injectable
}
