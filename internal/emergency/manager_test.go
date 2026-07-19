package emergency

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jpequegn/agent-behavior-control-plane/internal/control"
	"github.com/jpequegn/agent-behavior-control-plane/internal/flags"
)

func TestManagerValidatesAndExpiresControls(t *testing.T) {
	manager, provider := newTestManager(t)
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	if _, err := manager.Apply(Request{Target: "agent.global_halt", Value: true}, now); err == nil {
		t.Fatal("expected metadata validation error")
	}
	mutation, err := manager.Apply(Request{Target: "agent.global_halt", Value: true, Owner: "oncall", Reason: "incident", ExpiresAt: now.Add(time.Minute)}, now)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	manager.BeforeBoundary(now.Add(25 * time.Millisecond))
	if mutation.ObservedAt != nil || len(manager.Active(now.Add(2*time.Minute))) != 0 {
		t.Fatal("expired control should be removed")
	}
	if provider.Config().Flags[flags.FlagGlobalHalt] != false {
		t.Fatal("expiry should restore base configuration")
	}
}

func TestManagerPropagatesAtNextBoundaryAndIsolatesTools(t *testing.T) {
	manager, provider := newTestManager(t)
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	mutation, err := manager.Apply(Request{Target: flags.ToolHaltFlag("service.restart"), Value: true, Owner: "oncall", Reason: "restart incident", ExpiresAt: now.Add(time.Minute)}, now)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	manager.BeforeBoundary(now.Add(25 * time.Millisecond))
	active := manager.Active(now.Add(25 * time.Millisecond))
	if len(active) != 1 || active[0].ObservedAt == nil || active[0].ObservedAt.Sub(mutation.AppliedAt) > TargetPropagationBudget {
		t.Fatalf("propagation evidence missing: %+v", active)
	}
	evaluator, err := flags.NewEvaluator(provider, flags.DefaultCohortConfig())
	if err != nil {
		t.Fatalf("new evaluator: %v", err)
	}
	restart, err := evaluator.Evaluate(context.Background(), control.IncidentRestartWithoutEvidenceFixture(), control.EnvironmentDevelopment)
	if err != nil || !restart.PerToolHalt {
		t.Fatalf("restart halt not observed: %+v, %v", restart, err)
	}
	read, err := evaluator.Evaluate(context.Background(), control.IncidentReadFixture(), control.EnvironmentDevelopment)
	if err != nil || read.PerToolHalt {
		t.Fatalf("read should remain available: %+v, %v", read, err)
	}
}

func newTestManager(t *testing.T) (*Manager, *flags.LocalProvider) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "config", "flags.json"))
	if err != nil {
		t.Fatalf("read flags: %v", err)
	}
	config, err := flags.ParseFlagdJSON(content)
	if err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	provider, err := flags.NewLocalProvider(config)
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	manager, err := NewManager(provider)
	if err != nil {
		t.Fatalf("manager: %v", err)
	}
	return manager, provider
}
