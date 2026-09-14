package tray

func (a *App) updateStatus() {
	snapshot := a.menuSnapshot()
	items := snapshot.items
	if items == nil {
		return
	}
	if items.web != nil {
		items.web.SetLabel(snapshot.web)
	}
	selected := ""
	for _, rule := range snapshot.rules {
		if item := items.rules[rule.name]; item != nil {
			item.SetLabel(rule.status)
		}
		if !rule.localProxy {
			continue
		}
		if item := items.proxies[rule.name]; item != nil {
			item.SetLabel(rule.label)
			if snapshot.selected == rule.name {
				selected = rule.name
			}
		}
		if item := items.exports[rule.name]; item != nil {
			item.SetLabel(rule.label)
		}
	}
	for name, item := range items.proxies {
		item.SetChecked(selected == name)
	}
	if items.manual != nil {
		items.manual.SetChecked(selected == "")
	}
}
