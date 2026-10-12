package settingsui

import (
	"testing"
	"time"

	"floter/internal/i18n"
)

// The thresholds are the old build's `relativeTime`: sub-45s is the caller's
// "just now", then the largest unit whose threshold the age is under, rounded
// to nearest. These are the boundaries, on both languages the app ships.
func TestRelativeAgeThresholds(t *testing.T) {
	for _, tc := range []struct {
		age       time.Duration
		wantValue int
		wantUnit  i18n.RelativeUnit
	}{
		{45 * time.Second, 1, i18n.RelativeMinute},
		{59 * time.Second, 1, i18n.RelativeMinute},
		{60 * time.Second, 1, i18n.RelativeMinute},
		{90 * time.Second, 2, i18n.RelativeMinute},
		{3599 * time.Second, 60, i18n.RelativeMinute},
		{3600 * time.Second, 1, i18n.RelativeHour},
		{86399 * time.Second, 24, i18n.RelativeHour},
		{86400 * time.Second, 1, i18n.RelativeDay},
		{172800 * time.Second, 2, i18n.RelativeDay},
		{2591999 * time.Second, 30, i18n.RelativeDay},
		{2592000 * time.Second, 1, i18n.RelativeMonth},
		{31535999 * time.Second, 12, i18n.RelativeMonth},
		{31536000 * time.Second, 1, i18n.RelativeYear},
		{63072000 * time.Second, 2, i18n.RelativeYear},
	} {
		value, unit := relativeAge(tc.age)
		if value != tc.wantValue || unit != tc.wantUnit {
			t.Errorf("relativeAge(%v) = (%d, %q), want (%d, %q)",
				tc.age, value, unit, tc.wantValue, tc.wantUnit)
		}
	}
}

// Under 45 seconds the caller's own word is used, in both languages; past it
// the localized relative time, from minutes through years.
func TestFreshnessWhenIsLocalized(t *testing.T) {
	en := i18n.For("en").Settings
	zh := i18n.For("zh").Settings
	for _, tc := range []struct {
		age    time.Duration
		wantEN string
		wantZH string
	}{
		{10 * time.Second, "just now", "刚刚"},
		{44 * time.Second, "just now", "刚刚"},
		{45 * time.Second, "1 min. ago", "1分钟前"},
		{30 * time.Minute, "30 min. ago", "30分钟前"},
		{3 * time.Hour, "3 hr. ago", "3小时前"},
		{25 * time.Hour, "yesterday", "昨天"},
		{2 * 24 * time.Hour, "2 days ago", "前天"},
		{30 * 24 * time.Hour, "last mo.", "上个月"},
		{60 * 24 * time.Hour, "2 mo. ago", "2个月前"},
		{400 * 24 * time.Hour, "last yr.", "去年"},
		{800 * 24 * time.Hour, "2 yr. ago", "2年前"},
	} {
		if got := freshnessWhen(en, tc.age); got != tc.wantEN {
			t.Errorf("freshnessWhen(en, %v) = %q, want %q", tc.age, got, tc.wantEN)
		}
		if got := freshnessWhen(zh, tc.age); got != tc.wantZH {
			t.Errorf("freshnessWhen(zh, %v) = %q, want %q", tc.age, got, tc.wantZH)
		}
	}
}
