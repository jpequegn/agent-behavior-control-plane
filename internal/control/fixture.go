package control

func IncidentReadFixture() ProposedAction {
	return ProposedAction{
		Action:   Action{Tool: "metrics.read", Operation: OperationRead, Resource: "demo/INC-42"},
		Actor:    Actor{Role: "incident-agent", Tenant: "demo"},
		Budgets:  Budgets{SpentUSD: 0.12, ToolCalls: 2},
		Evidence: Evidence{VerifierStatus: "passed", Citations: []string{"metric:cpu-7"}},
		Rollout:  Rollout{Cohort: "canary", BehaviorVersion: "v3"},
		RunID:    "run-123",
		Task:     Task{Type: "incident-triage", Risk: RiskLow},
	}
}

func IncidentTicketFixture() ProposedAction {
	proposal := IncidentReadFixture()
	proposal.Action = Action{Tool: "ticket.update", Operation: OperationWrite, Resource: "demo/INC-42"}
	proposal.Task.Risk = RiskMedium
	return proposal
}

func IncidentRestartWithoutEvidenceFixture() ProposedAction {
	proposal := IncidentReadFixture()
	proposal.Action = Action{Tool: "service.restart", Operation: OperationWrite, Resource: "demo/api-gateway"}
	proposal.Evidence = Evidence{VerifierStatus: "missing"}
	proposal.Task.Risk = RiskHigh
	return proposal
}

func IncidentSubagentFixture() ProposedAction {
	proposal := IncidentReadFixture()
	proposal.Action = Action{Tool: "subagent.spawn", Operation: OperationSpawn, Resource: "demo/INC-42"}
	proposal.Task.Risk = RiskMedium
	return proposal
}
