package enforce

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/jpequegn/agent-behavior-control-plane/internal/audit"
	"github.com/jpequegn/agent-behavior-control-plane/internal/control"
	"github.com/jpequegn/agent-behavior-control-plane/internal/flags"
	"github.com/jpequegn/agent-behavior-control-plane/internal/policy"
)

type BudgetLimits struct {
	MaxSpendUSD  float64
	MaxToolCalls int
	MaxSubagents int
}

func DefaultBudgetLimits() BudgetLimits {
	return BudgetLimits{MaxSpendUSD: 5, MaxToolCalls: 10, MaxSubagents: 2}
}

func (b BudgetLimits) Validate() error {
	if b.MaxSpendUSD < 0 || b.MaxToolCalls < 0 || b.MaxSubagents < 0 {
		return errors.New("budget limits cannot be negative")
	}
	return nil
}

func (b BudgetLimits) Exhausted(budgets control.Budgets) bool {
	return budgets.SpentUSD > b.MaxSpendUSD || budgets.ToolCalls >= b.MaxToolCalls
}

type Request struct {
	ApprovalGranted    bool
	Environment        control.Environment
	Proposal           control.ProposedAction
	RequestedAutonomy  control.AutonomyTier
	SubagentsRequested int
	TrustedInstruction bool
}

type Result struct {
	Decision   control.PolicyDecision
	Executed   bool
	Suppressed bool
	ToolOutput string
}

type Engine struct {
	adapters     map[string]toolAdapter
	boundaryHook BoundaryHook
	budgets      BudgetLimits
	catalog      control.ToolCatalog
	flags        *flags.Evaluator
	ledger       *audit.Ledger
	policy       *policy.Evaluator
}

type BoundaryHook interface {
	BeforeBoundary(time.Time)
}

func NewEngine(flagEvaluator *flags.Evaluator, policyEvaluator *policy.Evaluator, ledger *audit.Ledger, catalog control.ToolCatalog, budgets BudgetLimits) (*Engine, error) {
	if flagEvaluator == nil || policyEvaluator == nil || ledger == nil {
		return nil, errors.New("flags, policy, and ledger are required")
	}
	if err := budgets.Validate(); err != nil {
		return nil, err
	}
	adapters := defaultAdapters()
	for _, definition := range catalog.Tools {
		adapter, ok := adapters[definition.Name]
		if !ok || adapter.operation() != definition.Operation {
			return nil, fmt.Errorf("missing controlled adapter for %s", definition.Name)
		}
	}
	return &Engine{
		adapters: adapters,
		budgets:  budgets,
		catalog:  catalog,
		flags:    flagEvaluator,
		ledger:   ledger,
		policy:   policyEvaluator,
	}, nil
}

func (e *Engine) WithBoundaryHook(hook BoundaryHook) *Engine {
	e.boundaryHook = hook
	return e
}

// Execute is the only exported path to synthetic tool side effects. It persists a decision before
// invoking an internal adapter, so a missing audit ledger prevents execution.
func (e *Engine) Execute(ctx context.Context, request Request) (Result, error) {
	if e.boundaryHook != nil {
		e.boundaryHook.BeforeBoundary(time.Now().UTC())
	}
	input := policy.DefaultInput(request.Proposal, "sha256:flags-unavailable")
	input.ApprovalGranted = request.ApprovalGranted
	input.RequestedAutonomy = request.RequestedAutonomy
	input.SubagentsRequested = request.SubagentsRequested
	input.TrustedInstruction = request.TrustedInstruction

	state, flagsErr := e.flags.Evaluate(ctx, request.Proposal, request.Environment)
	var decision control.PolicyDecision
	if flagsErr != nil {
		decision = e.policy.FailClosedDecision(input, "flag_evaluation_failed")
	} else {
		request.Proposal.Rollout.Cohort = string(state.Cohort)
		input.Proposal = request.Proposal
		input.FlagSnapshot = state.FlagSnapshot
		input.Controls = policy.Controls{
			GlobalHalt:   state.GlobalHalt,
			MaxAutonomy:  state.MaxAutonomy,
			MaxSubagents: e.budgets.MaxSubagents,
		}
		decision, _ = e.policy.Evaluate(ctx, input)
		switch {
		case state.PerToolHalt:
			decision = e.policy.FailClosedDecision(input, "tool_halted")
		case state.DisableSubagents && request.Proposal.Action.Operation == control.OperationSpawn:
			decision = e.policy.FailClosedDecision(input, "subagents_disabled")
		case state.DisableMemoryWrite && request.Proposal.Action.Tool == "memory.write":
			decision = e.policy.FailClosedDecision(input, "memory_writes_disabled")
		case e.budgets.Exhausted(request.Proposal.Budgets):
			decision = e.policy.FailClosedDecision(input, "budget_exhausted")
		}
	}

	event := control.NewAuditEvent(request.Proposal, decision)
	if err := e.ledger.Append(ctx, event); err != nil {
		return Result{Decision: decision}, err
	}
	result := Result{Decision: decision, Suppressed: decision.Decision != control.DecisionAllow}
	if result.Suppressed {
		return result, nil
	}
	adapter, ok := e.adapters[request.Proposal.Action.Tool]
	if !ok {
		return result, fmt.Errorf("controlled adapter not found for %s", request.Proposal.Action.Tool)
	}
	output, err := adapter.execute(ctx, request.Proposal)
	if err != nil {
		return result, err
	}
	result.Executed = true
	result.Suppressed = false
	result.ToolOutput = output
	return result, nil
}

type toolAdapter interface {
	execute(context.Context, control.ProposedAction) (string, error)
	operation() control.Operation
}

type syntheticAdapter struct {
	calls      atomic.Int64
	name       string
	toolAction control.Operation
}

func (a *syntheticAdapter) execute(_ context.Context, _ control.ProposedAction) (string, error) {
	a.calls.Add(1)
	return "executed synthetic tool " + a.name, nil
}

func (a *syntheticAdapter) operation() control.Operation {
	return a.toolAction
}

func defaultAdapters() map[string]toolAdapter {
	return map[string]toolAdapter{
		"metrics.read":    &syntheticAdapter{name: "metrics.read", toolAction: control.OperationRead},
		"ticket.update":   &syntheticAdapter{name: "ticket.update", toolAction: control.OperationWrite},
		"service.restart": &syntheticAdapter{name: "service.restart", toolAction: control.OperationWrite},
		"subagent.spawn":  &syntheticAdapter{name: "subagent.spawn", toolAction: control.OperationSpawn},
	}
}
