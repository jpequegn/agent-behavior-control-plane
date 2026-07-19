package policy

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jpequegn/agent-behavior-control-plane/internal/control"
	"github.com/open-policy-agent/opa/rego"
)

const policyQuery = "data.abcp.decision"

//go:embed incident.rego
var embeddedModule string

//go:embed version.txt
var embeddedVersion string

type Controls struct {
	GlobalHalt   bool                 `json:"global_halt"`
	MaxAutonomy  control.AutonomyTier `json:"max_autonomy"`
	MaxSubagents int                  `json:"max_subagents"`
}

type Input struct {
	ApprovalGranted    bool                   `json:"approval_granted"`
	Controls           Controls               `json:"controls"`
	FlagSnapshot       string                 `json:"flag_snapshot"`
	Proposal           control.ProposedAction `json:"proposal"`
	RequestedAutonomy  control.AutonomyTier   `json:"requested_autonomy"`
	SubagentsRequested int                    `json:"subagents_requested"`
	TrustedInstruction bool                   `json:"trusted_instruction"`
}

func DefaultInput(proposal control.ProposedAction, flagSnapshot string) Input {
	return Input{
		Controls: Controls{
			MaxAutonomy:  control.AutonomyAct,
			MaxSubagents: 2,
		},
		FlagSnapshot:       flagSnapshot,
		Proposal:           proposal,
		RequestedAutonomy:  control.AutonomyAct,
		TrustedInstruction: true,
	}
}

func (i Input) Validate() error {
	if err := i.Proposal.Validate(); err != nil {
		return fmt.Errorf("proposal: %w", err)
	}
	if i.FlagSnapshot == "" {
		return errors.New("flag snapshot is required")
	}
	if !i.Controls.MaxAutonomy.Valid() {
		return fmt.Errorf("invalid maximum autonomy %q", i.Controls.MaxAutonomy)
	}
	if !i.RequestedAutonomy.Valid() {
		return fmt.Errorf("invalid requested autonomy %q", i.RequestedAutonomy)
	}
	if i.Controls.MaxSubagents < 0 || i.SubagentsRequested < 0 {
		return errors.New("subagent counts cannot be negative")
	}
	return nil
}

type Evaluator struct {
	digest     string
	prepared   rego.PreparedEvalQuery
	prepareErr error
	version    string
}

func NewEvaluator(ctx context.Context) *Evaluator {
	return NewEvaluatorFromModule(ctx, embeddedModule, strings.TrimSpace(embeddedVersion))
}

func NewEvaluatorFromModule(ctx context.Context, module, version string) *Evaluator {
	digest := policyDigest(module)
	prepared, err := rego.New(
		rego.Query(policyQuery),
		rego.Module("incident.rego", module),
	).PrepareForEval(ctx)
	return &Evaluator{
		digest:     digest,
		prepared:   prepared,
		prepareErr: err,
		version:    strings.TrimSpace(version),
	}
}

func (e *Evaluator) Ready() error {
	if e == nil {
		return errors.New("policy evaluator is nil")
	}
	return e.prepareErr
}

func (e *Evaluator) Digest() string {
	if e == nil {
		return ""
	}
	return e.digest
}

func (e *Evaluator) Version() string {
	if e == nil {
		return ""
	}
	return e.version
}

func (e *Evaluator) Evaluate(ctx context.Context, input Input) (control.PolicyDecision, error) {
	if e == nil {
		return unavailableDecision(input, "policy_unavailable", ""), errors.New("policy evaluator is nil")
	}
	if err := input.Validate(); err != nil {
		return e.decision(input, control.DecisionBlock, []string{"missing_context"}), err
	}
	if e.prepareErr != nil {
		return e.decision(input, control.DecisionBlock, []string{"policy_unavailable"}), e.prepareErr
	}
	results, err := e.prepared.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return e.decision(input, control.DecisionBlock, []string{"policy_evaluation_failed"}), err
	}
	output, err := decodeDecision(results)
	if err != nil {
		return e.decision(input, control.DecisionBlock, []string{"policy_evaluation_failed"}), err
	}
	if !output.Decision.Valid() || len(output.ReasonCodes) == 0 {
		return e.decision(input, control.DecisionBlock, []string{"policy_evaluation_failed"}), errors.New("policy returned an invalid decision")
	}
	return e.decision(input, output.Decision, output.ReasonCodes), nil
}

type policyOutput struct {
	Decision    control.Decision `json:"decision"`
	ReasonCodes []string         `json:"reason_codes"`
}

func decodeDecision(results rego.ResultSet) (policyOutput, error) {
	if len(results) != 1 || len(results[0].Expressions) != 1 {
		return policyOutput{}, errors.New("policy returned no decision")
	}
	encoded, err := json.Marshal(results[0].Expressions[0].Value)
	if err != nil {
		return policyOutput{}, fmt.Errorf("encode policy output: %w", err)
	}
	var output policyOutput
	if err := json.Unmarshal(encoded, &output); err != nil {
		return policyOutput{}, fmt.Errorf("decode policy output: %w", err)
	}
	return output, nil
}

func (e *Evaluator) decision(input Input, decision control.Decision, reasons []string) control.PolicyDecision {
	snapshot := input.FlagSnapshot
	if snapshot == "" {
		snapshot = "sha256:missing"
	}
	version := e.version
	if version == "" {
		version = "policy-unavailable"
	}
	payload, _ := json.Marshal(struct {
		Decision control.Decision `json:"decision"`
		Digest   string           `json:"digest"`
		Reasons  []string         `json:"reasons"`
		RunID    string           `json:"run_id"`
		Snapshot string           `json:"snapshot"`
	}{decision, e.digest, reasons, input.Proposal.RunID, snapshot})
	id := sha256.Sum256(payload)
	return control.PolicyDecision{
		Decision:      decision,
		DecisionID:    "decision-" + hex.EncodeToString(id[:8]),
		FlagSnapshot:  snapshot,
		PolicyDigest:  e.digest,
		PolicyVersion: version,
		ReasonCodes:   append([]string(nil), reasons...),
		RecordedAt:    time.Now().UTC(),
	}
}

func unavailableDecision(input Input, reason, digest string) control.PolicyDecision {
	version := "policy-unavailable"
	if digest == "" {
		digest = policyDigest("")
	}
	evaluator := &Evaluator{digest: digest, version: version}
	return evaluator.decision(input, control.DecisionBlock, []string{reason})
}

func policyDigest(module string) string {
	digest := sha256.Sum256([]byte(module))
	return "sha256:" + hex.EncodeToString(digest[:])
}
