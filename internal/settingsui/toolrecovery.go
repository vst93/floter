package settingsui

// LocalToolRecovery belongs only to the synchronous form dispatch. A detected
// tool uses the same host operation without acquiring somebody else's draft.
// The returned outlet must be invoked on the UI thread after a failed request.
func (a *App) LocalToolRecovery() func() {
	if a.toolRecover != nil {
		return a.toolRecover
	}
	return func() {}
}

func (a *App) localToolRecovery() (func(), *int) {
	retired := new(int)
	disclosure := a.toolDisclosure
	draft := a.customDraft(a.toolName, a.toolProgram, a.toolCommand, a.toolArgs,
		a.toolVersionArgs, a.toolVersion, a.toolDescription, a.toolMode, a.toolLanguage, a.toolScript)
	// Keep editable spelling and hidden mode fields, not just normalized argv.
	draft.Name, draft.Program, draft.Command = a.toolName, a.toolProgram, a.toolCommand
	draft.Args, draft.VersionArgs, draft.Version = a.toolArgs, a.toolVersionArgs, a.toolVersion
	draft.Description = a.toolDescription
	draft.ScriptLanguage, draft.ScriptContent = a.toolLanguage, a.toolScript
	params := append([]toolParam(nil), a.toolParams...)
	advanced := a.toolAdvanced
	return func() {
		if a.toolFormKey != retired || a.toolOpen || a.toolDisclosure != disclosure {
			return
		}
		a.beginEdit("", draft)
		a.toolParams = append([]toolParam(nil), params...)
		a.toolAdvanced = advanced
	}, retired
}
