package shell

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"

	"floter/internal/extensions"
)

// Exporting and importing the integration list, through the native file
// dialogs: a document of what is installed and how it is configured, with the
// secrets left behind (see internal/extensions/sync.go).

// exportIntegrations asks for a file and writes the document.
func (a *App) exportIntegrations() {
	go func() {
		copy := a.SettingsCopy()
		document, err := extensions.ExportSync(a.Paths)
		if err != nil {
			log.Printf("floter: could not export the integrations: %v", err)
			a.setTransferStatus(copy.IntegrationsTransferBad)
			return
		}
		path, err := a.saveFile("floter-integrations-" + time.Now().Format("2006-01-02") + ".json")
		if err != nil || path == "" {
			return // the user closed the dialog
		}
		data, err := document.Marshal()
		if err != nil {
			log.Printf("floter: could not encode the integrations: %v", err)
			a.setTransferStatus(copy.IntegrationsTransferBad)
			return
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
			log.Printf("floter: could not write %s: %v", path, err)
			a.setTransferStatus(copy.IntegrationsTransferBad)
			return
		}
		a.setTransferStatus(copy.IntegrationsExported(len(document.Extensions), path))
	}()
}

// importIntegrations asks for a file and applies it.
func (a *App) importIntegrations() {
	go func() {
		copy := a.SettingsCopy()
		path, err := a.openFile()
		if err != nil || path == "" {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("floter: could not read %s: %v", path, err)
			a.setTransferStatus(copy.IntegrationsTransferBad)
			return
		}
		document, err := extensions.ReadSync(data)
		if err != nil {
			log.Printf("floter: could not read %s: %v", path, err)
			a.setTransferStatus(copy.IntegrationsTransferBad)
			return
		}
		report, err := extensions.ImportSync(a.Paths, document, a.confirmInstallPermissions)
		if err != nil {
			log.Printf("floter: could not import %s: %v", path, err)
			a.setTransferStatus(copy.IntegrationsTransferBad)
			return
		}
		for _, failure := range report.Failed {
			log.Printf("floter: %s was not imported: %s", failure.ID, failure.Reason)
		}
		for _, skipped := range report.Skipped {
			log.Printf("floter: %s was skipped: %s", skipped.ID, skipped.Reason)
		}
		a.setTransferStatus(copy.IntegrationsImported(len(report.Succeeded), len(report.Failed), len(report.Skipped)))
		a.RefreshIntegrations(context.Background())
	}()
}

// setTransferStatus puts a line on the Integrations page.
func (a *App) setTransferStatus(status string) {
	a.onMain(func() {
		if a.Settings == nil {
			return
		}
		a.Settings.TransferStatus = status
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
