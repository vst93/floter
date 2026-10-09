package browser

import (
	"context"
	"sort"
	"strings"
	"time"
)

// The two kinds a result can be.
const (
	KindHistory  = "history"
	KindBookmark = "bookmark"
)

// SearchField is which part of a row a query is matched against: `all` is the
// title or the URL, `title` and `url` narrow it. Anything else is `all`, as
// the old build's normalizer decided.
type SearchField string

const (
	FieldAll   SearchField = "all"
	FieldTitle SearchField = "title"
	FieldURL   SearchField = "url"
)

// SortOrder is how results are ordered: `relevance` keeps the launcher's own
// ranking (bookmarks in their file order, history newest first), the other
// three are explicit orderings a bookmark tool is expected to offer.
type SortOrder string

const (
	SortRelevance    SortOrder = "relevance"
	SortRecent       SortOrder = "recent"
	SortAlphabetical SortOrder = "alphabetical"
	SortVisits       SortOrder = "visits"
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

// matchesTerms reports whether every term appears in the row's fields.
func matchesTerms(result Result, terms []string, field SearchField) bool {
	if len(terms) == 0 {
		return true
	}
	haystacks := []string{strings.ToLower(result.Title), strings.ToLower(result.URL)}
	switch field {
	case FieldTitle:
		haystacks = haystacks[:1]
	case FieldURL:
		haystacks = haystacks[1:]
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

// orderResults sorts results in place for the configured order. `relevance`
// is deliberately a no-op: the order the merge produced *is* the launcher's
// ranking, and re-deriving a match score here would be a second, silently
// different one.
func orderResults(results []Result, order SortOrder) {
	switch order {
	case SortAlphabetical:
		sort.SliceStable(results, func(i, j int) bool {
			left, right := strings.ToLower(results[i].Label()), strings.ToLower(results[j].Label())
			if left != right {
				return left < right
			}
			return results[i].URL < results[j].URL
		})
	case SortRecent:
		sort.SliceStable(results, func(i, j int) bool {
			return stamp(results[i]).After(stamp(results[j]))
		})
	case SortVisits:
		sort.SliceStable(results, func(i, j int) bool {
			if results[i].Visits != results[j].Visits {
				return results[i].Visits > results[j].Visits
			}
			return stamp(results[i]).After(stamp(results[j]))
		})
	default:
		// relevance: the merge order, unchanged.
	}
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
	for _, term := range terms {
		var parts []string
		if field == FieldAll || field == FieldURL {
			parts = append(parts, "instr(lower("+urlColumn+"), ?) > 0")
			args = append(args, term)
		}
		if (field == FieldAll || field == FieldTitle) && titleColumn != "" {
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
