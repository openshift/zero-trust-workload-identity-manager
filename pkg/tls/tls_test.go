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
	utiltls "github.com/openshift/controller-runtime-common/pkg/tls"
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

	profile, err := utiltls.GetTLSProfileSpec(nil)
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

func TestGetOperatorAndOperandTLSConfig(t *testing.T) {
	oldProfile := configv1.TLSProfiles[configv1.TLSProfileOldType]
	modernProfile := configv1.TLSProfiles[configv1.TLSProfileModernType]
	setupLog := logr.Discard()

	tests := []struct {
		name               string
		adherence          configv1.TLSAdherencePolicy
		tlsSecurityProfile *configv1.TLSSecurityProfile
		wantNilOperator    bool
		wantNilOperand     bool
		wantMinTLSVersion  uint16
		wantCipherSuites   bool
		wantOperandMinTLS  configv1.TLSProtocolVersion
		wantOperandCiphers bool
	}{
		{
			name:      "NoOpinion skips operator and operand TLS profile",
			adherence: configv1.TLSAdherencePolicyNoOpinion,
			tlsSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileOldType,
			},
			wantNilOperator: true,
			wantNilOperand:  true,
		},
		{
			name:      "StrictAllComponents honors Old profile on operator only",
			adherence: configv1.TLSAdherencePolicyStrictAllComponents,
			tlsSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileOldType,
			},
			wantMinTLSVersion: libgocrypto.TLSVersionOrDie(string(oldProfile.MinTLSVersion)),
			wantCipherSuites:  true,
			wantNilOperand:    true,
		},
		{
			name:      "StrictAllComponents honors Modern profile without cipher suites",
			adherence: configv1.TLSAdherencePolicyStrictAllComponents,
			tlsSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileModernType,
			},
			wantMinTLSVersion: libgocrypto.TLSVersionOrDie(string(modernProfile.MinTLSVersion)),
			wantCipherSuites:  false,
			wantOperandMinTLS: modernProfile.MinTLSVersion,
		},
		{
			name:      "StrictAllComponents honors custom profile",
			adherence: configv1.TLSAdherencePolicyStrictAllComponents,
			tlsSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						Ciphers: []string{
							"ECDHE-RSA-AES128-GCM-SHA256",
							"ECDHE-RSA-AES256-GCM-SHA384",
						},
						MinTLSVersion: configv1.VersionTLS12,
					},
				},
			},
			wantMinTLSVersion:  tls.VersionTLS12,
			wantCipherSuites:   true,
			wantOperandMinTLS:  configv1.VersionTLS12,
			wantOperandCiphers: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlsConfig, err := getOperatorAndOperandTLSConfig(&configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSAdherence:       tt.adherence,
					TLSSecurityProfile: tt.tlsSecurityProfile,
				},
			}, setupLog)
			if err != nil {
				t.Fatalf("getOperatorAndOperandTLSConfig() error = %v", err)
			}

			if tt.wantNilOperator {
				if tlsConfig.Resolved.OperatorTLSConfig != nil {
					t.Fatal("expected nil operator TLS config")
				}
			} else {
				tlsCfg := applyTLSConfig(t, tlsConfig.Resolved.OperatorTLSConfig)
				if tlsCfg.MinVersion != tt.wantMinTLSVersion {
					t.Fatalf("MinVersion = %d, want %d", tlsCfg.MinVersion, tt.wantMinTLSVersion)
				}
				if tt.wantCipherSuites {
					if len(tlsCfg.CipherSuites) == 0 {
						t.Fatal("expected cipher suites to be configured")
					}
				} else if len(tlsCfg.CipherSuites) != 0 {
					t.Fatalf("expected no cipher suites, got %v", tlsCfg.CipherSuites)
				}
			}

			if tt.wantNilOperand {
				if tlsConfig.Resolved.OperandTLSConfig != nil {
					t.Fatalf("expected nil operand profile, got %#v", tlsConfig.Resolved.OperandTLSConfig)
				}
				return
			}

			if tlsConfig.Resolved.OperandTLSConfig == nil {
				t.Fatal("expected non-nil operand profile")
			}
			if tlsConfig.Resolved.OperandTLSConfig.MinTLSVersion != tt.wantOperandMinTLS {
				t.Fatalf("MinTLSVersion = %q, want %q", tlsConfig.Resolved.OperandTLSConfig.MinTLSVersion, tt.wantOperandMinTLS)
			}
			if tt.wantOperandCiphers && len(tlsConfig.Resolved.OperandTLSConfig.CipherSuites) == 0 {
				t.Fatal("expected cipher suites to be populated")
			}
		})
	}
}

