package webhook

import (
	"context"
	"fmt"
	"strings"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1alpha1 "github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	customClient "github.com/openshift/zero-trust-workload-identity-manager/pkg/client"
)

//+kubebuilder:webhook:path=/validate-operator-openshift-io-v1alpha1-spireserver,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.openshift.io,resources=spireservers,verbs=create;update,versions=v1alpha1,name=spireserver.operator.openshift.io,admissionReviewVersions=v1

//+kubebuilder:webhook:path=/validate-operator-openshift-io-v1alpha1-spireagent,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.openshift.io,resources=spireagents,verbs=create;update,versions=v1alpha1,name=spireagent.operator.openshift.io,admissionReviewVersions=v1

//+kubebuilder:webhook:path=/validate-operator-openshift-io-v1alpha1-spireoidcdiscoveryprovider,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.openshift.io,resources=spireoidcdiscoveryproviders,verbs=create;update,versions=v1alpha1,name=spireoidcdiscoveryprovider.operator.openshift.io,admissionReviewVersions=v1

func normalizeIssuer(issuer string, trustDomain string) string {
	if issuer == "" {
		return fmt.Sprintf("oidc-discovery.%s", trustDomain)
	}
	issuer = strings.TrimPrefix(issuer, "https://")
	issuer = strings.TrimPrefix(issuer, "http://")
	return issuer
}

// SpireServer validator
// +kubebuilder:object:generate=false
type SpireServerValidator struct {
	Client customClient.CustomCtrlClient
}

var (
	_ webhook.CustomValidator = (*SpireServerValidator)(nil)
	_ webhook.CustomValidator = (*SpireAgentValidator)(nil)
	_ webhook.CustomValidator = (*SpireOIDCDiscoveryProviderValidator)(nil)
)

func (v *SpireServerValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	server, ok := obj.(*v1alpha1.SpireServer)
	if !ok {
		return nil, fmt.Errorf("internal error: expected a SpireServer object but got %T", obj)
	}

	var validationErrors []string

	// Check SpireAgent if it exists and validate field consistency
	var agent v1alpha1.SpireAgent
	if err := v.Client.Get(ctx, types.NamespacedName{Name: "cluster"}, &agent); err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, fmt.Errorf("validation failed: unable to check for existing SpireAgent: %v", err)
		}
		// SpireAgent doesn't exist - that's allowed
	} else {
		// SpireAgent exists, validate consistency
		if server.Spec.TrustDomain != agent.Spec.TrustDomain {
			validationErrors = append(validationErrors, fmt.Sprintf("trustDomain %q must match the existing SpireAgent trustDomain %q", server.Spec.TrustDomain, agent.Spec.TrustDomain))
		}
		if server.Spec.ClusterName != agent.Spec.ClusterName {
			validationErrors = append(validationErrors, fmt.Sprintf("clusterName %q must match the existing SpireAgent clusterName %q", server.Spec.ClusterName, agent.Spec.ClusterName))
		}
	}

	// Check SpireOIDCDiscoveryProvider if it exists and validate field consistency
	var oidc v1alpha1.SpireOIDCDiscoveryProvider
	if err := v.Client.Get(ctx, types.NamespacedName{Name: "cluster"}, &oidc); err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, fmt.Errorf("validation failed: unable to check for existing SpireOIDCDiscoveryProvider: %v", err)
		}
		// SpireOIDCDiscoveryProvider doesn't exist - that's allowed
	} else {
		// SpireOIDCDiscoveryProvider exists, validate consistency
		if server.Spec.TrustDomain != oidc.Spec.TrustDomain {
			validationErrors = append(validationErrors, fmt.Sprintf("trustDomain %q must match the existing SpireOIDCDiscoveryProvider trustDomain %q", server.Spec.TrustDomain, oidc.Spec.TrustDomain))
		}
		// Validate JwtIssuer consistency
		serverIssuer := normalizeIssuer(server.Spec.JwtIssuer, server.Spec.TrustDomain)
		oidcIssuer := normalizeIssuer(oidc.Spec.JwtIssuer, oidc.Spec.TrustDomain)
		if serverIssuer != oidcIssuer {
			validationErrors = append(validationErrors, fmt.Sprintf("jwtIssuer %q (normalized: %q) must match the existing SpireOIDCDiscoveryProvider jwtIssuer (normalized: %q)", server.Spec.JwtIssuer, serverIssuer, oidcIssuer))
		}
	}

	// Return first validation error if any
	if len(validationErrors) > 0 {
		return nil, fmt.Errorf("validation failed: SpireServer %s. Please update the SpireServer configuration", validationErrors[0])
	}

	return nil, nil
}

