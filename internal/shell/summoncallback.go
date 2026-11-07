package shell

func (a *App) summonCallback() func() {
	return func() {
		if !a.shortcutsSuspended && !a.Settings.RecordingShortcut() {
			a.Toggle()
		}
	}
}

// Claim a tentative key without replacing the working callback. The caller
// activates it only after its write succeeds; failed attempts never become
// dispatchable, even if the same key is successfully claimed later.
func (a *App) registerSummonShortcut(key string) (activate func(), err error) {
	registration := new(bool)
	err = a.registerShortcut(key, func() {
		if a.summonRegistration == registration && !a.summonNeedsRestore &&
			!a.shortcutsSuspended && !a.Settings.RecordingShortcut() {
			a.Toggle()
		}
	})
	return func() { a.summonRegistration = registration }, err
}
