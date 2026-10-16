package shell

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
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

// exportScript writes the local-tool form's script to a file the user picks:
// the reference's `extensions_custom_export_script` behind the "Export script"
// button (`src-tauri/src/commands/extensions.rs:1516-1560`,
// `src/ExtensionsPanel.tsx:1951-1964`). The dialog is opened off the render
// thread, because it blocks until the user answers; a closed dialog writes
// nothing and says nothing, and a clean write is the success toast the old
// build raised. name is the file name the dialog offers, already carrying the
// language's extension (`settingsui.scriptExportName`).
func (a *App) exportScript(name, content string) {
	go func() {
		copy := a.SettingsCopy()
		path, err := a.saveFile(name)
		if err != nil {
			log.Printf("floter: could not pick a path for %s: %v", name, err)
			a.queueNotice(copy.LocalToolExportFailed, refusalNotice)
			return
		}
		if path == "" {
			return // the user closed the dialog
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			log.Printf("floter: could not write %s: %v", path, err)
			a.queueNotice(copy.LocalToolExportFailed, refusalNotice)
			return
		}
		a.queueToast(ui.Toast{
			ID:      scriptExportNoticeID,
			Title:   copy.LocalToolScriptExported,
			Type:    "success",
			Timeout: successNotice,
		})
	}()
}

// scriptExportNoticeID identifies the export's success toast, so a second
// export replaces the first rather than stacking beside it.
const scriptExportNoticeID = "script-export"

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
// The dialog's filter follows the name it was asked to save, so a `.py` script
// is offered as a Python file and the integration document as JSON.
func (a *App) saveFile(defaultName string) (string, error) {
	if a.saveFileDialog != nil {
		return a.saveFileDialog(defaultName)
	}
	extension := strings.TrimPrefix(filepath.Ext(defaultName), ".")
	filter := mygo.FileFilter{Name: "JSON", Extensions: []string{"json"}}
	if extension != "" {
		filter = mygo.FileFilter{Name: strings.ToUpper(extension), Extensions: []string{extension}}
	}
	return mygo.Dialog.Save(mygo.SaveDialogOptions{
		Title:       "floter",
		DefaultPath: filepath.Join(homeDirOrEmpty(), defaultName),
		Filters:     []mygo.FileFilter{filter},
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

// copyConfiguration puts one integration's host-owned configuration on the
// clipboard as the reference's `extensions_config_copy` did
// (`ExtensionsPanel.tsx:2019-2031`): the same export document the file export
// writes, then the success notice. The document is built from the provider's
// real schema — never a copy the form made — so a field the form cannot show
// still redacts correctly, and the clipboard write is the one place a secret
// could leak: the redaction is what stops it.
func (a *App) copyConfiguration(id string, values map[string]any, done func(error)) {
	finish := func(err error) {
		if done != nil {
			a.onMain(func() { done(err) })
		}
	}
	description, ok := a.Integrations.Description(id)
	if !ok || description.Configuration == nil {
		finish(nil)
		return
	}
	document, err := extensions.ExportConfiguration(id, description.Configuration.ConfigVersion,
		description.Configuration.Schema, values)
	if err != nil {
		log.Printf("floter: could not build the configuration copy for %s: %v", id, err)
		a.queueNotice(a.SettingsCopy().IntegrationsTransferBad, refusalNotice)
		finish(err)
		return
	}
	// The worker the reference awaited (`extensions_config_copy` then
	// `navigator.clipboard.writeText`): the write is put on the clipboard and
	// the handler is cleared when it lands, so the button's in-flight face is
	// gone with it and a second press during the write cannot start another.
	a.notifySuccess(a.SettingsCopy().ConfigCopied)
	text := string(document)
	go func() {
		mygo.Clipboard.WriteText(text)
		finish(nil)
	}()
}

// exportConfiguration asks for a path and writes one integration's host-owned
// configuration there, as the reference's `extensions_config_export` did
// (`src-tauri/src/commands/extensions.rs:2500-2530`, `ExtensionsPanel.tsx:
// 2036-2048`). The document is the reference's own
// `ConfigurationExportDocument` shape (see `extensions.ExportConfiguration`),
// with passwords replaced by the placeholder, so a credential never leaves the
// machine in the file. A clean write raises the success notice; a refused
// dialog or a failed document is silent-or-noticed the same way the
// integrations export is.
func (a *App) exportConfiguration(id string, values map[string]any, done func(error)) {
	finish := func(err error) {
		if done != nil {
			a.onMain(func() { done(err) })
		}
	}
	description, ok := a.Integrations.Description(id)
	if !ok || description.Configuration == nil {
		finish(nil)
		return
	}
	document, err := extensions.ExportConfiguration(id, description.Configuration.ConfigVersion,
		description.Configuration.Schema, values)
	if err != nil {
		log.Printf("floter: could not build the configuration export for %s: %v", id, err)
		a.queueNotice(a.SettingsCopy().IntegrationsTransferBad, refusalNotice)
		finish(err)
		return
	}
	go func() {
		path, err := a.saveFile("floter-" + id + "-config.json")
		if err != nil {
			// The dialog failed or the user closed it: the reference's
			// `extensions_config_export` treats a closed picker as "nothing to
			// do", not as a failure to report (it returns `Ok(None)`).
			log.Printf("floter: could not pick a path for the configuration export of %s: %v", id, err)
			finish(err)
			return
		}
		if path == "" {
			finish(nil) // the user closed the dialog
			return
		}
		if err := os.WriteFile(path, document, 0o600); err != nil {
			log.Printf("floter: could not write %s: %v", path, err)
			a.queueNotice(a.SettingsCopy().IntegrationsTransferBad, refusalNotice)
			finish(err)
			return
		}
		a.notifySuccess(a.SettingsCopy().ConfigExported)
		finish(nil)
	}()
}
