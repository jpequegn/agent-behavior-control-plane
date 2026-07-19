package abcp

import rego.v1

decision := {"decision": "HALT_RUN", "reason_codes": ["global_halt"]} if {
	input.controls.global_halt
} else := {"decision": "BLOCK", "reason_codes": ["untrusted_instruction"]} if {
	not input.trusted_instruction
} else := {"decision": "BLOCK", "reason_codes": ["role_not_authorized"]} if {
	not authorized_actor
} else := {"decision": "BLOCK", "reason_codes": ["tool_or_operation_not_allowed"]} if {
	not catalog_match
} else := {"decision": "BLOCK", "reason_codes": ["autonomy_limit_exceeded"]} if {
	not autonomy_within_limit
} else := {"decision": "BLOCK", "reason_codes": ["subagent_budget_exceeded"]} if {
	not subagent_within_budget
} else := {"decision": "BLOCK", "reason_codes": ["verified_evidence_required"]} if {
	input.proposal.action.tool == "ticket.update"
	not verified_evidence
} else := {"decision": "BLOCK", "reason_codes": ["verified_evidence_required"]} if {
	input.proposal.action.tool == "service.restart"
	not verified_evidence
} else := {"decision": "REQUIRE_APPROVAL", "reason_codes": ["high_risk_restart_requires_approval"]} if {
	input.proposal.action.tool == "service.restart"
	not input.approval_granted
} else := {"decision": "SHADOW", "reason_codes": ["shadow_cohort"]} if {
	input.proposal.rollout.cohort == "shadow"
} else := {"decision": "ALLOW", "reason_codes": ["policy_allow"]}

authorized_actor if {
	input.proposal.actor.role == "incident-agent"
}

catalog_match if {
	input.proposal.action.tool == "metrics.read"
	input.proposal.action.operation == "read"
}

catalog_match if {
	input.proposal.action.tool == "ticket.update"
	input.proposal.action.operation == "write"
}

catalog_match if {
	input.proposal.action.tool == "service.restart"
	input.proposal.action.operation == "write"
}

verified_evidence if {
	input.proposal.evidence.verifier_status == "passed"
	count(input.proposal.evidence.citations) > 0
}

autonomy_rank := {"observe": 0, "recommend": 1, "draft": 2, "act": 3}

autonomy_within_limit if {
	requested := autonomy_rank[input.requested_autonomy]
	maximum := autonomy_rank[input.controls.max_autonomy]
	requested <= maximum
}

subagent_within_budget if {
	input.subagents_requested >= 0
	input.subagents_requested <= input.controls.max_subagents
}
