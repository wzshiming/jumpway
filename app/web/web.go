package web

import (
	"bytes"
	"embed"
	_ "embed"
	"io/fs"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/wzshiming/jumpway/app/web/route"
	"github.com/wzshiming/openapiui/v2/swaggerui"
)

func Handler() http.Handler {
	m := mux.NewRouter()
	m.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	m.HandleFunc("/debug/pprof/profile", pprof.Profile)
	m.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	m.HandleFunc("/debug/pprof/trace", pprof.Trace)
	m.PathPrefix("/debug/pprof/").Handler(http.HandlerFunc(pprof.Index))
	m.Handle("/swaggerui/openapi.json",
		http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			config := bytes.Replace(config, []byte(`"/"`), []byte(`"/apis/"`), -1)
			http.ServeContent(rw, r, "openapi.json", time.Time{}, bytes.NewReader(config))
		}))
	m.PathPrefix("/swaggerui/").Handler(http.FileServer(http.FS(swaggerui.FS)))
	apis := route.Router()
	apis = handlers.CombinedLoggingHandler(os.Stdout, apis)
	m.PathPrefix("/apis/").Handler(http.StripPrefix("/apis", apis))
	m.PathPrefix("/").Handler(http.FileServer(http.FS(staticsFS)))
	return handlers.RecoveryHandler()(m)
}

//go:embed openapi/openapi.json
var config []byte

//go:embed statics
var fstmp embed.FS

var staticsFS fs.FS

func init() {
	f, err := fs.Sub(fstmp, "statics")
	if err != nil {
		os.Exit(2)
	}
	staticsFS = f
}
