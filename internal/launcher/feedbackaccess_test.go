package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestFeedbackIsAccessibleWithoutTakingSearchFocus(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	tt.Announcements()
	a.Feedback("Copied result")
	tt.Frame()
	if _, ok := tt.AccessByRole(ui.RoleStatus, "Copied result"); !ok {
		t.Fatal("feedback is missing its accessible status")
	}
	if got := tt.Announcements(); len(got) != 1 || got[0] != "Copied result" {
		t.Fatalf("status announcements = %v", got)
	}
	tt.Frame()
	if got := tt.Announcements(); len(got) != 0 {
		t.Fatalf("unchanged feedback was repeated: %v", got)
	}
	a.WarnFeedback("Could not copy result")
	tt.Frame()
	if _, ok := tt.AccessByRole(ui.RoleAlert, "Could not copy result"); !ok {
		t.Fatal("warning is missing its live alert role")
	}
	tt.Type("query")
	if a.Query != "query" {
		t.Fatalf("feedback took focus away from search: query = %q", a.Query)
	}
}
