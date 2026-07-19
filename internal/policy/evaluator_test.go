package policy

import (
	"context"
	"strings"
	"testing"

	"github.com/jpequegn/agent-behavior-control-plane/internal/control"
)

func TestPolicyDecisions(t *testing.T) {
	evaluator := NewEvaluator(context.Background())
	if err := evaluator.Ready(); err != nil {
		t.Fatalf("compile embedded policy: %v", err)
	}
	tests := []struct {
		name  string
		input Input
		want  control.Decision
	}{
		{name: "allows supported read", input: DefaultInput(control.IncidentReadFixture(), "sha256:flags"), want: control.DecisionAllow},
		{name: "shadows shadow cohort", input: shadowInput(), want: control.DecisionShadow},
		{name: "requires restart approval", input: restartInput(false), want: control.DecisionRequireApproval},
		{name: "blocks ticket without verified evidence", input: ticketWithoutEvidence(), want: control.DecisionBlock},
		{name: "halts global switch", input: haltedInput(), want: control.DecisionHaltRun},
		{name: "blocks untrusted instruction", input: untrustedInput(), want: control.DecisionBlock},
		{name: "blocks autonomy over maximum", input: autonomyExceededInput(), want: control.DecisionBlock},
		{name: "blocks subagent budget overage", input: subagentBudgetExceededInput(), want: control.DecisionBlock},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := evaluator.Evaluate(context.Background(), test.input)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if got.Decision != test.want {
				t.Fatalf("decision = %q, want %q (%+v)", got.Decision, test.want, got)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("decision validation: %v", err)
			}
			if !strings.HasPrefix(got.PolicyDigest, "sha256:") || got.FlagSnapshot != "sha256:flags" {
				t.Fatalf("missing reproducibility metadata: %+v", got)
			}
		})
	}
}

func TestPolicyCompileFailureFailsClosed(t *testing.T) {
	evaluator := NewEvaluatorFromModule(context.Background(), "package abcp\ndecision :=", "broken-policy")
	if evaluator.Ready() == nil {
		t.Fatal("expected compile failure")
	}
	input := restartInput(true)
	got, err := evaluator.Evaluate(context.Background(), input)
	if err == nil {
		t.Fatal("expected policy preparation error")
	}
	if got.Decision != control.DecisionBlock || got.ReasonCodes[0] != "policy_unavailable" {
		t.Fatalf("compile failure must block consequential action: %+v", got)
	}
}

func TestPolicyMissingContextFailsClosed(t *testing.T) {
	evaluator := NewEvaluator(context.Background())
	input := DefaultInput(control.IncidentTicketFixture(), "sha256:flags")
	input.RequestedAutonomy = ""
	got, err := evaluator.Evaluate(context.Background(), input)
	if err == nil {
		t.Fatal("expected missing-context error")
	}
	if got.Decision != control.DecisionBlock || got.ReasonCodes[0] != "missing_context" {
		t.Fatalf("missing context must block: %+v", got)
	}
}

func shadowInput() Input {
	proposal := control.IncidentReadFixture()
	proposal.Rollout.Cohort = "shadow"
	return DefaultInput(proposal, "sha256:flags")
}

func restartInput(approved bool) Input {
	proposal := control.IncidentRestartWithoutEvidenceFixture()
	proposal.Evidence = control.Evidence{VerifierStatus: "passed", Citations: []string{"metric:cpu-7"}}
	input := DefaultInput(proposal, "sha256:flags")
	input.ApprovalGranted = approved
	return input
}

func ticketWithoutEvidence() Input {
	proposal := control.IncidentTicketFixture()
	proposal.Evidence = control.Evidence{VerifierStatus: "missing"}
	return DefaultInput(proposal, "sha256:flags")
}

func haltedInput() Input {
	input := DefaultInput(control.IncidentReadFixture(), "sha256:flags")
	input.Controls.GlobalHalt = true
	return input
}

func untrustedInput() Input {
	input := DefaultInput(control.IncidentReadFixture(), "sha256:flags")
	input.TrustedInstruction = false
	return input
}

func autonomyExceededInput() Input {
	input := DefaultInput(control.IncidentReadFixture(), "sha256:flags")
	input.Controls.MaxAutonomy = control.AutonomyDraft
	return input
}

func subagentBudgetExceededInput() Input {
	input := DefaultInput(control.IncidentReadFixture(), "sha256:flags")
	input.SubagentsRequested = 3
	return input
}
