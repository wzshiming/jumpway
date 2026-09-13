package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wzshiming/jumpway/app/web/runtime"
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
	status  runtime.Status
}

func (fake *fakeRuntime) Reload() error {
	fake.reloads++
	return fake.err
}

func (fake *fakeRuntime) Status() runtime.Status {
	return fake.status
}

func setupConfigAPI(t *testing.T) (http.Handler, *fakeRuntime) {
	t.Helper()
	originalDir := config.GetConfigDir()
	config.SetConfigDir(t.TempDir())
	t.Cleanup(func() { config.SetConfigDir(originalDir) })
	originalRuntime := runtime.Get()
	fake := &fakeRuntime{}
	runtime.Set(fake)
	t.Cleanup(func() { runtime.Set(originalRuntime) })
	if err := os.WriteFile(filepath.Join(config.GetConfigDir(), "config.yaml"), []byte(testConfigYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	return Handler(), fake
}

func requestAPI(t *testing.T, handler http.Handler, method, target, body string, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if method == http.MethodPut {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("%s %s: status = %d, want %d; body = %s", method, target, response.Code, wantStatus, response.Body.String())
	}
	return response
}

func assertConfigYAML(t *testing.T, want string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(config.GetConfigDir(), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("config.yaml = %q, want %q", data, want)
	}
}

func TestGetConfig(t *testing.T) {
	handler, _ := setupConfigAPI(t)
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
			handler, fake := setupConfigAPI(t)
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
			conf, err := config.LoadConfig()
			if err != nil {
				t.Fatal(err)
			}
			if conf.Proxy.Port != test.wantPort {
				t.Fatalf("saved proxy.port = %d, want %d", conf.Proxy.Port, test.wantPort)
			}
			if test.wantReloads == 0 {
				assertConfigYAML(t, testConfigYAML)
			}
		})
	}
}

func TestRawConfig(t *testing.T) {
	handler, fake := setupConfigAPI(t)
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
	assertConfigYAML(t, raw.YAML)
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
			handler, fake := setupConfigAPI(t)
			response := requestAPI(t, handler, http.MethodPut, "/apis/configs/raw", test.body, http.StatusBadRequest)
			if !strings.Contains(response.Body.String(), test.wantError) {
				t.Fatalf("body = %q, want error containing %q", response.Body.String(), test.wantError)
			}
			if fake.reloads != 0 {
				t.Fatalf("reloads = %d, want 0", fake.reloads)
			}
			assertConfigYAML(t, testConfigYAML)
		})
	}
}

func TestConfigStatus(t *testing.T) {
	for _, errorMessage := range []string{"boom", ""} {
		t.Run("error="+errorMessage, func(t *testing.T) {
			handler, fake := setupConfigAPI(t)
			fake.status = runtime.Status{Address: "127.0.0.1:1088", Running: true, Error: errorMessage}
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
	handler, _ := setupConfigAPI(t)
	requestAPI(t, handler, http.MethodGet, "/apis/nope", "", http.StatusNotFound)
}
