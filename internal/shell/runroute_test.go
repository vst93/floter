package shell

import (
	"os"
	"path/filepath"
	"testing"

	"floter/internal/extensions"
)

// integrationList projects the manifest's declared run route onto the row, so
// the settings run form can state where a run's output lands. The reference
// read the same field to pick its hint (`src/extensions/ExtensionRow.tsx`
// hands `extension.output` to `RunParamForm`, which selects
// `customRunOutputHint*`). A manifest with no route is a background run.
func TestTheRowCarriesTheManifestsRunRoute(t *testing.T) {
	for _, tc := range []struct {
		name    string
		declare string
		want    string
	}{
		{"declared terminal", `, "output": "terminal"`, extensions.RouteTerminal},
		{"undeclared", "", extensions.RouteBackground},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paths := extensions.FromRoot(t.TempDir())
			if err := paths.Ensure(); err != nil {
				t.Fatal(err)
			}
			pkg := filepath.Join(paths.Extensions, "dev.floter.route")
			if err := os.MkdirAll(pkg, 0o755); err != nil {
				t.Fatal(err)
			}
			manifest := `{
  "schemaVersion": "2.0", "id": "dev.floter.route", "name": "Route Tool",
  "distribution": {"type": "local"},
  "runtime": {"type": "system", "executableNames": ["routetool"]},
  "provider": {"type": "executable", "argsPrefix": []}` + tc.declare + `
}`
			if err := os.WriteFile(filepath.Join(pkg, "floter.extension.json"), []byte(manifest), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(paths.RepositoryFile, []byte(`{
  "schemaVersion": 1,
  "extensions": {
    "dev.floter.route": {
      "id": "dev.floter.route", "name": "Route Tool", "state": "enabled", "enabled": true,
      "distributionSource": "local", "runtimeOwnership": "system",
      "packageVersion": "1.0.0", "manifestPath": "",
      "executablePath": "/nonexistent/routetool",
      "channel": "stable", "installedAt": 1, "updatedAt": 1
    }
  }
}`), 0o644); err != nil {
				t.Fatal(err)
			}
			a := reconnectApp(t, paths, &noticeLog{})
			list := a.integrationList()
			if len(list) != 1 || list[0].OutputMode != tc.want {
				t.Fatalf("rows = %+v, want one with OutputMode %q", list, tc.want)
			}
		})
	}
}
