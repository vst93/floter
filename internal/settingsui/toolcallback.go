package settingsui

// Native callbacks can pump UI events before returning. Only the frame that
// still owns these raw fields may write its local copies back to the form.
// Keep spelling and empty strings: normalization would hide a newer edit.
type toolCallbackState struct {
	form       *int
	open       bool
	disclosure uint64
	pageEpoch  uint64
	fields     [11]string
}

func (a *App) toolCallbackState() toolCallbackState {
	return toolCallbackState{
		form:       a.toolFormKey,
		open:       a.toolOpen,
		disclosure: a.toolDisclosure,
		pageEpoch:  a.toolPageEpoch,
		fields: [11]string{a.toolEditing, a.toolName, a.toolProgram, a.toolCommand,
			a.toolArgs, a.toolVersionArgs, a.toolVersion, a.toolDescription,
			a.toolMode, a.toolLanguage, a.toolScript},
	}
}
