package v1alpha1

const (
	// Degraded is the condition type used to inform state of the operator when
	// it has failed with irrecoverable error like permission issues.
	// DebugEnabled has the following options:
	//   Status:
	//   - True
	//   - False
	//   Reason:
	//   - Failed
	Degraded string = "Degraded"

	// Ready is the condition type used to inform state of readiness of the
	// operator to process spire enabling requests.
	//   Status:
	//   - True
	//   - False
	//   Reason:
	//   - Progressing
	//   - Failed
	//   - Ready: operand successfully deployed and ready
	Ready string = "Ready"

	// Upgradeable indicates whether the operator and operands are in a state
	// that allows for safe upgrades. It is True when all existing operand CRs
	// are ready, and CreateOnlyMode is not enabled. CRs that don't exist yet are OK.
	//   Status:
	//   - True: Safe to upgrade (all existing CRs are ready, CRs that don't exist are OK, and no CreateOnlyMode)
	//   - False: Not safe to upgrade (any existing CR is not ready, or CreateOnlyMode enabled)
	//   Reason:
	//   - Ready: All existing operands are ready or CRs don't exist yet
	//   - OperandsNotReady: Some existing operands are not ready, or CreateOnlyMode is enabled
	Upgradeable string = "Upgradeable"
)

const (
	ReasonFailed           string = "Failed"
	ReasonReady            string = "Ready"
	ReasonInProgress       string = "Progressing"
	ReasonOperandsNotReady string = "OperandsNotReady"
)
