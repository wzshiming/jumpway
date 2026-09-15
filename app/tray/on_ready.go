package tray

import (
	"github.com/wzshiming/jumpway/icon"
	"github.com/wzshiming/systheme"
)

func (a *App) onReady() {
	ico := icon.Black
	switch t, _ := systheme.GetTheme(); t {
	case systheme.Light:
		ico = icon.Black
	case systheme.Dark:
		ico = icon.White
	case systheme.Unknown:
		ico = icon.Gray
	}

	a.tray.
		SetIcon(ico).
		SetTemplateIcon(ico).
		SetTooltip("Jump Way").
		SetMenu(a.buildMenu()).
		Show()
	a.do(func() {
		a.selectSystemProxy("")
		_ = a.reload()
	})
}
