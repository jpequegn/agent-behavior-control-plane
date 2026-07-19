package flags

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/jpequegn/agent-behavior-control-plane/internal/control"
	of "github.com/open-feature/go-sdk/openfeature"
)

type Cohort string

const (
	CohortControl Cohort = "control"
	CohortCanary  Cohort = "canary"
	CohortShadow  Cohort = "shadow"
)

type CohortConfig struct {
	CanaryPercent uint8
	ShadowPercent uint8
}

func DefaultCohortConfig() CohortConfig {
	return CohortConfig{CanaryPercent: 5, ShadowPercent: 10}
}

func (c CohortConfig) Assign(targetingKey string) Cohort {
	digest := sha256.Sum256([]byte(targetingKey))
	slot := binary.BigEndian.Uint64(digest[:8]) % 100
	if slot < uint64(c.CanaryPercent) {
		return CohortCanary
	}
	if slot < uint64(c.CanaryPercent)+uint64(c.ShadowPercent) {
		return CohortShadow
	}
	return CohortControl
}

type BoundaryState struct {
	Cohort             Cohort
	DisableMemoryWrite bool
	DisableSubagents   bool
	FlagSnapshot       string
	GlobalHalt         bool
	MaxAutonomy        control.AutonomyTier
	ModelRoute         string
	PerToolHalt        bool
	TargetingKey       string
}

type Evaluator struct {
	client   *of.Client
	cohorts  CohortConfig
	provider *LocalProvider
}

func NewEvaluator(provider *LocalProvider, cohorts CohortConfig) (*Evaluator, error) {
	if provider == nil {
		return nil, fmt.Errorf("flag provider is required")
	}
	if int(cohorts.CanaryPercent)+int(cohorts.ShadowPercent) > 100 {
		return nil, fmt.Errorf("canary and shadow percentages cannot exceed 100")
	}
	if err := of.SetProviderAndWait(provider); err != nil {
		return nil, fmt.Errorf("set OpenFeature provider: %w", err)
	}
	return &Evaluator{
		client:   of.NewClient("agent-behavior-control-plane"),
		cohorts:  cohorts,
		provider: provider,
	}, nil
}

func (e *Evaluator) Evaluate(ctx context.Context, proposal control.ProposedAction, environment control.Environment) (BoundaryState, error) {
	if err := proposal.Validate(); err != nil {
		return BoundaryState{}, err
	}
	targetingKey := proposal.Actor.Tenant + ":" + proposal.RunID
	evaluationContext := of.NewEvaluationContext(targetingKey, map[string]any{
		"actor_role":  proposal.Actor.Role,
		"cohort":      string(e.cohorts.Assign(targetingKey)),
		"environment": string(environment),
		"risk":        string(proposal.Task.Risk),
		"run_id":      proposal.RunID,
		"tenant":      proposal.Actor.Tenant,
		"tool":        proposal.Action.Tool,
	})
	globalHalt, err := e.client.BooleanValue(ctx, FlagGlobalHalt, false, evaluationContext)
	if err != nil {
		return BoundaryState{}, fmt.Errorf("evaluate global halt: %w", err)
	}
	perToolHalt, err := e.client.BooleanValue(ctx, ToolHaltFlag(proposal.Action.Tool), false, evaluationContext)
	if err != nil {
		return BoundaryState{}, fmt.Errorf("evaluate per-tool halt: %w", err)
	}
	disableMemoryWrite, err := e.client.BooleanValue(ctx, FlagDisableMemoryWrite, false, evaluationContext)
	if err != nil {
		return BoundaryState{}, fmt.Errorf("evaluate memory-write control: %w", err)
	}
	disableSubagents, err := e.client.BooleanValue(ctx, FlagDisableSubagents, false, evaluationContext)
	if err != nil {
		return BoundaryState{}, fmt.Errorf("evaluate sub-agent control: %w", err)
	}
	maxAutonomy, err := e.client.StringValue(ctx, FlagMaxAutonomy, string(control.AutonomyObserve), evaluationContext)
	if err != nil {
		return BoundaryState{}, fmt.Errorf("evaluate autonomy control: %w", err)
	}
	modelRoute, err := e.client.StringValue(ctx, FlagModelRoute, "fallback", evaluationContext)
	if err != nil {
		return BoundaryState{}, fmt.Errorf("evaluate model route: %w", err)
	}
	state := BoundaryState{
		Cohort:             e.cohorts.Assign(targetingKey),
		DisableMemoryWrite: disableMemoryWrite,
		DisableSubagents:   disableSubagents,
		FlagSnapshot:       e.provider.Config().Snapshot(),
		GlobalHalt:         globalHalt,
		MaxAutonomy:        control.AutonomyTier(maxAutonomy),
		ModelRoute:         modelRoute,
		PerToolHalt:        perToolHalt,
		TargetingKey:       targetingKey,
	}
	if !state.MaxAutonomy.Valid() {
		return BoundaryState{}, fmt.Errorf("flag %s returned invalid autonomy tier %q", FlagMaxAutonomy, maxAutonomy)
	}
	return state, nil
}