func TestGetOperandTLSConfig(t *testing.T) {
	setupLog := logr.Discard()
	oldProfile := configv1.TLSProfiles[configv1.TLSProfileOldType]

	if config := getOperandTLSConfig(*oldProfile, setupLog); config != nil {
		t.Fatalf("expected nil operand config for Old profile, got %#v", config)
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

func TestGetOperatorAndOperandTLSConfig_strictInvalidCustom(t *testing.T) {
	_, err := getOperatorAndOperandTLSConfig(&configv1.APIServer{
		Spec: configv1.APIServerSpec{
			TLSAdherence: configv1.TLSAdherencePolicyStrictAllComponents,
			TLSSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
			},
		},
	}, logr.Discard())
	if err == nil {
		t.Fatal("expected error for invalid custom profile")
	}
}

func TestFetchConfigV1APIServer_notFound(t *testing.T) {
	k8sClient := newAPIServerTestClient(t, nil, http.StatusNotFound)

	_, err := fetchConfigV1APIServer(context.Background(), k8sClient)
	if err == nil {
		t.Fatal("expected error when APIServer is not found")
	}
}

func TestFetchAPIServerTLSConfig_clientCreationError(t *testing.T) {
	scheme := testScheme(t)

	_, err := FetchAPIServerTLSConfig(context.Background(), nil, scheme, logr.Discard())
	if err == nil {
		t.Fatal("expected error when rest config is nil, got nil")
	}
}

func TestFetchAPIServerTLSConfig_strictIntermediate(t *testing.T) {
	scheme := testScheme(t)
	intermediateProfile := defaultIntermediateProfile(t)

	srv := newAPIServerTLSConfigTestServer(t, &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: utiltls.APIServerName},
		Spec: configv1.APIServerSpec{
			TLSAdherence: configv1.TLSAdherencePolicyStrictAllComponents,
			TLSSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileIntermediateType,
			},
		},
	}, http.StatusOK)
	defer srv.Close()

	cfg := &rest.Config{
		Host: srv.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	result, err := FetchAPIServerTLSConfig(context.Background(), cfg, scheme, logr.Discard())
	if err != nil {
		t.Fatalf("FetchAPIServerTLSConfig() error = %v", err)
	}
	if result.InitialTLSAdherencePolicy != configv1.TLSAdherencePolicyStrictAllComponents {
		t.Fatalf("InitialTLSAdherencePolicy = %q, want %q", result.InitialTLSAdherencePolicy, configv1.TLSAdherencePolicyStrictAllComponents)
	}
	if !reflect.DeepEqual(result.InitialTLSProfileSpec, intermediateProfile) {
		t.Fatalf("InitialTLSProfileSpec = %#v, want %#v", result.InitialTLSProfileSpec, intermediateProfile)
	}
	if result.Resolved.OperatorTLSConfig == nil {
		t.Fatal("expected operator TLS config under strict adherence")
	}
	if result.Resolved.OperandTLSConfig == nil {
		t.Fatal("expected operand TLS config under strict adherence")
	}
}

