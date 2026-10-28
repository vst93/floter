package browser

import (
	"context"
	"sort"
	"strings"
	"time"

	"floter/internal/valuedomain"
)

// The two kinds a result can be.
const (
	KindHistory  = "history"
	KindBookmark = "bookmark"
)

// SearchField is which part of a row a query is matched against: `all` is the
// title or the URL, `title` and `url` narrow it. Anything else is `all`, as
// the old build's normalizer decided. The ids live in `internal/valuedomain`:
// this package is their second reader, not their author.
type SearchField string

const (
	FieldAll   SearchField = valuedomain.BrowserFieldAll
	FieldTitle SearchField = valuedomain.BrowserFieldTitle
	FieldURL   SearchField = valuedomain.BrowserFieldURL
)

// SortOrder is how results are ordered: `relevance` keeps the launcher's own
// ranking (bookmarks in their file order, history newest first), the other
// three are explicit orderings a bookmark tool is expected to offer.
type SortOrder string

const (
	SortRelevance    SortOrder = valuedomain.BrowserSortRelevance
	SortRecent       SortOrder = valuedomain.BrowserSortRecent
	SortAlphabetical SortOrder = valuedomain.BrowserSortAlphabetical
	SortVisits       SortOrder = valuedomain.BrowserSortVisits
)

// Options narrow a search: how far back history goes, how many results come
// back, how they are ordered and which fields they are matched against.
type Options struct {
	// Days limits history to the last N days; 0 means all of it. Bookmarks
	// are never filtered: they have no date.
	Days int
	// Limit caps the results; 0 means no cap.
	Limit int
	// SortOrder is the order the results come back in; the zero value is
	// `relevance`.
	SortOrder SortOrder
	// Field is what the query is matched against; the zero value is `all`.
	Field SearchField
}

// Search reads every profile's history and bookmarks and returns the entries
// matching the query.
//
// Bookmarks are filtered in memory (their file is read whole) and history in
// SQL, with the tokens pushed into the statement *before* its LIMIT: that is
// what lets a match older than the newest rows be found at all. A URL that is
// both a bookmark and a visit is one result, and the curated bookmark wins.
//
// A profile that cannot be read is skipped: a locked or unreadable database
// must not stop the others.
func Search(ctx context.Context, profiles []Profile, query string, options Options) []Result {
	terms := searchTokens(query)
	field := options.Field
	if field != FieldTitle && field != FieldURL {
		field = FieldAll
	}

	var bookmarks, history []Result
	for _, profile := range profiles {
		if profile.BookmarksFile != "" {
			for _, result := range readBookmarks(profile) {
				if matchesTerms(result, terms, field) {
					bookmarks = append(bookmarks, result)
				}
			}
		}
		if profile.HistoryDB != "" {
			history = append(history, readHistory(ctx, profile, terms, field, options.Days)...)
		}
	}

	// Bookmarks first, then history: a URL that is both is one row and the
	// curated bookmark wins.
	merged := make([]Result, 0, len(bookmarks)+len(history))
	seen := make(map[string]bool, len(bookmarks)+len(history))
	for _, result := range append(bookmarks, history...) {
		if result.URL == "" || seen[result.URL] {
			continue
		}
		seen[result.URL] = true
		merged = append(merged, result)
	}

	orderResults(merged, options.SortOrder)
	if options.Limit > 0 && len(merged) > options.Limit {
		merged = merged[:options.Limit]
	}
	return merged
}

// FilterTabs keeps the tabs whose title or URL matches the query under the
// configured search field. The shell applies it to the live-tab group, which
// comes from the browser unfiltered.
func FilterTabs(tabs []Tab, query string, field SearchField) []Tab {
	terms := searchTokens(query)
	if len(terms) == 0 {
		return tabs
	}
	out := make([]Tab, 0, len(tabs))
	for _, tab := range tabs {
		if matchesTerms(Result{Title: tab.Title, URL: tab.URL}, terms, field) {
			out = append(out, tab)
		}
	}
	return out
}

