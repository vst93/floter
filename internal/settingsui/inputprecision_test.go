package settingsui

import (
	"testing"

	"floter/internal/plugincfg"
)

func TestInputRefreshLargeSliderStaysFinite(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := plugincfg.Field{Key: "capacity", Label: "容量", Kind: plugincfg.Slider, Max: 1e308}
			_, _, tt := inputStateSheet(t, dark, field, 1e308)
			refreshDump(t, tt, "large-slider", dark)
			if got := boundedText(field, 1e308); got != formatFieldNumber(1e308) {
				t.Errorf("finite slider readout became %q", got)
			}
			if tt.HasText("+Inf") {
				t.Error("finite slider paints infinity")
			}
		})
	}
}

func TestInputRefreshNumberUsesSavedDecimalPrecision(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := plugincfg.Field{Key: "ratio", Label: "比例", Kind: plugincfg.Number, Max: 1, Step: 0.1}
			_, _, tt := inputStateSheet(t, dark, field, 0.3)
			inputStateNumberFocus(t, tt, field.Label)
			refreshDump(t, tt, "decimal-number", dark)
			if got := refreshRead(tt); got != "0.3" {
				t.Errorf("stored decimal paints floating tail: %q", got)
			}
			tt.Type("0.37")
			tt.Frame()
			if got := refreshRead(tt); got != "0.4" {
				t.Errorf("snapped decimal paints floating tail: %q", got)
			}
		})
	}
}
