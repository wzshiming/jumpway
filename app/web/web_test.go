package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/wzshiming/jumpway/app/web/services/configs"
	"github.com/wzshiming/jumpway/config"
)

const testConfigYAML = `current_context: a
contexts:
  - name: a
    way:
      - lb:
          - socks5://127.0.0.1:1080
proxy:
  host: 127.0.0.1
  port: 1087
# keep me
`

type fakeRuntime struct {
	reloads int
	err     error
	status  configs.Status
}

func (fake *fakeRuntime) Reload() error {
	fake.reloads++
	return fake.err
}

func (fake *fakeRuntime) Status() configs.Status {
	return fake.status
}

func setupConfigAPI(t *testing.T) (http.Handler, *fakeRuntime, *config.Store) {
	t.Helper()
	store := config.NewStore(t.TempDir())
	fake := &fakeRuntime{}
	if err := os.WriteFile(store.Path(), []byte(testConfigYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	return NewHandler(configs.NewConfigsService(store, fake)), fake, store
}

func requestAPI(t *testing.T, handler http.Handler, method, target, body string, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if method == http.MethodPut || method == http.MethodPost {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("%s %s: status = %d, want %d; body = %s", method, target, response.Code, wantStatus, response.Body.String())
	}
	return response
}

func assertConfigYAML(t *testing.T, store *config.Store, want string) {
	t.Helper()
	data, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("config.yaml = %q, want %q", data, want)
	}
}

func listContextsAPI(t *testing.T, handler http.Handler) []config.Context {
	t.Helper()
	response := requestAPI(t, handler, http.MethodGet, "/apis/configs/contexts", "", http.StatusOK)
	var contexts []config.Context
	if err := json.Unmarshal(response.Body.Bytes(), &contexts); err != nil {
		t.Fatal(err)
	}
	if contexts == nil {
		t.Fatal("contexts = null, want a JSON array")
	}
	return contexts
}

func TestGetConfig(t *testing.T) {
	handler, _, _ := setupConfigAPI(t)
	response := requestAPI(t, handler, http.MethodGet, "/apis/configs", "", http.StatusOK)
	var conf map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &conf); err != nil {
		t.Fatal(err)
	}
	if conf["current_context"] != "a" {
		t.Fatalf("current_context = %v, want a", conf["current_context"])
	}
	contexts, ok := conf["contexts"].([]any)
	if !ok || len(contexts) != 1 {
		t.Fatalf("contexts = %#v, want one context", conf["contexts"])
	}
	context, ok := contexts[0].(map[string]any)
	if !ok {
		t.Fatalf("context = %#v, want an object", contexts[0])
	}
	way, ok := context["way"].([]any)
	if !ok || len(way) != 1 || way[0] != "socks5://127.0.0.1:1080" {
		t.Fatalf("way = %#v, want a single string node", context["way"])
	}
}

