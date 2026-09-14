package config

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sync"
	"testing"

	bridgeconfig "github.com/wzshiming/bridge/config"
	"gopkg.in/yaml.v3"
)

func TestValidate(t *testing.T) {
	withWay := func(nodes ...bridgeconfig.Node) *Config {
		return &Config{Rules: []Rule{{Name: "a", Way: nodes}}}
	}
	tests := []struct {
		name      string
		conf      *Config
		wantError string
	}{
		{
			name: "zero_rules",
			conf: &Config{},
		},
		{
			name: "maximum_ports",
			conf: &Config{WebUI: Address{Port: 65535}, Rules: []Rule{{Name: "a", Listen: Listen{Host: "::1", Port: 65535}}}},
		},
		{
			name:      "nil_config",
			wantError: "config is nil",
		},
		{
			name:      "web_ui_port_out_of_range",
			conf:      &Config{WebUI: Address{Port: 65536}},
			wantError: "web_ui.port 65536 is out of range (0-65535)",
		},
		{
			name:      "empty_name",
			conf:      &Config{Rules: []Rule{{}}},
			wantError: "rules[0].name is empty",
		},
		{
			name:      "whitespace_name",
			conf:      &Config{Rules: []Rule{{Name: "a"}, {Name: " \t\n"}}},
			wantError: "rules[1].name is empty",
		},
		{
			name:      "duplicate_name",
			conf:      &Config{Rules: []Rule{{Name: "a"}, {Name: "a"}}},
			wantError: `duplicate rule name "a"`,
		},
		{
			name:      "slash_in_name",
			conf:      &Config{Rules: []Rule{{Name: "a/b"}}},
			wantError: `rules[0].name "a/b" must not contain "/"`,
		},
		{
			name:      "listen_port_out_of_range",
			conf:      &Config{Rules: []Rule{{Name: "a"}, {Name: "b", Listen: Listen{Port: 70000}}}},
			wantError: "rules[1].listen.port 70000 is out of range (0-65535)",
		},
		{
			name:      "password_without_username",
			conf:      &Config{Rules: []Rule{{Name: "a", Listen: Listen{Password: "secret"}}}},
			wantError: "rules[0].listen.password is set but username is empty",
		},
		{
			name:      "username_with_colon",
			conf:      &Config{Rules: []Rule{{Name: "a", Listen: Listen{Username: "us:er", Password: "secret"}}}},
			wantError: "rules[0].listen.username \"us:er\" must not contain \":\"",
		},
		{
			name: "username_and_password",
			conf: &Config{Rules: []Rule{{Name: "a", Listen: Listen{Username: "user", Password: "secret"}}}},
		},
		{
			name: "username_only",
			conf: &Config{Rules: []Rule{{Name: "a", Listen: Listen{Username: "user"}}}},
		},
		{
			name:      "no_lb",
			conf:      withWay(bridgeconfig.Node{}),
			wantError: "rules[0].way[0] has no proxy URL",
		},
		{
			name:      "empty_lb",
			conf:      withWay(bridgeconfig.Node{LB: []string{""}}),
			wantError: "rules[0].way[0] contains an empty proxy URL",
		},
		{
			name:      "whitespace_lb",
			conf:      withWay(bridgeconfig.Node{LB: []string{" \t\n"}}),
			wantError: "rules[0].way[0] contains an empty proxy URL",
		},
		{
			name:      "invalid_url",
			conf:      withWay(bridgeconfig.Node{LB: []string{"socks5://[::1"}}),
			wantError: `rules[0].way[0]: invalid proxy URL "socks5://[::1": parse "socks5://[::1": missing ']' in host (e.g. socks5://host:1080)`,
		},
		{
			name:      "ip_without_scheme",
			conf:      withWay(bridgeconfig.Node{LB: []string{"127.0.0.1:1080"}}),
			wantError: `rules[0].way[0]: invalid proxy URL "127.0.0.1:1080": parse "127.0.0.1:1080": first path segment in URL cannot contain colon (e.g. socks5://host:1080)`,
		},
		{
			name:      "no_scheme",
			conf:      withWay(bridgeconfig.Node{LB: []string{"/just/a/path"}}),
			wantError: `rules[0].way[0]: proxy URL "/just/a/path" has no scheme (e.g. socks5://host:1080)`,
		},
		{
			name: "remote_listen_no_lb",
			conf: &Config{Rules: []Rule{{Name: "a", Listen: Listen{
				Way: []bridgeconfig.Node{{}},
			}}}},
			wantError: "rules[0].listen.way[0] has no proxy URL",
		},
		{
			name: "remote_listen_empty_lb",
			conf: &Config{Rules: []Rule{{Name: "a", Listen: Listen{
				Way: []bridgeconfig.Node{{LB: []string{""}}},
			}}}},
			wantError: "rules[0].listen.way[0] contains an empty proxy URL",
		},
		{
			name: "remote_listen_invalid_url",
			conf: &Config{Rules: []Rule{{Name: "a"}, {Name: "b", Listen: Listen{
				Way: []bridgeconfig.Node{{LB: []string{"ssh://[::1"}}},
			}}}},
			wantError: `rules[1].listen.way[0]: invalid proxy URL "ssh://[::1": parse "ssh://[::1": missing ']' in host (e.g. socks5://host:1080)`,
		},
		{
			name: "remote_listen_no_scheme",
			conf: &Config{Rules: []Rule{{Name: "a", Listen: Listen{
				Way: []bridgeconfig.Node{{LB: []string{"/just/a/path"}}},
			}}}},
			wantError: `rules[0].listen.way[0]: proxy URL "/just/a/path" has no scheme (e.g. socks5://host:1080)`,
		},
		{
			name: "command",
			conf: withWay(bridgeconfig.Node{LB: []string{"cmd:ssh host nc %h %p"}}),
		},
		{
			name: "shadowsocks",
			conf: withWay(bridgeconfig.Node{LB: []string{"ss://aes-256-gcm:pass@host:8388"}}),
		},
		{
			name: "multi_lb",
			conf: withWay(bridgeconfig.Node{LB: []string{"socks5://a:1080", "http://b:8080"}}),
		},
		{
			name: "unknown_opaque_scheme",
			conf: withWay(bridgeconfig.Node{LB: []string{"host:1080"}}),
		},
		{
			name: "no_proxy_not_validated",
			conf: &Config{NoProxy: NoProxy{List: []string{"%zz"}, FromEnv: []string{""}, FromFile: []string{"%zz"}}},
		},
		{
			name:      "second_node_error",
			conf:      withWay(bridgeconfig.Node{LB: []string{"socks5://a:1080"}}, bridgeconfig.Node{}),
			wantError: "rules[0].way[1] has no proxy URL",
		},
		{
			name:      "second_lb_error",
			conf:      withWay(bridgeconfig.Node{LB: []string{"socks5://a:1080", ""}}),
			wantError: "rules[0].way[0] contains an empty proxy URL",
		},
		{
			name: "disabled_rule_url_error",
			conf: &Config{Rules: []Rule{
				{Name: "a"},
				{Name: "b", Disabled: true, Way: []bridgeconfig.Node{{}}},
			}},
			wantError: "rules[1].way[0] has no proxy URL",
		},
		{
			name:      "disabled_rule_name_error",
			conf:      &Config{Rules: []Rule{{Disabled: true}}},
			wantError: "rules[0].name is empty",
		},
		{
			name:      "disabled_rule_port_error",
			conf:      &Config{Rules: []Rule{{Name: "a", Disabled: true, Listen: Listen{Port: 70000}}}},
			wantError: "rules[0].listen.port 70000 is out of range (0-65535)",
		},
		{
			name:      "web_ui_port_before_rules",
			conf:      &Config{WebUI: Address{Port: 70000}, Rules: []Rule{{}}},
			wantError: "web_ui.port 70000 is out of range (0-65535)",
		},
		{
			name: "duplicate_local_address",
			conf: &Config{Rules: []Rule{
				{Name: "a", Listen: Listen{Host: "127.0.0.1", Port: 9000}},
				{Name: "b", Listen: Listen{Port: 9000}},
			}},
			wantError: `rules[1].listen address 127.0.0.1:9000 is already used by rule "a"`,
		},
		{
			name: "duplicate_web_ui_address",
			conf: &Config{WebUI: Address{Host: "127.0.0.1", Port: 1088}, Rules: []Rule{
				{Name: "a", Listen: Listen{Port: 1088}},
			}},
			wantError: "rules[0].listen address 127.0.0.1:1088 is already used by web_ui",
		},
		{
			name: "default_web_ui_host",
			conf: &Config{WebUI: Address{Port: 1088}, Rules: []Rule{
				{Name: "a", Listen: Listen{Host: "127.0.0.1", Port: 1088}},
			}},
			wantError: "rules[0].listen address 127.0.0.1:1088 is already used by web_ui",
		},
		{
			name: "disabled_duplicate_address",
			conf: &Config{Rules: []Rule{
				{Name: "a", Disabled: true, Listen: Listen{Port: 9000}},
				{Name: "b", Listen: Listen{Port: 9000}},
				{Name: "c", Disabled: true, Listen: Listen{Port: 9000}},
			}},
		},
		{
			name: "disabled_web_ui_address",
			conf: &Config{WebUI: Address{Port: 1088}, Rules: []Rule{
				{Name: "a", Disabled: true, Listen: Listen{Port: 1088}},
			}},
		},
		{
			name: "remote_and_local_address",
			conf: &Config{Rules: []Rule{
				{Name: "a", Listen: Listen{Port: 9000, Way: []bridgeconfig.Node{{LB: []string{"ssh://user@host:22"}}}}},
				{Name: "b", Listen: Listen{Port: 9000}},
				{Name: "c", Listen: Listen{Port: 9000, Way: []bridgeconfig.Node{{LB: []string{"ssh://user@other:22"}}}}},
			}},
		},
		{
			name: "remote_and_web_ui_address",
			conf: &Config{WebUI: Address{Port: 1088}, Rules: []Rule{
				{Name: "a", Listen: Listen{Port: 1088, Way: []bridgeconfig.Node{{LB: []string{"ssh://user@host:22"}}}}},
			}},
		},
		{
			name: "zero_ports",
			conf: &Config{Rules: []Rule{{Name: "a"}, {Name: "b"}}},
		},
		{
			name: "different_hosts",
			conf: &Config{WebUI: Address{Host: "::1", Port: 9000}, Rules: []Rule{
				{Name: "a", Listen: Listen{Port: 9000}},
				{Name: "b", Listen: Listen{Host: "localhost", Port: 9000}},
			}},
		},
		{
			name: "ipv6_collision",
			conf: &Config{Rules: []Rule{
				{Name: "a", Listen: Listen{Host: "::1", Port: 9000}},
				{Name: "b", Listen: Listen{Host: "::1", Port: 9000}},
			}},
			wantError: `rules[1].listen address [::1]:9000 is already used by rule "a"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Validate(test.conf)
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
			} else if err == nil || err.Error() != test.wantError {
				t.Fatalf("Validate() = %v, want %q", err, test.wantError)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	var conf Config
	if err := yaml.Unmarshal([]byte(defaultConfig), &conf); err != nil {
		t.Fatal(err)
	}
	if want := (Address{Host: "127.0.0.1", Port: 1088}); conf.WebUI != want {
		t.Fatalf("embedded web_ui = %#v, want %#v", conf.WebUI, want)
	}
	if len(conf.Rules) != 1 || conf.Rules[0].Name != "default" || len(conf.Rules[0].Way) != 2 {
		t.Fatalf("embedded default must contain the default rule with two nodes: %#v", conf.Rules)
	}
	if conf.Rules[0].Listen.Host != "127.0.0.1" || conf.Rules[0].Listen.Port != 1087 {
		t.Fatalf("embedded listen = %#v, want 127.0.0.1:1087", conf.Rules[0].Listen)
	}
	if err := Validate(&conf); err != nil {
		t.Fatalf("embedded default is invalid: %v", err)
	}
}

func TestListenAddress(t *testing.T) {
	for _, test := range []struct {
		name string
		host string
		port uint32
		want string
	}{
		{name: "default_host", port: 1088, want: "127.0.0.1:1088"},
		{name: "ipv4", host: "0.0.0.0", port: 1087, want: "0.0.0.0:1087"},
		{name: "ipv6", host: "::1", port: 1087, want: "[::1]:1087"},
		{name: "hostname", host: "localhost", port: 65535, want: "localhost:65535"},
		{name: "zero_port", want: "127.0.0.1:0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			address := Address{Host: test.host, Port: test.port}
			if got := address.String(); got != test.want {
				t.Fatalf("String() = %q, want %q", got, test.want)
			}
			listen := Listen{Host: test.host, Port: test.port}
			if got := listen.Address(); got != test.want {
				t.Fatalf("Address() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestListenRemote(t *testing.T) {
	for _, way := range [][]bridgeconfig.Node{nil, {}, {{LB: []string{"ssh://user@host:22"}}}} {
		listen := Listen{Way: way}
		if got, want := listen.Remote(), len(way) > 0; got != want {
			t.Fatalf("Remote() with way %v = %v, want %v", way, got, want)
		}
	}
}

func TestListenUser(t *testing.T) {
	for _, test := range []struct {
		name     string
		listen   Listen
		wantNil  bool
		wantPass bool
	}{
		{name: "no_username", wantNil: true},
		{name: "password_only", listen: Listen{Password: "secret"}, wantNil: true},
		{name: "username_only", listen: Listen{Username: "user"}},
		{name: "username_and_password", listen: Listen{Username: "user@host", Password: "p:a/ss"}, wantPass: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			user := test.listen.User()
			if test.wantNil {
				if user != nil {
					t.Fatalf("User() = %v, want nil", user)
				}
				return
			}
			if user == nil || user.Username() != test.listen.Username {
				t.Fatalf("User() = %v, want username %q", user, test.listen.Username)
			}
			if password, ok := user.Password(); password != test.listen.Password || ok != test.wantPass {
				t.Fatalf("Password() = %q, %v; want %q, %v", password, ok, test.listen.Password, test.wantPass)
			}
		})
	}
}

func TestDefaultDir(t *testing.T) {
	homeEnv := "HOME"
	if runtime.GOOS == "windows" {
		homeEnv = "USERPROFILE"
	}
	home := t.TempDir()
	t.Setenv(homeEnv, home)
	dir, err := DefaultDir()
	if want := filepath.Join(home, ".jumpway"); err != nil || dir != want {
		t.Fatalf("DefaultDir() = %q, %v; want %q, nil", dir, err, want)
	}
	t.Setenv(homeEnv, "")
	if _, err := DefaultDir(); err == nil {
		t.Fatal("DefaultDir() succeeded without a home directory")
	}
}

func TestStoreIsolation(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	other := NewStore(t.TempDir())
	if got := store.Dir(); got != dir {
		t.Fatalf("Dir() = %q, want %q", got, dir)
	}
	if got, want := store.Path(), filepath.Join(dir, "config.yaml"); got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
	const content = "web_ui:\n  port: 1088\n"
	if err := store.SaveRaw([]byte(content)); err != nil {
		t.Fatal(err)
	}
	const otherContent = "web_ui:\n  port: 1089\n"
	if err := other.SaveRaw([]byte(otherContent)); err != nil {
		t.Fatal(err)
	}
	for candidate, want := range map[*Store]string{store: content, other: otherContent} {
		data, err := candidate.LoadRaw()
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("LoadRaw() = %q, want %q", data, want)
		}
	}
}

func TestStoreInit(t *testing.T) {
	for _, name := range []string{"missing", "empty", "existing"} {
		t.Run(name, func(t *testing.T) {
			store := NewStore(filepath.Join(t.TempDir(), ".jumpway"))
			want := defaultConfig
			if name != "missing" {
				content := ""
				if name == "existing" {
					content = "# keep this\nweb_ui:\n  port: 1088\n"
					want = content
				}
				if err := store.SaveRaw([]byte(content)); err != nil {
					t.Fatal(err)
				}
			}
			if err := store.Init(); err != nil {
				t.Fatal(err)
			}
			data, err := store.LoadRaw()
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != want {
				t.Fatalf("LoadRaw() after Init() = %q, want %q", data, want)
			}
		})
	}
}

func TestNoProxyGetListFromFiles(t *testing.T) {
	home := t.TempDir()
	homeEnv := "HOME"
	if runtime.GOOS == "windows" {
		homeEnv = "USERPROFILE"
	}
	t.Setenv(homeEnv, home)
	if err := os.WriteFile(filepath.Join(home, "no_proxy.txt"), []byte("home.example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, hostname := range []string{"first.example", "second.example"} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "no_proxy.txt"), []byte(hostname+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		noProxy := NoProxy{FromFile: []string{"./no_proxy.txt", "~/no_proxy.txt"}}
		want := []string{"home.example", hostname}
		slices.Sort(want)
		if got := noProxy.GetList(dir); !reflect.DeepEqual(got, want) {
			t.Fatalf("GetList(%q) = %v, want %v", dir, got, want)
		}
	}
}

func TestNoProxyHTTPCacheIsolation(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		io.WriteString(writer, "cached.example\n")
	}))
	t.Cleanup(server.Close)
	noProxy := NoProxy{FromFile: []string{server.URL + "/no_proxy.txt"}}
	dirs := []string{t.TempDir(), t.TempDir()}
	for _, dir := range dirs {
		if got := noProxy.GetList(dir); !slices.Equal(got, []string{"cached.example"}) {
			t.Fatalf("GetList(%q) = %v, want [cached.example]", dir, got)
		}
	}
	server.Close()
	if requests != len(dirs) {
		t.Fatalf("HTTP requests = %d, want %d independent cache fills", requests, len(dirs))
	}
	for _, dir := range dirs {
		if got := noProxy.GetList(dir); !slices.Equal(got, []string{"cached.example"}) {
			t.Fatalf("cached GetList(%q) = %v, want [cached.example]", dir, got)
		}
	}
}

func TestStoreSaveRawAtomic(t *testing.T) {
	t.Run("replace", func(t *testing.T) {
		store := NewStore(t.TempDir())
		for _, content := range []string{"# first\nweb_ui:\n  port: 1088\n", "# latest\nweb_ui:\n  port: 0\n"} {
			if err := store.SaveRaw([]byte(content)); err != nil {
				t.Fatal(err)
			}
			data, err := store.LoadRaw()
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != content {
				t.Fatalf("LoadRaw() = %q, want %q", data, content)
			}
		}
		entries, err := os.ReadDir(store.Dir())
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Name() != "config.yaml" {
			t.Fatalf("unexpected config directory entries: %v", entries)
		}
		info, err := os.Stat(store.Path())
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Fatalf("config mode = %04o, want 0644", info.Mode().Perm())
		}
	})
	t.Run("concurrent", func(t *testing.T) {
		store := NewStore(t.TempDir())
		const saveCount = 16
		contents := make([][]byte, saveCount)
		for index := range contents {
			contents[index] = bytes.Repeat([]byte{byte('a' + index)}, 64*1024)
		}
		if err := store.SaveRaw(contents[0]); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		var saves sync.WaitGroup
		for _, content := range contents {
			saves.Add(1)
			go func() {
				defer saves.Done()
				<-start
				if err := store.SaveRaw(content); err != nil {
					t.Errorf("concurrent SaveRaw() failed: %v", err)
				}
			}()
		}
		close(start)
		for range saveCount {
			data, err := store.LoadRaw()
			if err != nil {
				t.Errorf("concurrent LoadRaw() failed: %v", err)
				continue
			}
			matched := false
			for _, content := range contents {
				if bytes.Equal(data, content) {
					matched = true
					break
				}
			}
			if !matched {
				t.Errorf("concurrent read returned partial or mixed content (%d bytes)", len(data))
			}
		}
		saves.Wait()
		entries, err := os.ReadDir(store.Dir())
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Name() != "config.yaml" {
			t.Fatalf("unexpected config directory entries after concurrent saves: %v", entries)
		}
	})
	t.Run("rename_failure", func(t *testing.T) {
		store := NewStore(t.TempDir())
		if err := os.MkdirAll(store.Path(), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := store.SaveRaw([]byte("replacement")); err == nil {
			t.Fatal("SaveRaw() succeeded with a directory as the destination")
		}
		entries, err := os.ReadDir(store.Dir())
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Name() != "config.yaml" || !entries[0].IsDir() {
			t.Fatalf("failed save changed the destination or left temporary files: %v", entries)
		}
	})
}

func TestStoreSaveRoundTrip(t *testing.T) {
	store := NewStore(t.TempDir())
	want := &Config{
		WebUI: Address{Host: "127.0.0.1", Port: 1088},
		Rules: []Rule{{Name: "a", Disabled: true, Listen: Listen{
			Host:     "::1",
			Port:     1087,
			Way:      []bridgeconfig.Node{{LB: []string{"ssh://user@host:22"}}},
			Username: "user",
			Password: "secret",
		}, Way: []bridgeconfig.Node{
			{LB: []string{"socks5://a:1080"}},
			{LB: []string{"http://b:8080", "cmd:ssh host nc %h %p"}},
		}}},
		NoProxy: NoProxy{
			List:     []string{"localhost", "127.0.0.0/8"},
			FromEnv:  []string{"NO_PROXY"},
			FromFile: []string{"./no_proxy.txt"},
		},
	}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(got.Rules[0].Way[0].LB, want.Rules[0].Way[0].LB) {
		t.Fatalf("single-node LB = %v, want %v", got.Rules[0].Way[0].LB, want.Rules[0].Way[0].LB)
	}
	data, err := store.LoadRaw()
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Rules []struct {
			Listen struct {
				Way []map[string][]string `yaml:"way"`
			} `yaml:"listen"`
			Way []map[string][]string `yaml:"way"`
		} `yaml:"rules"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("node did not marshal as a YAML mapping: %v", err)
	}
	if len(raw.Rules) == 0 || len(raw.Rules[0].Way) == 0 || !reflect.DeepEqual(raw.Rules[0].Way[0]["lb"], want.Rules[0].Way[0].LB) {
		t.Fatalf("single-LB node did not marshal as an lb list: %s", data)
	}
	if len(raw.Rules[0].Listen.Way) == 0 || !reflect.DeepEqual(raw.Rules[0].Listen.Way[0]["lb"], want.Rules[0].Listen.Way[0].LB) {
		t.Fatalf("listen node did not marshal as an lb list: %s", data)
	}
}

func TestJSONTags(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		keys    []string
		omitted []string
	}{
		{
			name:    "config",
			value:   Config{},
			keys:    []string{"web_ui", "rules", "no_proxy"},
			omitted: []string{"current_context", "contexts", "proxy", "WebUI", "Rules", "NoProxy"},
		},
		{
			name:    "enabled_rule",
			value:   Rule{},
			keys:    []string{"name", "listen", "way"},
			omitted: []string{"disabled"},
		},
		{
			name:  "disabled_rule",
			value: Rule{Disabled: true},
			keys:  []string{"name", "disabled", "listen", "way"},
		},
		{
			name:  "address",
			value: Address{},
			keys:  []string{"host", "port"},
		},
		{
			name:    "local_listen",
			value:   Listen{},
			keys:    []string{"host", "port"},
			omitted: []string{"way", "username", "password"},
		},
		{
			name: "remote_listen_with_credentials",
			value: Listen{
				Way:      []bridgeconfig.Node{{LB: []string{"ssh://user@host:22"}}},
				Username: "user",
				Password: "secret",
			},
			keys: []string{"host", "port", "way", "username", "password"},
		},
		{
			name:  "no_proxy",
			value: NoProxy{},
			keys:  []string{"list", "from_env", "from_file"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := json.Marshal(test.value)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatal(err)
			}
			for _, key := range test.keys {
				if _, ok := fields[key]; !ok {
					t.Errorf("JSON %s is missing key %q", data, key)
				}
			}
			for _, key := range test.omitted {
				if _, ok := fields[key]; ok {
					t.Errorf("JSON %s should omit key %q", data, key)
				}
			}
		})
	}
}
