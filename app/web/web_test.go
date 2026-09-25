package web

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/wzshiming/jumpway/app/web/services/configs"
	"github.com/wzshiming/jumpway/app/web/services/stats"
	"github.com/wzshiming/jumpway/config"
	"github.com/wzshiming/jumpway/metrics"
	"github.com/wzshiming/jumpway/netproc"
	"golang.org/x/net/html"
)

const testConfigYAML = "web_ui:\n" +
	"  host: 127.0.0.1\n" +
	"  port: 1088\n" +
	"rules:\n" +
	"  - name: a\n" +
	"    listen:\n" +
	"      host: 127.0.0.1\n" +
	"      port: 1087\n" +
	"    forward:\n" +
	"      way:\n" +
	"        - lb:\n" +
	"            - socks5://127.0.0.1:1080\n" +
	"# keep me\n"

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

type fakeSource struct {
	snapshot      metrics.Snapshot
	resets        int
	disconnected  []uint64
	disconnectErr error
}

func (fake *fakeSource) Snapshot() metrics.Snapshot {
	return fake.snapshot
}

func (fake *fakeSource) Reset() {
	fake.resets++
}

func (fake *fakeSource) Disconnect(id uint64) error {
	fake.disconnected = append(fake.disconnected, id)
	return fake.disconnectErr
}

func setupConfigAPI(t *testing.T) (http.Handler, *fakeRuntime, *config.Store, *fakeSource) {
	t.Helper()
	store := config.NewStore(t.TempDir())
	fake := &fakeRuntime{}
	source := &fakeSource{}
	if err := os.WriteFile(store.Path(), []byte(testConfigYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	metricsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "# stub metrics\n")
	})
	return NewHandler(configs.NewConfigsService(store, fake), stats.NewStatsService(source), metricsHandler), fake, store, source
}

func requestAPI(t *testing.T, handler http.Handler, method, target, body string, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	return requestEncoded(t, handler, method, target, body, "", wantStatus)
}

func requestEncoded(t *testing.T, handler http.Handler, method, target, body, acceptEncoding string, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if method == http.MethodPut || method == http.MethodPost {
		request.Header.Set("Content-Type", "application/json")
	}
	if acceptEncoding != "" {
		request.Header.Set("Accept-Encoding", acceptEncoding)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("%s %s: status = %d, want %d; body = %s", method, target, response.Code, wantStatus, response.Body.String())
	}
	return response
}

