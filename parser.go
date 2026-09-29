package transit

// This file will port lib/src/parser.c. Today it holds the constants of
// lib/src/error_costs.h, which the parser uses most. ERROR_STATE comes with
// the parser.

// The costs of the error recovery.
const (
	// errorCostPerRecovery is ERROR_COST_PER_RECOVERY.
	errorCostPerRecovery = 500
	// errorCostPerMissingTree is ERROR_COST_PER_MISSING_TREE.
	errorCostPerMissingTree = 110
	// errorCostPerSkippedTree is ERROR_COST_PER_SKIPPED_TREE.
	errorCostPerSkippedTree = 100
	// errorCostPerSkippedLine is ERROR_COST_PER_SKIPPED_LINE.
	errorCostPerSkippedLine = 30
	// errorCostPerSkippedChar is ERROR_COST_PER_SKIPPED_CHAR.
	errorCostPerSkippedChar = 1
)
