package shell

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/settingsui"
)

// Exporting and importing the integration list, through the native file
// dialogs: a document of what is installed and how it is configured, with the
// secrets left behind (see internal/extensions/sync.go).
//
// The reference's flow (`src/ExtensionsPanel.tsx:1206-1240`) is one transfer
// at a time: a second press while one runs is ignored, the two buttons are
// disabled and a busy line is shown, a clean export raises a success toast,
// a clean import leaves its report on the page, and any error is a toast.

// beginTransfer claims the one transfer slot, and reports whether it got it. A
// second call while one is in flight is refused (the reference's
// `syncOperationRef.current` guard).
func (a *App) beginTransfer(operation string) bool {
	a.transferMu.Lock()
	if a.transferRunning {
		a.transferMu.Unlock()
		return false
	}
	a.transferRunning = true
	a.transferMu.Unlock()
	a.setTransferBusy(operation)
	return true
}

// finishTransfer releases the slot and clears the busy line, whatever the
// transfer did.
func (a *App) finishTransfer() {
	a.setTransferBusy("")
	a.transferMu.Lock()
	a.transferRunning = false
	a.transferMu.Unlock()
}

// transferRunningNow reports whether a transfer holds the slot; the tests read
// it through here so the flag is never touched unsynchronized.
func (a *App) transferRunningNow() bool {
	a.transferMu.Lock()
	defer a.transferMu.Unlock()
	return a.transferRunning
}

// exportIntegrations asks for a file and writes the document.
func (a *App) exportIntegrations() {
	if !a.beginTransfer("export") {
		return
	}
	go func() {
		defer a.finishTransfer()
		copy := a.SettingsCopy()
		document, err := extensions.ExportSync(a.Paths)
		if err != nil {
			log.Printf("floter: could not export the integrations: %v", err)
			a.transferFailed(copy.IntegrationsTransferBad)
			return
		}
		path, err := a.saveFile("floter-integrations-" + time.Now().Format("2006-01-02") + ".json")
		if err != nil || path == "" {
			return // the user closed the dialog
		}
		data, err := document.Marshal()
		if err != nil {
			log.Printf("floter: could not encode the integrations: %v", err)
			a.transferFailed(copy.IntegrationsTransferBad)
			return
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
			log.Printf("floter: could not write %s: %v", path, err)
			a.transferFailed(copy.IntegrationsTransferBad)
			return
		}
		// The reference reports a clean export as a success toast naming the
		// count and the two things that never travel.
		a.transferSucceeded(copy.IntegrationsExported(len(document.Extensions)))
	}()
}

// importIntegrations asks for a file and applies it.
func (a *App) importIntegrations() {
	if !a.beginTransfer("import") {
		return
	}
	go func() {
		defer a.finishTransfer()
		copy := a.SettingsCopy()
		path, err := a.openFile()
		if err != nil || path == "" {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("floter: could not read %s: %v", path, err)
			a.transferFailed(copy.IntegrationsTransferBad)
			return
		}
		document, err := extensions.ReadSync(data)
		if err != nil {
			log.Printf("floter: could not read %s: %v", path, err)
			a.transferFailed(copy.IntegrationsTransferBad)
			return
		}
		report, err := extensions.ImportSync(a.Paths, document, func(approvals []extensions.PermissionApproval) bool {
			// One question for everything the import would install, as the
			// old build asked.
			for _, approval := range approvals {
				if !a.confirmInstallPermissions(approval) {
					return false
				}
			}
			return true
		})
		if err != nil {
			log.Printf("floter: could not import %s: %v", path, err)
			a.transferFailed(copy.IntegrationsTransferBad)
			return
		}
		for _, failure := range report.Failed {
			log.Printf("floter: %s was not imported: %s", failure.ID, failure.Reason)
		}
		for _, skipped := range report.Skipped {
			log.Printf("floter: %s was skipped: %s", skipped.ID, skipped.Reason)
		}
		// The report is the page's own notice block, drawn until the next
		// transfer clears it (the reference's `setImportReport`).
		a.setTransferReport(importReport(path, report))
		a.RefreshIntegrations(context.Background())
	}()
}

// importReport maps the extensions package's report onto the page's own.
func importReport(path string, report extensions.SyncReport) *settingsui.ImportReport {
	out := &settingsui.ImportReport{
		Path:      path,
		Succeeded: len(report.Succeeded),
		Skipped:   len(report.Skipped),
	}
	for _, failure := range report.Failed {
		out.Failed = append(out.Failed, settingsui.TransferFailure{ID: failure.ID, Reason: failure.Reason})
	}
	for _, warning := range report.ExcludedFields {
		out.ExcludedFields = append(out.ExcludedFields, settingsui.ExcludedField{
			ExtensionID: warning.ExtensionID,
			FieldCount:  warning.FieldCount,
		})
	}
	return out
}

// transferSucceeded raises the export's success toast.
func (a *App) transferSucceeded(message string) {
	a.queueToast(ui.Toast{
		ID:      transferNoticeID,
		Title:   message,
		Type:    "success",
		Timeout: successNotice,
	})
}

// transferFailed raises a transfer's error toast: the reference's
// `showError(errorMessage(...))`. The one sentence this build produces for a
// bad file is `IntegrationsTransferBad`, so the message is the same on every
// branch and the caller passes it.
func (a *App) transferFailed(message string) {
	a.queueNotice(message, refusalNotice)
}

// setTransferBusy puts (or clears) the busy line on the Integrations page.
func (a *App) setTransferBusy(operation string) {
	a.onMain(func() {
		if a.Settings == nil {
			return
		}
		a.Settings.TransferBusy = operation
		if a.Win != nil {
			a.Win.Invalidate()
		}
	})
}

// setTransferReport leaves the last import's report on the page.
func (a *App) setTransferReport(report *settingsui.ImportReport) {
	a.onMain(func() {
		if a.Settings == nil {
			return
		}
		a.Settings.TransferReport = report
		if a.Win != nil {
			a.Win.Invalidate()
		}
	})
}

// saveFile asks for a path to write to; empty when the user closed the dialog.
func (a *App) saveFile(defaultName string) (string, error) {
	if a.saveFileDialog != nil {
		return a.saveFileDialog(defaultName)
	}
	return mygo.Dialog.Save(mygo.SaveDialogOptions{
		Title:       "floter",
		DefaultPath: filepath.Join(homeDirOrEmpty(), defaultName),
		Filters:     []mygo.FileFilter{{Name: "JSON", Extensions: []string{"json"}}},
	})
}

// openFile asks for a path to read.
func (a *App) openFile() (string, error) {
	if a.openFileDialog != nil {
		return a.openFileDialog()
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:   "floter",
		Filters: []mygo.FileFilter{{Name: "JSON", Extensions: []string{"json"}}},
	})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

// homeDirOrEmpty is where a save dialog starts, when the platform names one.
func homeDirOrEmpty() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// transferNoticeID is the export toast's own id, distinct from the refusal and
// drift notices': news of a finished export must not replace either.
const transferNoticeID = "floter-transfer-notice"
