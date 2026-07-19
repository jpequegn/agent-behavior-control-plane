package enforce

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jpequegn/agent-behavior-control-plane/internal/audit"
	"github.com/jpequegn/agent-behavior-control-plane/internal/control"
	"github.com/jpequegn/agent-behavior-control-plane/internal/flags"
	"github.com/jpequegn/agent-behavior-control-plane/internal/policy"
)

func TestEngineRecordsAndGatesToolCalls(t *testing.T) {
	tests := []struct {
		name      string
		cohorts   flags.CohortConfig
		configure func(*flags.LocalProvider)
		request   Request
		want      control.Decision
		executed  bool
	}{
		{name: "allowed read executes", request: requestFor(control.IncidentReadFixture()), want: control.DecisionAllow, executed: true},
		{name: "ticket without evidence blocks", request: requestFor(ticketWithoutEvidence()), want: control.DecisionBlock},
		{name: "restart requires approval", request: requestFor(restartWithEvidence()), want: control.DecisionRequireApproval},
		{name: "global halt stops run", configure: setGlobalHalt, request: requestFor(control.IncidentReadFixture()), want: control.DecisionHaltRun},
		{name: "shadow suppresses side effect", cohorts: flags.CohortConfig{ShadowPercent: 100}, request: shadowRequest(), want: control.DecisionShadow},
		{name: "subagent halt suppresses spawn", configure: setSubagentHalt, request: requestFor(control.IncidentSubagentFixture()), want: control.DecisionBlock},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cohorts := test.cohorts
			if cohorts == (flags.CohortConfig{}) {
				cohorts = flags.DefaultCohortConfig()
			}
			engine, ledger, provider := newTestEngine(t, cohorts)
			if test.configure != nil {
				test.configure(provider)
			}
			result, err := engine.Execute(context.Background(), test.request)
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if result.Decision.Decision != test.want || result.Executed != test.executed {
				t.Fatalf("result = %+v, want decision %q executed=%t", result, test.want, test.executed)
			}
			event, err := ledger.Get(context.Background(), result.Decision.DecisionID)
			if err != nil {
				t.Fatalf("read persisted decision: %v", err)
			}
			if event.Decision.Decision != test.want || event.Action.Tool != test.request.Proposal.Action.Tool {
				t.Fatalf("unexpected sanitized event: %+v", event)
			}
		})
	}
}

func TestEngineRejectsDuplicateRecordBeforeSecondExecution(t *testing.T) {
	engine, _, _ := newTestEngine(t, flags.DefaultCohortConfig())
	request := requestFor(control.IncidentReadFixture())
	first, err := engine.Execute(context.Background(), request)
	if err != nil || !first.Executed {
		t.Fatalf("first execution = %+v, %v", first, err)
	}
	second, err := engine.Execute(context.Background(), request)
	if !errors.Is(err, audit.ErrDuplicateDecision) || second.Executed {
		t.Fatalf("duplicate execution = %+v, %v", second, err)
	}
	adapter := engine.adapters["metrics.read"].(*syntheticAdapter)
	if got := adapter.calls.Load(); got != 1 {
		t.Fatalf("adapter calls = %d, want 1", got)
	}
}

func newTestEngine(t *testing.T, cohorts flags.CohortConfig) (*Engine, *audit.Ledger, *flags.LocalProvider) {
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
		t.Fatalf("new provider: %v", err)
	}
	flagEvaluator, err := flags.NewEvaluator(provider, cohorts)
	if err != nil {
		t.Fatalf("new flags evaluator: %v", err)
	}
	ledger, err := audit.Open(":memory:")
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	t.Cleanup(func() { _ = ledger.Close() })
	engine, err := NewEngine(flagEvaluator, policy.NewEvaluator(context.Background()), ledger, control.DefaultIncidentToolCatalog(), DefaultBudgetLimits())
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return engine, ledger, provider
}

func requestFor(proposal control.ProposedAction) Request {
	return Request{
		Environment:        control.EnvironmentDevelopment,
		Proposal:           proposal,
		RequestedAutonomy:  control.AutonomyAct,
		TrustedInstruction: true,
	}
}

func ticketWithoutEvidence() control.ProposedAction {
	proposal := control.IncidentTicketFixture()
	proposal.Evidence = control.Evidence{VerifierStatus: "missing"}
	return proposal
}

func restartWithEvidence() control.ProposedAction {
	proposal := control.IncidentRestartWithoutEvidenceFixture()
	proposal.Evidence = control.Evidence{VerifierStatus: "passed", Citations: []string{"metric:cpu-7"}}
	return proposal
}

func shadowRequest() Request {
	request := requestFor(control.IncidentReadFixture())
	return request
}

func setGlobalHalt(provider *flags.LocalProvider) {
	config := provider.Config()
	config.Flags[flags.FlagGlobalHalt] = true
	if err := provider.Update(config); err != nil {
		panic(err)
	}
}

func setSubagentHalt(provider *flags.LocalProvider) {
	config := provider.Config()
	config.Flags[flags.ToolHaltFlag("subagent.spawn")] = true
	if err := provider.Update(config); err != nil {
		panic(err)
	}
}
