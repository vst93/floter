package versioning

import "testing"

func TestParseReadsEverySpellingTheRepositoryHas(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"0.3.25", "0.3.25"},
		{"mygo-v0.3.25", "0.3.25"},
		{"v0.3.25", "0.3.25"},
		{"refs/tags/mygo-v0.3.25", "0.3.25"},
		{"mygo-v0.3.25^{}", "0.3.25"},
		{"0.3.26-preview.1", "0.3.26-preview.1"},
		{"mygo-v0.3.26-rc.2", "0.3.26-rc.2"},
		{"1.0.0+build.7", "1.0.0"},
		{" 0.3.25 ", "0.3.25"},
	}
	for _, test := range cases {
		got, err := Parse(test.in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", test.in, err)
		}
		if got.String() != test.want {
			t.Fatalf("Parse(%q) = %q, want %q", test.in, got.String(), test.want)
		}
	}
}

func TestParseRejectsWhatTheReleaseStepWouldReject(t *testing.T) {
	for _, in := range []string{"", "0.3", "0.3.25.1", "v0.3.25.1", "0.03.25", "a.b.c", "0.3.25-", "0.3.25-prev_1", "0.3.-5"} {
		if version, err := Parse(in); err == nil {
			t.Fatalf("Parse(%q) = %q, want an error", in, version.String())
		}
	}
}

// The bug this package was written for: `mygo.json` is stale (the workflow
// never commits the version back), so deriving from it republished a version
// users already had — silently, under `--clobber`.
func TestAStaleMygoJSONCanNeverNameAPublishedVersion(t *testing.T) {
	next, err := Next("0.3.24", []string{"mygo-v0.3.18", "mygo-v0.3.19", "mygo-v0.3.25"})
	if err != nil {
		t.Fatal(err)
	}
	if next != "0.3.26" {
		t.Fatalf("Next = %q, want 0.3.26 (the highest tag bumps, not the stale file)", next)
	}
}

// An existing core is ambiguous for the updater whichever kind of release it
// was: 0.3.26-preview.1 already answers "what is 0.3.26".
func TestAPreviewOfTheCandidateSkipsIt(t *testing.T) {
	next, err := Next("0.3.25", []string{"mygo-v0.3.25", "mygo-v0.3.26-preview.1"})
	if err != nil {
		t.Fatal(err)
	}
	if next != "0.3.27" {
		t.Fatalf("Next = %q, want 0.3.27 (0.3.26 is taken by its preview)", next)
	}
}

// The file is a floor, not a ceiling: a hand-edited jump forward is honoured,
// because that is how a minor bump is announced.
func TestAMygoJSONAheadOfTheTagsRaisesTheFloor(t *testing.T) {
	next, err := Next("0.4.0", []string{"mygo-v0.3.25"})
	if err != nil {
		t.Fatal(err)
	}
	if next != "0.4.1" {
		t.Fatalf("Next = %q, want 0.4.1", next)
	}
}

func TestTheFirstReleaseAfterAnEmptyTagList(t *testing.T) {
	next, err := Next("0.3.24", nil)
	if err != nil {
		t.Fatal(err)
	}
	if next != "0.3.25" {
		t.Fatalf("Next = %q, want 0.3.25", next)
	}
}

// A stray tag on the remote must not stop a release; it does not become the
// floor either, since it names nothing buildable.
func TestUnparsableTagsAreIgnored(t *testing.T) {
	next, err := Next("0.3.25", []string{"mygo-vnext", "nightly", "mygo-v0.3.25"})
	if err != nil {
		t.Fatal(err)
	}
	if next != "0.3.26" {
		t.Fatalf("Next = %q, want 0.3.26", next)
	}
	// An unparsable *current* value is fatal: the release cannot derive from it.
	if _, err := Next("main", []string{"mygo-v0.3.25"}); err == nil {
		t.Fatal("Next with an unparsable mygo.json should fail")
	}
}

// Whatever Next answers must be a version the workflow's own regex accepts and
// that is not already taken — the two properties the release step depends on.
func TestNextIsAlwaysFreeAndWellFormed(t *testing.T) {
	existing := []string{"mygo-v0.3.20", "mygo-v0.3.21", "mygo-v0.3.22", "mygo-v0.3.23", "mygo-v0.3.24", "mygo-v0.3.25"}
	next, err := Next("0.3.21", existing)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(next); err != nil {
		t.Fatalf("Next returned %q, which does not parse: %v", next, err)
	}
	for _, tag := range existing {
		if tag == "mygo-v"+next {
			t.Fatalf("Next returned a taken version %q", next)
		}
	}
}
