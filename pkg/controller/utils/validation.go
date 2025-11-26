package utils

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/core"
	corevalidation "k8s.io/kubernetes/pkg/apis/core/validation"
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

// ValidateCommonConfigAffinity validates the affinity configuration using Kubernetes validation functions.
func ValidateCommonConfigAffinity(affinity *corev1.Affinity) error {
	if affinity == nil {
		return nil
	}

	internalAffinity := (*core.Affinity)(unsafe.Pointer(affinity))

	opts := corevalidation.PodValidationOptions{}
	fldPath := field.NewPath("affinity")
	errs := ValidateAffinity(internalAffinity, opts, fldPath)

	if len(errs) > 0 {
		return fieldErrorListToError(errs)
	}

	return nil
}

// ValidateCommonConfigTolerations validates tolerations configuration using Kubernetes validation functions.
func ValidateCommonConfigTolerations(tolerations []*corev1.Toleration) error {
	if len(tolerations) == 0 {
		return nil
	}

	internalTolerations := make([]core.Toleration, 0, len(tolerations))
	for _, t := range tolerations {
		if t != nil {
			internalTolerations = append(internalTolerations, *(*core.Toleration)(unsafe.Pointer(t)))
		}
	}

	fldPath := field.NewPath("tolerations")
	errs := corevalidation.ValidateTolerations(internalTolerations, fldPath)

	if len(errs) > 0 {
		return fieldErrorListToError(errs)
	}

	return nil
}

// ValidateCommonConfigNodeSelector validates node selector configuration using Kubernetes validation functions.
func ValidateCommonConfigNodeSelector(nodeSelector map[string]string) error {
	if len(nodeSelector) == 0 {
		return nil
	}

	fldPath := field.NewPath("nodeSelector")
	errs := metav1validation.ValidateLabels(nodeSelector, fldPath)

	if len(errs) > 0 {
		return fieldErrorListToError(errs)
	}

	return nil
}

// ValidateCommonConfigResources validates resource requirements configuration using Kubernetes validation functions.
func ValidateCommonConfigResources(resources *corev1.ResourceRequirements) error {
	if resources == nil {
		return nil
	}

	internalResources := (*core.ResourceRequirements)(unsafe.Pointer(resources))

	fldPath := field.NewPath("resources")
	errs := corevalidation.ValidateContainerResourceRequirements(internalResources, nil, fldPath, corevalidation.PodValidationOptions{})

	if len(errs) > 0 {
		return fieldErrorListToError(errs)
	}

	return nil
}

// ValidateCommonConfigLabels validates labels configuration using Kubernetes validation functions.
func ValidateCommonConfigLabels(labels map[string]string) error {
	if len(labels) == 0 {
		return nil
	}

	// Use Kubernetes public ValidateLabels function
	fldPath := field.NewPath("labels")
	errs := metav1validation.ValidateLabels(labels, fldPath)

	if len(errs) > 0 {
		return fieldErrorListToError(errs)
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

// fieldErrorListToError converts field.ErrorList to a single error
func fieldErrorListToError(errs field.ErrorList) error {
	if len(errs) == 0 {
		return nil
	}
	var errMsgs []string
	for _, err := range errs {
		errMsgs = append(errMsgs, err.Error())
	}
	return fmt.Errorf("%s", strings.Join(errMsgs, "; "))
}
