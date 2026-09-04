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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	openshifttls "github.com/openshift/controller-runtime-common/pkg/tls"
	libgocrypto "github.com/openshift/library-go/pkg/crypto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := configv1.Install(scheme); err != nil {
		t.Fatalf("failed to install configv1 scheme: %v", err)
	}

	return scheme
}

func defaultIntermediateProfile(t *testing.T) configv1.TLSProfileSpec {
	t.Helper()

	profile, err := openshifttls.GetTLSProfileSpec(nil)
	if err != nil {
		t.Fatalf("GetTLSProfileSpec(nil) error = %v", err)
	}

	return profile
}

func applyTLSConfig(t *testing.T, tlsConfig func(*tls.Config)) *tls.Config {
	t.Helper()

	if tlsConfig == nil {
		t.Fatal("expected TLSConfig function to be set")
	}

	cfg := &tls.Config{}
	tlsConfig(cfg)
	return cfg
}

func newAPIServerTestClient(t *testing.T, apiServer *configv1.APIServer, clusterStatus int) client.Client {
	t.Helper()

	scheme := testScheme(t)
	srv := newAPIServerTLSConfigTestServer(t, apiServer, clusterStatus)
	t.Cleanup(srv.Close)

	cfg := &rest.Config{
		Host: srv.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("client.New() error = %v", err)
	}

	return k8sClient
}

func newAPIServerTLSConfigTestServer(t *testing.T, apiServer *configv1.APIServer, clusterStatus int) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"kind":"APIVersions","apiVersion":"v1","versions":["v1"]}`))
		case "/apis":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"kind":"APIGroupList","apiVersion":"v1","groups":[{"name":"config.openshift.io","versions":[{"groupVersion":"config.openshift.io/v1","version":"v1"}]}]}`))
		case "/apis/config.openshift.io/v1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"kind":"APIResourceList","apiVersion":"v1","groupVersion":"config.openshift.io/v1","resources":[{"name":"apiservers","namespaced":false,"kind":"APIServer","verbs":["get","list","watch"]}]}`))
		case "/apis/config.openshift.io/v1/apiservers/cluster":
			w.Header().Set("Content-Type", "application/json")
			if clusterStatus != http.StatusOK {
				w.WriteHeader(clusterStatus)
				return
			}
			if apiServer == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			obj := apiServer.DeepCopy()
			obj.APIVersion = "config.openshift.io/v1"
			obj.Kind = "APIServer"
			body, err := json.Marshal(obj)
			if err != nil {
				t.Fatalf("failed to marshal APIServer: %v", err)
			}
			_, _ = w.Write(body)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
}

func TestFetchAPIServerTLSConfig_honorsProfileRegardlessOfAdherence(t *testing.T) {
	intermediateProfile := defaultIntermediateProfile(t)
	k8sClient := newAPIServerTestClient(t, &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: openshifttls.APIServerName},
		Spec: configv1.APIServerSpec{
			TLSAdherence: configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly,
			TLSSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileIntermediateType,
			},
		},
	}, http.StatusOK)

	result, err := FetchAPIServerTLSConfig(context.Background(), k8sClient, logr.Discard())
	if err != nil {
		t.Fatalf("FetchAPIServerTLSConfig() error = %v", err)
	}
	if !reflect.DeepEqual(result.InitialTLSProfileSpec, intermediateProfile) {
		t.Fatalf("InitialTLSProfileSpec = %#v, want %#v", result.InitialTLSProfileSpec, intermediateProfile)
	}
	if result.Resolved.OperatorTLSConfig == nil {
		t.Fatal("expected operator TLS config regardless of tlsAdherence")
	}
	if result.Resolved.OperandTLSConfig == nil {
		t.Fatal("expected operand TLS config regardless of tlsAdherence")
	}
}

