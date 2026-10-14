package settingsui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// The last import's report, and the notice block that draws it.
//
// The reference keeps `importReport` in panel state and renders it under the
// transfer row (`src/ExtensionsPanel.tsx:537-545,770,2347-2385`): a
// warning when the export had left fields out, then either the summary of a
// clean run or the failure detail. This file carries the same shape.
//
// One deliberate deviation: the reference's import is all-or-nothing, so its
// failure notice leads with `importRolledBack` ("Import failed and all changes
// were rolled back"). This build's `extensions.ImportSync` applies entries one
// by one (P3-l in docs/mygo-rewrite-plan.md), so a failure does not undo the
// entries that already landed and the notice must not claim it did. The
// failure detail (`importFailedDetail` / `importFailedMore`) is the
// reference's own copy and is shown unchanged.

// ImportReport is what the last import did, shown on the Integrations page
// until the next transfer clears it.
type ImportReport struct {
	// Path is the file the import read (the reference's notice tooltip).
	Path string
	// Succeeded and Skipped are counts; Failed names what could not be applied.
	Succeeded int
	Skipped   int
	Failed    []TransferFailure
	// ExcludedFields names the integrations whose export left fields out (a
	// secret or a device path), so the notice can say what will not come back.
	ExcludedFields []ExcludedField
}

// TransferFailure is one entry an import could not apply, with the reason the
// import reported.
type TransferFailure struct {
	ID     string
	Reason string
}

// ExcludedField is one integration the export left fields out of, and how
// many.
type ExcludedField struct {
	ExtensionID string
	FieldCount  int
}

// transferReport draws what the last import did: the excluded-fields warning,
// then the summary of a clean run or the failure detail.
func (a *App) transferReport(c *ui.Context, copy i18n.Settings) {
	report := a.TransferReport
	if report == nil {
		return
	}
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(t.Space(1.5)).Children(func() {
		if len(report.ExcludedFields) > 0 {
			names := make([]string, 0, len(report.ExcludedFields))
			for _, warning := range report.ExcludedFields {
				names = append(names, warning.ExtensionID)
			}
			transferNotice(c, t.Warning, "alert", copy.IntegrationsImportExcludedFields(strings.Join(names, ", ")))
		}
		if len(report.Failed) > 0 {
			// The reference shows the first three failures by `id: message`,
			// then "and N more" (`ExtensionsPanel.tsx:2370-2374`).
			lines := make([]string, 0, len(report.Failed))
			for _, failure := range report.Failed {
				lines = append(lines, failure.ID+": "+failure.Reason)
			}
			shown := lines
			if len(shown) > 3 {
				shown = shown[:3]
			}
			body := copy.IntegrationsImportFailedDetail(strings.Join(shown, "; "))
			if extra := len(lines) - len(shown); extra > 0 {
				body += " (" + copy.IntegrationsImportFailedMore(extra) + ")"
			}
			transferNotice(c, t.Warning, "alert", body)
			return
		}
		transferNotice(c, t.Accent, "check", copy.IntegrationsImportSummary(report.Succeeded, report.Skipped))
	})
}

// transferNotice is one line of the report block: the reference's
// `.extensions-notice` mark beside its sentence, in the kind's own colour.
func transferNotice(c *ui.Context, color ui.Color, mark, text string) {
	t := c.Theme()
	ui.Row(c).FillWidth().Gap(t.Space(2)).AlignItems(ui.Start).Children(func() {
		ui.Icon(c, mustGlyph(mark)).Size(t.Space(4), t.Space(4)).TextColor(color)
		ui.Text(c, text).FontSize(t.FontSize - 1).TextColor(color).Grow(1)
	})
}
