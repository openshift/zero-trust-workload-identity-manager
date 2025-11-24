/*
Copyright 2025.

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

package utils

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StatusManager is an interface that defines methods needed for status management
type StatusManager interface {
	AddCondition(conditionType, reason, message string, status metav1.ConditionStatus)
}

// ValidationResult represents the result of a validation operation
type ValidationResult struct {
	FieldName      string
	ConditionType  string
	ConditionValue string
	ErrorMessage   string
	Error          error
}

// ValidateAndUpdateStatus validates common configuration and updates status manager
func ValidateAndUpdateStatus(
	logger logr.Logger,
	statusMgr StatusManager,
	resourceKind string,
	resourceName string,
	affinity *corev1.Affinity,
	tolerations []*corev1.Toleration,
	nodeSelector map[string]string,
	resources *corev1.ResourceRequirements,
	labels map[string]string,
) error {
	validationResults := ValidateCommonConfigWithDetails(affinity, tolerations, nodeSelector, resources, labels)

	if len(validationResults) > 0 {
		// Log and add status conditions for each validation failure
		for _, result := range validationResults {
			logger.Error(result.Error, fmt.Sprintf("%s validation failed", result.FieldName), "name", resourceName)
			statusMgr.AddCondition(result.ConditionType, result.ConditionValue, result.ErrorMessage, metav1.ConditionFalse)
		}
		return fmt.Errorf("%s/%s validation failed: %w", resourceKind, resourceName, validationResults[0].Error)
	}

	return nil
}

// ValidateCommonConfigWithDetails validates common configuration fields and returns detailed error information
func ValidateCommonConfigWithDetails(affinity *corev1.Affinity, tolerations []*corev1.Toleration, nodeSelector map[string]string, resources *corev1.ResourceRequirements, labels map[string]string) []ValidationResult {
	var results []ValidationResult

	// Validate affinity
	if err := ValidateCommonConfigAffinity(affinity); err != nil {
		results = append(results, ValidationResult{
			FieldName:      "affinity",
			ConditionType:  ConditionTypeConfigurationValid,
			ConditionValue: ConditionReasonInvalidAffinity,
			ErrorMessage:   fmt.Sprintf("Affinity validation failed: %v", err),
			Error:          err,
		})
	}

	// Validate tolerations
	if err := ValidateCommonConfigTolerations(tolerations); err != nil {
		results = append(results, ValidationResult{
			FieldName:      "tolerations",
			ConditionType:  ConditionTypeConfigurationValid,
			ConditionValue: ConditionReasonInvalidTolerations,
			ErrorMessage:   fmt.Sprintf("Tolerations validation failed: %v", err),
			Error:          err,
		})
	}

	// Validate node selector
	if err := ValidateCommonConfigNodeSelector(nodeSelector); err != nil {
		results = append(results, ValidationResult{
			FieldName:      "nodeSelector",
			ConditionType:  ConditionTypeConfigurationValid,
			ConditionValue: ConditionReasonInvalidNodeSelector,
			ErrorMessage:   fmt.Sprintf("NodeSelector validation failed: %v", err),
			Error:          err,
		})
	}

	// Validate resources
	if err := ValidateCommonConfigResources(resources); err != nil {
		results = append(results, ValidationResult{
			FieldName:      "resources",
			ConditionType:  ConditionTypeConfigurationValid,
			ConditionValue: ConditionReasonInvalidResources,
			ErrorMessage:   fmt.Sprintf("Resources validation failed: %v", err),
			Error:          err,
		})
	}

	// Validate labels
	if err := ValidateCommonConfigLabels(labels); err != nil {
		results = append(results, ValidationResult{
			FieldName:      "labels",
			ConditionType:  ConditionTypeConfigurationValid,
			ConditionValue: ConditionReasonInvalidLabels,
			ErrorMessage:   fmt.Sprintf("Labels validation failed: %v", err),
			Error:          err,
		})
	}

	return results
}

// ValidateCommonConfigAffinity validates the affinity configuration
// This includes node affinity, pod affinity, and pod anti-affinity
func ValidateCommonConfigAffinity(affinity *corev1.Affinity) error {
	if affinity == nil {
		return nil
	}

	// Validate Node Affinity
	if err := validateNodeAffinity(affinity.NodeAffinity); err != nil {
		return fmt.Errorf("invalid node affinity: %w", err)
	}

	// Validate Pod Affinity
	if err := validatePodAffinity(affinity.PodAffinity); err != nil {
		return fmt.Errorf("invalid pod affinity: %w", err)
	}

	// Validate Pod Anti-Affinity
	if err := validatePodAntiAffinity(affinity.PodAntiAffinity); err != nil {
		return fmt.Errorf("invalid pod anti-affinity: %w", err)
	}

	return nil
}

// validateNodeAffinity validates node affinity configuration
func validateNodeAffinity(nodeAffinity *corev1.NodeAffinity) error {
	if nodeAffinity == nil {
		return nil
	}

	// Validate required during scheduling ignored during execution
	if nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		if err := validateNodeSelector(nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution); err != nil {
			return fmt.Errorf("invalid required node selector: %w", err)
		}
	}

	// Validate preferred during scheduling ignored during execution
	if len(nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
		for i, term := range nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			if term.Weight < 1 || term.Weight > 100 {
				return fmt.Errorf("preferred node affinity term %d has invalid weight %d, must be between 1 and 100", i, term.Weight)
			}
			if err := validateNodeSelectorTerm(&term.Preference); err != nil {
				return fmt.Errorf("invalid preferred node selector term %d: %w", i, err)
			}
		}
	}

	return nil
}

// validateNodeSelector validates a node selector
func validateNodeSelector(nodeSelector *corev1.NodeSelector) error {
	if nodeSelector == nil {
		return nil
	}

	if len(nodeSelector.NodeSelectorTerms) == 0 {
		return fmt.Errorf("node selector must have at least one term")
	}

	for i, term := range nodeSelector.NodeSelectorTerms {
		if err := validateNodeSelectorTerm(&term); err != nil {
			return fmt.Errorf("invalid node selector term %d: %w", i, err)
		}
	}

	return nil
}

// validateNodeSelectorTerm validates a single node selector term
func validateNodeSelectorTerm(term *corev1.NodeSelectorTerm) error {
	if term == nil {
		return nil
	}

	// At least one of MatchExpressions or MatchFields must be non-empty
	if len(term.MatchExpressions) == 0 && len(term.MatchFields) == 0 {
		return fmt.Errorf("node selector term must have at least one match expression or match field")
	}

	// Validate match expressions
	for i, expr := range term.MatchExpressions {
		if err := validateNodeSelectorRequirement(&expr); err != nil {
			return fmt.Errorf("invalid match expression %d: %w", i, err)
		}
	}

	// Validate match fields
	for i, field := range term.MatchFields {
		if err := validateNodeSelectorRequirement(&field); err != nil {
			return fmt.Errorf("invalid match field %d: %w", i, err)
		}
	}

	return nil
}

// validateNodeSelectorRequirement validates a node selector requirement
func validateNodeSelectorRequirement(req *corev1.NodeSelectorRequirement) error {
	if req == nil {
		return nil
	}

	if req.Key == "" {
		return fmt.Errorf("node selector requirement key cannot be empty")
	}

	// Validate operator
	validOperators := map[corev1.NodeSelectorOperator]bool{
		corev1.NodeSelectorOpIn:           true,
		corev1.NodeSelectorOpNotIn:        true,
		corev1.NodeSelectorOpExists:       true,
		corev1.NodeSelectorOpDoesNotExist: true,
		corev1.NodeSelectorOpGt:           true,
		corev1.NodeSelectorOpLt:           true,
	}

	if !validOperators[req.Operator] {
		return fmt.Errorf("invalid node selector operator %q", req.Operator)
	}

	// Validate values based on operator
	switch req.Operator {
	case corev1.NodeSelectorOpIn, corev1.NodeSelectorOpNotIn:
		if len(req.Values) == 0 {
			return fmt.Errorf("node selector requirement with operator %q must have at least one value", req.Operator)
		}
	case corev1.NodeSelectorOpExists, corev1.NodeSelectorOpDoesNotExist:
		if len(req.Values) > 0 {
			return fmt.Errorf("node selector requirement with operator %q must not have values", req.Operator)
		}
	case corev1.NodeSelectorOpGt, corev1.NodeSelectorOpLt:
		if len(req.Values) != 1 {
			return fmt.Errorf("node selector requirement with operator %q must have exactly one value", req.Operator)
		}
	}

	return nil
}

// validatePodAffinity validates pod affinity configuration
func validatePodAffinity(podAffinity *corev1.PodAffinity) error {
	if podAffinity == nil {
		return nil
	}

	// Validate required during scheduling ignored during execution
	for i, term := range podAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
		if err := validatePodAffinityTerm(&term); err != nil {
			return fmt.Errorf("invalid required pod affinity term %d: %w", i, err)
		}
	}

	// Validate preferred during scheduling ignored during execution
	for i, term := range podAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
		if term.Weight < 1 || term.Weight > 100 {
			return fmt.Errorf("preferred pod affinity term %d has invalid weight %d, must be between 1 and 100", i, term.Weight)
		}
		if err := validatePodAffinityTerm(&term.PodAffinityTerm); err != nil {
			return fmt.Errorf("invalid preferred pod affinity term %d: %w", i, err)
		}
	}

	return nil
}

// validatePodAntiAffinity validates pod anti-affinity configuration
func validatePodAntiAffinity(podAntiAffinity *corev1.PodAntiAffinity) error {
	if podAntiAffinity == nil {
		return nil
	}

	// Validate required during scheduling ignored during execution
	for i, term := range podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
		if err := validatePodAffinityTerm(&term); err != nil {
			return fmt.Errorf("invalid required pod anti-affinity term %d: %w", i, err)
		}
	}

	// Validate preferred during scheduling ignored during execution
	for i, term := range podAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
		if term.Weight < 1 || term.Weight > 100 {
			return fmt.Errorf("preferred pod anti-affinity term %d has invalid weight %d, must be between 1 and 100", i, term.Weight)
		}
		if err := validatePodAffinityTerm(&term.PodAffinityTerm); err != nil {
			return fmt.Errorf("invalid preferred pod anti-affinity term %d: %w", i, err)
		}
	}

	return nil
}

// validatePodAffinityTerm validates a pod affinity term
func validatePodAffinityTerm(term *corev1.PodAffinityTerm) error {
	if term == nil {
		return nil
	}

	// Topology key is required
	if term.TopologyKey == "" {
		return fmt.Errorf("topology key cannot be empty")
	}

	// Validate label selector
	if term.LabelSelector != nil {
		if err := validateLabelSelector(term.LabelSelector); err != nil {
			return fmt.Errorf("invalid label selector: %w", err)
		}
	}

	// Validate namespace selector
	if term.NamespaceSelector != nil {
		if err := validateLabelSelector(term.NamespaceSelector); err != nil {
			return fmt.Errorf("invalid namespace selector: %w", err)
		}
	}

	return nil
}

// validateLabelSelector validates a label selector
func validateLabelSelector(selector *metav1.LabelSelector) error {
	if selector == nil {
		return nil
	}

	// Validate match labels
	for key := range selector.MatchLabels {
		if key == "" {
			return fmt.Errorf("label selector match label key cannot be empty")
		}
	}

	// Validate match expressions
	for i, expr := range selector.MatchExpressions {
		if err := validateLabelSelectorRequirement(&expr); err != nil {
			return fmt.Errorf("invalid label selector match expression %d: %w", i, err)
		}
	}

	return nil
}

// validateLabelSelectorRequirement validates a label selector requirement
func validateLabelSelectorRequirement(req *metav1.LabelSelectorRequirement) error {
	if req == nil {
		return nil
	}

	if req.Key == "" {
		return fmt.Errorf("label selector requirement key cannot be empty")
	}

	// Validate operator
	validOperators := map[metav1.LabelSelectorOperator]bool{
		metav1.LabelSelectorOpIn:           true,
		metav1.LabelSelectorOpNotIn:        true,
		metav1.LabelSelectorOpExists:       true,
		metav1.LabelSelectorOpDoesNotExist: true,
	}

	if !validOperators[req.Operator] {
		return fmt.Errorf("invalid label selector operator %q", req.Operator)
	}

	// Validate values based on operator
	switch req.Operator {
	case metav1.LabelSelectorOpIn, metav1.LabelSelectorOpNotIn:
		if len(req.Values) == 0 {
			return fmt.Errorf("label selector requirement with operator %q must have at least one value", req.Operator)
		}
	case metav1.LabelSelectorOpExists, metav1.LabelSelectorOpDoesNotExist:
		if len(req.Values) > 0 {
			return fmt.Errorf("label selector requirement with operator %q must not have values", req.Operator)
		}
	}

	return nil
}

// ValidateCommonConfigTolerations validates tolerations configuration
func ValidateCommonConfigTolerations(tolerations []*corev1.Toleration) error {
	if len(tolerations) == 0 {
		return nil
	}

	for i, toleration := range tolerations {
		if toleration == nil {
			continue
		}

		// Validate operator
		validOperators := map[corev1.TolerationOperator]bool{
			corev1.TolerationOpEqual:  true,
			corev1.TolerationOpExists: true,
		}

		if toleration.Operator != "" && !validOperators[toleration.Operator] {
			return fmt.Errorf("toleration %d has invalid operator %q", i, toleration.Operator)
		}

		// If operator is Equal (or empty, which defaults to Equal), value must be present
		if (toleration.Operator == "" || toleration.Operator == corev1.TolerationOpEqual) && toleration.Value == "" && toleration.Key != "" {
			// Empty value is allowed only if key is also empty (matches all)
		}

		// Validate effect if present
		if toleration.Effect != "" {
			validEffects := map[corev1.TaintEffect]bool{
				corev1.TaintEffectNoSchedule:       true,
				corev1.TaintEffectPreferNoSchedule: true,
				corev1.TaintEffectNoExecute:        true,
			}

			if !validEffects[toleration.Effect] {
				return fmt.Errorf("toleration %d has invalid effect %q", i, toleration.Effect)
			}
		}

		// TolerationSeconds is only valid for NoExecute effect
		if toleration.TolerationSeconds != nil && toleration.Effect != corev1.TaintEffectNoExecute {
			return fmt.Errorf("toleration %d has TolerationSeconds but effect is not NoExecute", i)
		}
	}

	return nil
}

// ValidateCommonConfigNodeSelector validates node selector configuration
func ValidateCommonConfigNodeSelector(nodeSelector map[string]string) error {
	if len(nodeSelector) == 0 {
		return nil
	}

	var invalidKeys []string
	for key, value := range nodeSelector {
		if key == "" {
			invalidKeys = append(invalidKeys, "(empty key)")
		}
		// Note: Empty values are valid in Kubernetes nodeSelector (e.g., node-role.kubernetes.io/control-plane: "")
		// Only the key needs to be non-empty
		_ = value // Explicitly ignore value validation
	}

	if len(invalidKeys) > 0 {
		return fmt.Errorf("node selector has invalid entries: %s", strings.Join(invalidKeys, ", "))
	}

	return nil
}

// ValidateCommonConfigResources validates resource requirements configuration
func ValidateCommonConfigResources(resources *corev1.ResourceRequirements) error {
	if resources == nil {
		return nil
	}

	// Validate that limits are not less than requests
	if resources.Limits != nil && resources.Requests != nil {
		for resourceName, limitValue := range resources.Limits {
			if requestValue, exists := resources.Requests[resourceName]; exists {
				if limitValue.Cmp(requestValue) < 0 {
					return fmt.Errorf("resource %q limit (%s) is less than request (%s)", resourceName, limitValue.String(), requestValue.String())
				}
			}
		}
	}

	return nil
}

// ValidateCommonConfigLabels validates labels configuration
func ValidateCommonConfigLabels(labels map[string]string) error {
	if len(labels) == 0 {
		return nil
	}

	var invalidLabels []string
	for key, value := range labels {
		if key == "" {
			invalidLabels = append(invalidLabels, "(empty key)")
			continue
		}

		// Kubernetes label key validation (simplified)
		// Keys can have optional prefix separated by /
		parts := strings.Split(key, "/")
		if len(parts) > 2 {
			invalidLabels = append(invalidLabels, fmt.Sprintf("%q (too many / separators)", key))
			continue
		}

		// Label values have a max length of 63 characters
		if len(value) > 63 {
			invalidLabels = append(invalidLabels, fmt.Sprintf("%q (value too long: %d > 63)", key, len(value)))
		}

		// Label key name part (after optional prefix) has max length of 63
		keyName := key
		if len(parts) == 2 {
			keyName = parts[1]
		}
		if len(keyName) > 63 {
			invalidLabels = append(invalidLabels, fmt.Sprintf("%q (key name too long: %d > 63)", key, len(keyName)))
		}
	}

	if len(invalidLabels) > 0 {
		return fmt.Errorf("labels have invalid entries: %s", strings.Join(invalidLabels, ", "))
	}

	return nil
}

// ValidateCommonConfig validates all common configuration fields
func ValidateCommonConfig(affinity *corev1.Affinity, tolerations []*corev1.Toleration, nodeSelector map[string]string, resources *corev1.ResourceRequirements, labels map[string]string) error {
	// Validate affinity
	if err := ValidateCommonConfigAffinity(affinity); err != nil {
		return fmt.Errorf("affinity validation failed: %w", err)
	}

	// Validate tolerations
	if err := ValidateCommonConfigTolerations(tolerations); err != nil {
		return fmt.Errorf("tolerations validation failed: %w", err)
	}

	// Validate node selector
	if err := ValidateCommonConfigNodeSelector(nodeSelector); err != nil {
		return fmt.Errorf("node selector validation failed: %w", err)
	}

	// Validate resources
	if err := ValidateCommonConfigResources(resources); err != nil {
		return fmt.Errorf("resources validation failed: %w", err)
	}

	// Validate labels
	if err := ValidateCommonConfigLabels(labels); err != nil {
		return fmt.Errorf("labels validation failed: %w", err)
	}

	return nil
}