func TestGetOperatorTLSConfig_oldProfile(t *testing.T) {
	oldProfile := configv1.TLSProfiles[configv1.TLSProfileOldType]
	defaultProfile, err := openshifttls.GetTLSProfileSpec(nil)
	if err != nil {
		t.Fatalf("GetTLSProfileSpec(nil) error = %v", err)
	}

	operatorTLSConfig := getOperatorTLSConfig(*oldProfile, logr.Discard())
	tlsCfg := applyTLSConfig(t, operatorTLSConfig)
	if tlsCfg.MinVersion != libgocrypto.TLSVersionOrDie(string(oldProfile.MinTLSVersion)) {
		t.Fatalf("MinVersion = %d, want %d", tlsCfg.MinVersion, libgocrypto.TLSVersionOrDie(string(oldProfile.MinTLSVersion)))
	}
	if len(tlsCfg.CipherSuites) == 0 {
		t.Fatal("expected cipher suites for Old profile on operator TLS")
	}

	operandConfig := getOperandTLSConfig(*oldProfile, logr.Discard())
	if operandConfig == nil {
		t.Fatal("expected default operand config for Old profile")
	}
	if operandConfig.MinTLSVersion != defaultProfile.MinTLSVersion {
		t.Fatalf("operand MinTLSVersion = %q, want default %q", operandConfig.MinTLSVersion, defaultProfile.MinTLSVersion)
	}
}

func TestGetOperandTLSConfig(t *testing.T) {
	setupLog := logr.Discard()
	oldProfile := configv1.TLSProfiles[configv1.TLSProfileOldType]
	defaultProfile, err := openshifttls.GetTLSProfileSpec(nil)
	if err != nil {
		t.Fatalf("GetTLSProfileSpec(nil) error = %v", err)
	}

	oldOperandConfig := getOperandTLSConfig(*oldProfile, setupLog)
	if oldOperandConfig == nil {
		t.Fatal("expected default operand config for Old profile")
	}
	if oldOperandConfig.MinTLSVersion != defaultProfile.MinTLSVersion {
		t.Fatalf("operand MinTLSVersion = %q, want default %q", oldOperandConfig.MinTLSVersion, defaultProfile.MinTLSVersion)
	}

	customConfig := getOperandTLSConfig(configv1.TLSProfileSpec{
		Ciphers: []string{
			"ECDHE-RSA-AES128-GCM-SHA256",
			"ECDHE-RSA-AES256-GCM-SHA384",
		},
		MinTLSVersion: configv1.VersionTLS12,
	}, setupLog)
	if customConfig == nil {
		t.Fatal("expected non-nil operand config")
	}
	if customConfig.MinTLSVersion != configv1.VersionTLS12 {
		t.Fatalf("MinTLSVersion = %q, want %q", customConfig.MinTLSVersion, configv1.VersionTLS12)
	}
	if !reflect.DeepEqual(customConfig.CipherSuites, []string{
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
	}) {
		t.Fatalf("CipherSuites = %q, want IANA cipher names", customConfig.CipherSuites)
	}
}

func TestFetchAPIServerTLSConfig_oldProfilePreservesResolvedInitialSpec(t *testing.T) {
	oldProfile := configv1.TLSProfiles[configv1.TLSProfileOldType]
	defaultProfile := defaultIntermediateProfile(t)
	k8sClient := newAPIServerTestClient(t, &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: openshifttls.APIServerName},
		Spec: configv1.APIServerSpec{
			TLSSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileOldType,
			},
		},
	}, http.StatusOK)

	result, err := FetchAPIServerTLSConfig(context.Background(), k8sClient, logr.Discard())
	if err != nil {
		t.Fatalf("FetchAPIServerTLSConfig() error = %v", err)
	}
	if !reflect.DeepEqual(result.InitialTLSProfileSpec, *oldProfile) {
		t.Fatalf("InitialTLSProfileSpec = %#v, want resolved Old profile %#v", result.InitialTLSProfileSpec, *oldProfile)
	}
	if result.Resolved.OperandTLSConfig == nil {
		t.Fatal("expected operand TLS config from default profile")
	}
	if result.Resolved.OperandTLSConfig.MinTLSVersion != defaultProfile.MinTLSVersion {
		t.Fatalf("operand MinTLSVersion = %q, want default %q", result.Resolved.OperandTLSConfig.MinTLSVersion, defaultProfile.MinTLSVersion)
	}
}

func TestFetchAPIServerTLSConfig_invalidCustomFallsBackToDefault(t *testing.T) {
	defaultProfile := defaultIntermediateProfile(t)
	k8sClient := newAPIServerTestClient(t, &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: openshifttls.APIServerName},
		Spec: configv1.APIServerSpec{
			TLSSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
			},
		},
	}, http.StatusOK)

	result, err := FetchAPIServerTLSConfig(context.Background(), k8sClient, logr.Discard())
	if err != nil {
		t.Fatalf("FetchAPIServerTLSConfig() error = %v", err)
	}
	if !reflect.DeepEqual(result.InitialTLSProfileSpec, defaultProfile) {
		t.Fatalf("InitialTLSProfileSpec = %#v, want default Intermediate %#v", result.InitialTLSProfileSpec, defaultProfile)
	}
	if result.Resolved.OperatorTLSConfig == nil || result.Resolved.OperandTLSConfig == nil {
		t.Fatal("expected operator and operand TLS config from default profile")
	}
}

