package audit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jpequegn/agent-behavior-control-plane/internal/control"
)

func TestLedgerAppendGetAndRejectDuplicate(t *testing.T) {
	ledger, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	t.Cleanup(func() { _ = ledger.Close() })
	event := control.NewAuditEvent(control.IncidentTicketFixture(), control.PolicyDecision{
		Decision: control.DecisionAllow, DecisionID: "decision-1", FlagSnapshot: "sha256:flags",
		PolicyDigest: "sha256:policy", PolicyVersion: "policy-v1", ReasonCodes: []string{"policy_allow"},
		RecordedAt: time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC),
	})
	if err := ledger.Append(context.Background(), event); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := ledger.Append(context.Background(), event); !errors.Is(err, ErrDuplicateDecision) {
		t.Fatalf("duplicate error = %v, want %v", err, ErrDuplicateDecision)
	}
	got, err := ledger.Get(context.Background(), "decision-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Decision.DecisionID != event.Decision.DecisionID || got.Action.Tool != "ticket.update" {
		t.Fatalf("unexpected event: %+v", got)
	}
	if _, err := ledger.db.ExecContext(context.Background(), `DELETE FROM audit_events WHERE decision_id = ?`, "decision-1"); err == nil {
		t.Fatal("expected append-only trigger to reject delete")
	}
}
