package plugincfg

// The write path behind the plugin configuration overlay.
//
// The old build's R29 painted a change immediately and fired its write off
// without waiting; the write swallowed its own failure, so a refused write left
// the control showing a value the backend never stored — the review's 「静默
// 失效」. R113 gave the write a result: `writeConfigChange` turns a rejection
// into `false` (it catches on purpose, but hides nothing), and
// `configChangeOutcome` then says what the overlay does with it, the same
// optimistic-rollback shape the settings surface uses: accepted advances the
// committed snapshot, refused restores the painted values to it and names the
// failure line.
//
// This file is that decision, pure. The overlay paints, the shell writes, and
// `ConfigChangeOutcome` is the one place that says which snapshot survives, so
// a control can never keep showing a value that was never stored.

// ConfigChangeOutcome is the old build's `configChangeOutcome`: what one
// painted change resolves to once its write answers.
//
// `next` is the values the control painted; `committed` is the snapshot the
// backend last confirmed (the values before this change); `accepted` is the
// write's own result. On acceptance the painted values stand and the failure
// line clears; on refusal the committed snapshot comes back and `failed` is
// true, so the sheet prints its one line instead of keeping a value the plugin
// never stored.
//
// The caller may replace the returned values with the backend's own
// normalization after an accepted write (the plugin has the last word); the
// rollback values are never replaced, which is the half R113 is about.
func ConfigChangeOutcome(next, committed Values, accepted bool) (values Values, failed bool) {
	if accepted {
		return next, false
	}
	return committed, true
}
