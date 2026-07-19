package rollout

import (
	"errors"
	"fmt"
	"time"

	"github.com/jpequegn/agent-behavior-control-plane/internal/emergency"
	"github.com/jpequegn/agent-behavior-control-plane/internal/flags"
)

type Event struct {
	ApprovalRequired bool
	BehaviorVersion  string
	CostUSD          float64
	Cohort           flags.Cohort
	DecisionID       string
	FlagSnapshot     string
	ID               string
	Latency          time.Duration
	PolicyDigest     string
	UnsafeProposal   bool
	VerifiedSuccess  bool
	FalseBlock       bool
}

type Report struct {
	ApprovalBurden float64
	AverageCostUSD float64
	AverageLatency time.Duration
	Cohort         flags.Cohort
	FalseBlocks    int
	SampleSize     int
	Unsafe         int
	Verified       int
}

func Summarize(cohort flags.Cohort, events []Event) Report {
	report := Report{Cohort: cohort, SampleSize: len(events)}
	var cost float64
	var latency time.Duration
	var approvals int
	for _, event := range events {
		cost += event.CostUSD
		latency += event.Latency
		if event.VerifiedSuccess {
			report.Verified++
		}
		if event.UnsafeProposal {
			report.Unsafe++
		}
		if event.FalseBlock {
			report.FalseBlocks++
		}
		if event.ApprovalRequired {
			approvals++
		}
	}
	if report.SampleSize > 0 {
		report.AverageCostUSD = cost / float64(report.SampleSize)
		report.AverageLatency = latency / time.Duration(report.SampleSize)
		report.ApprovalBurden = float64(approvals) / float64(report.SampleSize)
	}
	return report
}

type PromotionGate struct {
	MinSampleSize int
	MaxUnsafe     int
}

func (g PromotionGate) Approve(report Report, operatorApproved bool) error {
	if !operatorApproved {
		return errors.New("promotion requires explicit operator approval")
	}
	if report.SampleSize < g.MinSampleSize {
		return fmt.Errorf("promotion requires %d samples, got %d", g.MinSampleSize, report.SampleSize)
	}
	if report.Unsafe > g.MaxUnsafe {
		return fmt.Errorf("unsafe proposals %d exceed threshold %d", report.Unsafe, g.MaxUnsafe)
	}
	return nil
}

type EvidencePacket struct {
	CandidateVersion string
	FlagSnapshot     string
	Metric           string
	PolicyDigest     string
	Threshold        int
	TriggeringEvents []string
}

type RollbackController struct {
	Controls  *emergency.Manager
	Expiry    time.Duration
	Threshold int
}

func (c RollbackController) Evaluate(candidateVersion string, events []Event, now time.Time) (EvidencePacket, error) {
	unsafe := make([]Event, 0)
	for _, event := range events {
		if event.UnsafeProposal {
			unsafe = append(unsafe, event)
		}
	}
	if len(unsafe) <= c.Threshold {
		return EvidencePacket{}, nil
	}
	if c.Controls == nil {
		return EvidencePacket{}, errors.New("rollback controls are required")
	}
	packet := EvidencePacket{CandidateVersion: candidateVersion, Metric: "unsafe_proposals", Threshold: c.Threshold}
	for _, event := range unsafe {
		packet.TriggeringEvents = append(packet.TriggeringEvents, event.ID)
		if packet.FlagSnapshot == "" {
			packet.FlagSnapshot = event.FlagSnapshot
		}
		if packet.PolicyDigest == "" {
			packet.PolicyDigest = event.PolicyDigest
		}
	}
	_, err := c.Controls.Apply(emergency.Request{
		Target: flags.FlagMaxAutonomy, Value: "draft", Owner: "rollback-controller",
		Reason: "unsafe candidate proposals", ExpiresAt: now.Add(c.Expiry),
	}, now)
	if err != nil {
		return EvidencePacket{}, err
	}
	return packet, nil
}
