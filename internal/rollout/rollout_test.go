package rollout

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jpequegn/agent-behavior-control-plane/internal/emergency"
	"github.com/jpequegn/agent-behavior-control-plane/internal/flags"
)

func TestCohortsRemainReproducible(t *testing.T) {
	config := flags.DefaultCohortConfig()
	if config.Assign("demo:run-1") != config.Assign("demo:run-1") {
		t.Fatal("cohort assignment changed")
	}
}

func TestPromotionRequiresSampleAndOperator(t *testing.T) {
	gate := PromotionGate{MinSampleSize: 3, MaxUnsafe: 0}
	report := Summarize(flags.CohortCanary, []Event{{VerifiedSuccess: true}})
	if err := gate.Approve(report, true); err == nil {
		t.Fatal("expected sample-size rejection")
	}
	report = Summarize(flags.CohortCanary, []Event{{}, {}, {}})
	if err := gate.Approve(report, false); err == nil {
		t.Fatal("expected operator-approval rejection")
	}
	if err := gate.Approve(report, true); err != nil {
		t.Fatalf("approved promotion: %v", err)
	}
}

func TestUnsafeEventReducesAutonomyAndCapturesEvidence(t *testing.T) {
	manager, provider := testManager(t)
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	controller := RollbackController{Controls: manager, Expiry: time.Hour, Threshold: 0}
	packet, err := controller.Evaluate("candidate-v2", []Event{{ID: "event-1", UnsafeProposal: true, FlagSnapshot: "sha256:flags", PolicyDigest: "sha256:policy"}}, now)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if packet.Metric != "unsafe_proposals" || packet.TriggeringEvents[0] != "event-1" {
		t.Fatalf("bad evidence: %+v", packet)
	}
	if got := provider.Config().Flags[flags.FlagMaxAutonomy]; got != "draft" {
		t.Fatalf("autonomy = %v, want draft", got)
	}
}

func testManager(t *testing.T) (*emergency.Manager, *flags.LocalProvider) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "config", "flags.json"))
	if err != nil {
		t.Fatal(err)
	}
	config, err := flags.ParseFlagdJSON(content)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := flags.NewLocalProvider(config)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := emergency.NewManager(provider)
	if err != nil {
		t.Fatal(err)
	}
	return manager, provider
}
