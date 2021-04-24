package app

import (
	"github.com/getlantern/systray"
	"github.com/wzshiming/jumpway/icon"
)

func (a *App) onReady() {
	systray.SetTemplateIcon(icon.White, icon.White)
	systray.SetTitle("")
	systray.SetTooltip("Jump Way")

	mStatus := systray.AddMenuItem("", "")
	a.ItemStatus(mStatus)

	systray.AddSeparator()

	mDaemon := systray.AddMenuItemCheckbox("Daemon", "", false)
	go a.ItemDaemon(mDaemon)

	systray.AddSeparator()

	mManualMode := systray.AddMenuItemCheckbox("Manual Mode", "", false)
	mGlobalMode := systray.AddMenuItemCheckbox("Global Mode", "", false)
	go a.ItemProxyMode(mGlobalMode, mManualMode)

	mExportCommand := systray.AddMenuItem("Proxy Export Line To Clipboard", "")
	go a.ItemExportCommand(mExportCommand)

	systray.AddSeparator()

	mConfig := systray.AddMenuItem("Config", "")
	{
		mEditConfig := mConfig.AddSubMenuItem("Edit", "")
		go a.ItemEditConfig(mEditConfig)
		mReloadConfig := mConfig.AddSubMenuItem("Reload", "")
		go a.ItemReloadConfig(mReloadConfig)
	}

	systray.AddSeparator()

	mAbout := systray.AddMenuItem("About", "")
	go a.ItemAbout(mAbout)

	mQuit := systray.AddMenuItem("Quit", "")
	go a.ItemQuit(mQuit)
}
