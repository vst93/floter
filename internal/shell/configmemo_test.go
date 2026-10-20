package shell

import (
	"testing"
	"time"

	"floter/internal/extensions"
)

// The stored-configuration memo: the settings page rebuilds the integration list
// sixty times a second, and each configurable row's starting values used to cost
// two blocking reads (the document and the secrets generation it names). The
// rule under test is the policy, not a stopwatch:
//
//   - the visit's first frame reads in place, so a card shows its values on the
//     frame that draws it;
//   - a frame inside the TTL is a map lookup;
//   - a stale memo is served as-is and refreshed once behind the list, not once
//     per frame.
func TestTheStoredConfigurationIsReadOncePerBand(t *testing.T) {
	items := []extensions.Integration{{Entry: extensions.Entry{ID: "alpha"}, Paths: extensions.Paths{Data: "/data"}}}
	reads := 0
	compute := func(given []extensions.Integration) map[string]storedConfig {
		reads++
		out := map[string]storedConfig{}
		for _, integration := range given {
			out[integration.Entry.ID] = storedConfig{
				key:    storedConfigKey(integration.Paths, integration.Entry.ID),
				values: map[string]any{"k": "v"},
				ok:     true,
			}
		}
		return out
	}
	var memo storedConfigMemoCache
	clock := time.Unix(1000, 0)

	got := refreshStoredConfigs(&memo, items, compute, nil, clock)
	if reads != 1 {
		t.Fatalf("the first frame read %d times, want 1 (in place)", reads)
	}
	if fact := got["alpha"]; !fact.ok || fact.values["k"] != "v" {
		t.Fatalf("the first frame answered %+v, want the stored values", fact)
	}

	// A frame a sixtieth of a second later: memo, no read.
	refreshStoredConfigs(&memo, items, compute, nil, clock.Add(time.Second/60))
	if reads != 1 {
		t.Fatalf("a frame inside the band read again (%d reads), want a lookup", reads)
	}

	// A stale memo: served as-is now, one background pass asked for.
	refreshStoredConfigs(&memo, items, compute, nil, clock.Add(storedConfigTTL+time.Second))
	if reads != 1 {
		t.Fatalf("the stale frame read in place (%d reads), want it served and refreshed behind", reads)
	}
}

// A newly installed integration is answered on the frame that first carries it,
// even inside the band: the alternative is a row whose form starts empty.
func TestANewRowIsAnsweredOnItsFirstFrame(t *testing.T) {
	first := extensions.Integration{Entry: extensions.Entry{ID: "alpha"}, Paths: extensions.Paths{Data: "/data"}}
	second := extensions.Integration{Entry: extensions.Entry{ID: "beta"}, Paths: extensions.Paths{Data: "/data"}}
	compute := func(given []extensions.Integration) map[string]storedConfig {
		out := map[string]storedConfig{}
		for _, integration := range given {
			out[integration.Entry.ID] = storedConfig{
				key: storedConfigKey(integration.Paths, integration.Entry.ID), ok: true,
			}
		}
		return out
	}
	var memo storedConfigMemoCache
	clock := time.Unix(2000, 0)

	refreshStoredConfigs(&memo, []extensions.Integration{first}, compute, nil, clock)
	got := refreshStoredConfigs(&memo, []extensions.Integration{first, second}, compute, nil, clock.Add(time.Second/60))
	if fact, ok := got["beta"]; !ok || !fact.ok {
		t.Fatalf("the new row's values = %+v (known %v), want them read on its first frame", fact, ok)
	}
}
