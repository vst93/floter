package shell

import (
	"math"

	"github.com/egoist/mygo"
)

// SettingsHeight keeps a useful reading area at every control size. Content
// scrolls inside a 580-DIP panel, limited to 80% of the active work area.
func SettingsHeight(_ string, workAreaHeight float64) int {
	if workAreaHeight <= 0 {
		return settingsWindowHeight
	}
	return max(1, int(math.Floor(min(float64(settingsWindowHeight), workAreaHeight*0.8, workAreaHeight-24))))
}

// panelHeightLimit also protects plugin configuration sheets and first frames
// when the platform has not reported a work area yet.
func (a *App) panelHeightLimit() int {
	if a.workArea <= 0 {
		return 720
	}
	return max(1, int(math.Floor(min(720, a.workArea-24))))
}

func (a *App) clampPanelHeight(height int) int {
	if a.Surf == SurfaceTerminal {
		return height
	}
	limit := a.panelHeightLimit()
	if a.Surf == SurfaceSettings {
		limit = SettingsHeight(a.settings().UIScale, a.workArea)
	}
	return min(height, limit)
}

// Refresh after a move and before a surface switch: the primary display's
// dimensions may have nothing to do with the screen holding this window.
func (a *App) refreshWorkArea() {
	if a.Win == nil {
		return
	}
	x, y := a.Win.Position()
	w, h := a.Win.Size()
	if d, ok := displayContaining(mygo.Screen.Displays(), x, y, w, h); ok && d.WorkArea.Height > 0 {
		a.workArea = float64(d.WorkArea.Height)
	}
}

func (a *App) applyHeightLimit() {
	if a.Win == nil {
		return
	}
	limit := 0
	if a.Surf != SurfaceTerminal {
		limit = a.clampPanelHeight(a.panelHeightLimit())
	}
	if a.lastMaxH != limit {
		a.Win.SetMaximumSize(0, limit)
		a.lastMaxH = limit
	}
}

// A panel may have been summoned while short and later grown. Its height
// can fit the display while its bottom still falls below the work area.
func (a *App) fitPanelPosition(height int) {
	if a.Win == nil || a.Surf == SurfaceTerminal {
		return
	}
	x, y := a.Win.Position()
	w, h := a.Win.Size()
	display, ok := displayContaining(mygo.Screen.Displays(), x, y, w, h)
	if !ok || display.WorkArea.Height <= 0 {
		return
	}
	area := display.WorkArea
	if next := panelY(y, area.Y, area.Height, height); next != y {
		a.Win.SetPosition(x, next)
	}
}

func panelY(y, areaY, areaHeight, height int) int {
	return max(areaY, min(y, areaY+areaHeight-height-12))
}
