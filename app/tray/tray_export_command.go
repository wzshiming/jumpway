package tray

import (
	"errors"
	"fmt"

	"github.com/atotto/clipboard"
	"github.com/gogpu/systray"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
)

type exportKind struct {
	label   string
	command func(address string) string
}

var exportKinds = []exportKind{
	{label: "Shell", command: exportShell},
	{label: "Shell git", command: exportShellGit},
	{label: "Cmd", command: exportCmd},
	{label: "Cmd git", command: exportCmdGit},
	{label: "PowerShell", command: exportPowerShell},
	{label: "PowerShell git", command: exportPowerShellGit},
}

var writeClipboard = clipboard.WriteAll

func (a *App) addExportCommands(menu *systray.Menu, rule ruleMenuEntry) *systray.MenuItem {
	sub := systray.NewMenu()
	for _, kind := range exportKinds {
		sub.Add(kind.label, func() { a.do(func() { a.exportCommand(kind, rule.name) }) })
	}
	return menu.AddSubmenu(rule.label, sub)
}

func (a *App) exportCommand(kind exportKind, name string) {
	address := a.ruleAddress(name)
	if address == "" {
		log.Error(errors.New(i18n.NoLocalRule()), i18n.ExportCommand())
		return
	}
	err := writeClipboard(kind.command(address))
	if err != nil {
		log.Error(err, i18n.WriteClipboard())
	}
}

func exportShell(address string) string {
	return fmt.Sprintf("export http_proxy=http://%s https_proxy=http://%s; ", address, address)
}

func exportShellGit(address string) string {
	return fmt.Sprintf(`export GIT_SSH_COMMAND='ssh -o ProxyCommand="nc -x %s %%h %%p"' http_proxy=http://%s https_proxy=http://%s; `, address, address, address)
}

func exportCmd(address string) string {
	return fmt.Sprintf(`set "http_proxy=http://%s" && set "https_proxy=http://%s"`, address, address)
}

func exportCmdGit(address string) string {
	return fmt.Sprintf(`set "GIT_SSH_COMMAND=ssh -o ProxyCommand='connect -S %s %%h %%p'" && set "http_proxy=http://%s" && set "https_proxy=http://%s"`, address, address, address)
}

func exportPowerShell(address string) string {
	return fmt.Sprintf("$env:http_proxy='http://%s'; $env:https_proxy='http://%s'; ", address, address)
}

func exportPowerShellGit(address string) string {
	return fmt.Sprintf(`$env:GIT_SSH_COMMAND='ssh -o ProxyCommand="connect -S %s %%h %%p"'; $env:http_proxy='http://%s'; $env:https_proxy='http://%s'; `, address, address, address)
}
