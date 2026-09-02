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
	openshifttls "github.com/openshift/controller-runtime-common/pkg/tls"
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
		InitialTLSAdherencePolicy: configv1.TLSAdherencePolicyNoOpinion, // Default value. Ztwim ignores tlsAdherence; profile application is unchanged.
		InitialTLSProfileSpec:     configv1.TLSProfileSpec{},
	}

	tlsConfig.InitialTLSProfileSpec, err = openshifttls.FetchAPIServerTLSProfile(ctx, k8sClient)
	if err != nil {
		if errors.IsNotFound(err) {
			// 404 error: continue with default profile spec.
			setupLog.Info(openshifttls.APIServerName, "not found. Continuing with default profile spec.")
			// Assign default spec which is intermediate
			tlsConfig.InitialTLSProfileSpec = *configv1.TLSProfiles[libgocrypto.DefaultTLSProfileType]
			tlsConfig.Resolved.OperatorTLSConfig = getOperatorTLSConfig(tlsConfig.InitialTLSProfileSpec, setupLog)
			tlsConfig.Resolved.OperandTLSConfig = getOperandTLSConfig(tlsConfig.InitialTLSProfileSpec, setupLog)
		} else {
			// Non-404 error: return error, do not proceed. Never start with unkonwn tlsProfile.
			return nil, fmt.Errorf("error while fetching %q: %w", openshifttls.APIServerName, err)
		}
	} else {
		tlsConfig.Resolved.OperatorTLSConfig = getOperatorTLSConfig(tlsConfig.InitialTLSProfileSpec, setupLog)
		tlsConfig.Resolved.OperandTLSConfig = getOperandTLSConfig(tlsConfig.InitialTLSProfileSpec, setupLog)
	}

	return tlsConfig, nil
}

func getOperatorTLSConfig(tlsProfileSpec configv1.TLSProfileSpec, setupLog logr.Logger) func(*tls.Config) {
	profileTLSConfig, unsupportedCiphers := openshifttls.NewTLSConfigFromProfile(tlsProfileSpec)
	if len(unsupportedCiphers) > 0 {
		setupLog.Info("TLS configuration contains unsupported ciphers that will be ignored", "unsupportedCiphers", unsupportedCiphers)
	}

	return profileTLSConfig
}

// getOperandTLSConfig converts a cluster TLS profile spec into operand-facing TLS settings.
func getOperandTLSConfig(tlsProfileSpec configv1.TLSProfileSpec, setupLog logr.Logger) *OperandTLSConfig {
	var operandTLSCfg *OperandTLSConfig
	// If the minimum TLS version is less than 1.2, return nil. SPIRE does not support TLS 1.0 and TLS 1.1.
	if tlsProfileSpec.MinTLSVersion == configv1.VersionTLS10 || tlsProfileSpec.MinTLSVersion == configv1.VersionTLS11 {
		setupLog.Info("TLS profile specifies a minimum TLS version that is less than 1.2. Returning default TLS Profile for operand")
		defaultTLSProfile := *configv1.TLSProfiles[libgocrypto.DefaultTLSProfileType]
		operandTLSCfg = &OperandTLSConfig{
			MinTLSVersion: defaultTLSProfile.MinTLSVersion,
			CipherSuites:  libgocrypto.OpenSSLToIANACipherSuites(defaultTLSProfile.Ciphers),
		}

		return operandTLSCfg
	}

	operandTLSCfg = &OperandTLSConfig{
		MinTLSVersion: tlsProfileSpec.MinTLSVersion,
		CipherSuites:  libgocrypto.OpenSSLToIANACipherSuites(tlsProfileSpec.Ciphers),
	}

	//if the operand TLS config is empty, return nil
	if operandTLSCfg.MinTLSVersion == "" && len(operandTLSCfg.CipherSuites) == 0 && len(operandTLSCfg.CurvePreferences) == 0 {
		return nil
	}

	if len(operandTLSCfg.CipherSuites) == 0 {
		operandTLSCfg.CipherSuites = nil
	}
	if len(operandTLSCfg.CurvePreferences) == 0 {
		operandTLSCfg.CurvePreferences = nil
	}

	return operandTLSCfg
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
