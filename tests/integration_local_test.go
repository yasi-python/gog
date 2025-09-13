package tests

import (
	"context"
	"testing"
	"time"

	"github.com/yasi-python/go/pkg/probe"
)

func TestLocalProbeUntestable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	res := probe.LocalOrigin{}.ProbeNode(ctx, probe.Node{Host:"",Port:0}, probe.Options{Timeout: 300*time.Millisecond})
	if !res.Success {
		t.Fatalf("untestable should be success=true for safety")
	}
}