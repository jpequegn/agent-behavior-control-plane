package control

import (
	"errors"
	"fmt"
	"time"
)

type Decision string

const (
	DecisionAllow           Decision = "ALLOW"
	DecisionShadow          Decision = "SHADOW"
	DecisionRequireApproval Decision = "REQUIRE_APPROVAL"
	DecisionBlock           Decision = "BLOCK"
	DecisionHaltRun         Decision = "HALT_RUN"
)

func (d Decision) Valid() bool {
	switch d {
	case DecisionAllow, DecisionShadow, DecisionRequireApproval, DecisionBlock, DecisionHaltRun:
		return true
	default:
		return false
	}
}

type Risk string

const (
	RiskLow    Risk = "low"
	RiskMedium Risk = "medium"
	RiskHigh   Risk = "high"
)

func (r Risk) Valid() bool {
	return r == RiskLow || r == RiskMedium || r == RiskHigh
}

type Operation string

const (
	OperationRead  Operation = "read"
	OperationWrite Operation = "write"
	OperationSpawn Operation = "spawn"
)

func (o Operation) Valid() bool {
	return o == OperationRead || o == OperationWrite || o == OperationSpawn
}

type AutonomyTier string

const (
	AutonomyObserve   AutonomyTier = "observe"
	AutonomyRecommend AutonomyTier = "recommend"
	AutonomyDraft     AutonomyTier = "draft"
	AutonomyAct       AutonomyTier = "act"
)

func (a AutonomyTier) Valid() bool {
	switch a {
	case AutonomyObserve, AutonomyRecommend, AutonomyDraft, AutonomyAct:
		return true
	default:
		return false
	}
}

type Environment string

const (
	EnvironmentDevelopment Environment = "development"
	EnvironmentProduction  Environment = "production"
)

type FailMode string

const (
	FailOpen   FailMode = "open"
	FailClosed FailMode = "closed"
)

type Actor struct {
	Role   string `json:"role"`
	Tenant string `json:"tenant"`
}

type Task struct {
	Risk Risk   `json:"risk"`
	Type string `json:"type"`
}

type Action struct {
	Operation Operation `json:"operation"`
	Resource  string    `json:"resource"`
	Tool      string    `json:"tool"`
}

type Evidence struct {
	Citations      []string `json:"citations"`
	VerifierStatus string   `json:"verifier_status"`
}

func (e Evidence) Verified() bool {
	return e.VerifierStatus == "passed"
}

type Rollout struct {
	BehaviorVersion string `json:"behavior_version"`
	Cohort          string `json:"cohort"`
}

type Budgets struct {
	SpentUSD  float64 `json:"spent_usd"`
	ToolCalls int     `json:"tool_calls"`
}

type ProposedAction struct {
	Action   Action   `json:"action"`
	Actor    Actor    `json:"actor"`
	Budgets  Budgets  `json:"budgets"`
	Evidence Evidence `json:"evidence"`
	Rollout  Rollout  `json:"rollout"`
	RunID    string   `json:"run_id"`
	Task     Task     `json:"task"`
}

func (p ProposedAction) Validate() error {
	if p.RunID == "" {
		return errors.New("run_id is required")
	}
	if p.Actor.Role == "" || p.Actor.Tenant == "" {
		return errors.New("actor role and tenant are required")
	}
	if p.Task.Type == "" || !p.Task.Risk.Valid() {
		return errors.New("task type and valid risk are required")
	}
	if p.Action.Tool == "" || p.Action.Resource == "" || !p.Action.Operation.Valid() {
		return errors.New("action tool, resource, and valid operation are required")
	}
	if p.Rollout.Cohort == "" || p.Rollout.BehaviorVersion == "" {
		return errors.New("rollout cohort and behavior version are required")
	}
	if p.Budgets.SpentUSD < 0 || p.Budgets.ToolCalls < 0 {
		return errors.New("budgets cannot be negative")
	}
	return nil
}

type PolicyDecision struct {
	Decision      Decision  `json:"decision"`
	DecisionID    string    `json:"decision_id"`
	FlagSnapshot  string    `json:"flag_snapshot"`
	PolicyDigest  string    `json:"policy_digest"`
	PolicyVersion string    `json:"policy_version"`
	ReasonCodes   []string  `json:"reason_codes"`
	RecordedAt    time.Time `json:"recorded_at"`
}

func (d PolicyDecision) Validate() error {
	if !d.Decision.Valid() {
		return fmt.Errorf("invalid decision %q", d.Decision)
	}
	if d.DecisionID == "" || d.PolicyVersion == "" || d.PolicyDigest == "" || d.FlagSnapshot == "" {
		return errors.New("decision_id, policy_version, policy_digest, and flag_snapshot are required")
	}
	if d.RecordedAt.IsZero() {
		return errors.New("recorded_at is required")
	}
	return nil
}

type AuditAction struct {
	Operation Operation `json:"operation"`
	Risk      Risk      `json:"risk"`
	Tool      string    `json:"tool"`
}

type AuditEvidence struct {
	CitationCount int  `json:"citation_count"`
	Verified      bool `json:"verified"`
}

type AuditEvent struct {
	Action        AuditAction    `json:"action"`
	ActorRole     string         `json:"actor_role"`
	AuditEvidence AuditEvidence  `json:"evidence"`
	Cohort        string         `json:"cohort"`
	CorrelationID string         `json:"correlation_id"`
	Decision      PolicyDecision `json:"decision"`
	RunID         string         `json:"run_id"`
}

func NewAuditEvent(request ProposedAction, decision PolicyDecision) AuditEvent {
	return AuditEvent{
		Action: AuditAction{
			Operation: request.Action.Operation,
			Risk:      request.Task.Risk,
			Tool:      request.Action.Tool,
		},
		ActorRole: request.Actor.Role,
		AuditEvidence: AuditEvidence{
			CitationCount: len(request.Evidence.Citations),
			Verified:      request.Evidence.Verified(),
		},
		Cohort:        request.Rollout.Cohort,
		CorrelationID: request.RunID,
		Decision:      decision,
		RunID:         request.RunID,
	}
}
