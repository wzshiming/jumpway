package web

import (
	"bytes"
	"embed"
	_ "embed"
	"io/fs"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/wzshiming/jumpway/app/web/route"
	"github.com/wzshiming/jumpway/app/web/services/configs"
	"github.com/wzshiming/jumpway/app/web/services/stats"
	"github.com/wzshiming/openapiui/v2/swaggerui"
)

func NewHandler(svc *configs.ConfigsService, statsSvc *stats.StatsService, metricsHandler http.Handler) http.Handler {
	m := mux.NewRouter()
	m.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	m.HandleFunc("/debug/pprof/profile", pprof.Profile)
	m.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	m.HandleFunc("/debug/pprof/trace", pprof.Trace)
	m.PathPrefix("/debug/pprof/").Handler(http.HandlerFunc(pprof.Index))
	m.Handle("/swaggerui/openapi.json",
		http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			http.ServeContent(rw, r, "openapi.json", time.Time{}, bytes.NewReader(openapiJSON))
		}))
	m.PathPrefix("/swaggerui/").Handler(http.FileServer(http.FS(swaggerui.FS)))
	m.Handle("/metrics", metricsHandler)
	// The generated Router builds a zero-value service; register the injected instance here.
	apis := route.RouteConfigsService(mux.NewRouter(), svc)
	route.RouteStatsService(apis, statsSvc)
	apiHandler := handlers.CombinedLoggingHandler(os.Stdout, apis)
	m.PathPrefix("/apis/").Handler(http.StripPrefix("/apis", handlers.CompressHandler(http.MaxBytesHandler(apiHandler, 1<<20))))
	m.PathPrefix("/").Handler(handlers.CompressHandler(cacheControl(http.FileServer(http.FS(staticsFS)))))
	return handlers.RecoveryHandler()(m)
}

// cacheControl caches Vite's content-hashed /assets/ forever and has everything else revalidated.
func cacheControl(files http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := "no-cache"
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			value = "public, max-age=31536000, immutable"
		}
		w.Header().Set("Cache-Control", value)
		files.ServeHTTP(&errorHeaders{ResponseWriter: w, encoding: w.Header().Get("Content-Encoding")}, r)
	})
}

// errorHeaders restores what FileServer's error path strips since Go 1.23: the encoding the compressing
// writer already committed to, and a Cache-Control that keeps errors out of caches.
type errorHeaders struct {
	http.ResponseWriter
	encoding string
}

func (e *errorHeaders) WriteHeader(code int) {
	if code >= http.StatusBadRequest {
		h := e.Header()
		h.Set("Cache-Control", "no-cache")
		if e.encoding != "" {
			h.Set("Content-Encoding", e.encoding)
		}
	}
	e.ResponseWriter.WriteHeader(code)
}

//go:embed openapi/openapi.json
var openapiJSON []byte

// statics is built from app/web/ui (make -C app/web ui) and committed.
//
//go:embed statics
var fstmp embed.FS

var staticsFS fs.FS

func init() {
	// Rewrite the base path once so requests serve the prefixed spec directly.
	openapiJSON = bytes.ReplaceAll(openapiJSON, []byte(`"/"`), []byte(`"/apis/"`))
	f, err := fs.Sub(fstmp, "statics")
	if err != nil {
		os.Exit(2)
	}
	staticsFS = f
}