func TestCurrentContext(t *testing.T) {
	handler, fake, store := setupConfigAPI(t)
	response := requestAPI(t, handler, http.MethodGet, "/apis/configs/current-context", "", http.StatusOK)
	var current struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	if current.Name != "a" {
		t.Fatalf("current context = %q, want a", current.Name)
	}
	response = requestAPI(t, handler, http.MethodPut, "/apis/configs/current-context", `{"name":"missing"}`, http.StatusBadRequest)
	if !strings.Contains(response.Body.String(), `current_context "missing" does not match any context`) {
		t.Fatalf("body = %q, want unknown current context error", response.Body.String())
	}
	if fake.reloads != 0 {
		t.Fatalf("reloads = %d, want 0", fake.reloads)
	}
	assertConfigYAML(t, store, testConfigYAML)
	conf, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	conf.Contexts = append(conf.Contexts, config.Context{Name: "b"})
	if err := store.Save(conf); err != nil {
		t.Fatal(err)
	}
	requestAPI(t, handler, http.MethodPut, "/apis/configs/current-context", `{"name":"b"}`, http.StatusOK)
	if fake.reloads != 1 {
		t.Fatalf("reloads = %d, want 1", fake.reloads)
	}
	conf, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if conf.CurrentContext != "b" || len(conf.Contexts) != 2 || conf.Proxy.Port != 1087 {
		t.Fatalf("saved config = %#v, want current b with both contexts and unchanged proxy", conf)
	}
	response = requestAPI(t, handler, http.MethodGet, "/apis/configs/current-context", "", http.StatusOK)
	if err := json.Unmarshal(response.Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	if current.Name != "b" {
		t.Fatalf("current context = %q, want b", current.Name)
	}
}

func TestListContexts(t *testing.T) {
	handler, _, _ := setupConfigAPI(t)
	contexts := listContextsAPI(t, handler)
	if len(contexts) != 1 || contexts[0].Name != "a" || len(contexts[0].Way) != 1 || !slices.Equal(contexts[0].Way[0].LB, []string{"socks5://127.0.0.1:1080"}) {
		t.Fatalf("contexts = %#v, want seeded context a", contexts)
	}
}

func TestCreateContext(t *testing.T) {
	handler, fake, store := setupConfigAPI(t)
	requestAPI(t, handler, http.MethodPost, "/apis/configs/contexts", `{"name":"b","way":[{"lb":["socks5://h:1080"]}]}`, http.StatusOK)
	contexts := listContextsAPI(t, handler)
	if len(contexts) != 2 || contexts[0].Name != "a" || contexts[1].Name != "b" || len(contexts[1].Way) != 1 || !slices.Equal(contexts[1].Way[0].LB, []string{"socks5://h:1080"}) {
		t.Fatalf("contexts = %#v, want a and b with the supplied way", contexts)
	}
	if fake.reloads != 1 {
		t.Fatalf("reloads = %d, want 1", fake.reloads)
	}
	conf, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if conf.CurrentContext != "a" || conf.Proxy.Port != 1087 {
		t.Fatalf("saved config = %#v, want current a and unchanged proxy", conf)
	}
}

func TestCreateFirstContext(t *testing.T) {
	handler, fake, store := setupConfigAPI(t)
	if err := store.SaveRaw([]byte("proxy:\n  host: 127.0.0.1\n  port: 1087\n")); err != nil {
		t.Fatal(err)
	}
	if contexts := listContextsAPI(t, handler); len(contexts) != 0 {
		t.Fatalf("contexts = %#v, want empty array", contexts)
	}
	requestAPI(t, handler, http.MethodPost, "/apis/configs/contexts", `{"name":"b","way":[{"lb":["socks5://h:1080"]}]}`, http.StatusOK)
	if fake.reloads != 1 {
		t.Fatalf("reloads = %d, want 1", fake.reloads)
	}
	conf, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if conf.CurrentContext != "b" || len(conf.Contexts) != 1 || conf.Contexts[0].Name != "b" {
		t.Fatalf("saved config = %#v, want current b as the only context", conf)
	}
}

func TestContextMutationInvalid(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		target    string
		body      string
		wantError string
	}{
		{name: "duplicate", method: http.MethodPost, target: "/contexts", body: `{"name":"a"}`, wantError: `context "a" already exists`},
		{name: "empty_name", method: http.MethodPost, target: "/contexts", body: `{}`, wantError: "context name is empty"},
		{name: "blank_name", method: http.MethodPost, target: "/contexts", body: `{"name":"  "}`, wantError: "context name is empty"},
		{name: "null_create", method: http.MethodPost, target: "/contexts", body: `null`, wantError: "context name is empty"},
		{name: "invalid_create_way", method: http.MethodPost, target: "/contexts", body: `{"name":"b","way":[{"lb":[]}]}`, wantError: "has no proxy URL"},
		{name: "unknown_update", method: http.MethodPut, target: "/contexts/zzz", body: `{"name":"b"}`, wantError: `context "zzz" not found`},
		{name: "null_update", method: http.MethodPut, target: "/contexts/a", body: `null`, wantError: "context is nil"},
		{name: "invalid_update_way", method: http.MethodPut, target: "/contexts/a", body: `{"way":[{"lb":["/no-scheme"]}]}`, wantError: "has no scheme"},
		{name: "unknown_delete", method: http.MethodDelete, target: "/contexts/zzz", wantError: `context "zzz" not found`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, fake, store := setupConfigAPI(t)
			response := requestAPI(t, handler, test.method, "/apis/configs"+test.target, test.body, http.StatusBadRequest)
			if !strings.Contains(response.Body.String(), test.wantError) {
				t.Fatalf("body = %q, want error containing %q", response.Body.String(), test.wantError)
			}
			if fake.reloads != 0 {
				t.Fatalf("reloads = %d, want 0", fake.reloads)
			}
			assertConfigYAML(t, store, testConfigYAML)
		})
	}
}

