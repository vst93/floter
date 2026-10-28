package browser

import (
	"testing"

	"floter/internal/valuedomain"
)

// TestEveryOfferedSortOrderIsImplemented is the pin `sortOrders` states: the
// picker offers `internal/valuedomain.BrowserSortOrders`, and every one of
// those ids has to mean something here.
//
// The failure it refuses is the shape this package had before P11-x: a
// `switch` whose `default` was "relevance". Adding a fourth ordering to the
// picker (and the loader, which reads the same list) then stored and offered an
// id that this package silently treated as the launcher's ranking — no error,
// no wrong-looking screen, just the wrong order.
//
// `relevance` is the one id with no comparator on purpose: the merge order *is*
// the ranking, so re-deriving a score here would be a second answer. That is
// why the count is `len(sortOrders)+1` rather than an equality.
func TestEveryOfferedSortOrderIsImplemented(t *testing.T) {
	if len(sortOrders)+1 != len(valuedomain.BrowserSortOrders) {
		t.Errorf("%d comparators for %d offered orderings; relevance is the only one without one",
			len(sortOrders), len(valuedomain.BrowserSortOrders))
	}
	for _, id := range valuedomain.BrowserSortOrders {
		_, implemented := sortOrders[SortOrder(id)]
		wantImplemented := SortOrder(id) != SortRelevance
		if implemented != wantImplemented {
			t.Errorf("ordering %q implemented = %v, want %v", id, implemented, wantImplemented)
		}
	}
	for id := range sortOrders {
		if !valuedomain.Has(valuedomain.BrowserSortOrders, string(id)) {
			t.Errorf("comparator for %q, which the picker never offers", id)
		}
	}
}

// TestEveryOfferedSearchFieldIsImplemented is the same pin for the search
// field: every offered id has a column rule, and the rules are distinct.
//
// The distinctness half is the part that catches a real mistake: a new field
// that copied `all`'s rule would pass a mere membership check while matching
// exactly like `all`, which is the silent fallback the old `switch` had.
func TestEveryOfferedSearchFieldIsImplemented(t *testing.T) {
	if len(fieldColumns) != len(valuedomain.BrowserSearchFields) {
		t.Errorf("%d column rules for %d offered fields", len(fieldColumns), len(valuedomain.BrowserSearchFields))
	}
	seen := map[[2]bool]string{}
	for _, id := range valuedomain.BrowserSearchFields {
		columns, ok := fieldColumns[SearchField(id)]
		if !ok {
			t.Errorf("field %q has no column rule, so it silently matches everything", id)
			continue
		}
		if !columns.URL && !columns.Title {
			t.Errorf("field %q matches neither column, so it can never find anything", id)
		}
		key := [2]bool{columns.URL, columns.Title}
		if other, duplicate := seen[key]; duplicate {
			t.Errorf("fields %q and %q choose the same columns", other, id)
		}
		seen[key] = id
	}
	for id := range fieldColumns {
		if !valuedomain.Has(valuedomain.BrowserSearchFields, string(id)) {
			t.Errorf("column rule for %q, which the picker never offers", id)
		}
	}

	// An id the domain does not ship keeps the documented fallback: both
	// columns, the same answer `all` gives. It is not a fourth row.
	if url, title := columnsFor("not-a-shipped-field"); !url || !title {
		t.Errorf("unknown field matches url=%v title=%v, want both", url, title)
	}
}

// TestTheTypedConstantsSpellTheDomain keeps the two typed constants honest: the
// browser package may read the ids from `internal/valuedomain`, but a constant
// can always be re-spelled by hand, and then the domain and this package would
// disagree without either one changing.
func TestTheTypedConstantsSpellTheDomain(t *testing.T) {
	for _, test := range []struct {
		name string
		got  string
		want string
	}{
		{"SortRelevance", string(SortRelevance), valuedomain.BrowserSortRelevance},
		{"SortRecent", string(SortRecent), valuedomain.BrowserSortRecent},
		{"SortAlphabetical", string(SortAlphabetical), valuedomain.BrowserSortAlphabetical},
		{"SortVisits", string(SortVisits), valuedomain.BrowserSortVisits},
		{"FieldAll", string(FieldAll), valuedomain.BrowserFieldAll},
		{"FieldTitle", string(FieldTitle), valuedomain.BrowserFieldTitle},
		{"FieldURL", string(FieldURL), valuedomain.BrowserFieldURL},
	} {
		if test.got != test.want {
			t.Errorf("%s = %q, want %q", test.name, test.got, test.want)
		}
	}
}
