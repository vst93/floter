package shell

import (
	"strings"
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
)

func TestDiagnosisSummaryCombinedFailures(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := diagnosisFailedLifecycle(t, dark)
			f.request(t, 0, "error", []extensions.DiagnosisCheck{{ID: "account", Status: "error", Message: "需要重新登录"}}, true)
			diagnosisWait(t, f.start(t))
			tt := diagnosisBoundaryFrame(t, f, "combined-failures", dark)
			row := f.a.integrationList()[0]
			if !row.DiagnosisFailed || row.DiagnosisStatus != "error" || !strings.Contains(row.Diagnosis, "unhealthy") || !strings.Contains(row.Diagnosis, "需要重新登录") {
				t.Errorf("provider findings erased lifecycle reason: %q", row.Diagnosis)
			}
			if !tt.HasText("Unhealthy; 需要重新登录") || !tt.HasText("需要重新登录") {
				t.Error("combined failure summary hides one stage behind provider status")
			}
		})
	}
}

func TestDiagnosisSummarySuccessSpelling(t *testing.T) {
	for _, status := range []string{" ok ", "OK"} {
		for _, dark := range []bool{false, true} {
			t.Run(status+"/"+map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
				f := newDiagnosisFixture(t, dark)
				f.a.onScreen = func() bool { return true }
				f.a.Open(SurfaceSettings)
				clearNotices(f.a)
				f.request(t, 0, status, []extensions.DiagnosisCheck{{ID: "connection", Status: "ok", Message: "连接检查正常"}}, true)
				diagnosisWait(t, f.start(t))
				notices := noticeTitles(f.a)
				diagnosisBoundaryFrame(t, f, "summary-spelling-"+strings.TrimSpace(status), dark)
				row := f.a.integrationList()[0]
				if row.DiagnosisFailed || len(notices) != 1 || notices[0] != i18n.NotificationsFor("zh").IntegrationChecked(f.manifest.Name) {
					t.Errorf("successful provider status is painted/notified as failure: status=%q failed=%v notices=%q", status, row.DiagnosisFailed, notices)
				}
				if row.DiagnosisStatus != status {
					t.Error("provider's original status spelling was lost")
				}
			})
		}
	}
}