func TestGetContext(t *testing.T) {
	handler, _, _ := setupConfigAPI(t)
	response := requestAPI(t, handler, http.MethodGet, "/apis/configs/contexts/a", "", http.StatusOK)
	var got config.Context
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "a" || len(got.Way) != 1 || !slices.Equal(got.Way[0].LB, []string{"socks5://127.0.0.1:1080"}) {
		t.Fatalf("context = %#v, want seeded context a", got)
	}
	response = requestAPI(t, handler, http.MethodGet, "/apis/configs/contexts/zzz", "", http.StatusBadRequest)
	if !strings.Contains(response.Body.String(), `context "zzz" not found`) {
		t.Fatalf("body = %q, want context not found error", response.Body.String())
	}
}

func TestUpdateContext(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		want string
	}{
		{name: "rename_current", body: `{"name":"a2","way":[{"lb":["socks5://h:1080"]}]}`, want: "a2"},
		{name: "empty_name_keeps_path", body: `{"name":"","way":[{"lb":["socks5://h:1080"]}]}`, want: "a"},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, fake, store := setupConfigAPI(t)
			requestAPI(t, handler, http.MethodPut, "/apis/configs/contexts/a", test.body, http.StatusOK)
			contexts := listContextsAPI(t, handler)
			if len(contexts) != 1 || contexts[0].Name != test.want || len(contexts[0].Way) != 1 || !slices.Equal(contexts[0].Way[0].LB, []string{"socks5://h:1080"}) {
				t.Fatalf("contexts = %#v, want %q with replaced way", contexts, test.want)
			}
			response := requestAPI(t, handler, http.MethodGet, "/apis/configs/current-context", "", http.StatusOK)
			var current struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &current); err != nil {
				t.Fatal(err)
			}
			if current.Name != test.want {
				t.Fatalf("current context = %q, want %q", current.Name, test.want)
			}
			if fake.reloads != 1 {
				t.Fatalf("reloads = %d, want 1", fake.reloads)
			}
			conf, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if conf.Proxy.Port != 1087 {
				t.Fatalf("proxy.port = %d, want unchanged 1087", conf.Proxy.Port)
			}
		})
	}
}

func TestRenameContextConflict(t *testing.T) {
	handler, fake, store := setupConfigAPI(t)
	conf, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	conf.Contexts = append(conf.Contexts, config.Context{Name: "b"})
	if err := store.Save(conf); err != nil {
		t.Fatal(err)
	}
	before, err := store.LoadRaw()
	if err != nil {
		t.Fatal(err)
	}
	response := requestAPI(t, handler, http.MethodPut, "/apis/configs/contexts/a", `{"name":"b"}`, http.StatusBadRequest)
	if !strings.Contains(response.Body.String(), `context "b" already exists`) {
		t.Fatalf("body = %q, want duplicate context error", response.Body.String())
	}
	if fake.reloads != 0 {
		t.Fatalf("reloads = %d, want 0", fake.reloads)
	}
	assertConfigYAML(t, store, string(before))
}

func TestDeleteContext(t *testing.T) {
	handler, fake, store := setupConfigAPI(t)
	conf, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	conf.Contexts = append(conf.Contexts, config.Context{Name: "b"})
	if err := store.Save(conf); err != nil {
		t.Fatal(err)
	}
	before, err := store.LoadRaw()
	if err != nil {
		t.Fatal(err)
	}
	response := requestAPI(t, handler, http.MethodDelete, "/apis/configs/contexts/a", "", http.StatusBadRequest)
	if !strings.Contains(response.Body.String(), `context "a" is the current context; switch to another context first`) {
		t.Fatalf("body = %q, want current context deletion error", response.Body.String())
	}
	if fake.reloads != 0 {
		t.Fatalf("reloads = %d, want 0", fake.reloads)
	}
	assertConfigYAML(t, store, string(before))
	response = requestAPI(t, handler, http.MethodDelete, "/apis/configs/contexts/b", "", http.StatusOK)
	if strings.TrimSpace(response.Body.String()) != "null" {
		t.Fatalf("body = %q, want null", response.Body.String())
	}
	if contexts := listContextsAPI(t, handler); len(contexts) != 1 || contexts[0].Name != "a" {
		t.Fatalf("contexts = %#v, want only a", contexts)
	}
	if fake.reloads != 1 {
		t.Fatalf("reloads = %d, want 1", fake.reloads)
	}
	response = requestAPI(t, handler, http.MethodDelete, "/apis/configs/contexts/a", "", http.StatusOK)
	if strings.TrimSpace(response.Body.String()) != "null" {
		t.Fatalf("body = %q, want null", response.Body.String())
	}
	if contexts := listContextsAPI(t, handler); len(contexts) != 0 {
		t.Fatalf("contexts = %#v, want empty array", contexts)
	}
	response = requestAPI(t, handler, http.MethodGet, "/apis/configs/current-context", "", http.StatusOK)
	var current struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	if current.Name != "" {
		t.Fatalf("current context = %q, want empty", current.Name)
	}
	response = requestAPI(t, handler, http.MethodGet, "/apis/configs", "", http.StatusOK)
	if err := json.Unmarshal(response.Body.Bytes(), &conf); err != nil {
		t.Fatal(err)
	}
	if len(conf.Contexts) != 0 || conf.CurrentContext != "" || conf.Proxy.Port != 1087 {
		t.Fatalf("config = %#v, want no contexts and unchanged proxy", conf)
	}
	if fake.reloads != 2 {
		t.Fatalf("reloads = %d, want 2", fake.reloads)
	}
}

