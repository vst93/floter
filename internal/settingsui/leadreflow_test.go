package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func leadReflowDump(t *testing.T, tt *ui.Tester, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13LA_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-la-run-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLeadReflowRunEditors(t *testing.T) {
	for _, kind := range []string{"text", "number", "path"} {
		t.Run(kind, func(t *testing.T) { checkCatalogReflow(t, "run-controls", kind, "local", "recommended", "detected") })
	}
}

func TestLeadReflowRunChoices(t *testing.T) {
	for _, kind := range []string{"boolean", "select"} {
		t.Run(kind, func(t *testing.T) { checkCatalogReflow(t, "run-controls", kind, "local", "recommended", "detected") })
	}
}
