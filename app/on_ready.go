package app

import (
	"github.com/getlantern/systray"
	"github.com/wzshiming/jumpway/icon"
)

func (a *App) onReady() {
	systray.SetTemplateIcon(icon.Icon, icon.Icon)
	systray.SetTitle("")
	systray.SetTooltip("Jump Way")

	systray.AddSeparator()

	mDaemon := systray.AddMenuItemCheckbox("Daemon", "Daemon", false)
	go a.ItemDaemon(mDaemon)

	mSystemProxy := systray.AddMenuItemCheckbox("System Proxy", "System Proxy with http(s)", false)
	go a.ItemSystemProxy(mSystemProxy)

	mConfig := systray.AddMenuItem("Config", "Config")
	{
		mEditConfig := mConfig.AddSubMenuItem("Edit", "Edit config")
		go a.ItemEditConfig(mEditConfig)
		mReloadConfig := mConfig.AddSubMenuItem("Reload", "Reload config")
		go a.ItemReloadConfig(mReloadConfig)
	}

	systray.AddSeparator()

	mAbout := systray.AddMenuItem("About", "About the project")
	go a.ItemAbout(mAbout)

	mQuit := systray.AddMenuItem("Quit", "Quit the app")
	go a.ItemQuit(mQuit)
}