func gunzip(t *testing.T, response *httptest.ResponseRecorder) []byte {
	t.Helper()
	if encoding := response.Header().Values("Content-Encoding"); !slices.Equal(encoding, []string{"gzip"}) {
		t.Fatalf("Content-Encoding = %q, want [gzip]", encoding)
	}
	reader, err := gzip.NewReader(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return data
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

func listRulesAPI(t *testing.T, handler http.Handler) []config.Rule {
	t.Helper()
	response := requestAPI(t, handler, http.MethodGet, "/apis/configs/rules", "", http.StatusOK)
	var rules []config.Rule
	if err := json.Unmarshal(response.Body.Bytes(), &rules); err != nil {
		t.Fatal(err)
	}
	if rules == nil {
		t.Fatal("rules = null, want a JSON array")
	}
	return rules
}

func TestStatsGet(t *testing.T) {
	handler, _, _, fake := setupConfigAPI(t)
	fake.snapshot = metrics.Snapshot{
		Since: "2026-09-15T12:00:00Z",
		Rules: []metrics.RuleStats{{
			Name: "a",
			Stats: metrics.Stats{
				Up:           10,
				Down:         20,
				PeakRateUp:   4,
				PeakRateDown: 8,
				LastUp:       "2026-09-15T12:00:02.123Z",
				LastDown:     "2026-09-15T12:00:03.456Z",
				Active:       1,
				Total:        3,
				Dials:        2,
				DialFailures: 1,
				LatencyMs:    1.5,
				AvgLatencyMs: 2.5,
			},
			Listen: []metrics.Hop{},
			Forward: []metrics.Hop{{
				Index:       0,
				ParentIndex: -1,
				URLs:        []metrics.URLStats{{URL: "ssh://u:xxxxx@h:22"}},
			}},
			Targets: []metrics.Target{{Address: "example.com:443", Via: "ssh://u:xxxxx@h:22"}},
			Connections: []metrics.Connection{{
				ID: 42, Client: "127.0.0.1:12345", Target: "example.com:443", Via: "ssh://u:xxxxx@h:22",
				Process: &netproc.Process{PID: 4242, Name: "curl"},
				Path:    []metrics.PathHop{{Index: 0, URL: "ssh://u:xxxxx@h:22", Dialed: true}},
				Started: "2026-09-15T12:00:01.123Z",
				Stats: metrics.Stats{
					Up: 3, Down: 5, RateUp: 1, RateDown: 2, PeakRateUp: 3, PeakRateDown: 5,
					LastUp: "2026-09-15T12:00:02.123Z", LastDown: "2026-09-15T12:00:03.456Z",
				},
			}},
		}},
	}
	response := requestAPI(t, handler, http.MethodGet, "/apis/stats", "", http.StatusOK)
	var snapshot map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["since"] != fake.snapshot.Since {
		t.Fatalf("since = %#v, want %q", snapshot["since"], fake.snapshot.Since)
	}
	rules, ok := snapshot["rules"].([]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("rules = %#v, want one rule", snapshot["rules"])
	}
	rule, ok := rules[0].(map[string]any)
	if !ok || rule["name"] != "a" {
		t.Fatalf("rule = %#v, want rule a", rules[0])
	}
	counters, ok := rule["stats"].(map[string]any)
	if !ok {
		t.Fatalf("stats = %#v, want an object", rule["stats"])
	}
	for key, want := range map[string]float64{
		"up": 10, "down": 20, "active": 1, "total": 3,
		"peak_rate_up": 4, "peak_rate_down": 8,
		"dials": 2, "dial_failures": 1, "latency_ms": 1.5, "avg_latency_ms": 2.5,
	} {
		if counters[key] != want {
			t.Errorf("stats.%s = %#v, want %v", key, counters[key], want)
		}
	}
	for key, want := range map[string]string{"last_up": fake.snapshot.Rules[0].Stats.LastUp, "last_down": fake.snapshot.Rules[0].Stats.LastDown} {
		if counters[key] != want {
			t.Errorf("stats.%s = %#v, want %q", key, counters[key], want)
		}
	}
	listen, ok := rule["listen"].([]any)
	if !ok || listen == nil || len(listen) != 0 {
		t.Fatalf("listen = %#v, want an empty array", rule["listen"])
	}
	forward, ok := rule["forward"].([]any)
	if !ok || len(forward) != 1 {
		t.Fatalf("forward = %#v, want one hop", rule["forward"])
	}
	hop, ok := forward[0].(map[string]any)
	if !ok || hop["index"] != float64(0) || hop["parent_index"] != float64(-1) {
		t.Fatalf("hop = %#v, want index 0 and parent_index -1", forward[0])
	}
	urls, ok := hop["urls"].([]any)
	if !ok || len(urls) != 1 {
		t.Fatalf("urls = %#v, want one URL", hop["urls"])
	}
	url, ok := urls[0].(map[string]any)
	if !ok || url["url"] != "ssh://u:xxxxx@h:22" {
		t.Fatalf("url = %#v, want the redacted SSH URL", urls[0])
	}
	targets, ok := rule["targets"].([]any)
	if !ok || len(targets) != 1 {
		t.Fatalf("targets = %#v, want one target", rule["targets"])
	}
	target, ok := targets[0].(map[string]any)
	if !ok || target["address"] != "example.com:443" || target["via"] != "ssh://u:xxxxx@h:22" {
		t.Fatalf("target = %#v, want example.com:443 via the redacted SSH URL", targets[0])
	}
	connections, ok := rule["connections"].([]any)
	if !ok || len(connections) != 1 {
		t.Fatalf("connections = %#v, want one connection", rule["connections"])
	}
	want := map[string]any{
		"id": float64(42), "client": "127.0.0.1:12345", "target": "example.com:443", "via": "ssh://u:xxxxx@h:22",
		"process": map[string]any{"pid": float64(4242), "name": "curl"},
		"path":    []any{map[string]any{"index": float64(0), "url": "ssh://u:xxxxx@h:22", "dialed": true}},
		"started": "2026-09-15T12:00:01.123Z",
		"stats": map[string]any{
			"up": float64(3), "down": float64(5), "rate_up": float64(1), "rate_down": float64(2),
			"peak_rate_up": float64(3), "peak_rate_down": float64(5),
			"active": float64(0), "total": float64(0), "dials": float64(0), "dial_failures": float64(0),
			"latency_ms": float64(0), "avg_latency_ms": float64(0),
			"last_up": "2026-09-15T12:00:02.123Z", "last_down": "2026-09-15T12:00:03.456Z",
		},
	}
	if !reflect.DeepEqual(connections[0], want) {
		t.Fatalf("connection = %#v, want %#v", connections[0], want)
	}
}

func TestStatsReset(t *testing.T) {
	handler, _, _, fake := setupConfigAPI(t)
	for want := 1; want <= 2; want++ {
		response := requestAPI(t, handler, http.MethodDelete, "/apis/stats", "", http.StatusOK)
		if strings.TrimSpace(response.Body.String()) != "null" {
			t.Fatalf("body = %q, want null", response.Body.String())
		}
		if fake.resets != want {
			t.Fatalf("resets = %d, want %d", fake.resets, want)
		}
	}
}

func TestStatsDisconnect(t *testing.T) {
	for _, test := range []struct {
		name   string
		path   string
		id     uint64
		err    error
		status int
	}{
		{name: "success", path: "42", id: 42, status: http.StatusOK},
		{name: "max uint64", path: "18446744073709551615", id: ^uint64(0), status: http.StatusOK},
		{name: "source error", path: "42", id: 42, err: errors.New("connection 42 not found"), status: http.StatusBadRequest},
		{name: "invalid", path: "invalid", status: http.StatusBadRequest},
		{name: "negative", path: "-1", status: http.StatusBadRequest},
		{name: "overflow", path: "18446744073709551616", status: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, _, _, fake := setupConfigAPI(t)
			fake.disconnectErr = test.err
			response := requestAPI(t, handler, http.MethodDelete, "/apis/stats/connections/"+test.path, "", test.status)
			if test.id != 0 {
				if !slices.Equal(fake.disconnected, []uint64{test.id}) {
					t.Fatalf("disconnected = %v, want [%d]", fake.disconnected, test.id)
				}
			} else if len(fake.disconnected) != 0 {
				t.Fatalf("invalid ID reached source: %v", fake.disconnected)
			}
			if test.err != nil && !strings.Contains(response.Body.String(), test.err.Error()) {
				t.Fatalf("body = %q, want %q", response.Body.String(), test.err.Error())
			}
			if test.status == http.StatusOK && strings.TrimSpace(response.Body.String()) != "null" {
				t.Fatalf("body = %q, want null", response.Body.String())
			}
			if fake.resets != 0 {
				t.Fatal("disconnect reset statistics")
			}
		})
	}
}

func TestMetricsEndpoint(t *testing.T) {
	handler, _, _, _ := setupConfigAPI(t)
	response := requestAPI(t, handler, http.MethodGet, "/metrics", "", http.StatusOK)
	if response.Body.String() != "# stub metrics\n" {
		t.Fatalf("body = %q, want stub metrics", response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "text/plain" {
		t.Fatalf("Content-Type = %q, want text/plain", contentType)
	}
}

func TestStaticsIndex(t *testing.T) {
	handler, _, _, _ := setupConfigAPI(t)
	response := requestAPI(t, handler, http.MethodGet, "/", "", http.StatusOK)
	if mediaType := mediaTypeOf(t, response); mediaType != "text/html" {
		t.Fatalf("GET /: Content-Type = %q, want text/html", response.Header().Get("Content-Type"))
	}
	document, err := html.Parse(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	attr := func(node *html.Node, key string) string {
		for _, attribute := range node.Attr {
			if attribute.Key == key {
				return attribute.Val
			}
		}
		return ""
	}
	javascript := []string{"text/javascript", "application/javascript"}
	stylesheet := []string{"text/css"}
	type asset struct {
		kind       string
		reference  string
		mediaTypes []string
	}
	var assets []asset
	counts := map[string]int{}
	for node := range document.Descendants() {
		if node.Type != html.ElementNode {
			continue
		}
		switch node.Data {
		case "script":
			if src := attr(node, "src"); src != "" {
				assets = append(assets, asset{kind: "script", reference: src, mediaTypes: javascript})
			}
		case "link":
			href := attr(node, "href")
			switch attr(node, "rel") {
			case "stylesheet":
				assets = append(assets, asset{kind: "stylesheet", reference: href, mediaTypes: stylesheet})
			case "modulepreload":
				assets = append(assets, asset{kind: "script", reference: href, mediaTypes: javascript})
			case "icon":
				assets = append(assets, asset{kind: "icon", reference: href, mediaTypes: []string{"image/"}})
			}
		case "img":
			assets = append(assets, asset{kind: "image", reference: attr(node, "src"), mediaTypes: []string{"image/"}})
		}
	}
	base := &url.URL{Path: "/"}
	for _, candidate := range assets {
		counts[candidate.kind]++
		reference, err := url.Parse(candidate.reference)
		if err != nil {
			t.Errorf("%s %q: %v", candidate.kind, candidate.reference, err)
			continue
		}
		if reference.Scheme != "" || reference.Host != "" {
			t.Errorf("%s %q is not served from this origin", candidate.kind, candidate.reference)
			continue
		}
		target := base.ResolveReference(reference).EscapedPath()
		response := requestAPI(t, handler, http.MethodGet, target, "", http.StatusOK)
		mediaType := mediaTypeOf(t, response)
		if !slices.ContainsFunc(candidate.mediaTypes, func(want string) bool { return strings.HasPrefix(mediaType, want) }) {
			t.Errorf("%s %s: Content-Type = %q, want one of %q", candidate.kind, target, response.Header().Get("Content-Type"), candidate.mediaTypes)
		}
		if response.Body.Len() == 0 {
			t.Errorf("%s %s: empty body", candidate.kind, target)
		}
	}
	if counts["script"] == 0 || counts["stylesheet"] == 0 || counts["icon"] == 0 {
		t.Fatalf("index.html references %d scripts, %d stylesheets and %d icons, want at least one of each", counts["script"], counts["stylesheet"], counts["icon"])
	}
	requestAPI(t, handler, http.MethodGet, "/does-not-exist.js", "", http.StatusNotFound)
	requestAPI(t, handler, http.MethodGet, "/assets/does-not-exist.js", "", http.StatusNotFound)
}

func mediaTypeOf(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	mediaType, _, err := mime.ParseMediaType(response.Header().Get("Content-Type"))
	if err != nil {
		t.Fatalf("Content-Type = %q: %v", response.Header().Get("Content-Type"), err)
	}
	return mediaType
}

func indexScript(t *testing.T, handler http.Handler) string {
	t.Helper()
	response := requestAPI(t, handler, http.MethodGet, "/", "", http.StatusOK)
	document, err := html.Parse(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	for node := range document.Descendants() {
		if node.Type != html.ElementNode || node.Data != "script" {
			continue
		}
		for _, attribute := range node.Attr {
			if attribute.Key == "src" && strings.HasPrefix(attribute.Val, "/assets/") {
				return attribute.Val
			}
		}
	}
	t.Fatal("index.html references no /assets/ script")
	return ""
}

func TestStaticsAssetCompression(t *testing.T) {
	handler, _, _, _ := setupConfigAPI(t)
	target := indexScript(t, handler)
	plain := requestAPI(t, handler, http.MethodGet, target, "", http.StatusOK)
	compressed := requestEncoded(t, handler, http.MethodGet, target, "", "gzip", http.StatusOK)
	if encoding := plain.Header().Values("Content-Encoding"); len(encoding) != 0 {
		t.Fatalf("plain Content-Encoding = %q, want none", encoding)
	}
	if mediaType := mediaTypeOf(t, plain); mediaType != "text/javascript" && mediaType != "application/javascript" {
		t.Fatalf("Content-Type = %q, want JavaScript", plain.Header().Get("Content-Type"))
	}
	if got, want := compressed.Header().Get("Content-Type"), plain.Header().Get("Content-Type"); got != want {
		t.Fatalf("gzip Content-Type = %q, want %q", got, want)
	}
	const immutable = "public, max-age=31536000, immutable"
	for name, response := range map[string]*httptest.ResponseRecorder{"plain": plain, "gzip": compressed} {
		if got := response.Header().Get("Cache-Control"); got != immutable {
			t.Errorf("%s Cache-Control = %q, want %q", name, got, immutable)
		}
	}
	if vary := strings.Join(compressed.Header().Values("Vary"), ","); !strings.Contains(vary, "Accept-Encoding") {
		t.Errorf("Vary = %q, want Accept-Encoding", vary)
	}
	if compressed.Body.Len() >= plain.Body.Len() {
		t.Errorf("gzip body = %d bytes, want fewer than %d", compressed.Body.Len(), plain.Body.Len())
	}
	if body := gunzip(t, compressed); !bytes.Equal(body, plain.Body.Bytes()) {
		t.Fatalf("gunzipped body = %d bytes, want the %d plain bytes", len(body), plain.Body.Len())
	}
}

func TestStaticsCacheControl(t *testing.T) {
	handler, _, _, _ := setupConfigAPI(t)
	for _, test := range []struct {
		target string
		status int
		body   string
	}{
		{target: "/", status: http.StatusOK},
		{target: "/missing.js", status: http.StatusNotFound, body: "404 page not found\n"},
		{target: "/assets/missing.js", status: http.StatusNotFound, body: "404 page not found\n"},
	} {
		t.Run(test.target, func(t *testing.T) {
			plain := requestAPI(t, handler, http.MethodGet, test.target, "", test.status)
			compressed := requestEncoded(t, handler, http.MethodGet, test.target, "", "gzip", test.status)
			for name, response := range map[string]*httptest.ResponseRecorder{"plain": plain, "gzip": compressed} {
				if got := response.Header().Get("Cache-Control"); got != "no-cache" {
					t.Errorf("%s Cache-Control = %q, want no-cache", name, got)
				}
			}
			if encoding := plain.Header().Values("Content-Encoding"); len(encoding) != 0 {
				t.Fatalf("plain Content-Encoding = %q, want none", encoding)
			}
			if body := gunzip(t, compressed); !bytes.Equal(body, plain.Body.Bytes()) {
				t.Fatalf("gunzipped body = %q, want %q", body, plain.Body.Bytes())
			}
			if test.body != "" && plain.Body.String() != test.body {
				t.Fatalf("body = %q, want %q", plain.Body.String(), test.body)
			}
		})
	}
}

// FileServer answers an unsatisfiable Range through its error path, which drops the headers set before it ran.
func TestStaticsRangeError(t *testing.T) {
	handler, _, _, _ := setupConfigAPI(t)
	target := indexScript(t, handler)
	for _, target := range []string{target, "/"} {
		t.Run(target, func(t *testing.T) {
			responses := map[string]*httptest.ResponseRecorder{}
			for _, encoding := range []string{"", "gzip"} {
				request := httptest.NewRequest(http.MethodGet, target, nil)
				request.Header.Set("Range", "bytes=999999999-")
				if encoding != "" {
					request.Header.Set("Accept-Encoding", encoding)
				}
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				if response.Code != http.StatusRequestedRangeNotSatisfiable {
					t.Fatalf("Accept-Encoding %q: status = %d, want 416", encoding, response.Code)
				}
				if got := response.Header().Get("Cache-Control"); got != "no-cache" {
					t.Errorf("Accept-Encoding %q: Cache-Control = %q, want no-cache", encoding, got)
				}
				responses[encoding] = response
			}
			if encoding := responses[""].Header().Values("Content-Encoding"); len(encoding) != 0 {
				t.Fatalf("plain Content-Encoding = %q, want none", encoding)
			}
			if body := gunzip(t, responses["gzip"]); !bytes.Equal(body, responses[""].Body.Bytes()) {
				t.Fatalf("gunzipped body = %q, want %q", body, responses[""].Body.Bytes())
			}
		})
	}
}

func TestStatsCompression(t *testing.T) {
	handler, _, _, fake := setupConfigAPI(t)
	fake.snapshot = metrics.Snapshot{
		Since: "2026-09-15T12:00:00Z",
		Rules: []metrics.RuleStats{{Name: "a", Stats: metrics.Stats{Up: 10, Down: 20}, Listen: []metrics.Hop{}, Forward: []metrics.Hop{}}},
	}
	plain := requestAPI(t, handler, http.MethodGet, "/apis/stats", "", http.StatusOK)
	compressed := requestEncoded(t, handler, http.MethodGet, "/apis/stats", "", "gzip", http.StatusOK)
	if got, want := compressed.Header().Get("Content-Type"), plain.Header().Get("Content-Type"); got != want || mediaTypeOf(t, plain) != "application/json" {
		t.Fatalf("gzip Content-Type = %q, plain = %q; want identical application/json", got, want)
	}
	body := gunzip(t, compressed)
	if !bytes.Equal(body, plain.Body.Bytes()) {
		t.Fatalf("gunzipped body = %s, want %s", body, plain.Body.Bytes())
	}
	var snapshot map[string]any
	if err := json.Unmarshal(body, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["since"] != fake.snapshot.Since {
		t.Fatalf("since = %#v, want %q", snapshot["since"], fake.snapshot.Since)
	}
	rules, ok := snapshot["rules"].([]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("rules = %#v, want one rule", snapshot["rules"])
	}
	rule, ok := rules[0].(map[string]any)
	if !ok || rule["name"] != "a" {
		t.Fatalf("rule = %#v, want rule a", rules[0])
	}
	if counters, ok := rule["stats"].(map[string]any); !ok || counters["up"] != float64(10) || counters["down"] != float64(20) {
		t.Fatalf("stats = %#v, want up 10 and down 20", rule["stats"])
	}
}

func TestMetricsCompression(t *testing.T) {
	var acceptEncoding string
	exposition := metrics.NewHandler(metrics.NewRegistry())
	metricsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acceptEncoding = r.Header.Get("Accept-Encoding")
		exposition.ServeHTTP(w, r)
	})
	handler := NewHandler(configs.NewConfigsService(config.NewStore(t.TempDir()), &fakeRuntime{}), stats.NewStatsService(&fakeSource{}), metricsHandler)
	response := requestEncoded(t, handler, http.MethodGet, "/metrics", "", "gzip", http.StatusOK)
	// CompressHandler drops Accept-Encoding before delegating, so promhttp only sees it when nothing wraps /metrics.
	if acceptEncoding != "gzip" {
		t.Fatalf("promhttp saw Accept-Encoding = %q, want gzip", acceptEncoding)
	}
	encoding := response.Header().Values("Content-Encoding")
	if len(encoding) > 1 {
		t.Fatalf("Content-Encoding = %q, want at most one value", encoding)
	}
	body := response.Body.Bytes()
	if slices.Equal(encoding, []string{"gzip"}) {
		body = gunzip(t, response)
	}
	if !strings.Contains(string(body), "jumpway_stats_reset_timestamp_seconds") {
		t.Fatalf("body = %.200q, want jumpway_ metrics", body)
	}
}

func TestWebUICompression(t *testing.T) {
	handler, fake, store, _ := setupConfigAPI(t)
	response := requestEncoded(t, handler, http.MethodPut, "/apis/configs/web-ui", `{"host":"127.0.0.1","port":70000}`, "gzip", http.StatusBadRequest)
	if mediaType := mediaTypeOf(t, response); mediaType != "text/plain" {
		t.Fatalf("Content-Type = %q, want text/plain", response.Header().Get("Content-Type"))
	}
	if body := gunzip(t, response); !strings.Contains(string(body), "web_ui.port") {
		t.Fatalf("body = %q, want invalid web_ui.port error", body)
	}
	if fake.reloads != 0 {
		t.Fatalf("reloads = %d, want 0", fake.reloads)
	}
	assertConfigYAML(t, store, testConfigYAML)
	response = requestEncoded(t, handler, http.MethodPut, "/apis/configs/web-ui", `{"host":"localhost","port":1098}`, "gzip", http.StatusOK)
	if body := gunzip(t, response); strings.TrimSpace(string(body)) != "null" {
		t.Fatalf("body = %q, want null", body)
	}
	if fake.reloads != 1 {
		t.Fatalf("reloads = %d, want 1", fake.reloads)
	}
	conf, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if conf.WebUI.Host != "localhost" || conf.WebUI.Port != 1098 {
		t.Fatalf("saved web_ui = %#v, want localhost:1098", conf.WebUI)
	}
}

func TestGetConfig(t *testing.T) {
	handler, _, _, _ := setupConfigAPI(t)
	response := requestAPI(t, handler, http.MethodGet, "/apis/configs", "", http.StatusOK)
	var conf map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &conf); err != nil {
		t.Fatal(err)
	}
	address, ok := conf["web_ui"].(map[string]any)
	if !ok || address["host"] != "127.0.0.1" || address["port"] != float64(1088) {
		t.Fatalf("web_ui = %#v, want the default web UI address", conf["web_ui"])
	}
	rules, ok := conf["rules"].([]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("rules = %#v, want one rule", conf["rules"])
	}
	rule, ok := rules[0].(map[string]any)
	if !ok || rule["name"] != "a" {
		t.Fatalf("rule = %#v, want rule a", rules[0])
	}
	listen, ok := rule["listen"].(map[string]any)
	if !ok || listen["host"] != "127.0.0.1" || listen["port"] != float64(1087) {
		t.Fatalf("listen = %#v, want the seeded rule address", rule["listen"])
	}
	forward, ok := rule["forward"].(map[string]any)
	if !ok {
		t.Fatalf("forward = %#v, want an object", rule["forward"])
	}
	way, ok := forward["way"].([]any)
	if !ok || len(way) != 1 || way[0] != "socks5://127.0.0.1:1080" {
		t.Fatalf("forward.way = %#v, want a single string node", forward["way"])
	}
	if _, ok := rule["way"]; ok {
		t.Fatalf("rule contains removed way key: %#v", rule)
	}
	for _, key := range []string{"host", "port"} {
		if _, ok := forward[key]; ok {
			t.Errorf("proxy forward contains %q: %#v", key, forward)
		}
	}
	if _, ok := conf["no_proxy"].(map[string]any); !ok {
		t.Fatalf("no_proxy = %#v, want an object", conf["no_proxy"])
	}
	for _, key := range []string{"current_context", "contexts", "proxy"} {
		if _, ok := conf[key]; ok {
			t.Errorf("config contains removed key %q", key)
		}
	}
}

func TestListRules(t *testing.T) {
	handler, _, _, _ := setupConfigAPI(t)
	rules := listRulesAPI(t, handler)
	if len(rules) != 1 || rules[0].Name != "a" || len(rules[0].Forward.Way) != 1 || !slices.Equal(rules[0].Forward.Way[0].LB, []string{"socks5://127.0.0.1:1080"}) {
		t.Fatalf("rules = %#v, want seeded rule a", rules)
	}
}

func TestCreateRule(t *testing.T) {
	handler, fake, store, _ := setupConfigAPI(t)
	requestAPI(t, handler, http.MethodPost, "/apis/configs/rules", `{"name":"b","listen":{"host":"::1","port":9000,"way":[{"lb":["ssh://user@host:22"]}],"username":"user","password":"secret"},"forward":{"way":[{"lb":["socks5://h:1080"]}]}}`, http.StatusOK)
	rules := listRulesAPI(t, handler)
	if len(rules) != 2 || rules[0].Name != "a" || rules[1].Name != "b" || len(rules[1].Forward.Way) != 1 || !slices.Equal(rules[1].Forward.Way[0].LB, []string{"socks5://h:1080"}) {
		t.Fatalf("rules = %#v, want a and b with the supplied forward.way", rules)
	}
	listen := rules[1].Listen
	if listen.Address() != "[::1]:9000" || !listen.Remote() || !slices.Equal(listen.Way[0].LB, []string{"ssh://user@host:22"}) || listen.Username != "user" || listen.Password != "secret" {
		t.Fatalf("listen = %#v, want the remote listener and credentials", listen)
	}
	if fake.reloads != 1 {
		t.Fatalf("reloads = %d, want 1", fake.reloads)
	}
	conf, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(conf.Rules, rules) || conf.WebUI.Port != 1088 || conf.Rules[0].Listen.Port != 1087 {
		t.Fatalf("saved config = %#v, want saved rules with unchanged web UI and rule a", conf)
	}
}

func TestCreatePortForwardRule(t *testing.T) {
	handler, fake, store, _ := setupConfigAPI(t)
	requestAPI(t, handler, http.MethodPost, "/apis/configs/rules", `{"name":"db","listen":{"port":15432},"forward":{"host":"10.0.0.5","port":5432,"way":[{"lb":["ssh://u@bastion:22"]}]}}`, http.StatusOK)
	response := requestAPI(t, handler, http.MethodGet, "/apis/configs/rules/db", "", http.StatusOK)
	var rule config.Rule
	if err := json.Unmarshal(response.Body.Bytes(), &rule); err != nil {
		t.Fatal(err)
	}
	if rule.Name != "db" || rule.Listen.Host != "" || rule.Listen.Port != 15432 || rule.Forward.Host != "10.0.0.5" || rule.Forward.Port != 5432 {
		t.Fatalf("rule = %#v, want db forwarding 15432 to 10.0.0.5:5432", rule)
	}
	if len(rule.Forward.Way) != 1 || !slices.Equal(rule.Forward.Way[0].LB, []string{"ssh://u@bastion:22"}) {
		t.Fatalf("forward.way = %#v, want the supplied SSH chain", rule.Forward.Way)
	}
	if fake.reloads != 1 {
		t.Fatalf("reloads = %d, want 1", fake.reloads)
	}
	conf, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(conf.Rules) != 2 || !reflect.DeepEqual(conf.Rules[1], rule) || conf.Rules[0].Name != "a" || conf.WebUI.Port != 1088 {
		t.Fatalf("saved config = %#v, want db and unchanged rule a and web UI", conf)
	}
}

func TestCreateVirtualRule(t *testing.T) {
	handler, fake, store, _ := setupConfigAPI(t)
	requestAPI(t, handler, http.MethodPost, "/apis/configs/rules", `{"name":"entry","listen":{"port":15433},"forward":{"virtual":"x"}}`, http.StatusOK)
	requestAPI(t, handler, http.MethodPost, "/apis/configs/rules", `{"name":"exit","listen":{"virtual":"x"}}`, http.StatusOK)
	response := requestAPI(t, handler, http.MethodGet, "/apis/configs/rules/entry", "", http.StatusOK)
	if want := `{"name":"entry","listen":{"host":"","port":15433},"forward":{"virtual":"x"}}`; strings.TrimSpace(response.Body.String()) != want {
		t.Fatalf("rule = %s, want %s", response.Body.String(), want)
	}
	if fake.reloads != 2 {
		t.Fatalf("reloads = %d, want 2", fake.reloads)
	}
	conf, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(conf.Rules) != 3 || conf.Rules[1].Forward.Virtual != "x" || conf.Rules[2].Listen.Virtual != "x" || !conf.Rules[2].Forward.IsProxy() {
		t.Fatalf("saved config = %#v, want virtual entry and exit rules", conf.Rules)
	}
}

func TestRuleProtocolsRoundTrip(t *testing.T) {
	handler, fake, store, _ := setupConfigAPI(t)
	const body = `{"name":"multi","listen":{"port":9000,"password":"shared","protocols":[{"type":"http","username":"alice"},{"type":"socks5","username":"bob","password":"bob-secret"},{"type":"ss","cipher":"aes-256-gcm"}]}}`
	requestAPI(t, handler, http.MethodPost, "/apis/configs/rules", body, http.StatusOK)
	want := []config.Protocol{{Type: "http", Username: "alice"}, {Type: "socks5", Username: "bob", Password: "bob-secret"}, {Type: "ss", Cipher: "aes-256-gcm"}}
	response := requestAPI(t, handler, http.MethodGet, "/apis/configs/rules/multi", "", http.StatusOK)
	var rule config.Rule
	if err := json.Unmarshal(response.Body.Bytes(), &rule); err != nil {
		t.Fatal(err)
	}
	if rule.Listen.Password != "shared" || !reflect.DeepEqual(rule.Listen.Protocols, want) {
		t.Fatalf("listen = %#v, want shared password and %#v", rule.Listen, want)
	}
	if want := `"protocols":[{"type":"http","username":"alice"},{"type":"socks5","username":"bob","password":"bob-secret"},{"type":"ss","cipher":"aes-256-gcm"}]`; !strings.Contains(response.Body.String(), want) {
		t.Fatalf("rule = %s, want %s", response.Body.String(), want)
	}
	conf, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(conf.Rules) != 2 || !reflect.DeepEqual(conf.Rules[1], rule) {
		t.Fatalf("saved rule = %#v, want %#v", conf.Rules[1], rule)
	}
	requestAPI(t, handler, http.MethodPut, "/apis/configs/rules/multi", `{"name":"multi","listen":{"port":9000,"protocols":[{"type":"socks4","username":"carol"}]}}`, http.StatusOK)
	response = requestAPI(t, handler, http.MethodGet, "/apis/configs/rules/multi", "", http.StatusOK)
	if want := `{"name":"multi","listen":{"host":"","port":9000,"protocols":[{"type":"socks4","username":"carol"}]},"forward":{}}`; strings.TrimSpace(response.Body.String()) != want {
		t.Fatalf("updated rule = %s, want %s", response.Body.String(), want)
	}
	if fake.reloads != 2 {
		t.Fatalf("reloads = %d, want 2", fake.reloads)
	}
	if rules := listRulesAPI(t, handler); len(rules) != 2 || rules[0].Listen.Protocols != nil {
		t.Fatalf("rules = %#v, want rule a without protocols", rules)
	}
}

func TestRawConfigProtocols(t *testing.T) {
	handler, fake, store, _ := setupConfigAPI(t)
	const yaml = "rules:\n" +
		"  - name: multi\n" +
		"    listen:\n" +
		"      port: 9000\n" +
		"      username: alice\n" +
		"      password: shared\n" +
		"      protocols:\n" +
		"        - type: http\n" +
		"        - type: ss\n" +
		"          cipher: aes-256-gcm\n" +
		"          password: ss-secret\n"
	data, err := json.Marshal(map[string]string{"yaml": yaml})
	if err != nil {
		t.Fatal(err)
	}
	requestAPI(t, handler, http.MethodPut, "/apis/configs/raw", string(data), http.StatusOK)
	if fake.reloads != 1 {
		t.Fatalf("reloads = %d, want 1", fake.reloads)
	}
	assertConfigYAML(t, store, yaml)
	response := requestAPI(t, handler, http.MethodGet, "/apis/configs/rules/multi", "", http.StatusOK)
	var rule config.Rule
	if err := json.Unmarshal(response.Body.Bytes(), &rule); err != nil {
		t.Fatal(err)
	}
	want := []config.Protocol{{Type: "http"}, {Type: "ss", Password: "ss-secret", Cipher: "aes-256-gcm"}}
	if rule.Listen.Username != "alice" || rule.Listen.Password != "shared" || !reflect.DeepEqual(rule.Listen.Protocols, want) {
		t.Fatalf("listen = %#v, want shared credentials and %#v", rule.Listen, want)
	}
	response = requestAPI(t, handler, http.MethodPut, "/apis/configs/raw", `{"yaml":"rules:\n  - name: a\n    listen:\n      protocols:\n        - type: ss\n"}`, http.StatusBadRequest)
	if !strings.Contains(response.Body.String(), "rules[0].listen.protocols[0].cipher is empty") || fake.reloads != 1 {
		t.Fatalf("body = %q, reloads = %d; want cipher error without reload", response.Body.String(), fake.reloads)
	}
	assertConfigYAML(t, store, yaml)
}

func TestCreateFirstRule(t *testing.T) {
	handler, fake, store, _ := setupConfigAPI(t)
	if err := store.SaveRaw([]byte("web_ui:\n  host: 127.0.0.1\n  port: 1088\n")); err != nil {
		t.Fatal(err)
	}
	if rules := listRulesAPI(t, handler); len(rules) != 0 {
		t.Fatalf("rules = %#v, want empty array", rules)
	}
	requestAPI(t, handler, http.MethodPost, "/apis/configs/rules", `{"name":"b","forward":{"way":[{"lb":["socks5://h:1080"]}]}}`, http.StatusOK)
	if fake.reloads != 1 {
		t.Fatalf("reloads = %d, want 1", fake.reloads)
	}
	conf, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(conf.Rules) != 1 || conf.Rules[0].Name != "b" || conf.WebUI.Port != 1088 {
		t.Fatalf("saved config = %#v, want b as the only rule and unchanged web UI", conf)
	}
}

func TestRuleMutationInvalid(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		target    string
		body      string
		wantError string
	}{
		{name: "duplicate", method: http.MethodPost, target: "/rules", body: `{"name":"a"}`, wantError: `rule "a" already exists`},
		{name: "empty_name", method: http.MethodPost, target: "/rules", body: `{}`, wantError: "rule name is empty"},
		{name: "blank_name", method: http.MethodPost, target: "/rules", body: `{"name":"  "}`, wantError: "rule name is empty"},
		{name: "null_create", method: http.MethodPost, target: "/rules", body: `null`, wantError: "rule name is empty"},
		{name: "slash_name", method: http.MethodPost, target: "/rules", body: `{"name":"a/b"}`, wantError: `rules[1].name "a/b" must not contain "/"`},
		{name: "empty_create_way", method: http.MethodPost, target: "/rules", body: `{"name":"b","forward":{"way":[{"lb":[]}]}}`, wantError: "rules[1].forward.way[0] has no proxy URL"},
		{name: "invalid_create_way", method: http.MethodPost, target: "/rules", body: `{"name":"b","forward":{"way":[{"lb":["socks5://[::1"]}]}}`, wantError: "rules[1].forward.way[0]: invalid proxy URL"},
		{name: "forward_way_no_scheme", method: http.MethodPost, target: "/rules", body: `{"name":"b","forward":{"way":[{"lb":["x"]}]}}`, wantError: `rules[1].forward.way[0]: proxy URL "x" has no scheme`},
		{name: "invalid_listen_way", method: http.MethodPost, target: "/rules", body: `{"name":"b","listen":{"way":[{"lb":["ssh://[::1"]}]}}`, wantError: "rules[1].listen.way[0]: invalid proxy URL"},
		{name: "listen_port", method: http.MethodPost, target: "/rules", body: `{"name":"b","listen":{"port":70000}}`, wantError: "rules[1].listen.port 70000"},
		{name: "forward_port", method: http.MethodPost, target: "/rules", body: `{"name":"db","forward":{"port":70000}}`, wantError: "rules[1].forward.port 70000"},
		{name: "forward_host_without_port", method: http.MethodPost, target: "/rules", body: `{"name":"db","forward":{"host":"10.0.0.5"}}`, wantError: "rules[1].forward.host is set but port is 0"},
		{name: "virtual_listen_with_port", method: http.MethodPost, target: "/rules", body: `{"name":"b","listen":{"virtual":"x","port":1}}`, wantError: "rules[1].listen.virtual must not be combined with port"},
		{name: "virtual_self_loop", method: http.MethodPost, target: "/rules", body: `{"name":"b","listen":{"virtual":"x"},"forward":{"virtual":"x"}}`, wantError: `rules[1].forward.virtual "x" loops back to its own listener`},
		{name: "port_forward_with_username", method: http.MethodPost, target: "/rules", body: `{"name":"db","listen":{"port":15432,"username":"user"},"forward":{"host":"10.0.0.5","port":5432}}`, wantError: "rules[1].listen.username is only used by proxy rules"},
		{name: "password_without_username", method: http.MethodPost, target: "/rules", body: `{"name":"b","listen":{"password":"secret"}}`, wantError: "rules[1].listen.password is set but username is empty"},
		{name: "unknown_protocol", method: http.MethodPost, target: "/rules", body: `{"name":"b","listen":{"protocols":[{"type":"http"},{"type":"quic"}]}}`, wantError: `rules[1].listen.protocols[1].type "quic" is unknown`},
		{name: "protocol_password_without_username", method: http.MethodPut, target: "/rules/a", body: `{"listen":{"protocols":[{"type":"socks5","password":"secret"}]}}`, wantError: "rules[0].listen.protocols[0].password is set but username is empty"},
		{name: "web_ui_collision", method: http.MethodPost, target: "/rules", body: `{"name":"b","listen":{"port":1088}}`, wantError: "rules[1].listen address 127.0.0.1:1088 is already used by web_ui"},
		{name: "rule_collision", method: http.MethodPost, target: "/rules", body: `{"name":"b","listen":{"port":1087}}`, wantError: `rules[1].listen address 127.0.0.1:1087 is already used by rule "a"`},
		{name: "unknown_update", method: http.MethodPut, target: "/rules/zzz", body: `{"name":"b"}`, wantError: `rule "zzz" not found`},
		{name: "null_update", method: http.MethodPut, target: "/rules/a", body: `null`, wantError: "rule is nil"},
		{name: "blank_update_name", method: http.MethodPut, target: "/rules/a", body: `{"name":"  "}`, wantError: "rules[0].name is empty"},
		{name: "invalid_update_way", method: http.MethodPut, target: "/rules/a", body: `{"forward":{"way":[{"lb":["/no-scheme"]}]}}`, wantError: "rules[0].forward.way[0]: proxy URL"},
		{name: "unknown_delete", method: http.MethodDelete, target: "/rules/zzz", wantError: `rule "zzz" not found`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, fake, store, _ := setupConfigAPI(t)
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

func TestGetRule(t *testing.T) {
	handler, _, _, _ := setupConfigAPI(t)
	response := requestAPI(t, handler, http.MethodGet, "/apis/configs/rules/a", "", http.StatusOK)
	var got config.Rule
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "a" || len(got.Forward.Way) != 1 || !slices.Equal(got.Forward.Way[0].LB, []string{"socks5://127.0.0.1:1080"}) {
		t.Fatalf("rule = %#v, want seeded rule a", got)
	}
	if got.Listen.Address() != "127.0.0.1:1087" || got.Disabled {
		t.Fatalf("rule = %#v, want the enabled local listener", got)
	}
	response = requestAPI(t, handler, http.MethodGet, "/apis/configs/rules/zzz", "", http.StatusBadRequest)
	if !strings.Contains(response.Body.String(), `rule "zzz" not found`) {
		t.Fatalf("body = %q, want rule not found error", response.Body.String())
	}
}

func TestUpdateRule(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		want string
	}{
		{name: "replace", body: `{"name":"a","disabled":true,"listen":{"port":9000},"forward":{"way":[{"lb":["socks5://h:1080"]}]}}`, want: "a"},
		{name: "rename", body: `{"name":"a2","disabled":true,"listen":{"port":9000},"forward":{"way":[{"lb":["socks5://h:1080"]}]}}`, want: "a2"},
		{name: "empty_name_keeps_path", body: `{"name":"","disabled":true,"listen":{"port":9000},"forward":{"way":[{"lb":["socks5://h:1080"]}]}}`, want: "a"},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, fake, store, _ := setupConfigAPI(t)
			requestAPI(t, handler, http.MethodPut, "/apis/configs/rules/a", test.body, http.StatusOK)
			rules := listRulesAPI(t, handler)
			if len(rules) != 1 || rules[0].Name != test.want || len(rules[0].Forward.Way) != 1 || !slices.Equal(rules[0].Forward.Way[0].LB, []string{"socks5://h:1080"}) {
				t.Fatalf("rules = %#v, want %q with replaced forward.way", rules, test.want)
			}
			if !rules[0].Disabled || rules[0].Listen.Address() != "127.0.0.1:9000" {
				t.Fatalf("rule = %#v, want the disabled replacement listener", rules[0])
			}
			if fake.reloads != 1 {
				t.Fatalf("reloads = %d, want 1", fake.reloads)
			}
			conf, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if conf.WebUI.Port != 1088 || !reflect.DeepEqual(conf.Rules, rules) {
				t.Fatalf("saved config = %#v, want replaced rules and unchanged web UI", conf)
			}
			requestAPI(t, handler, http.MethodGet, "/apis/configs/rules/"+test.want, "", http.StatusOK)
			if test.want != "a" {
				requestAPI(t, handler, http.MethodGet, "/apis/configs/rules/a", "", http.StatusBadRequest)
			}
		})
	}
}

func TestRenameRuleConflict(t *testing.T) {
	handler, fake, store, _ := setupConfigAPI(t)
	conf, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	conf.Rules = append(conf.Rules, config.Rule{Name: "b"})
	if err := store.Save(conf); err != nil {
		t.Fatal(err)
	}
	before, err := store.LoadRaw()
	if err != nil {
		t.Fatal(err)
	}
	response := requestAPI(t, handler, http.MethodPut, "/apis/configs/rules/a", `{"name":"b"}`, http.StatusBadRequest)
	if !strings.Contains(response.Body.String(), `rule "b" already exists`) {
		t.Fatalf("body = %q, want duplicate rule error", response.Body.String())
	}
	if fake.reloads != 0 {
		t.Fatalf("reloads = %d, want 0", fake.reloads)
	}
	assertConfigYAML(t, store, string(before))
}

func TestDeleteRule(t *testing.T) {
	handler, fake, store, _ := setupConfigAPI(t)
	conf, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	conf.Rules = append(conf.Rules, config.Rule{Name: "b"})
	if err := store.Save(conf); err != nil {
		t.Fatal(err)
	}
	response := requestAPI(t, handler, http.MethodDelete, "/apis/configs/rules/a", "", http.StatusOK)
	if strings.TrimSpace(response.Body.String()) != "null" {
		t.Fatalf("body = %q, want null", response.Body.String())
	}
	if rules := listRulesAPI(t, handler); len(rules) != 1 || rules[0].Name != "b" {
		t.Fatalf("rules = %#v, want only b", rules)
	}
	if fake.reloads != 1 {
		t.Fatalf("reloads = %d, want 1", fake.reloads)
	}
	response = requestAPI(t, handler, http.MethodDelete, "/apis/configs/rules/b", "", http.StatusOK)
	if strings.TrimSpace(response.Body.String()) != "null" {
		t.Fatalf("body = %q, want null", response.Body.String())
	}
	if rules := listRulesAPI(t, handler); len(rules) != 0 {
		t.Fatalf("rules = %#v, want empty array", rules)
	}
	response = requestAPI(t, handler, http.MethodGet, "/apis/configs", "", http.StatusOK)
	if err := json.Unmarshal(response.Body.Bytes(), &conf); err != nil {
		t.Fatal(err)
	}
	if len(conf.Rules) != 0 || conf.WebUI.Port != 1088 {
		t.Fatalf("config = %#v, want no rules and unchanged web UI", conf)
	}
	if fake.reloads != 2 {
		t.Fatalf("reloads = %d, want 2", fake.reloads)
	}
	before, err := store.LoadRaw()
	if err != nil {
		t.Fatal(err)
	}
	response = requestAPI(t, handler, http.MethodDelete, "/apis/configs/rules/b", "", http.StatusBadRequest)
	if !strings.Contains(response.Body.String(), `rule "b" not found`) || fake.reloads != 2 {
		t.Fatalf("repeated delete = %q, reloads = %d; want not found without reload", response.Body.String(), fake.reloads)
	}
	assertConfigYAML(t, store, string(before))
}

func TestCreateRulesConcurrent(t *testing.T) {
	handler, fake, _, _ := setupConfigAPI(t)
	const count = 20
	responses := make(chan *httptest.ResponseRecorder, count)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for index := 0; index < count; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			body := fmt.Sprintf(`{"name":"rule-%d","forward":{"way":[{"lb":["socks5://h:1080"]}]}}`, index)
			request := httptest.NewRequest(http.MethodPost, "/apis/configs/rules", strings.NewReader(body))
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
	rules := listRulesAPI(t, handler)
	if len(rules) != count+1 {
		t.Fatalf("rule count = %d, want %d", len(rules), count+1)
	}
	names := make(map[string]bool, len(rules))
	for _, candidate := range rules {
		names[candidate.Name] = true
	}
	if !names["a"] {
		t.Fatal("seeded rule a was lost")
	}
	for index := 0; index < count; index++ {
		if name := fmt.Sprintf("rule-%d", index); !names[name] {
			t.Errorf("rule %q was lost", name)
		}
	}
	if fake.reloads != count {
		t.Fatalf("reloads = %d, want %d", fake.reloads, count)
	}
}

func TestWebUI(t *testing.T) {
	handler, fake, store, _ := setupConfigAPI(t)
	response := requestAPI(t, handler, http.MethodGet, "/apis/configs/web-ui", "", http.StatusOK)
	var address config.Address
	if err := json.Unmarshal(response.Body.Bytes(), &address); err != nil {
		t.Fatal(err)
	}
	if address.Host != "127.0.0.1" || address.Port != 1088 {
		t.Fatalf("web_ui = %#v, want the default web UI address", address)
	}
	response = requestAPI(t, handler, http.MethodPut, "/apis/configs/web-ui", `{"host":"127.0.0.1","port":70000}`, http.StatusBadRequest)
	if !strings.Contains(response.Body.String(), "web_ui.port") {
		t.Fatalf("body = %q, want invalid web_ui.port error", response.Body.String())
	}
	if fake.reloads != 0 {
		t.Fatalf("reloads = %d, want 0", fake.reloads)
	}
	assertConfigYAML(t, store, testConfigYAML)
	requestAPI(t, handler, http.MethodPut, "/apis/configs/web-ui", `{"host":"localhost","port":1098}`, http.StatusOK)
	if fake.reloads != 1 {
		t.Fatalf("reloads = %d, want 1", fake.reloads)
	}
	conf, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if conf.WebUI.Host != "localhost" || conf.WebUI.Port != 1098 || len(conf.Rules) != 1 || conf.Rules[0].Name != "a" || conf.Rules[0].Listen.Port != 1087 {
		t.Fatalf("saved config = %#v, want updated web UI address and unchanged rule", conf)
	}
	response = requestAPI(t, handler, http.MethodGet, "/apis/configs/web-ui", "", http.StatusOK)
	if err := json.Unmarshal(response.Body.Bytes(), &address); err != nil {
		t.Fatal(err)
	}
	if address != conf.WebUI {
		t.Fatalf("web_ui = %#v, want saved %#v", address, conf.WebUI)
	}
}

func TestNoProxy(t *testing.T) {
	handler, fake, store, _ := setupConfigAPI(t)
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
	if len(conf.Rules) != 1 || conf.Rules[0].Name != "a" || conf.Rules[0].Listen.Port != 1087 || conf.WebUI.Port != 1088 {
		t.Fatalf("saved config = %#v, want unchanged rules and web UI", conf)
	}
}

func TestConfigResourceNullBody(t *testing.T) {
	for _, test := range []struct {
		target    string
		wantError string
	}{
		{target: "", wantError: "config is nil"},
		{target: "/web-ui", wantError: "web_ui is nil"},
		{target: "/no-proxy", wantError: "no-proxy is nil"},
	} {
		t.Run(test.target, func(t *testing.T) {
			handler, fake, store, _ := setupConfigAPI(t)
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
			body:       `{"web_ui":{"port":70000}}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "web_ui.port",
			wantPort:   1088,
		},
		{
			name:        "valid",
			wantStatus:  http.StatusOK,
			wantReloads: 1,
			wantPort:    1098,
		},
		{
			name:        "reload_failure_keeps_saved_config",
			reloadError: errors.New("listen tcp: address already in use"),
			wantStatus:  http.StatusBadRequest,
			wantError:   "saved, but listen tcp: address already in use",
			wantReloads: 1,
			wantPort:    1098,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, fake, store, _ := setupConfigAPI(t)
			fake.err = test.reloadError
			body := test.body
			if body == "" {
				response := requestAPI(t, handler, http.MethodGet, "/apis/configs", "", http.StatusOK)
				var conf map[string]any
				if err := json.Unmarshal(response.Body.Bytes(), &conf); err != nil {
					t.Fatal(err)
				}
				address, ok := conf["web_ui"].(map[string]any)
				if !ok {
					t.Fatalf("web_ui = %#v, want an object", conf["web_ui"])
				}
				address["port"] = test.wantPort
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
			if conf.WebUI.Port != test.wantPort || len(conf.Rules) != 1 || conf.Rules[0].Name != "a" || len(conf.Rules[0].Forward.Way) != 1 {
				t.Fatalf("saved config = %#v, want web_ui.port %d and unchanged rule", conf, test.wantPort)
			}
			if test.wantReloads == 0 {
				assertConfigYAML(t, store, testConfigYAML)
			}
		})
	}
}

func TestUpdateConfigBodyTooLarge(t *testing.T) {
	handler, fake, store, _ := setupConfigAPI(t)
	body := `{"web_ui":{"port":1098}}`
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
	handler, fake, store, _ := setupConfigAPI(t)
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
		{name: "malformed_yaml", body: `{"yaml":"rules: [oops"}`, wantError: "yaml:"},
		{name: "blank_yaml", body: `{"yaml":"   "}`, wantError: "config is empty"},
		{name: "null", body: `null`, wantError: "config is empty"},
		{name: "invalid_port", body: `{"yaml":"web_ui:\n  port: 70000\n"}`, wantError: "web_ui.port"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, fake, store, _ := setupConfigAPI(t)
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
	for _, test := range []struct {
		name    string
		err     string
		target  string
		attempt int
		remote  bool
		running bool
	}{
		{name: "running", running: true},
		{name: "retrying", err: "boom", attempt: 2, remote: true},
		{name: "port_forward", target: "10.0.0.5:5432", running: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, fake, _, _ := setupConfigAPI(t)
			fake.status = configs.Status{
				Address: "127.0.0.1:1088",
				Running: true,
				Error:   test.err,
				Rules: []configs.RuleStatus{{
					Name:    "a",
					Address: "127.0.0.1:1087",
					Target:  test.target,
					Remote:  test.remote,
					Running: test.running,
					Attempt: test.attempt,
					Error:   test.err,
				}},
			}
			response := requestAPI(t, handler, http.MethodGet, "/apis/configs/status", "", http.StatusOK)
			var status map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
				t.Fatal(err)
			}
			if status["address"] != fake.status.Address || status["running"] != fake.status.Running {
				t.Fatalf("status = %#v, want %#v", status, fake.status)
			}
			rules, ok := status["rules"].([]any)
			if !ok || len(rules) != 1 {
				t.Fatalf("rules = %#v, want one rule status", status["rules"])
			}
			rule, ok := rules[0].(map[string]any)
			if !ok || rule["name"] != "a" || rule["address"] != "127.0.0.1:1087" || rule["remote"] != test.remote || rule["running"] != test.running {
				t.Fatalf("rule status = %#v, want %#v", rules[0], fake.status.Rules[0])
			}
			target, present := rule["target"]
			if test.target == "" {
				if present {
					t.Fatalf("rule status contains empty target: %#v", rule)
				}
			} else if target != test.target {
				t.Fatalf("target = %#v, want %q", target, test.target)
			}
			for _, fields := range []map[string]any{status, rule} {
				gotError, present := fields["error"]
				if test.err == "" {
					if present {
						t.Fatalf("status contains error: %#v", fields)
					}
				} else if gotError != test.err {
					t.Fatalf("error = %#v, want %q", gotError, test.err)
				}
			}
			attempt, present := rule["attempt"]
			if test.attempt == 0 {
				if present {
					t.Fatalf("rule status contains zero attempt: %#v", rule)
				}
			} else if attempt != float64(test.attempt) {
				t.Fatalf("attempt = %#v, want %d", attempt, test.attempt)
			}
		})
	}
}

func TestUnknownAPIRoute(t *testing.T) {
	handler, _, _, _ := setupConfigAPI(t)
	for _, target := range []string{"/apis/nope", "/apis/configs/current-context", "/apis/configs/contexts", "/apis/configs/proxy"} {
		requestAPI(t, handler, http.MethodGet, target, "", http.StatusNotFound)
	}
}

func TestConfigAPIIsolation(t *testing.T) {
	handler, fake, store, _ := setupConfigAPI(t)
	otherHandler, otherFake, otherStore, _ := setupConfigAPI(t)
	fake.status = configs.Status{Address: "127.0.0.1:1088", Running: true}
	otherFake.status = configs.Status{Address: "127.0.0.1:1089", Error: "not running"}
	requestAPI(t, handler, http.MethodPut, "/apis/configs", `{"web_ui":{"port":1098}}`, http.StatusOK)
	if fake.reloads != 1 || otherFake.reloads != 0 {
		t.Fatalf("reloads = %d, %d; want 1, 0", fake.reloads, otherFake.reloads)
	}
	conf, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if conf.WebUI.Port != 1098 {
		t.Fatalf("saved web_ui.port = %d, want 1098", conf.WebUI.Port)
	}
	assertConfigYAML(t, otherStore, testConfigYAML)
	for candidate, want := range map[http.Handler]configs.Status{handler: fake.status, otherHandler: otherFake.status} {
		response := requestAPI(t, candidate, http.MethodGet, "/apis/configs/status", "", http.StatusOK)
		var got configs.Status
		if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("status = %#v, want %#v", got, want)
		}
	}
}