func (v *SpireServerValidator) ValidateUpdate(ctx context.Context, oldObj runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	oldServer, ok := oldObj.(*v1alpha1.SpireServer)
	if !ok {
		return nil, fmt.Errorf("internal error: expected old SpireServer object but got %T", oldObj)
	}
	newServer, ok := newObj.(*v1alpha1.SpireServer)
	if !ok {
		return nil, fmt.Errorf("internal error: expected new SpireServer object but got %T", newObj)
	}
	// Immutability: trustDomain and clusterName
	if oldServer.Spec.TrustDomain != newServer.Spec.TrustDomain {
		return nil, fmt.Errorf("validation failed: trustDomain field is immutable and cannot be changed from %q to %q. Please create a new SpireServer resource instead", oldServer.Spec.TrustDomain, newServer.Spec.TrustDomain)
	}
	if oldServer.Spec.ClusterName != newServer.Spec.ClusterName {
		return nil, fmt.Errorf("validation failed: clusterName field is immutable and cannot be changed from %q to %q. Please create a new SpireServer resource instead", oldServer.Spec.ClusterName, newServer.Spec.ClusterName)
	}

	var validationErrors []string

	// Check SpireAgent if it exists and validate field consistency with the new values
	var agent v1alpha1.SpireAgent
	if err := v.Client.Get(ctx, types.NamespacedName{Name: "cluster"}, &agent); err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, fmt.Errorf("validation failed: unable to check for existing SpireAgent: %v", err)
		}
		// SpireAgent doesn't exist - that's allowed
	} else {
		// SpireAgent exists, validate consistency with new server values
		if newServer.Spec.TrustDomain != agent.Spec.TrustDomain {
			validationErrors = append(validationErrors, fmt.Sprintf("trustDomain %q must match the existing SpireAgent trustDomain %q", newServer.Spec.TrustDomain, agent.Spec.TrustDomain))
		}
		if newServer.Spec.ClusterName != agent.Spec.ClusterName {
			validationErrors = append(validationErrors, fmt.Sprintf("clusterName %q must match the existing SpireAgent clusterName %q", newServer.Spec.ClusterName, agent.Spec.ClusterName))
		}
	}

	// Check SpireOIDCDiscoveryProvider if it exists and validate field consistency with the new values
	var oidc v1alpha1.SpireOIDCDiscoveryProvider
	if err := v.Client.Get(ctx, types.NamespacedName{Name: "cluster"}, &oidc); err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, fmt.Errorf("validation failed: unable to check for existing SpireOIDCDiscoveryProvider: %v", err)
		}
		// SpireOIDCDiscoveryProvider doesn't exist - that's allowed
	} else {
		// SpireOIDCDiscoveryProvider exists, validate consistency with new server values
		if newServer.Spec.TrustDomain != oidc.Spec.TrustDomain {
			validationErrors = append(validationErrors, fmt.Sprintf("trustDomain %q must match the existing SpireOIDCDiscoveryProvider trustDomain %q", newServer.Spec.TrustDomain, oidc.Spec.TrustDomain))
		}
		// Validate JwtIssuer consistency with new server values
		newServerIssuer := normalizeIssuer(newServer.Spec.JwtIssuer, newServer.Spec.TrustDomain)
		oidcIssuer := normalizeIssuer(oidc.Spec.JwtIssuer, oidc.Spec.TrustDomain)
		if newServerIssuer != oidcIssuer {
			validationErrors = append(validationErrors, fmt.Sprintf("jwtIssuer %q (normalized: %q) must match the existing SpireOIDCDiscoveryProvider jwtIssuer (normalized: %q)", newServer.Spec.JwtIssuer, newServerIssuer, oidcIssuer))
		}
	}

	// Return first validation error if any
	if len(validationErrors) > 0 {
		return nil, fmt.Errorf("validation failed: SpireServer update %s. Please update the SpireServer configuration", validationErrors[0])
	}

	return nil, nil
}

