package rbn

import (
	"testing"
	"time"
)

func TestDefaultTelnetBatchPolicyFlushesOnCountOrDelay(t *testing.T) {
	policy := DefaultTelnetBatchPolicy()
	first := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)

	if policy.ShouldFlush(0, first, first.Add(10*time.Second)) {
		t.Fatal("empty batch should not flush")
	}
	if !policy.ShouldFlush(5, first, first) {
		t.Fatal("five records should flush immediately")
	}
	if policy.ShouldFlush(4, first, first.Add(4*time.Second)) {
		t.Fatal("four records before five seconds should not flush")
	}
	if !policy.ShouldFlush(4, first, first.Add(5*time.Second)) {
		t.Fatal("four records at five seconds should flush")
	}
}
