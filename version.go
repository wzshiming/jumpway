package jumpway

import (
	"runtime/debug"
)

const (
	AppName        = "JumpWay"
	AppDescription = AppName + " Service"
)

// Version is the build's module version: the tag Go stamped, else the short VCS revision (with -dirty), else "".
func Version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return versionFrom(info)
}

func versionFrom(info *debug.BuildInfo) string {
	if info == nil {
		return ""
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	var revision string
	modified := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if revision == "" {
		return ""
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	if modified {
		revision += "-dirty"
	}
	return revision
}
