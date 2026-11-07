//go:build !windows

package shell

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDiagnosisRecoveryCommands(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			paths, id := driftAppFixture(t, "1.0.0", "1.0.0")
			a := driftApp(t, paths)
			owner, _ := a.Integrations.Inventory().WithID(id)
			program := owner.Entry.ExecutablePath
			script, err := os.ReadFile(program)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(program, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			a.runCommandReprobe(owner, a.beginDiagnosis(id))
			if !a.integrationList()[0].DiagnosisFailed {
				t.Fatal("initial command failure never arrived")
			}
			if err := os.WriteFile(program, script, 0o755); err != nil {
				t.Fatal(err)
			}
			// Join the same worker the settings action starts before rendering.
			a.runCommandReprobe(owner, a.beginDiagnosis(id))
			if len(a.Integrations.CommandRegistry()) != 3 {
				t.Fatal("command recovery did not refresh the descriptor")
			}
			f := &diagnosisFixture{a: a, paths: paths, entry: owner.Entry}
			diagnosisRecoveryFrame(t, f, "commands-recovered", dark)
			row := a.integrationList()[0]
			if row.Diagnosis != "" || row.DiagnosisFailed || row.DiagnosisStatus != "" || len(row.DiagnosisChecks) != 0 {
				t.Errorf("successful command retry retained old diagnosis: %q failed=%v", row.Diagnosis, row.DiagnosisFailed)
			}
			// The successful retry really rebuilt the descriptor on disk.
			if _, err := os.Stat(filepath.Join(filepath.Dir(owner.Entry.ManifestPath), "help-probe.json")); err != nil {
				t.Fatal(err)
			}
			a.Integrations.Refresh(context.Background())
		})
	}
}
