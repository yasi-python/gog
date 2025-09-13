package tests

import (
	"testing"
	"time"

	"github.com/yasi-python/go/pkg/decision"
)

func TestDecisionPaths(t *testing.T) {
	in := decision.DecisionInput{
		Stats: decision.ConfigStats{Attempts: 10, Failures: 10, ConsecutiveFailures: 10},
		MinAttempts: 200, DeleteLB: 0.995, Z: 2.575829,
		ConsecFailToQ: 10, Now: time.Now(),
	}
	d := decision.Evaluate(in)
	if d.Action != decision.ActionQuarantine {
		t.Fatalf("expected quarantine, got %v", d.Action)
	}

	in2 := decision.DecisionInput{
		Stats: decision.ConfigStats{Attempts: 300, Failures: 300, ConsecutiveFailures: 0},
		MinAttempts: 200, DeleteLB: 0.90, Z: 2.575829,
		ConsecFailToQ: 10, Now: time.Now(),
	}
	d2 := decision.Evaluate(in2)
	if d2.Action != decision.ActionDelete {
		t.Fatalf("expected delete, got %v", d2.Action)
	}
}