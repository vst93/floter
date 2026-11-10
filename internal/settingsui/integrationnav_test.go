package settingsui

import (
	"strings"
	"testing"

	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func TestIntegrationNavigationShowsOneDetailAndRestoresOverview(t *testing.T) {
	a := navigationFixture()
	copy := i18n.For("zh").Settings
	tt := renderThemed(t, a, 720, 580)
	if tt.HasText("Search") {
		t.Fatal("overview built command forms")
	}
	a.Body.Y = 120
	tt.Frame()
	before := a.Body.Y
	if err := tt.Click(copy.IntegrationsManage + " Ripgrep"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.integrationNav.detail == nil || a.integrationNav.detail.ID != "0" {
		t.Fatal("manage did not open selected tool")
	}
	if tt.HasText("Docker") {
		t.Fatal("detail still renders other tools")
	}
	if !tt.HasText("Search") {
		t.Fatalf("detail lost command: %v", tt.Texts())
	}
	// A draft belongs to the tool, independent of the mounted viewport.
	a.toolName = "unfinished local draft"
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.integrationNav.detail != nil {
		t.Fatal("Escape did not return to overview")
	}
	if a.toolName != "unfinished local draft" {
		t.Fatal("navigation discarded draft")
	}
	if absFloat(a.Body.Y-before) > 1 {
		t.Fatalf("overview scroll = %v, want %v", a.Body.Y, before)
	}
}

func TestIntegrationSearchTabsAndScrollingNavigation(t *testing.T) {
	a := navigationFixture()
	copy := i18n.For("zh").Settings
	tt := renderThemed(t, a, 720, 580)
	heading, _ := tt.Find(copy.IntegrationsMine)
	a.Body.Y = a.Body.MaxY
	tt.Frame()
	after, _ := tt.Find(copy.IntegrationsMine)
	if heading.Y <= after.Y {
		t.Fatal("navigation occupies a fixed band in the small window")
	}
	a.Body.Y = 0
	tt.Frame()
	if err := tt.Click(copy.IntegrationsSearch); err != nil {
		t.Fatal(err)
	}
	tt.Type("docker")
	tt.Frame()
	if !tt.HasText("Docker") || tt.HasText("Ripgrep") {
		t.Fatalf("filter failed: %v", tt.Texts())
	}
	if a.Body.MaxY > 1 {
		t.Fatalf("one filtered tool still scrolls %v", a.Body.MaxY)
	}
	if err := tt.Click(copy.IntegrationsAdd); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(copy.IntegrationsDiscoverEmpty) {
		t.Fatal("add page missing detection empty state")
	}
	if tt.HasText("Docker") {
		t.Fatal("add page still renders installed list")
	}
	if err := tt.Click(copy.IntegrationsMine); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.integrationNav.query != "docker" {
		t.Fatal("navigation discarded search")
	}
}

func TestIntegrationDetailFollowsIdentityAndNewHighlight(t *testing.T) {
	a := navigationFixture()
	items := a.Integrations()
	a.openIntegrationDetail(integrationKey{ID: "1"})
	items[0], items[1] = items[1], items[0]
	a.syncIntegrationNavigation(items)
	if a.integrationNav.detail == nil || a.integrationNav.detail.ID != "1" {
		t.Fatal("refresh lost target")
	}
	items[2].Highlighted = true
	a.syncIntegrationNavigation(items)
	if a.integrationNav.detail.ID != "2" {
		t.Fatal("registration highlight did not open target")
	}
	a.closeIntegrationDetail()
	a.syncIntegrationNavigation(items)
	if a.integrationNav.detail != nil {
		t.Fatal("persistent highlight prevents returning")
	}
	a.openIntegrationDetail(integrationKey{ID: "gone"})
	a.syncIntegrationNavigation(items)
	if a.integrationNav.detail != nil {
		t.Fatal("removed tool leaves an empty detail")
	}
}

func TestIntegrationSearchMatchesAllTermsAndCommands(t *testing.T) {
	item := Integration{ID: "git", Name: "Git", Commands: []Command{{ID: "status", Name: "Working tree"}}}
	if !integrationMatches(item, "GIT tree") || integrationMatches(item, "git docker") {
		t.Fatal("multiword command search failed")
	}
	if !integrationMatches(item, strings.Repeat(" ", 3)) {
		t.Fatal("blank filter should show all")
	}
}

func TestIntegrationNavigationKeepsKeyboardAndConfigDraft(t *testing.T) {
	a := navigationFixture()
	copy := a.copy()
	tt := renderThemed(t, a, 720, 580)
	if err := tt.Click(copy.IntegrationsMine); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	if a.integrationNav.tab != 1 || !tt.Focused(copy.IntegrationsAdd) {
		t.Fatal("keyboard navigation lost focus after changing category")
	}
	tt.Key(0, ui.KeyLeft)
	tt.Frame()
	if a.integrationNav.tab != 0 {
		t.Fatal("keyboard cannot return to tools")
	}
	item := a.installedIntegrations()[0]
	item.Config = []ConfigField{{Key: "endpoint", Type: "text"}}
	draft := a.draft(item)
	draft.live = map[string]string{"endpoint": "unfinished://"}
	a.openIntegrationDetail(integrationKey{ID: item.ID})
	tt.Frame()
	a.closeIntegrationDetail()
	tt.Frame()
	a.openIntegrationDetail(integrationKey{ID: item.ID})
	tt.Frame()
	if a.draft(item).live["endpoint"] != "unfinished://" {
		t.Fatal("returning to tool discarded editor draft")
	}
}