func TestCreateContextsConcurrent(t *testing.T) {
	handler, fake, _ := setupConfigAPI(t)
	const count = 20
	responses := make(chan *httptest.ResponseRecorder, count)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for index := 0; index < count; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			body := fmt.Sprintf(`{"name":"context-%d","way":[{"lb":["socks5://h:1080"]}]}`, index)
			request := httptest.NewRequest(http.MethodPost, "/apis/configs/contexts", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			responses <- response
		}()
	}
	close(start)
	workers.Wait()
	close(responses)
	for response := range responses {
		if response.Code != http.StatusOK {
			t.Fatalf("concurrent POST status = %d, want 200; body = %s", response.Code, response.Body.String())
		}
	}
	contexts := listContextsAPI(t, handler)
	if len(contexts) != count+1 {
		t.Fatalf("context count = %d, want %d", len(contexts), count+1)
	}
	names := make(map[string]bool, len(contexts))
	for _, candidate := range contexts {
		names[candidate.Name] = true
	}
	if !names["a"] {
		t.Fatal("seeded context a was lost")
	}
	for index := 0; index < count; index++ {
		if name := fmt.Sprintf("context-%d", index); !names[name] {
			t.Errorf("context %q was lost", name)
		}
	}
	if fake.reloads != count {
		t.Fatalf("reloads = %d, want %d", fake.reloads, count)
	}
}

func TestProxy(t *testing.T) {
	handler, fake, store := setupConfigAPI(t)
	response := requestAPI(t, handler, http.MethodGet, "/apis/configs/proxy", "", http.StatusOK)
	var proxy config.Proxy
	if err := json.Unmarshal(response.Body.Bytes(), &proxy); err != nil {
		t.Fatal(err)
	}
	if proxy.Host != "127.0.0.1" || proxy.Port != 1087 {
		t.Fatalf("proxy = %#v, want seeded listen address", proxy)
	}
	response = requestAPI(t, handler, http.MethodPut, "/apis/configs/proxy", `{"host":"127.0.0.1","port":70000}`, http.StatusBadRequest)
	if !strings.Contains(response.Body.String(), "proxy.port") {
		t.Fatalf("body = %q, want invalid proxy.port error", response.Body.String())
	}
	if fake.reloads != 0 {
		t.Fatalf("reloads = %d, want 0", fake.reloads)
	}
	assertConfigYAML(t, store, testConfigYAML)
	requestAPI(t, handler, http.MethodPut, "/apis/configs/proxy", `{"host":"localhost","port":1088}`, http.StatusOK)
	if fake.reloads != 1 {
		t.Fatalf("reloads = %d, want 1", fake.reloads)
	}
	conf, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if conf.Proxy.Host != "localhost" || conf.Proxy.Port != 1088 || conf.CurrentContext != "a" || len(conf.Contexts) != 1 {
		t.Fatalf("saved config = %#v, want updated listen address and unchanged context", conf)
	}
}

