package spire_server

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGetSpireControllerManagerValidatingWebhookConfiguration(t *testing.T) {
	tests := []struct {
		name         string
		customLabels map[string]string
	}{
		{
			name:         "without custom labels",
			customLabels: nil,
		},
		{
			name: "with custom labels",
			customLabels: map[string]string{
				"admission-type": "validating",
				"security-tier":  "high",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhook := getSpireControllerManagerValidatingWebhookConfiguration(tt.customLabels)

			if webhook == nil {
				t.Fatal("Expected ValidatingWebhookConfiguration, got nil")
			}

			if webhook.Name != "spire-controller-manager-webhook" {
				t.Errorf("Expected ValidatingWebhookConfiguration name 'spire-controller-manager-webhook', got '%s'", webhook.Name)
			}

			// Check labels
			if len(webhook.Labels) == 0 {
				t.Error("Expected ValidatingWebhookConfiguration to have labels, got none")
			}

			if val, ok := webhook.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
				t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
			}

			if val, ok := webhook.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentControlPlane {
				t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentControlPlane)
			}

			// Check custom labels if specified
			for key, expectedValue := range tt.customLabels {
				if val, ok := webhook.Labels[key]; !ok || val != expectedValue {
					t.Errorf("Expected custom label '%s=%s', got '%s'", key, expectedValue, val)
				}
			}
		})
	}

	t.Run("preserves all asset labels", func(t *testing.T) {
		webhookWithoutCustom := getSpireControllerManagerValidatingWebhookConfiguration(nil)
		assetLabels := make(map[string]string)
		for k, v := range webhookWithoutCustom.Labels {
			assetLabels[k] = v
		}

		customLabels := map[string]string{"webhook-version": "v1"}
		webhookWithCustom := getSpireControllerManagerValidatingWebhookConfiguration(customLabels)

		for k, v := range assetLabels {
			if webhookWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved when custom labels were added, got '%s'", k, v, webhookWithCustom.Labels[k])
			}
		}

		if val, ok := webhookWithCustom.Labels["webhook-version"]; !ok || val != "v1" {
			t.Errorf("Custom label was not added")
		}
	})
}

// newWebhookTestReconciler creates a reconciler for Webhook tests
func newWebhookTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpireServerReconciler {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = admissionv1.AddToScheme(scheme)
	return &SpireServerReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}
}

func TestReconcileWebhook(t *testing.T) {
	tests := []struct {
		name           string
		server         *v1alpha1.SpireServer
		setupClient    func(*fakes.FakeCustomCtrlClient)
		createOnlyMode bool
		useEmptyScheme bool
		expectError    bool
		expectCreate   bool
		expectUpdate   bool
	}{
		{
			name: "create success",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-controller-manager-webhook"))
				fc.CreateReturns(nil)
			},
			expectError:  false,
			expectCreate: true,
		},
		{
			name: "create error",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-controller-manager-webhook"))
				fc.CreateReturns(errors.New("create failed"))
			},
			expectError: true,
		},
		{
			name: "get error",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(errors.New("connection refused"))
			},
			expectError: true,
		},
		{
			name: "update success",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireServerSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingWebhook := &admissionv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-controller-manager-webhook",
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if webhook, ok := obj.(*admissionv1.ValidatingWebhookConfiguration); ok {
						*webhook = *existingWebhook
					}
					return nil
				}
				fc.UpdateReturns(nil)
			},
			expectError:  false,
			expectUpdate: true,
		},
		{
			name: "update error",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireServerSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingWebhook := &admissionv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-controller-manager-webhook",
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if webhook, ok := obj.(*admissionv1.ValidatingWebhookConfiguration); ok {
						*webhook = *existingWebhook
					}
					return nil
				}
				fc.UpdateReturns(errors.New("update conflict"))
			},
			expectError:  true,
			expectUpdate: true,
		},
		{
			name: "create only mode skips update",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireServerSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingWebhook := &admissionv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-controller-manager-webhook",
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if webhook, ok := obj.(*admissionv1.ValidatingWebhookConfiguration); ok {
						*webhook = *existingWebhook
					}
					return nil
				}
			},
			createOnlyMode: true,
			expectError:    false,
			expectUpdate:   false,
		},
		{
			name: "set controller ref error",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient:    func(fc *fakes.FakeCustomCtrlClient) {},
			useEmptyScheme: true,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			var reconciler *SpireServerReconciler
			if tt.useEmptyScheme {
				reconciler = &SpireServerReconciler{
					ctrlClient:    fakeClient,
					ctx:           context.Background(),
					log:           logr.Discard(),
					scheme:        runtime.NewScheme(),
					eventRecorder: record.NewFakeRecorder(100),
				}
			} else {
				reconciler = newWebhookTestReconciler(fakeClient)
			}
			tt.setupClient(fakeClient)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileWebhook(context.Background(), tt.server, statusMgr, tt.createOnlyMode)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if tt.expectCreate && fakeClient.CreateCallCount() != 1 {
				t.Errorf("Expected Create to be called once, called %d times", fakeClient.CreateCallCount())
			}
			if tt.expectUpdate && fakeClient.UpdateCallCount() != 1 {
				t.Errorf("Expected Update to be called once, called %d times", fakeClient.UpdateCallCount())
			}
			if !tt.expectUpdate && fakeClient.UpdateCallCount() != 0 {
				t.Error("Expected Update not to be called")
			}
		})
	}
}