func (v *SpireServerValidator) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// SpireAgent validator
// +kubebuilder:object:generate=false
type SpireAgentValidator struct {
	Client customClient.CustomCtrlClient
}

func (v *SpireAgentValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	agent, ok := obj.(*v1alpha1.SpireAgent)
	if !ok {
		return nil, fmt.Errorf("internal error: expected a SpireAgent object but got %T", obj)
	}

	var validationErrors []string

	// Check SpireServer if it exists and validate field consistency
	var server v1alpha1.SpireServer
	if err := v.Client.Get(ctx, types.NamespacedName{Name: "cluster"}, &server); err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, fmt.Errorf("validation failed: unable to check for existing SpireServer: %v", err)
		}
		// SpireServer doesn't exist - that's allowed
	} else {
		// SpireServer exists, validate field consistency
		if agent.Spec.TrustDomain != server.Spec.TrustDomain {
			validationErrors = append(validationErrors, fmt.Sprintf("trustDomain %q must match the existing SpireServer trustDomain %q", agent.Spec.TrustDomain, server.Spec.TrustDomain))
		}
		if agent.Spec.ClusterName != server.Spec.ClusterName {
			validationErrors = append(validationErrors, fmt.Sprintf("clusterName %q must match the existing SpireServer clusterName %q", agent.Spec.ClusterName, server.Spec.ClusterName))
		}
	}

	// Check SpireOIDCDiscoveryProvider if it exists and validate field consistency
	var oidc v1alpha1.SpireOIDCDiscoveryProvider
	if err := v.Client.Get(ctx, types.NamespacedName{Name: "cluster"}, &oidc); err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, fmt.Errorf("validation failed: unable to check for existing SpireOIDCDiscoveryProvider: %v", err)
		}
		// SpireOIDCDiscoveryProvider doesn't exist - that's allowed
	} else {
		// SpireOIDCDiscoveryProvider exists, validate trustDomain consistency
		if agent.Spec.TrustDomain != oidc.Spec.TrustDomain {
			validationErrors = append(validationErrors, fmt.Sprintf("trustDomain %q must match the existing SpireOIDCDiscoveryProvider trustDomain %q", agent.Spec.TrustDomain, oidc.Spec.TrustDomain))
		}
	}

	// Return first validation error if any
	if len(validationErrors) > 0 {
		return nil, fmt.Errorf("validation failed: SpireAgent %s. Please update the SpireAgent configuration", validationErrors[0])
	}

	return nil, nil
}

func (v *SpireAgentValidator) ValidateUpdate(ctx context.Context, oldObj runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	oldAgent, ok := oldObj.(*v1alpha1.SpireAgent)
	if !ok {
		return nil, fmt.Errorf("internal error: expected old SpireAgent object but got %T", oldObj)
	}
	newAgent, ok := newObj.(*v1alpha1.SpireAgent)
	if !ok {
		return nil, fmt.Errorf("internal error: expected new SpireAgent object but got %T", newObj)
	}
	// Immutability: trustDomain and clusterName
	if oldAgent.Spec.TrustDomain != newAgent.Spec.TrustDomain {
		return nil, fmt.Errorf("validation failed: trustDomain field is immutable and cannot be changed from %q to %q. Please create a new SpireAgent resource instead", oldAgent.Spec.TrustDomain, newAgent.Spec.TrustDomain)
	}
	if oldAgent.Spec.ClusterName != newAgent.Spec.ClusterName {
		return nil, fmt.Errorf("validation failed: clusterName field is immutable and cannot be changed from %q to %q. Please create a new SpireAgent resource instead", oldAgent.Spec.ClusterName, newAgent.Spec.ClusterName)
	}

	var validationErrors []string

	// Check SpireServer if it exists and validate field consistency with the new values
	var server v1alpha1.SpireServer
	if err := v.Client.Get(ctx, types.NamespacedName{Name: "cluster"}, &server); err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, fmt.Errorf("validation failed: unable to check for existing SpireServer: %v", err)
		}
		// SpireServer doesn't exist - that's allowed
	} else {
		// SpireServer exists, validate field consistency with new agent values
		if newAgent.Spec.TrustDomain != server.Spec.TrustDomain {
			validationErrors = append(validationErrors, fmt.Sprintf("trustDomain %q must match the existing SpireServer trustDomain %q", newAgent.Spec.TrustDomain, server.Spec.TrustDomain))
		}
		if newAgent.Spec.ClusterName != server.Spec.ClusterName {
			validationErrors = append(validationErrors, fmt.Sprintf("clusterName %q must match the existing SpireServer clusterName %q", newAgent.Spec.ClusterName, server.Spec.ClusterName))
		}
	}

	// Check SpireOIDCDiscoveryProvider if it exists and validate field consistency with the new values
	var oidc v1alpha1.SpireOIDCDiscoveryProvider
	if err := v.Client.Get(ctx, types.NamespacedName{Name: "cluster"}, &oidc); err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, fmt.Errorf("validation failed: unable to check for existing SpireOIDCDiscoveryProvider: %v", err)
		}
		// SpireOIDCDiscoveryProvider doesn't exist - that's allowed
	} else {
		// SpireOIDCDiscoveryProvider exists, validate trustDomain consistency with new agent values
		if newAgent.Spec.TrustDomain != oidc.Spec.TrustDomain {
			validationErrors = append(validationErrors, fmt.Sprintf("trustDomain %q must match the existing SpireOIDCDiscoveryProvider trustDomain %q", newAgent.Spec.TrustDomain, oidc.Spec.TrustDomain))
		}
	}

	// Return first validation error if any
	if len(validationErrors) > 0 {
		return nil, fmt.Errorf("validation failed: SpireAgent update %s. Please update the SpireAgent configuration", validationErrors[0])
	}

	return nil, nil
}

