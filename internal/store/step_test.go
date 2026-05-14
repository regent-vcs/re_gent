package store

import "testing"

func TestNormalizeCauses_UsesCausesAsCanonicalSource(t *testing.T) {
	step := &Step{
		Cause:  Cause{ToolName: "Stale", ToolUseID: "old"},
		Causes: []Cause{{ToolName: "Write", ToolUseID: "new"}},
	}

	step.NormalizeCauses()

	if step.Cause.ToolName != "Write" || step.Cause.ToolUseID != "new" {
		t.Fatalf("legacy cause was not canonicalized from Causes: %#v", step.Cause)
	}
}
