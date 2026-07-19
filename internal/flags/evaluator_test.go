package flags

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jpequegn/agent-behavior-control-plane/internal/control"
)

func loadDefaultConfig(t *testing.T) Config {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "config", "flags.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	config, err := ParseFlagdJSON(content)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	return config
}

func TestCohortAssignmentIsStable(t *testing.T) {
	config := DefaultCohortConfig()
	first := config.Assign("demo:run-123")
	for range 20 {
		if got := config.Assign("demo:run-123"); got != first {
			t.Fatalf("cohort = %q, want %q", got, first)
		}
	}
	seen := map[Cohort]bool{}
	for index := range 10_000 {
		seen[config.Assign(fmt.Sprintf("target-%d", index))] = true
	}
	for _, cohort := range []Cohort{CohortControl, CohortCanary, CohortShadow} {
		if !seen[cohort] {
			t.Fatalf("did not assign cohort %q", cohort)
		}
	}
}

func TestEvaluatorRechecksFlagsAtEachBoundary(t *testing.T) {
	config := loadDefaultConfig(t)
	provider, err := NewLocalProvider(config)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	evaluator, err := NewEvaluator(provider, DefaultCohortConfig())
	if err != nil {
		t.Fatalf("new evaluator: %v", err)
	}
	proposal := control.IncidentTicketFixture()
	first, err := evaluator.Evaluate(context.Background(), proposal, control.EnvironmentDevelopment)
	if err != nil {
		t.Fatalf("first boundary: %v", err)
	}
	changed := provider.Config()
	changed.Flags[ToolHaltFlag("ticket.update")] = true
	if err := provider.Update(changed); err != nil {
		t.Fatalf("update flags: %v", err)
	}
	second, err := evaluator.Evaluate(context.Background(), proposal, control.EnvironmentDevelopment)
	if err != nil {
		t.Fatalf("second boundary: %v", err)
	}
	if first.PerToolHalt || !second.PerToolHalt {
		t.Fatalf("flag update not observed at next boundary: first=%t second=%t", first.PerToolHalt, second.PerToolHalt)
	}
	if first.FlagSnapshot == second.FlagSnapshot {
		t.Fatalf("flag snapshot did not change")
	}
}

func TestEvaluatorReadsAllSafetyControls(t *testing.T) {
	config := loadDefaultConfig(t)
	config.Flags[FlagGlobalHalt] = true
	config.Flags[FlagDisableMemoryWrite] = true
	config.Flags[FlagDisableSubagents] = true
	config.Flags[FlagMaxAutonomy] = "draft"
	config.Flags[FlagModelRoute] = "incident-model-fallback"
	provider, err := NewLocalProvider(config)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	evaluator, err := NewEvaluator(provider, DefaultCohortConfig())
	if err != nil {
		t.Fatalf("new evaluator: %v", err)
	}
	state, err := evaluator.Evaluate(context.Background(), control.IncidentRestartWithoutEvidenceFixture(), control.EnvironmentProduction)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !state.GlobalHalt || !state.DisableMemoryWrite || !state.DisableSubagents {
		t.Fatalf("safety controls were not applied: %+v", state)
	}
	if state.MaxAutonomy != control.AutonomyDraft || state.ModelRoute != "incident-model-fallback" {
		t.Fatalf("route controls were not applied: %+v", state)
	}
}

func TestMalformedOrMissingFlagDataFails(t *testing.T) {
	if _, err := ParseFlagdJSON([]byte(`{"flags":`)); err == nil {
		t.Fatal("expected malformed JSON error")
	}
	if _, err := ParseFlagdJSON([]byte(`{"flags":{}}`)); err == nil {
		t.Fatal("expected malformed flag document error")
	}
	provider, err := NewLocalProvider(Config{Flags: map[string]any{FlagGlobalHalt: false}})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	evaluator, err := NewEvaluator(provider, DefaultCohortConfig())
	if err != nil {
		t.Fatalf("new evaluator: %v", err)
	}
	if _, err := evaluator.Evaluate(context.Background(), control.IncidentReadFixture(), control.EnvironmentDevelopment); err == nil {
		t.Fatal("expected missing flag evaluation error")
	}
}
