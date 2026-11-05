package settingsui

import (
	"testing"

	"floter/internal/plugincfg"
)

func TestChoiceIdentityUnknownSelectValue(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := pluginIdentitySheet()
			sheet.Schema.Fields = []plugincfg.Field{{Key: "mode", Label: "搜索方式", Kind: plugincfg.Select, Options: []plugincfg.Option{{Value: "files", Label: "文件"}, {Value: "apps", Label: "应用"}}}}
			sheet.Values["mode"] = "bookmarks"
			tt := pluginIdentityFrame(t, dark, &sheet)
			choiceIdentityDump(t, tt, "unknown-select", dark)
			if !tt.HasText("bookmarks") || tt.HasText("文件") {
				t.Errorf("unknown stored option is displayed as the first choice: %q", tt.Texts())
			}
			if sheet.Values["mode"] != "bookmarks" {
				t.Fatal("render rewrote the unknown value")
			}
			if err := tt.Click("bookmarks"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if err := tt.Click("文件"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if sheet.Values["mode"] != "files" {
				t.Errorf("unknown value could not be replaced: %v", sheet.Values["mode"])
			}
		})
	}
}

func choiceUnlabelledOption(t *testing.T, kind plugincfg.Kind, dark bool) {
	t.Helper()
	name := map[plugincfg.Kind]string{plugincfg.Select: "select", plugincfg.Radio: "radio", plugincfg.Checkboxes: "checkboxes"}[kind]
	sheet := pluginIdentitySheet()
	sheet.Schema.Fields = []plugincfg.Field{{Key: "sources", Label: "搜索来源", Kind: kind, Options: []plugincfg.Option{{Value: "files"}, {Value: "apps", Label: "应用"}}}}
	sheet.Values["sources"] = "files"
	if kind == plugincfg.Checkboxes {
		sheet.Values["sources"] = []string{"files"}
	}
	tt := pluginIdentityFrame(t, dark, &sheet)
	choiceIdentityDump(t, tt, "unlabelled-"+name, dark)
	if !tt.HasText("files") {
		t.Fatalf("%s option with empty label has no visible name: %q", name, tt.Texts())
	}
	if kind == plugincfg.Select {
		if err := tt.Click("files"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	if kind != plugincfg.Checkboxes {
		if err := tt.Click("应用"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if sheet.Values["sources"] != "apps" {
			t.Fatalf("named option wrote %v", sheet.Values["sources"])
		}
		if kind == plugincfg.Select {
			if err := tt.Click("应用"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
		}
	}
	if err := tt.Click("files"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if kind == plugincfg.Checkboxes {
		if got := sheet.Values["sources"]; !configValuesEqual(got, []string{}) {
			t.Fatalf("unlabelled checkbox did not clear: %v", got)
		}
		if err := tt.Click("files"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if got := sheet.Values["sources"]; !configValuesEqual(got, []string{"files"}) {
			t.Fatalf("unlabelled checkbox wrote %v", got)
		}
	} else if sheet.Values["sources"] != "files" {
		t.Errorf("fallback label changed the stored ID: %v", sheet.Values["sources"])
	}
}

func TestChoiceIdentityUnlabelledSelect(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) { choiceUnlabelledOption(t, plugincfg.Select, dark) })
	}
}

func TestChoiceIdentityUnlabelledRadio(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) { choiceUnlabelledOption(t, plugincfg.Radio, dark) })
	}
}

func TestChoiceIdentityUnlabelledCheckboxes(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) { choiceUnlabelledOption(t, plugincfg.Checkboxes, dark) })
	}
}
