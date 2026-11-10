package glassmap

import "testing"

func TestReadableOpacityHasNoDeadRange(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "windows"} {
		for _, step := range []string{"off", "frosted", "regular", "liquid"} {
			var previous, previousContent float32
			for opacity := uint8(72); opacity <= 100; opacity++ {
				s := SpecForOS(goos, step, opacity)
				if diff(s.Composite(), float32(opacity)/100) {
					t.Fatalf("%d%% renders as %v", opacity, s.Composite())
				}
				content := 1 - (1-s.Composite())*(1-s.ContentAlpha())
				if s.Composite() <= previous || content <= previousContent {
					t.Fatalf("%s/%s/%d: dead slider range", goos, step, opacity)
				}
				previous, previousContent = s.Composite(), content
			}
			low, high := SpecForOS(goos, step, 72), SpecForOS(goos, step, 100)
			if diff(low.Composite(), .72) || diff(high.Composite(), 1) {
				t.Fatalf("%s/%s readable bounds %v..%v", goos, step, low.Composite(), high.Composite())
			}
		}
	}
}

func TestOpacityRecommendationsSnapWithinOnePoint(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		for _, high := range []bool{false, true} {
			for _, stop := range OpacityStops(high, terminal) {
				for _, offset := range []float64{-1, 0, 1} {
					if got := SnapOpacity(stop+offset, high, terminal); got != stop {
						t.Fatalf("%v + %v snaps to %v", stop, offset, got)
					}
				}
			}
		}
	}
	for _, v := range []float64{77, 80, 83, 89, 92, 95} {
		if got := SnapOpacity(v, false, false); got != v {
			t.Fatalf("off-stop %v snapped to %v", v, got)
		}
	}
}

func TestTerminalOpacityRangeAndSolidEffects(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "windows"} {
		for _, step := range []string{"off", "frosted", "regular", "liquid"} {
			for opacity := uint8(66); opacity <= 100; opacity++ {
				s := TerminalSpecForPreferencesOS(goos, step, opacity, false)
				if diff(s.Composite(), float32(opacity)/100) {
					t.Fatalf("%s/%s terminal %d%% is remapped to %v", goos, step, opacity, s.Composite())
				}
			}
			for _, terminal := range []bool{false, true} {
				s := SpecForOS(goos, step, 100)
				if terminal {
					s = TerminalSpecForPreferencesOS(goos, step, 100, false)
				}
				if s.TintAlpha != 1 || s.Glass || s.Material != 0 || s.Blur != 0 || s.LensScale != 0 || s.HazeAlpha != 0 {
					t.Fatalf("100%% keeps a background effect: %+v", s)
				}
			}
		}
	}
}
