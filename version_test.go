package jumpway

import (
	"runtime/debug"
	"testing"
)

func TestVersionFrom(t *testing.T) {
	settings := func(pairs ...string) []debug.BuildSetting {
		var out []debug.BuildSetting
		for i := 0; i+1 < len(pairs); i += 2 {
			out = append(out, debug.BuildSetting{Key: pairs[i], Value: pairs[i+1]})
		}
		return out
	}
	for _, test := range []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{name: "nil", info: nil, want: ""},
		{name: "tagged", info: &debug.BuildInfo{Main: debug.Module{Version: "v0.5.0"}}, want: "v0.5.0"},
		{
			name: "pseudo_version",
			info: &debug.BuildInfo{Main: debug.Module{Version: "v0.5.1-0.20260925120000-abcdef123456"}},
			want: "v0.5.1-0.20260925120000-abcdef123456",
		},
		{
			name: "devel_revision",
			info: &debug.BuildInfo{
				Main:     debug.Module{Version: "(devel)"},
				Settings: settings("vcs", "git", "vcs.revision", "abcdef1234567890abcdef1234567890abcdef12", "vcs.modified", "false"),
			},
			want: "abcdef123456",
		},
		{
			name: "devel_revision_modified",
			info: &debug.BuildInfo{
				Main:     debug.Module{Version: "(devel)"},
				Settings: settings("vcs.revision", "abcdef1234567890abcdef1234567890abcdef12", "vcs.modified", "true"),
			},
			want: "abcdef123456-dirty",
		},
		{
			name: "empty_version_short_revision",
			info: &debug.BuildInfo{Settings: settings("vcs.revision", "abc", "vcs.modified", "true")},
			want: "abc-dirty",
		},
		{name: "devel_without_vcs", info: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, want: ""},
		{name: "nothing", info: &debug.BuildInfo{}, want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := versionFrom(test.info); got != test.want {
				t.Fatalf("versionFrom() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Skip("no build info in this binary")
	}
	got := Version()
	t.Logf("Version() = %q (Main.Version %q)", got, info.Main.Version)
	if want := versionFrom(info); got != want {
		t.Fatalf("Version() = %q, want %q", got, want)
	}
}
