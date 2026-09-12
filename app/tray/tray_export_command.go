package tray

import (
	"fmt"
	"net"

	"github.com/atotto/clipboard"
	"github.com/gogpu/systray"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
)

func (a *App) ItemExportCommand(menu *systray.Menu) {
	sub := systray.NewMenu()
	sub.Add("Shell", func() { a.do(a.exportCommandShell) })
	sub.Add("Cmd", func() { a.do(a.exportCommandCmd) })
	sub.Add("PowerShell", func() { a.do(a.exportCommandPowerShell) })
	sub.Add("Shell git", func() { a.do(a.exportCommandShellGit) })
	menu.AddSubmenu(i18n.ExportCommand(), sub)
}

func (a *App) exportCommandShell() {
	command := fmt.Sprintf("export http_proxy=http://%s https_proxy=http://%s; ", a.Address, a.Address)
	a.writeClipboard(command)
}

func (a *App) exportCommandCmd() {
	command := fmt.Sprintf("set http_proxy=http://%s && set https_proxy=http://%s", a.Address, a.Address)
	a.writeClipboard(command)
}

func (a *App) exportCommandPowerShell() {
	command := fmt.Sprintf("$env:http_proxy='http://%s'; $env:https_proxy='http://%s'; ", a.Address, a.Address)
	a.writeClipboard(command)
}

func (a *App) exportCommandShellGit() {
	host, port, _ := net.SplitHostPort(a.Address)
	command := fmt.Sprintf("export GIT_SSH_COMMAND='ssh -o ProxyCommand=\"nc -x %s:%s %%h %%p\"' http_proxy=http://%s https_proxy=http://%s; ", host, port, a.Address, a.Address)
	a.writeClipboard(command)
}

func (a *App) writeClipboard(command string) {
	err := clipboard.WriteAll(command)
	if err != nil {
		log.Error(err, i18n.WriteClipboard())
	}
}
