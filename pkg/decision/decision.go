package decision

import (
	"time"

	"github.com/yasi-python/go/pkg/stats"
)

type ConfigStats struct {
	ID                  string
	Attempts            int
	Successes           int
	Failures            int
	LastSuccessUnix     int64
	LastFailureUnix     int64
	ConsecutiveFailures int
}

type DecisionInput struct {
	Stats           ConfigStats
	Z               float64
	MinAttempts     int
	DeleteLB        float64
	ConsecFailToQ   int
	Now             time.Time
}

type Action string

const (
	ActionKeep       Action = "keep"
	ActionQuarantine Action = "quarantine"
	ActionDelete     Action = "delete"
)

type Decision struct {
	Action             Action
	FailureLB          float64
	Reason             string
}

func Evaluate(in DecisionInput) Decision {
	s := in.Stats
	if s.Attempts == 0 {
		return Decision{Action: ActionKeep, FailureLB: 0, Reason: "no_attempts"}
	}
	// failure rate lower bound
	failuresLB := stats.WilsonLowerBound(s.Failures, s.Attempts, in.Z)
	// quarantine on consecutive failures
	if s.ConsecutiveFailures >= in.ConsecFailToQ {
		return Decision{Action: ActionQuarantine, FailureLB: failuresLB, Reason: "consecutive_failures"}
	}
	// delete if enough attempts and high LB of failure
	if s.Attempts >= in.MinAttempts && failuresLB >= in.DeleteLB {
		return Decision{Action: ActionDelete, FailureLB: failuresLB, Reason: "high_failure_lb"}
	}
	return Decision{Action: ActionKeep, FailureLB: failuresLB, Reason: "normal"}
}