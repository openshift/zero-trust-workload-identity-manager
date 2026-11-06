package client

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestUpstreamCacheScope(t *testing.T) {
	options := upstreamCacheObjects("operator-namespace")
	found := false
	for object, option := range options {
		if _, ok := object.(*corev1.Secret); !ok {
			continue
		}
		found = true
		if len(option.Namespaces) != 1 {
			t.Fatal("Secret cache must be limited to one namespace")
		}
		if _, ok := option.Namespaces["operator-namespace"]; !ok {
			t.Fatal("Secret cache must use the operator namespace")
		}
		if option.Label != nil {
			t.Fatal("administrator Secrets must not require managed labels")
		}
	}
	if !found {
		t.Fatal("Secret cache missing")
	}
	for _, resource := range informerResources {
		if _, ok := resource.(*corev1.Secret); ok {
			return
		}
	}
	t.Fatal("Secret informer must be registered before cached reads")
}

func TestUpstreamPodCacheScope(t *testing.T) {
	found := false
	for object, option := range upstreamCacheObjects("operator-namespace") {
		if _, ok := object.(*corev1.Pod); !ok {
			continue
		}
		found = true
		if len(option.Namespaces) != 1 {
			t.Fatal("Pod cache must be namespace scoped")
		}
		if _, ok := option.Namespaces["operator-namespace"]; !ok {
			t.Fatal("incorrect namespace")
		}
		if option.Label == nil || !option.Label.Matches(labels.Set(utils.SpireServerLabels(nil))) || option.Label.Matches(labels.Set(utils.SpireAgentLabels(nil))) || option.Label.Matches(labels.Set{}) {
			t.Fatal("Pod selector must select managed server pods only")
		}
	}
	if !found {
		t.Fatal("Pod cache missing")
	}
	for _, resource := range informerResources {
		if _, ok := resource.(*corev1.Pod); ok {
			return
		}
	}
	t.Fatal("Pod informer not preregistered")
}
