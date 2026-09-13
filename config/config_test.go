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
	"strings"
	"sync"
	"testing"

	bridgeconfig "github.com/wzshiming/bridge/config"
	"gopkg.in/yaml.v3"
)

func TestValidate(t *testing.T) {
	var defaultConf Config
	if err := yaml.Unmarshal([]byte(defaultConfig), &defaultConf); err != nil {
		t.Fatal(err)
	}
	if len(defaultConf.Contexts) == 0 || len(defaultConf.Contexts[0].Way) == 0 {
		t.Fatal("embedded default must contain a context with at least one node")
	}
	withWay := func(nodes ...bridgeconfig.Node) *Config {
		return &Config{
			CurrentContext: "a",
			Contexts:       []Context{{Name: "a", Way: nodes}},
		}
	}
	tests := []struct {
		name      string
		conf      *Config
		wantError string
		wantHint  string
	}{
		{
			name: "default",
			conf: &defaultConf,
		},
		{
			name: "empty_config",
			conf: &Config{},
		},
		{
			name: "maximum_port",
			conf: &Config{Proxy: Proxy{Port: 65535}},
		},
		{
			name:      "nil_config",
			wantError: "config is nil",
		},
		{
			name:      "port_out_of_range",
			conf:      &Config{Proxy: Proxy{Port: 65536}},
			wantError: "proxy.port 65536 is out of range (0-65535)",
		},
		{
			name:      "empty_name",
			conf:      &Config{Contexts: []Context{{}}},
			wantError: "contexts[0].name is empty",
		},
		{
			name:      "whitespace_name",
			conf:      &Config{CurrentContext: "a", Contexts: []Context{{Name: "a"}, {Name: " \t\n"}}},
			wantError: "contexts[1].name is empty",
		},
		{
			name:      "duplicate_name",
			conf:      &Config{CurrentContext: "a", Contexts: []Context{{Name: "a"}, {Name: "a"}}},
			wantError: `duplicate context name "a"`,
		},
		{
			name:      "slash_in_name",
			conf:      &Config{CurrentContext: "a/b", Contexts: []Context{{Name: "a/b"}}},
			wantError: `contexts[0].name "a/b" must not contain "/"`,
		},
		{
			name:      "unknown_current_context",
			conf:      &Config{CurrentContext: "missing", Contexts: []Context{{Name: "a"}}},
			wantError: `current_context "missing" does not match any context`,
		},
		{
			name:      "current_context_without_contexts",
			conf:      &Config{CurrentContext: "missing"},
			wantError: `current_context "missing" does not match any context`,
		},
		{
			name:      "empty_current_context",
			conf:      &Config{Contexts: []Context{{Name: "a"}}},
			wantError: `current_context "" does not match any context`,
		},
		{
			name:      "no_lb",
			conf:      withWay(bridgeconfig.Node{}),
			wantError: "contexts[0].way[0] has no proxy URL",
		},
		{
			name:      "empty_lb",
			conf:      withWay(bridgeconfig.Node{LB: []string{""}}),
			wantError: "contexts[0].way[0] contains an empty proxy URL",
		},
		{
			name:      "whitespace_lb",
			conf:      withWay(bridgeconfig.Node{LB: []string{" \t\n"}}),
			wantError: "contexts[0].way[0] contains an empty proxy URL",
		},
		{
			name:      "invalid_url",
			conf:      withWay(bridgeconfig.Node{LB: []string{"socks5://[::1"}}),
			wantError: `contexts[0].way[0]: invalid proxy URL "socks5://[::1":`,
		},
		{
			name:      "ip_without_scheme",
			conf:      withWay(bridgeconfig.Node{LB: []string{"127.0.0.1:1080"}}),
			wantError: "invalid proxy URL",
			wantHint:  "socks5://host:1080",
		},
		{
			name:      "no_scheme",
			conf:      withWay(bridgeconfig.Node{LB: []string{"/just/a/path"}}),
			wantError: `contexts[0].way[0]: proxy URL "/just/a/path" has no scheme (e.g. socks5://host:1080)`,
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
			wantError: "contexts[0].way[1] has no proxy URL",
		},
		{
			name:      "second_lb_error",
			conf:      withWay(bridgeconfig.Node{LB: []string{"socks5://a:1080", ""}}),
			wantError: "contexts[0].way[0] contains an empty proxy URL",
		},
		{
			name: "inactive_context_error",
			conf: &Config{CurrentContext: "a", Contexts: []Context{
				{Name: "a"},
				{Name: "b", Way: []bridgeconfig.Node{{}}},
			}},
			wantError: "contexts[1].way[0] has no proxy URL",
		},
		{
			name:      "port_before_context",
			conf:      &Config{Proxy: Proxy{Port: 70000}, Contexts: []Context{{}}},
			wantError: "proxy.port 70000 is out of range (0-65535)",
		},
		{
			name: "names_before_current_context_and_urls",
			conf: &Config{CurrentContext: "missing", Contexts: []Context{
				{Name: "a", Way: []bridgeconfig.Node{{}}},
				{Name: "a"},
			}},
			wantError: `duplicate context name "a"`,
		},
		{
			name:      "current_context_before_urls",
			conf:      &Config{Contexts: []Context{{Name: "a", Way: []bridgeconfig.Node{{}}}}},
			wantError: `current_context "" does not match any context`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Validate(test.conf)
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Validate() = %v, want error containing %q", err, test.wantError)
			}
			if test.wantHint != "" && (err == nil || !strings.Contains(err.Error(), test.wantHint)) {
				t.Fatalf("Validate() = %v, want hint containing %q", err, test.wantHint)
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
	const content = "proxy:\n  port: 1088\n"
	if err := store.SaveRaw([]byte(content)); err != nil {
		t.Fatal(err)
	}
	const otherContent = "proxy:\n  port: 1089\n"
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
					content = "# keep this\nproxy:\n  port: 1088\n"
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
		for _, content := range []string{"# first\nproxy:\n  port: 1080\n", "# latest\nproxy:\n  port: 0\n"} {
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
		CurrentContext: "a",
		Contexts: []Context{{Name: "a", Way: []bridgeconfig.Node{
			{LB: []string{"socks5://a:1080"}},
			{LB: []string{"http://b:8080", "cmd:ssh host nc %h %p"}},
		}}},
		Proxy: Proxy{Host: "127.0.0.1", Port: 1087},
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
	if !reflect.DeepEqual(got.Contexts[0].Way[0].LB, want.Contexts[0].Way[0].LB) {
		t.Fatalf("single-node LB = %v, want %v", got.Contexts[0].Way[0].LB, want.Contexts[0].Way[0].LB)
	}
	data, err := store.LoadRaw()
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Contexts []struct {
			Way []map[string][]string `yaml:"way"`
		} `yaml:"contexts"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("node did not marshal as a YAML mapping: %v", err)
	}
	if len(raw.Contexts) == 0 || len(raw.Contexts[0].Way) == 0 || !reflect.DeepEqual(raw.Contexts[0].Way[0]["lb"], want.Contexts[0].Way[0].LB) {
		t.Fatalf("single-LB node did not marshal as an lb list: %s", data)
	}
}

func TestJSONTags(t *testing.T) {
	tests := []struct {
		name  string
		value any
		keys  []string
	}{
		{
			name:  "config",
			value: &Config{CurrentContext: "a", Proxy: Proxy{Port: 1}},
			keys:  []string{"current_context", "contexts", "proxy", "no_proxy", "host", "port", "list", "from_env", "from_file"},
		},
		{
			name:  "context",
			value: Context{},
			keys:  []string{"name", "way"},
		},
		{
			name:  "proxy",
			value: Proxy{},
			keys:  []string{"host", "port"},
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
			for _, key := range test.keys {
				if !strings.Contains(string(data), `"`+key+`":`) {
					t.Errorf("JSON %s is missing key %q", data, key)
				}
			}
			if strings.Contains(string(data), `"CurrentContext"`) {
				t.Errorf("JSON %s contains a Go field name", data)
			}
		})
	}
}