func TestFetchAPIServerTLSConfig_nonStrictSkipsOperatorTLS(t *testing.T) {
	scheme := testScheme(t)
	intermediateProfile := defaultIntermediateProfile(t)

	srv := newAPIServerTLSConfigTestServer(t, &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: utiltls.APIServerName},
		Spec: configv1.APIServerSpec{
			TLSAdherence: configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly,
			TLSSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileIntermediateType,
			},
		},
	}, http.StatusOK)
	defer srv.Close()

	cfg := &rest.Config{
		Host: srv.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	result, err := FetchAPIServerTLSConfig(context.Background(), cfg, scheme, logr.Discard())
	if err != nil {
		t.Fatalf("FetchAPIServerTLSConfig() error = %v", err)
	}
	if result.InitialTLSAdherencePolicy != configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly {
		t.Fatalf("InitialTLSAdherencePolicy = %q, want %q", result.InitialTLSAdherencePolicy, configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly)
	}
	if !reflect.DeepEqual(result.InitialTLSProfileSpec, intermediateProfile) {
		t.Fatalf("InitialTLSProfileSpec = %#v, want %#v", result.InitialTLSProfileSpec, intermediateProfile)
	}
	if result.Resolved.OperatorTLSConfig != nil {
		t.Fatal("expected nil operator TLS config under non-strict adherence")
	}
	if result.Resolved.OperandTLSConfig != nil {
		t.Fatal("expected nil operand TLS config under non-strict adherence")
	}
}

func TestFetchAPIServerTLSConfig_notFoundSeedsDefaultBaseline(t *testing.T) {
	scheme := testScheme(t)
	defaultProfile := defaultIntermediateProfile(t)

	srv := newAPIServerTLSConfigTestServer(t, nil, http.StatusNotFound)
	defer srv.Close()

	cfg := &rest.Config{
		Host: srv.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	result, err := FetchAPIServerTLSConfig(context.Background(), cfg, scheme, logr.Discard())
	if err != nil {
		t.Fatalf("FetchAPIServerTLSConfig() error = %v", err)
	}
	if result.InitialTLSAdherencePolicy != configv1.TLSAdherencePolicyNoOpinion {
		t.Fatalf("InitialTLSAdherencePolicy = %q, want %q", result.InitialTLSAdherencePolicy, configv1.TLSAdherencePolicyNoOpinion)
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
	scheme := testScheme(t)

	srv := newAPIServerTLSConfigTestServer(t, nil, http.StatusForbidden)
	defer srv.Close()

	cfg := &rest.Config{
		Host: srv.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	_, err := FetchAPIServerTLSConfig(context.Background(), cfg, scheme, logr.Discard())
	if err == nil {
		t.Fatal("expected error when APIServer fetch fails with non-404 error")
	}
}

func TestFetchAPIServerTLSConfig_strictInvalidCustomFails(t *testing.T) {
	scheme := testScheme(t)

	srv := newAPIServerTLSConfigTestServer(t, &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: utiltls.APIServerName},
		Spec: configv1.APIServerSpec{
			TLSAdherence: configv1.TLSAdherencePolicyStrictAllComponents,
			TLSSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
			},
		},
	}, http.StatusOK)
	defer srv.Close()

	cfg := &rest.Config{
		Host: srv.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	_, err := FetchAPIServerTLSConfig(context.Background(), cfg, scheme, logr.Discard())
	if err == nil {
		t.Fatal("expected error for strict adherence with invalid custom profile")
	}
}

func TestFetchAPIServerTLSConfig_nonStrictInvalidCustomFails(t *testing.T) {
	scheme := testScheme(t)

	srv := newAPIServerTLSConfigTestServer(t, &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: utiltls.APIServerName},
		Spec: configv1.APIServerSpec{
			TLSAdherence: configv1.TLSAdherencePolicyNoOpinion,
			TLSSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
			},
		},
	}, http.StatusOK)
	defer srv.Close()

	cfg := &rest.Config{
		Host: srv.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	_, err := FetchAPIServerTLSConfig(context.Background(), cfg, scheme, logr.Discard())
	if err == nil {
		t.Fatal("expected error for invalid custom profile")
	}
}
