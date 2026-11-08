package settingsui

import (
	"strconv"
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
	// SkippedDetails retains the reasons behind the unchanged count.
	SkippedDetails []TransferFailure
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

// commandTransferFeedback observes only the report's visible content. Paths
// and excluded-field counts are tooltip details and must not reclaim scrolling.
func (a *App) commandTransferFeedback(copy i18n.Settings) string {
	report := a.TransferReport
	if report == nil {
		return ""
	}
	var feedback strings.Builder
	for _, excluded := range report.ExcludedFields {
		feedback.WriteString(strconv.Quote(excluded.ExtensionID))
	}
	feedback.WriteByte(0)
	if len(report.Failed) == 0 || report.Succeeded > 0 || report.Skipped > 0 {
		feedback.WriteString(strconv.Quote(copy.IntegrationsImportSummary(report.Succeeded, report.Skipped)))
	}
	feedback.WriteByte(0)
	for _, failure := range report.Failed {
		feedback.WriteString(strconv.Quote(failure.ID))
		feedback.WriteString(strconv.Quote(failure.Reason))
	}
	feedback.WriteByte(0)
	for _, skipped := range report.SkippedDetails {
		feedback.WriteString(strconv.Quote(skipped.ID))
		feedback.WriteString(strconv.Quote(skipped.Reason))
	}
	return feedback.String()
}

// transferReport draws what the last import did: the excluded-fields warning,
// then the summary of a clean run or the failure detail.
func (a *App) transferReport(c *ui.Context, copy i18n.Settings) {
	report := a.TransferReport
	if report == nil {
		return
	}
	t := c.Theme()
	tokens := a.tokens(c)
	block := ui.Column(c).FillWidth().Gap(t.Space(1.5))
	block.Children(func() {
		if len(report.ExcludedFields) > 0 {
			names := make([]string, 0, len(report.ExcludedFields))
			details := make([]string, 0, len(report.ExcludedFields))
			for _, warning := range report.ExcludedFields {
				names = append(names, warning.ExtensionID)
				details = append(details, warning.ExtensionID+": "+strconv.Itoa(warning.FieldCount))
			}
			warning := a.transferNotice(c, t.Warning, "alert", copy.IntegrationsImportExcludedFields(strings.Join(names, ", ")))
			tokens.Tip(c, warning, strings.Join(details, "\n"))
		}
		// Entries are applied independently: a failure must not hide the
		// integrations already changed, or the count that remained unchanged.
		if len(report.Failed) == 0 || report.Succeeded > 0 || report.Skipped > 0 {
			a.transferNotice(c, tokens.TextStrong, "check", copy.IntegrationsImportSummary(report.Succeeded, report.Skipped))
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
			a.transferNotice(c, t.Warning, "alert", body)
			// Keep the established three-entry summary, but make its tail
			// readable in the page's existing scroll area without hover.
			for _, line := range lines[len(shown):] {
				a.transferNotice(c, t.Warning, "alert", line)
			}
		}
		if len(report.SkippedDetails) > 0 {
			lines := make([]string, 0, len(report.SkippedDetails))
			for _, skipped := range report.SkippedDetails {
				lines = append(lines, skipped.ID+": "+skipped.Reason)
			}
			a.transferNotice(c, t.Warning, "alert", strings.Join(lines, "\n"))
		}
	})
	if report.Path != "" {
		tokens.Tip(c, block, report.Path)
	}
}

// transferNotice is one line of the report block: the reference's
// `.extensions-notice` mark beside its sentence, in the kind's own colour.
//
// P8-d 重定基线（§5 判据）：导入成功的 ✓ + 句曾是 `t.Accent`。成功是一个**结果
// 语义**，不是「哪个被选中 / 哪个是开的」；accent 在这里只充当「这条重要」的
// 高亮，属于 §5 禁止的那一类。改 `TextStrong`（同一行的失败/警告仍然是
// `t.Warning` 语义色：语义色不是 accent，不在本预算内）。
func (a *App) transferNotice(c *ui.Context, color ui.Color, mark, text string) ui.Element {
	t := c.Theme()
	return ui.Row(c).FillWidth().Gap(t.Space(2)).AlignItems(ui.Start).Children(func() {
		ui.Icon(c, mustGlyph(mark)).Size(t.Space(4), t.Space(4)).TextColor(color)
		ui.Text(c, text).FontSize(a.tokens(c).Caption).TextColor(color).Grow(1).MinWidth(0)
	})
}
