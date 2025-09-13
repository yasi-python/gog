package tests

import (
	"math"
	"testing"

	"github.com/yasi-python/go/pkg/stats"
)

func almostEqual(a, b, eps float64) bool { return math.Abs(a-b) < eps }

func TestWilsonLB(t *testing.T) {
	z := 2.575829 // ~99%
	// 100% failures over 200 attempts -> LB close to ~0.985+ (depends on z), just sanity
	lb := stats.WilsonLowerBound(200, 200, z)
	if lb < 0.967 {
		t.Fatalf("expected high LB, got %f", lb)
	}
	// 0 failures
	lb0 := stats.WilsonLowerBound(0, 200, z)
	if !almostEqual(lb0, 0.0, 1e-6) {
		t.Fatalf("expected ~0, got %f", lb0)
	}
}