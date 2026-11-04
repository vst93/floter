package shell

import (
	"sync/atomic"
	"testing"
	"time"

	"floter/internal/extensions"
)

// A completed refresh must be fresh to the clock that asked for it. Waiting
// for publication before the next frame makes the P13-go interleaving exact.
func TestARefreshPublishesInTheRequestClock(t *testing.T) {
	var memo integrationFactsCache
	items := []extensions.Integration{{Entry: extensions.Entry{ID: "clock"}}}
	facts := map[string]integrationFact{"clock": {key: integrationFactKey(items[0])}}
	now := time.Now().Add(time.Hour)
	memo.publish(facts, now.Add(-integrationFactsTTL))
	var calls atomic.Int32
	landed := make(chan struct{}, 2)
	release := make(chan struct{})
	defer close(release)
	compute := func([]extensions.Integration) map[string]integrationFact {
		if calls.Add(1) > 1 {
			// If a fresh answer is erroneously re-asked, keep that pass pending
			// until the assertion reads it. No scheduler race in the probe.
			<-release
		}
		return facts
	}
	refreshIntegrationFacts(&memo, items, compute, func() { landed <- struct{}{} }, now)
	select {
	case <-landed:
	case <-time.After(5 * time.Second):
		t.Fatal("background refresh never published")
	}
	_, completed, pending := memo.snapshot()
	if !completed.After(now) {
		t.Errorf("completion clock %v must follow request clock %v, including elapsed computation", completed, now)
	}
	if pending {
		t.Error("published refresh still pending")
	}
	refreshIntegrationFacts(&memo, items, compute, nil, now)
	_, _, pending = memo.snapshot()
	if pending {
		t.Error("the same frame clock started another refresh after publication")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("fresh memo recomputed: calls=%d, want 1", got)
	}
}