func (v *SpireAgentValidator) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// OIDC validator
// +kubebuilder:object:generate=false
type SpireOIDCDiscoveryProviderValidator struct {
	Client customClient.CustomCtrlClient
}

func (v *SpireOIDCDiscoveryProviderValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	oidc, ok := obj.(*v1alpha1.SpireOIDCDiscoveryProvider)
	if !ok {
		return nil, fmt.Errorf("internal error: expected a SpireOIDCDiscoveryProvider object but got %T", obj)
	}

	var validationErrors []string

	// Check SpireServer if it exists and validate field consistency (but don't require it to exist)
	var server v1alpha1.SpireServer
	if err := v.Client.Get(ctx, types.NamespacedName{Name: "cluster"}, &server); err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, fmt.Errorf("validation failed: unable to check for existing SpireServer: %v", err)
		}
		// SpireServer doesn't exist - that's allowed
	} else {
		// SpireServer exists, validate trustDomain consistency
		if oidc.Spec.TrustDomain != server.Spec.TrustDomain {
			validationErrors = append(validationErrors, fmt.Sprintf("trustDomain %q must match the existing SpireServer trustDomain %q", oidc.Spec.TrustDomain, server.Spec.TrustDomain))
		}
		// Validate JwtIssuer consistency with SpireServer
		serverIssuer := normalizeIssuer(server.Spec.JwtIssuer, server.Spec.TrustDomain)
		oidcIssuer := normalizeIssuer(oidc.Spec.JwtIssuer, oidc.Spec.TrustDomain)
		if serverIssuer != oidcIssuer {
			validationErrors = append(validationErrors, fmt.Sprintf("jwtIssuer %q (normalized: %q) must match the existing SpireServer jwtIssuer (normalized: %q)", oidc.Spec.JwtIssuer, oidcIssuer, serverIssuer))
		}
	}

	// Check SpireAgent if it exists and validate field consistency (but don't require it to exist)
	var agent v1alpha1.SpireAgent
	if err := v.Client.Get(ctx, types.NamespacedName{Name: "cluster"}, &agent); err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, fmt.Errorf("validation failed: unable to check for existing SpireAgent: %v", err)
		}
		// SpireAgent doesn't exist - that's allowed
	} else {
		// SpireAgent exists, validate trustDomain consistency
		if oidc.Spec.TrustDomain != agent.Spec.TrustDomain {
			validationErrors = append(validationErrors, fmt.Sprintf("trustDomain %q must match the existing SpireAgent trustDomain %q", oidc.Spec.TrustDomain, agent.Spec.TrustDomain))
		}
	}

	// Return first validation error if any
	if len(validationErrors) > 0 {
		return nil, fmt.Errorf("validation failed: SpireOIDCDiscoveryProvider %s. Please update the SpireOIDCDiscoveryProvider configuration", validationErrors[0])
	}

	return nil, nil
}

