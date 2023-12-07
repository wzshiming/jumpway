package tray

import (
	"fyne.io/systray"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/icon"
)

func (a *App) onReady() {
	systray.SetTemplateIcon(icon.White, icon.White)
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
