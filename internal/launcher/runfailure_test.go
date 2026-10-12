package launcher

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// A keyed run failure is rendered in the stored language, and the sentence
// names the fact that makes it actionable.
func TestRunErrorMessageIsLocalized(t *testing.T) {
	failure := extensions.PathFailure(extensions.RunErrorProgramMissing, "/opt/bin/node")
	en := runErrorMessage(i18n.For("en").Settings, failure)
	if !strings.Contains(en, "/opt/bin/node") || !strings.Contains(en, "does not exist") {
		t.Errorf("en = %q", en)
	}
	zh := runErrorMessage(i18n.For("zh").Settings, failure)
	if !strings.Contains(zh, "/opt/bin/node") || !strings.Contains(zh, "文件不存在") {
		t.Errorf("zh = %q", zh)
	}
}

// The interpreter-missing sentence keeps the old formatter's joins: the
// candidate names on one line, the searched directories as a vertical list.
func TestInterpreterMissingJoinsItsLists(t *testing.T) {
	failure := extensions.NewRunFailure(extensions.RunErrorInterpreterMissing, map[string]any{
		"language": "python",
		"names":    []string{"python3", "python"},
		"searched": []string{"/usr/bin", "/usr/local/bin"},
	})
	en := runErrorMessage(i18n.For("en").Settings, failure)
	if !strings.Contains(en, "python3 / python") {
		t.Errorf("names are not one line: %q", en)
	}
	if !strings.Contains(en, "/usr/bin\n/usr/local/bin") {
		t.Errorf("directories are not a vertical list: %q", en)
	}
}

// A message that did not come from the keyed path keeps its own words.
func TestFailureReasonKeepsUnrecognizedProse(t *testing.T) {
	a := testApp()
	prose := "extensions: the command has no program to run"
	if got := a.failureReason(errors.New(prose)); got != prose {
		t.Errorf("failureReason = %q, want %q", got, prose)
	}
	if got := a.failureReason(nil); got != "" {
		t.Errorf("failureReason(nil) = %q", got)
	}
}

// The output view's one-line summary localises a keyed failure, and follows
// the language the store is set to.
func TestRunStatusLocalizesAKeyedFailure(t *testing.T) {
	a := testApp()
	run := extensions.CapturedRun{}
	runErr := error(extensions.RunSpawnFailure("/opt/bin/node", fs.ErrNotExist))
	status := a.RunStatus(run, runErr)
	if !strings.Contains(status, "/opt/bin/node") || !strings.Contains(status, "does not exist") {
		t.Errorf("status = %q", status)
	}
	if updateErr := a.Store.Update(func(s *settings.Settings) { s.Language = "zh" }); updateErr != nil {
		t.Fatal(updateErr)
	}
	status = a.RunStatus(run, runErr)
	if !strings.Contains(status, "文件不存在") {
		t.Errorf("zh status = %q", status)
	}
}
