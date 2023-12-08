package tray

import (
	"bytes"
	"image/png"
	"runtime"

	"fyne.io/systray"
	toicon "github.com/Kodeworks/golang-image-ico"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/icon"
	"github.com/wzshiming/jumpway/log"
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

	if runtime.GOOS == "windows" {
		buf := bytes.NewBuffer(nil)
		img, err := png.Decode(bytes.NewReader(ico))
		if err != nil {
			log.Error(err, "Unable to decode icon")
		} else {
			err = toicon.Encode(buf, img)
			if err != nil {
				log.Error(err, "Unable to encode icon")
			} else {
				ico = buf.Bytes()
				systray.SetTemplateIcon(ico, ico)
			}
		}
	} else {
		systray.SetTemplateIcon(ico, ico)
	}

	systray.SetTitle("")
	systray.SetTooltip("Jump Way")

	mStatus := systray.AddMenuItem("", "")
	a.ItemStatus(mStatus)

	systray.AddSeparator()

	mDaemon := systray.AddMenuItemCheckbox(i18n.Daemon(), "", false)
	go a.ItemDaemon(mDaemon)

	systray.AddSeparator()

	mManualMode := systray.AddMenuItemCheckbox(i18n.ManualProxy(), "", false)
	mGlobalMode := systray.AddMenuItemCheckbox(i18n.SystemProxy(), "", false)
	go a.ItemProxyMode(mGlobalMode, mManualMode)

	mExportCommand := systray.AddMenuItem(i18n.ExportCommand(), "")
	go a.ItemExportCommand(mExportCommand)

	systray.AddSeparator()

	mConfig := systray.AddMenuItem(i18n.Config(), "")
	{
		mEditConfig := mConfig.AddSubMenuItem(i18n.EditConfig(), "")
		go a.ItemEditConfig(mEditConfig)
		mReloadConfig := mConfig.AddSubMenuItem(i18n.ReloadConfig(), "")
		go a.ItemReloadConfig(mReloadConfig)
		mView := mConfig.AddSubMenuItem(i18n.ViewEditConfig(), "")
		go a.ItemView(mView)
	}

	systray.AddSeparator()

	mLog := systray.AddMenuItem(i18n.Log(), "")
	go a.ItemLog(mLog)

	mAbout := systray.AddMenuItem(i18n.About(), "")
	go a.ItemAbout(mAbout)

	mQuit := systray.AddMenuItem(i18n.Quit(), "")
	go a.ItemQuit(mQuit)
}