func TestNoProxy(t *testing.T) {
	handler, fake, store := setupConfigAPI(t)
	const seeded = testConfigYAML + `no_proxy:
  list: [old.example]
  from_env: [NO_PROXY]
  from_file: [./no_proxy.txt]
`
	if err := store.SaveRaw([]byte(seeded)); err != nil {
		t.Fatal(err)
	}
	response := requestAPI(t, handler, http.MethodGet, "/apis/configs/no-proxy", "", http.StatusOK)
	var noProxy config.NoProxy
	if err := json.Unmarshal(response.Body.Bytes(), &noProxy); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(noProxy.List, []string{"old.example"}) || !slices.Equal(noProxy.FromEnv, []string{"NO_PROXY"}) || !slices.Equal(noProxy.FromFile, []string{"./no_proxy.txt"}) {
		t.Fatalf("no-proxy = %#v, want seeded bypass lists", noProxy)
	}
	requestAPI(t, handler, http.MethodPut, "/apis/configs/no-proxy", `{"list":["a","b"],"from_env":[],"from_file":[]}`, http.StatusOK)
	if fake.reloads != 1 {
		t.Fatalf("reloads = %d, want 1", fake.reloads)
	}
	conf, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(conf.NoProxy.List, []string{"a", "b"}) || len(conf.NoProxy.FromEnv) != 0 || len(conf.NoProxy.FromFile) != 0 {
		t.Fatalf("saved no-proxy = %#v, want replaced bypass lists", conf.NoProxy)
	}
	if conf.CurrentContext != "a" || len(conf.Contexts) != 1 || conf.Proxy.Port != 1087 {
		t.Fatalf("saved config = %#v, want unchanged context and proxy", conf)
	}
}

func TestConfigResourceNullBody(t *testing.T) {
	for _, test := range []struct {
		target    string
		wantError string
	}{
		{target: "", wantError: "config is nil"},
		{target: "/current-context", wantError: "current context is nil"},
		{target: "/proxy", wantError: "proxy is nil"},
		{target: "/no-proxy", wantError: "no-proxy is nil"},
	} {
		t.Run(test.target, func(t *testing.T) {
			handler, fake, store := setupConfigAPI(t)
			response := requestAPI(t, handler, http.MethodPut, "/apis/configs"+test.target, `null`, http.StatusBadRequest)
			if !strings.Contains(response.Body.String(), test.wantError) {
				t.Fatalf("body = %q, want error containing %q", response.Body.String(), test.wantError)
			}
			if fake.reloads != 0 {
				t.Fatalf("reloads = %d, want 0", fake.reloads)
			}
			assertConfigYAML(t, store, testConfigYAML)
		})
	}
}

func TestUpdateConfig(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		reloadError error
		wantStatus  int
		wantError   string
		wantReloads int
		wantPort    uint32
	}{
		{
			name:       "invalid_port",
			body:       `{"proxy":{"port":70000}}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "proxy.port",
			wantPort:   1087,
		},
		{
			name:        "valid",
			wantStatus:  http.StatusOK,
			wantReloads: 1,
			wantPort:    1088,
		},
		{
			name:        "reload_failure_keeps_saved_config",
			reloadError: errors.New("listen tcp: address already in use"),
			wantStatus:  http.StatusBadRequest,
			wantError:   "listen tcp: address already in use",
			wantReloads: 1,
			wantPort:    1088,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, fake, store := setupConfigAPI(t)
			fake.err = test.reloadError
			body := test.body
			if body == "" {
				response := requestAPI(t, handler, http.MethodGet, "/apis/configs", "", http.StatusOK)
				var conf map[string]any
				if err := json.Unmarshal(response.Body.Bytes(), &conf); err != nil {
					t.Fatal(err)
				}
				proxy, ok := conf["proxy"].(map[string]any)
				if !ok {
					t.Fatalf("proxy = %#v, want an object", conf["proxy"])
				}
				proxy["port"] = test.wantPort
				data, err := json.Marshal(conf)
				if err != nil {
					t.Fatal(err)
				}
				body = string(data)
			}
			response := requestAPI(t, handler, http.MethodPut, "/apis/configs", body, test.wantStatus)
			if test.wantError != "" && !strings.Contains(response.Body.String(), test.wantError) {
				t.Fatalf("body = %q, want error containing %q", response.Body.String(), test.wantError)
			}
			if fake.reloads != test.wantReloads {
				t.Fatalf("reloads = %d, want %d", fake.reloads, test.wantReloads)
			}
			conf, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if conf.Proxy.Port != test.wantPort {
				t.Fatalf("saved proxy.port = %d, want %d", conf.Proxy.Port, test.wantPort)
			}
			if test.wantReloads == 0 {
				assertConfigYAML(t, store, testConfigYAML)
			}
		})
	}
}

func TestUpdateConfigBodyTooLarge(t *testing.T) {
	handler, fake, store := setupConfigAPI(t)
	body := `{"proxy":{"port":1088}}`
	body += strings.Repeat(" ", (2<<20)-len(body))
	request := httptest.NewRequest(http.MethodPut, "/apis/configs", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code < http.StatusBadRequest {
		t.Errorf("status = %d, want >= 400", response.Code)
	}
	if fake.reloads != 0 {
		t.Errorf("reloads = %d, want 0", fake.reloads)
	}
	assertConfigYAML(t, store, testConfigYAML)
}

func TestRawConfig(t *testing.T) {
	handler, fake, store := setupConfigAPI(t)
	response := requestAPI(t, handler, http.MethodGet, "/apis/configs/raw", "", http.StatusOK)
	var raw struct {
		YAML string `json:"yaml"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if raw.YAML != testConfigYAML {
		t.Fatalf("yaml = %q, want %q", raw.YAML, testConfigYAML)
	}
	raw.YAML += "# added"
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	requestAPI(t, handler, http.MethodPut, "/apis/configs/raw", string(data), http.StatusOK)
	if fake.reloads != 1 {
		t.Fatalf("reloads = %d, want 1", fake.reloads)
	}
	assertConfigYAML(t, store, raw.YAML)
}

func TestUpdateRawConfigInvalid(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantError string
	}{
		{name: "malformed_yaml", body: `{"yaml":"proxy: [oops"}`, wantError: "yaml:"},
		{name: "blank_yaml", body: `{"yaml":"   "}`, wantError: "config is empty"},
		{name: "null", body: `null`, wantError: "config is empty"},
		{name: "invalid_port", body: `{"yaml":"proxy:\n  port: 70000\n"}`, wantError: "proxy.port"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, fake, store := setupConfigAPI(t)
			response := requestAPI(t, handler, http.MethodPut, "/apis/configs/raw", test.body, http.StatusBadRequest)
			if !strings.Contains(response.Body.String(), test.wantError) {
				t.Fatalf("body = %q, want error containing %q", response.Body.String(), test.wantError)
			}
			if fake.reloads != 0 {
				t.Fatalf("reloads = %d, want 0", fake.reloads)
			}
			assertConfigYAML(t, store, testConfigYAML)
		})
	}
}

