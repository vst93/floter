package shell

// A native callback can have been queued before the recorder released its
// key. Input belongs to the recorder until the host resumes its claims.
func (a *App) summonCallback() func() {
	return func() {
		if !a.shortcutsSuspended && !a.Settings.RecordingShortcut() {
			a.Toggle()
		}
	}
}
