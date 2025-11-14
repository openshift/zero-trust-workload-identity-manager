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
	// that allows for safe upgrades. It is True when all operands are healthy
	// or not yet created, and CreateOnlyMode is not enabled.
	//   Status:
	//   - True: Safe to upgrade (operands ready or don't exist yet, and no CreateOnlyMode)
	//   - False: Not safe to upgrade (operands exist but failing, or CreateOnlyMode enabled)
	//   Reason:
	//   - Ready: All operands are ready or don't exist
	//   - OperandsNotReady: Some operands exist but are not ready, or CreateOnlyMode is enabled
	Upgradeable string = "Upgradeable"
)

const (
	ReasonFailed           string = "Failed"
	ReasonReady            string = "Ready"
	ReasonInProgress       string = "Progressing"
	ReasonOperandsNotReady string = "OperandsNotReady"
)