func TestFetchAPIServerTLSConfig_intermediateProfile(t *testing.T) {
	intermediateProfile := defaultIntermediateProfile(t)
	k8sClient := newAPIServerTestClient(t, &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: openshifttls.APIServerName},
		Spec: configv1.APIServerSpec{
			TLSSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileIntermediateType,
			},
		},
	}, http.StatusOK)

	result, err := FetchAPIServerTLSConfig(context.Background(), k8sClient, logr.Discard())
	if err != nil {
		t.Fatalf("FetchAPIServerTLSConfig() error = %v", err)
	}
	if !reflect.DeepEqual(result.InitialTLSProfileSpec, intermediateProfile) {
		t.Fatalf("InitialTLSProfileSpec = %#v, want %#v", result.InitialTLSProfileSpec, intermediateProfile)
	}
	if result.Resolved.OperatorTLSConfig == nil {
		t.Fatal("expected operator TLS config")
	}
	if result.Resolved.OperandTLSConfig == nil {
		t.Fatal("expected operand TLS config")
	}
}

func TestFetchAPIServerTLSConfig_notFoundSeedsDefaultBaseline(t *testing.T) {
	defaultProfile := defaultIntermediateProfile(t)
	k8sClient := newAPIServerTestClient(t, nil, http.StatusNotFound)

	result, err := FetchAPIServerTLSConfig(context.Background(), k8sClient, logr.Discard())
	if err != nil {
		t.Fatalf("FetchAPIServerTLSConfig() error = %v", err)
	}
	if result.Resolved.OperatorTLSConfig == nil {
		t.Fatal("expected operator TLS config seeded from default Intermediate profile on 404")
	}
	if result.Resolved.OperandTLSConfig == nil {
		t.Fatal("expected operand TLS config seeded from default Intermediate profile on 404")
	}
	if !reflect.DeepEqual(result.InitialTLSProfileSpec, defaultProfile) {
		t.Fatalf("InitialTLSProfileSpec = %#v, want default Intermediate baseline %#v", result.InitialTLSProfileSpec, defaultProfile)
	}
}

func TestFetchAPIServerTLSConfig_getFailureReturnsError(t *testing.T) {
	k8sClient := newAPIServerTestClient(t, nil, http.StatusForbidden)

	_, err := FetchAPIServerTLSConfig(context.Background(), k8sClient, logr.Discard())
	if err == nil {
		t.Fatal("expected error when APIServer fetch fails with non-404 error")
	}
}

func TestGetInjectableTLSConfigForOperand(t *testing.T) {
	if got := GetInjectableTLSConfigForOperand(nil); got != nil {
		t.Fatalf("expected nil injectable config, got %#v", got)
	}

	partial := GetInjectableTLSConfigForOperand(&OperandTLSConfig{MinTLSVersion: configv1.VersionTLS12})
	if len(partial) != 1 || partial["min_tls_version"] != configv1.VersionTLS12 {
		t.Fatalf("expected partial injectable config, got %#v", partial)
	}

	full := GetInjectableTLSConfigForOperand(&OperandTLSConfig{
		MinTLSVersion:    configv1.VersionTLS13,
		CipherSuites:     []string{"TLS_AES_128_GCM_SHA256"},
		CurvePreferences: []string{"X25519"},
	})
	if len(full) != 3 {
		t.Fatalf("expected full injectable config with 3 fields, got %#v", full)
	}

	if got := GetInjectableTLSConfigForOperand(&OperandTLSConfig{}); got != nil {
		t.Fatalf("expected nil injectable config for empty operand profile, got %#v", got)
	}
}

func TestGetOperandTLSConfig_emptyProfileReturnsNil(t *testing.T) {
	if config := getOperandTLSConfig(configv1.TLSProfileSpec{}, logr.Discard()); config != nil {
		t.Fatalf("expected nil operand config for empty profile, got %#v", config)
	}
}