func (v *SpireOIDCDiscoveryProviderValidator) ValidateUpdate(ctx context.Context, oldObj runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	oldOIDC, ok := oldObj.(*v1alpha1.SpireOIDCDiscoveryProvider)
	if !ok {
		return nil, fmt.Errorf("internal error: expected old SpireOIDCDiscoveryProvider object but got %T", oldObj)
	}
	newOIDC, ok := newObj.(*v1alpha1.SpireOIDCDiscoveryProvider)
	if !ok {
		return nil, fmt.Errorf("internal error: expected new SpireOIDCDiscoveryProvider object but got %T", newObj)
	}
	// Immutability: trustDomain
	if oldOIDC.Spec.TrustDomain != newOIDC.Spec.TrustDomain {
		return nil, fmt.Errorf("validation failed: trustDomain field is immutable and cannot be changed from %q to %q. Please create a new SpireOIDCDiscoveryProvider resource instead", oldOIDC.Spec.TrustDomain, newOIDC.Spec.TrustDomain)
	}

	var validationErrors []string

	// Check SpireServer if it exists and validate field consistency with the new values
	var server v1alpha1.SpireServer
	if err := v.Client.Get(ctx, types.NamespacedName{Name: "cluster"}, &server); err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, fmt.Errorf("validation failed: unable to check for existing SpireServer: %v", err)
		}
		// SpireServer doesn't exist - that's allowed
	} else {
		// SpireServer exists, validate consistency with new OIDC values
		if newOIDC.Spec.TrustDomain != server.Spec.TrustDomain {
			validationErrors = append(validationErrors, fmt.Sprintf("trustDomain %q must match the existing SpireServer trustDomain %q", newOIDC.Spec.TrustDomain, server.Spec.TrustDomain))
		}
		// Validate JwtIssuer consistency with new OIDC values
		serverIssuer := normalizeIssuer(server.Spec.JwtIssuer, server.Spec.TrustDomain)
		newOidcIssuer := normalizeIssuer(newOIDC.Spec.JwtIssuer, newOIDC.Spec.TrustDomain)
		if serverIssuer != newOidcIssuer {
			validationErrors = append(validationErrors, fmt.Sprintf("jwtIssuer %q (normalized: %q) must match the existing SpireServer jwtIssuer (normalized: %q)", newOIDC.Spec.JwtIssuer, newOidcIssuer, serverIssuer))
		}
	}

	// Check SpireAgent if it exists and validate field consistency with the new values
	var agent v1alpha1.SpireAgent
	if err := v.Client.Get(ctx, types.NamespacedName{Name: "cluster"}, &agent); err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, fmt.Errorf("validation failed: unable to check for existing SpireAgent: %v", err)
		}
		// SpireAgent doesn't exist - that's allowed
	} else {
		// SpireAgent exists, validate trustDomain consistency with new OIDC values
		if newOIDC.Spec.TrustDomain != agent.Spec.TrustDomain {
			validationErrors = append(validationErrors, fmt.Sprintf("trustDomain %q must match the existing SpireAgent trustDomain %q", newOIDC.Spec.TrustDomain, agent.Spec.TrustDomain))
		}
	}

	// Return first validation error if any
	if len(validationErrors) > 0 {
		return nil, fmt.Errorf("validation failed: SpireOIDCDiscoveryProvider update %s. Please update the SpireOIDCDiscoveryProvider configuration", validationErrors[0])
	}

	return nil, nil
}

func (v *SpireOIDCDiscoveryProviderValidator) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// Register all validators using the controllers' custom client
func Register(mgr ctrl.Manager) error {
	c, err := customClient.NewCustomClient(mgr)
	if err != nil {
		return err
	}
	if err := ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.SpireServer{}).
		WithValidator(&SpireServerValidator{Client: c}).
		Complete(); err != nil {
		return err
	}
	if err := ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.SpireAgent{}).
		WithValidator(&SpireAgentValidator{Client: c}).
		Complete(); err != nil {
		return err
	}
	if err := ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.SpireOIDCDiscoveryProvider{}).
		WithValidator(&SpireOIDCDiscoveryProviderValidator{Client: c}).
		Complete(); err != nil {
		return err
	}
	return nil
}
