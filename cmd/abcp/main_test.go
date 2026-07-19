package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jpequegn/agent-behavior-control-plane/internal/audit"
	"github.com/jpequegn/agent-behavior-control-plane/internal/control"
)

func TestRootCommandHelp(t *testing.T) {
	t.Parallel()
	command := newRootCommand()
	output := &bytes.Buffer{}
	command.SetOut(output)
	command.SetArgs([]string{"--help"})

	if err := command.Execute(); err != nil {
		t.Fatalf("execute help: %v", err)
	}
	if !bytes.Contains(output.Bytes(), []byte("serve")) || !bytes.Contains(output.Bytes(), []byte("ledger")) {
		t.Fatalf("help did not include expected commands: %s", output.String())
	}
}

func TestLedgerGetCommand(t *testing.T) {
	databasePath := t.TempDir() + "/audit.db"
	ledger, err := audit.Open(databasePath)
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	event := control.NewAuditEvent(control.IncidentReadFixture(), control.PolicyDecision{
		Decision: control.DecisionAllow, DecisionID: "decision-cli", FlagSnapshot: "sha256:flags",
		PolicyDigest: "sha256:policy", PolicyVersion: "policy-v1", ReasonCodes: []string{"policy_allow"},
		RecordedAt: time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC),
	})
	if err := ledger.Append(context.Background(), event); err != nil {
		t.Fatalf("append event: %v", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatalf("close ledger: %v", err)
	}

	command := newRootCommand()
	output := &bytes.Buffer{}
	command.SetOut(output)
	command.SetArgs([]string{"ledger", "get", "--db", databasePath, "decision-cli"})
	if err := command.Execute(); err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	var got control.AuditEvent
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if got.Decision.DecisionID != "decision-cli" {
		t.Fatalf("unexpected output: %+v", got)
	}
}