func TestConfigStatus(t *testing.T) {
	for _, errorMessage := range []string{"boom", ""} {
		t.Run("error="+errorMessage, func(t *testing.T) {
			handler, fake, _ := setupConfigAPI(t)
			fake.status = configs.Status{Address: "127.0.0.1:1088", Running: true, Error: errorMessage}
			response := requestAPI(t, handler, http.MethodGet, "/apis/configs/status", "", http.StatusOK)
			var status map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
				t.Fatal(err)
			}
			if status["address"] != fake.status.Address || status["running"] != fake.status.Running {
				t.Fatalf("status = %#v, want %#v", status, fake.status)
			}
			gotError, present := status["error"]
			if errorMessage == "" {
				if present {
					t.Fatalf("status contains error: %#v", gotError)
				}
			} else if gotError != errorMessage {
				t.Fatalf("error = %#v, want %q", gotError, errorMessage)
			}
		})
	}
}

func TestUnknownAPIRoute(t *testing.T) {
	handler, _, _ := setupConfigAPI(t)
	requestAPI(t, handler, http.MethodGet, "/apis/nope", "", http.StatusNotFound)
}

func TestConfigAPIIsolation(t *testing.T) {
	handler, fake, store := setupConfigAPI(t)
	otherHandler, otherFake, otherStore := setupConfigAPI(t)
	fake.status = configs.Status{Address: "127.0.0.1:1088", Running: true}
	otherFake.status = configs.Status{Address: "127.0.0.1:1089", Error: "not running"}
	requestAPI(t, handler, http.MethodPut, "/apis/configs", `{"proxy":{"port":1088}}`, http.StatusOK)
	if fake.reloads != 1 || otherFake.reloads != 0 {
		t.Fatalf("reloads = %d, %d; want 1, 0", fake.reloads, otherFake.reloads)
	}
	conf, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if conf.Proxy.Port != 1088 {
		t.Fatalf("saved proxy.port = %d, want 1088", conf.Proxy.Port)
	}
	assertConfigYAML(t, otherStore, testConfigYAML)
	for candidate, want := range map[http.Handler]configs.Status{handler: fake.status, otherHandler: otherFake.status} {
		response := requestAPI(t, candidate, http.MethodGet, "/apis/configs/status", "", http.StatusOK)
		var got configs.Status
		if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("status = %#v, want %#v", got, want)
		}
	}
}
