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

func (a *App) addExportCommands(menu *systray.Menu, rule ruleMenuEntry) *systray.MenuItem {
	sub := systray.NewMenu()
	sub.Add("Shell", func() { a.do(func() { a.exportCommandShell(a.ruleAddress(rule.name)) }) })
	sub.Add("Cmd", func() { a.do(func() { a.exportCommandCmd(a.ruleAddress(rule.name)) }) })
	sub.Add("PowerShell", func() { a.do(func() { a.exportCommandPowerShell(a.ruleAddress(rule.name)) }) })
	sub.Add("Shell git", func() { a.do(func() { a.exportCommandShellGit(a.ruleAddress(rule.name)) }) })
	return menu.AddSubmenu(rule.label, sub)
}

func (a *App) exportCommandShell(address string) {
	if address == "" {
		log.Error(errors.New(i18n.NoLocalRule()), i18n.ExportCommand())
		return
	}
	command := fmt.Sprintf("export http_proxy=http://%s https_proxy=http://%s; ", address, address)
	a.writeClipboard(command)
}

func (a *App) exportCommandCmd(address string) {
	if address == "" {
		log.Error(errors.New(i18n.NoLocalRule()), i18n.ExportCommand())
		return
	}
	command := fmt.Sprintf("set http_proxy=http://%s && set https_proxy=http://%s", address, address)
	a.writeClipboard(command)
}

func (a *App) exportCommandPowerShell(address string) {
	if address == "" {
		log.Error(errors.New(i18n.NoLocalRule()), i18n.ExportCommand())
		return
	}
	command := fmt.Sprintf("$env:http_proxy='http://%s'; $env:https_proxy='http://%s'; ", address, address)
	a.writeClipboard(command)
}

func (a *App) exportCommandShellGit(address string) {
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
