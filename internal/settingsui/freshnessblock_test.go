package settingsui

import (
	"testing"
	"time"

	"floter/internal/extensions"
)

// The freshness block is the reference's drawer section, not one ruled card
// row: `ExtensionsPanel.tsx:3076-3176` draws a block label, a two-cell
// `.extension-metadata` grid (Last probe / Result), and the
// `.extension-freshness__commands` line whose count is the strong register and
// whose delta word steps up only when the list actually moved
// (`extensions.css:1997-2119`).
//
// The port had flattened all of it into the row's grey subline, one register
// for every fact. These pin the three facts as their own nodes.
func TestTheFreshnessFactsAreTheirOwnNodes(t *testing.T) {
	count, previous := 6, 4
	a := New(newStore(t), Actions{
		Freshness: func(id string) (extensions.Freshness, bool) {
			if id != "dev.floter.tool" {
				return extensions.Freshness{}, false
			}
			return extensions.Freshness{
				AtSeconds:            time.Now().Add(-90 * time.Second).Unix(),
				Source:               extensions.FreshnessProbe,
				Result:               extensions.FreshnessSuccess,
				CommandCount:         &count,
				PreviousCommandCount: &previous,
				Delta:                extensions.DeltaIncrease,
			}, true
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{ID: "dev.floter.tool", Name: "Tool", Enabled: true, Running: true}}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	// The block label and the two metadata labels are the reference's words.
	for _, want := range []string{"Freshness", "Last probe", "Result", "Commands"} {
		if !tt.HasText(want) {
			t.Errorf("the block is missing %q: %q", want, tt.Texts())
		}
	}
	// The count is its own node at the strong register, not part of a sentence.
	if !tt.HasText("6") {
		t.Errorf("the count is not its own node: %q", tt.Texts())
	}
	// The delta word is the reference's own preposition ("the previous probe",
	// not the port's "the scan before").
	if !tt.HasText("+2 since the previous probe") {
		t.Errorf("the delta word is not the reference's: %q", tt.Texts())
	}
}

// The health-sourced time is a secondary note line, and "never probed" is its
// own word rather than a zero count (`ExtensionsPanel.tsx:3136-3170`).
func TestTheFreshnessNeverProbedWordIsItsOwnNode(t *testing.T) {
	var freshness extensions.Freshness
	a := New(newStore(t), Actions{
		Freshness: func(id string) (extensions.Freshness, bool) {
			return freshness, id == "dev.floter.tool"
		},
	})
	a.Integrations = func() []Integration {
		return []Integration{{ID: "dev.floter.tool", Name: "Tool", Enabled: true, Running: true}}
	}
	tt := render(t, a, 720, 900)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	freshness = extensions.Freshness{Source: extensions.FreshnessHealth, Result: extensions.FreshnessUnknown}
	tt.Frame()
	if !tt.HasText("Never probed") {
		t.Errorf("the never-probed word is missing: %q", tt.Texts())
	}
	if !tt.HasText("from the health report") {
		t.Errorf("the health-source note is missing: %q", tt.Texts())
	}
}
