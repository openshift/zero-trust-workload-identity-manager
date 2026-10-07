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
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	openshifttls "github.com/openshift/controller-runtime-common/pkg/tls"
	libgocrypto "github.com/openshift/library-go/pkg/crypto"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	minTLSVersionKeyForSPIRE = "min_tls_version"
	cipherSuitesKeyForSPIRE  = "cipher_suites"
	minTLSVersionKeyForSCM   = "minTLSVersion"
	cipherSuitesKeyForSCM    = "cipherSuites"
)

// OpenShiftTLSProfile holds the cluster TLS profile fetched at startup and metadata for the SecurityProfileWatcher.
type OpenShiftTLSProfile struct {
	// OperatorGoTLSConfig applies TLS settings to a tls.Config. Used for operator metrics and webhook.
	OperatorGoTLSConfig func(*tls.Config)
	// InitialTLSAdherencePolicy is the cluster-wide TLS adherence policy fetched at startup.
	InitialTLSAdherencePolicy configv1.TLSAdherencePolicy
	// InitialTLSProfileSpec is the cluster-wide TLS profile spec fetched at startup.
	InitialTLSProfileSpec configv1.TLSProfileSpec
}

// FetchAPIServerTLSConfig fetches TLS settings from apiservers/cluster.
func FetchAPIServerTLSConfig(ctx context.Context, k8sClient client.Client, setupLog logr.Logger) (*OpenShiftTLSProfile, error) {
	var err error
	profile := &OpenShiftTLSProfile{
		InitialTLSAdherencePolicy: configv1.TLSAdherencePolicyNoOpinion, // ZTWIM ignores tlsAdherence; profile application is unchanged.
	}

	profile.InitialTLSProfileSpec, err = openshifttls.FetchAPIServerTLSProfile(ctx, k8sClient)
	if err != nil {
		if apierrors.IsNotFound(err) || errors.Is(err, openshifttls.ErrCustomProfileNil) {
			// 404 error or invalid custom profile error falls back to default profile. Same as openshift-apiserver behaviour.
			setupLog.Info("Issue with fetching tlsProfile. Continuing with default profile.", "name", openshifttls.APIServerName)
			// Assign default spec which is intermediate
			profile.InitialTLSProfileSpec = *configv1.TLSProfiles[libgocrypto.DefaultTLSProfileType]
			profile.OperatorGoTLSConfig = operatorGoTLSConfig(profile.InitialTLSProfileSpec, setupLog)
		} else {
			// Non-404 error: return error, do not proceed. Never start with unkonwn tlsProfile.
			return nil, fmt.Errorf("error while fetching %q: %w", openshifttls.APIServerName, err)
		}
	} else {
		profile.OperatorGoTLSConfig = operatorGoTLSConfig(profile.InitialTLSProfileSpec, setupLog)
	}

	setupLog.Info("TLS profile", "profile", profile.InitialTLSProfileSpec)

	return profile, nil
}

func operatorGoTLSConfig(tlsProfileSpec configv1.TLSProfileSpec, setupLog logr.Logger) func(*tls.Config) {
	if tlsProfileSpec.MinTLSVersion == configv1.VersionTLS10 || tlsProfileSpec.MinTLSVersion == configv1.VersionTLS11 {
		setupLog.Info("TLS profile specifies a minimum TLS version that is less than 1.2. SPIFFE/SPIRE operands will use the default TLS profile")
	}

	profileTLSConfig, unsupportedCiphers := openshifttls.NewTLSConfigFromProfile(tlsProfileSpec)
	if len(unsupportedCiphers) > 0 {
		setupLog.Info("TLS configuration contains unsupported ciphers that will be ignored", "unsupportedCiphers", unsupportedCiphers)
	}

	return profileTLSConfig
}

func injectTLSConfigMap(spec *configv1.TLSProfileSpec, minTLSVersionKey, cipherSuitesKey string) map[string]interface{} {
	if spec == nil {
		return nil
	}

	var minTLSVersion configv1.TLSProtocolVersion
	var cipherSuites []string

	if spec.MinTLSVersion == configv1.VersionTLS10 || spec.MinTLSVersion == configv1.VersionTLS11 {
		minTLSVersion = configv1.TLSProfiles[libgocrypto.DefaultTLSProfileType].MinTLSVersion
		cipherSuites = libgocrypto.OpenSSLToIANACipherSuites(configv1.TLSProfiles[libgocrypto.DefaultTLSProfileType].Ciphers)
	} else {
		minTLSVersion = spec.MinTLSVersion
		cipherSuites = libgocrypto.OpenSSLToIANACipherSuites(spec.Ciphers)
	}

	injectable := map[string]interface{}{}

	if minTLSVersion != "" {
		injectable[minTLSVersionKey] = minTLSVersion
	}
	if len(cipherSuites) > 0 {
		injectable[cipherSuitesKey] = cipherSuites
	}
	if len(injectable) == 0 {
		return nil
	}

	return injectable
}

// GetTLSConfigForSpiffeSpire returns snake_case TLS settings for SPIRE server, agent, and OIDC configs.
// Only non-empty fields are included. Returns nil when there is nothing to inject.
func GetTLSConfigForSpiffeSpire(spec *configv1.TLSProfileSpec) map[string]interface{} {
	return injectTLSConfigMap(spec, minTLSVersionKeyForSPIRE, cipherSuitesKeyForSPIRE)
}

// GetTLSConfigForSCM returns camelCase TLS settings for spire-controller-manager YAML.
// Only non-empty fields are included. Returns nil when there is nothing to inject.
func GetTLSConfigForSCM(spec *configv1.TLSProfileSpec) map[string]interface{} {
	return injectTLSConfigMap(spec, minTLSVersionKeyForSCM, cipherSuitesKeyForSCM)
}
