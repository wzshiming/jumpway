package tray

import (
	"errors"
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
	address := a.primaryAddress()
	if address == "" {
		log.Error(errors.New(i18n.NoLocalRule()), i18n.ExportCommand())
		return
	}
	command := fmt.Sprintf("export http_proxy=http://%s https_proxy=http://%s; ", address, address)
	a.writeClipboard(command)
}

func (a *App) exportCommandCmd() {
	address := a.primaryAddress()
	if address == "" {
		log.Error(errors.New(i18n.NoLocalRule()), i18n.ExportCommand())
		return
	}
	command := fmt.Sprintf("set http_proxy=http://%s && set https_proxy=http://%s", address, address)
	a.writeClipboard(command)
}

func (a *App) exportCommandPowerShell() {
	address := a.primaryAddress()
	if address == "" {
		log.Error(errors.New(i18n.NoLocalRule()), i18n.ExportCommand())
		return
	}
	command := fmt.Sprintf("$env:http_proxy='http://%s'; $env:https_proxy='http://%s'; ", address, address)
	a.writeClipboard(command)
}

func (a *App) exportCommandShellGit() {
	address := a.primaryAddress()
	if address == "" {
		log.Error(errors.New(i18n.NoLocalRule()), i18n.ExportCommand())
		return
	}
	host, port, _ := net.SplitHostPort(address)
	command := fmt.Sprintf("export GIT_SSH_COMMAND='ssh -o ProxyCommand=\"nc -x %s:%s %%h %%p\"' http_proxy=http://%s https_proxy=http://%s; ", host, port, address, address)
	a.writeClipboard(command)
}

func (a *App) writeClipboard(command string) {
	err := clipboard.WriteAll(command)
	if err != nil {
		log.Error(err, i18n.WriteClipboard())
	}
}
