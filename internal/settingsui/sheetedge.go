package settingsui

import (
	"github.com/egoist/mygo/ui"

	"floter/internal/theme"
)

// sheetEdgeFace keeps the scroll veil translucent so it does not paint a solid
// stripe across the window's glass. The clipping viewport contains the text.
func sheetEdgeFace(tokens theme.Tokens, _ *ui.Theme) ui.Color {
	return tokens.Sunken
}
