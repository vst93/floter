package launcher

import (
	"testing"
)

// The captured output takes the *list's* place, under the field row it keeps.
//
// The reference prints a command's output under the search field, in the slot
// `LauncherResults` takes (`main:src/App.tsx:3634-3642`: `pluginText` inside
// `.launcher-bottom`), so the field row is still there above it — it is the
// command line the run came from, and it stays readable and editable.
//
// The port returned from the whole panel instead: `View` saw `a.output != nil`
// and drew the output page *instead of* the surface, so no field row was built
// and the band it would have occupied was left as blank glass above the strip.
// The user read that as 「展示结果上方会有一块和输入框同等高度的空白」, and the
// second half of the same report — 「之前应该是直接在搜索页下显示的，输入框应该
// 保持，只是列表部分展示结果」 — is what this pin holds.
func TestTheOutputTakesTheListsPlaceUnderTheField(t *testing.T) {
	a := testApp()
	tt := runListOutput(t, a, longOutput(6))

	// ① The field row is still drawn, with its own box above the output.
	field, ok := tt.Find("Search")
	if !ok {
		t.Fatalf("the output page draws no search field: %q", tt.Texts())
	}
	if field.H <= 0 {
		t.Fatalf("the field's box is empty: %+v", field)
	}

	// ② The field's own text sits in the band the page spends on it, and the
	//    output starts below that band — not over the field.
	spacing := float32(a.Spacing)
	bandEnd := spacing*(TopInsetUnits+fieldRowUnits) + spacing*breathUnits
	title := mustFind(t, tt, a.output.Title)
	if field.Y+field.H > bandEnd+1 {
		t.Errorf("the field's box ends at y=%.1f, past the band the page spends on it (%.1f)",
			field.Y+field.H, bandEnd)
	}
	if title.Y < bandEnd-1 {
		t.Errorf("the output's title starts at y=%.1f, inside the field row's band (ends %.1f): "+
			"the output is drawn over the field instead of under it", title.Y, bandEnd)
	}

	// ③ …and the gap is exactly the breath the result list itself gets, so no
	//    second field band is reserved above the strip.
	if gap := title.Y - bandEnd; gap > spacing*2 {
		t.Errorf("the output's title starts %.1fu below the field band, want at most one breath (%.1f): "+
			"a reserved field band sits above it", gap, spacing*2)
	}
}
