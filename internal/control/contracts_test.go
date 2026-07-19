package control

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestProposedActionJSONAndValidation(t *testing.T) {
	t.Parallel()
	encoded := `{"run_id":"run-123","actor":{"role":"incident-agent","tenant":"demo"},"task":{"type":"incident-triage","risk":"high"},"action":{"tool":"ticket.update","operation":"write","resource":"INC-42"},"evidence":{"verifier_status":"passed","citations":["metric:cpu-7"]},"rollout":{"cohort":"canary","behavior_version":"v3"},"budgets":{"spent_usd":0.42,"tool_calls":8}}`
	var proposal ProposedAction
	if err := json.Unmarshal([]byte(encoded), &proposal); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := proposal.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestProposedActionValidationErrorsAreStable(t *testing.T) {
	t.Parallel()
	proposal := IncidentReadFixture()
	proposal.RunID = ""
	if err := proposal.Validate(); err == nil || err.Error() != "run_id is required" {
		t.Fatalf("validation error = %v", err)
	}
}

func TestAuditEventSanitizesResourceAndCitations(t *testing.T) {
	t.Parallel()
	proposal := IncidentTicketFixture()
	proposal.Action.Resource = "demo/INC-42?secret=should-not-appear"
	proposal.Evidence.Citations = []string{"metric:cpu-7", "token:should-not-appear"}
	decision := PolicyDecision{
		Decision: DecisionRequireApproval, DecisionID: "dec-456", FlagSnapshot: "sha256:flags",
		PolicyDigest: "sha256:policy", PolicyVersion: "policy-v1",
		RecordedAt: time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC),
	}
	event := NewAuditEvent(proposal, decision)
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal audit event: %v", err)
	}
	if strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), "token:") {
		t.Fatalf("audit event leaked raw request data: %s", encoded)
	}
	if event.AuditEvidence.CitationCount != 2 || !event.AuditEvidence.Verified {
		t.Fatalf("unexpected audit evidence: %+v", event.AuditEvidence)
	}
}
