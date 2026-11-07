package shell

import (
	"strings"
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
)

func TestDiagnosisMessageSuccessSpelling(t *testing.T) {
	for _, status := range []string{" ok ", "OK"} {
		for _, dark := range []bool{false, true} {
			t.Run(status+"/"+map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
				f := newDiagnosisFixture(t, dark)
				f.a.onScreen = func() bool { return true }
				f.a.Open(SurfaceSettings)
				clearNotices(f.a)
				f.request(t, 0, "ok", []extensions.DiagnosisCheck{{ID: "connection", Status: status, Message: "连接检查正常"}}, true)
				diagnosisWait(t, f.start(t))
				notices := noticeTitles(f.a)
				tt := diagnosisBoundaryFrame(t, f, "check-spelling-"+strings.TrimSpace(status), dark)
				row := f.a.integrationList()[0]
				if row.DiagnosisFailed || row.Diagnosis != "ok" || len(notices) != 1 || notices[0] != i18n.NotificationsFor("zh").IntegrationChecked(f.manifest.Name) {
					t.Errorf("successful individual check turns overall diagnosis into a failure: status=%q summary=%q notices=%q", status, row.Diagnosis, notices)
				}
				if len(row.DiagnosisChecks) != 1 || row.DiagnosisChecks[0].Status != status || !tt.HasText("连接检查正常") {
					t.Error("original check spelling or detail was lost")
				}
			})
		}
	}
}

func TestDiagnosisMessageWhitespaceFailure(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := newDiagnosisFixture(t, dark)
			f.a.Open(SurfaceSettings)
			f.request(t, 0, "ok", []extensions.DiagnosisCheck{{ID: "network.connection", Status: "error", Message: " \t "}}, true)
			diagnosisWait(t, f.start(t))
			tt := diagnosisBoundaryFrame(t, f, "whitespace-failure", dark)
			row := f.a.integrationList()[0]
			if !row.DiagnosisFailed || row.Diagnosis != "network.connection" || !tt.HasText("Network.connection") || tt.HasText("Ok") {
				t.Errorf("blank failure message leaves an OK summary instead of check identity: summary=%q", row.Diagnosis)
			}
			if len(row.DiagnosisChecks) != 1 || row.DiagnosisChecks[0].Message != " \t " || !tt.HasText("network.connection") {
				t.Error("raw check data or existing detail fallback was lost")
			}
		})
	}
}