// searchTokens splits a query the way every search in the app does: lower-case
// words, all of which must match.
func searchTokens(query string) []string {
	return strings.Fields(strings.ToLower(query))
}

// fieldColumns is which of a row's two columns each search field matches. It
// is the one place that decides, so the in-memory filter (`matchesTerms`) and
// the SQL clause (`matchesClause`) cannot disagree about what `title` means.
//
// The three shipped fields are the whole table; `columnsFor` falls back to
// "both" for an id the table does not hold, which is the documented answer for
// an unknown field (the same one the old normalizer gave), not a fourth entry.
var fieldColumns = map[SearchField]struct {
	URL   bool
	Title bool
}{
	FieldAll:   {URL: true, Title: true},
	FieldTitle: {URL: false, Title: true},
	FieldURL:   {URL: true, Title: false},
}

// columnsFor reports whether a field matches the URL and the title.
func columnsFor(field SearchField) (url, title bool) {
	columns, ok := fieldColumns[field]
	if !ok {
		return true, true
	}
	return columns.URL, columns.Title
}

// matchesTerms reports whether every term appears in the row's fields.
func matchesTerms(result Result, terms []string, field SearchField) bool {
	if len(terms) == 0 {
		return true
	}
	url, title := columnsFor(field)
	haystacks := make([]string, 0, 2)
	if title {
		haystacks = append(haystacks, strings.ToLower(result.Title))
	}
	if url {
		haystacks = append(haystacks, strings.ToLower(result.URL))
	}
	for _, term := range terms {
		found := false
		for _, haystack := range haystacks {
			if strings.Contains(haystack, term) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// sortOrders is how each ordering compares two rows. `relevance` is
// deliberately absent: the order the merge produced *is* the launcher's
// ranking, so it has no comparator to re-derive a second, silently different
// score with. A missing entry is therefore the no-op `orderResults` takes.
var sortOrders = map[SortOrder]func(a, b Result) bool{
	SortAlphabetical: func(a, b Result) bool {
		left, right := strings.ToLower(a.Label()), strings.ToLower(b.Label())
		if left != right {
			return left < right
		}
		return a.URL < b.URL
	},
	SortRecent: func(a, b Result) bool {
		return stamp(a).After(stamp(b))
	},
	SortVisits: func(a, b Result) bool {
		if a.Visits != b.Visits {
			return a.Visits > b.Visits
		}
		return stamp(a).After(stamp(b))
	},
}

// orderResults sorts results in place for the configured order.
func orderResults(results []Result, order SortOrder) {
	less, ok := sortOrders[order]
	if !ok {
		// relevance: the merge order, unchanged.
		return
	}
	sort.SliceStable(results, func(i, j int) bool { return less(results[i], results[j]) })
}

// stamp is when a row is from: its last visit, or when a bookmark was added.
func stamp(result Result) time.Time {
	if !result.Visited.IsZero() {
		return result.Visited
	}
	return result.Added
}

// matchesClause builds the SQL that applies the search terms to one history
// shape, and its arguments. `instr` is used rather than `LIKE` so a term
// containing `%` or `_` is a literal, with nothing to escape.
//
// titleColumn is empty for a shape with no title (Safari's): a title-only
// search then matches nothing rather than everything.
func matchesClause(terms []string, field SearchField, urlColumn, titleColumn string) (string, []any) {
	if len(terms) == 0 {
		return "", nil
	}
	var clauses []string
	var args []any
	url, title := columnsFor(field)
	for _, term := range terms {
		var parts []string
		if url {
			parts = append(parts, "instr(lower("+urlColumn+"), ?) > 0")
			args = append(args, term)
		}
		if title && titleColumn != "" {
			parts = append(parts, "instr(lower("+titleColumn+"), ?) > 0")
			args = append(args, term)
		}
		if len(parts) == 0 {
			// A title-only search of a shape with no title can never match.
			return " AND 0", nil
		}
		clauses = append(clauses, "("+strings.Join(parts, " OR ")+")")
	}
	return " AND " + strings.Join(clauses, " AND "), args
}
