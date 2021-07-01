// Code generated; DO NOT EDIT.
// file ./route/route_gen.go

package route

import (
	json "encoding/json"
	"fmt"
	ioutil "io/ioutil"
	http "net/http"

	mux "github.com/gorilla/mux"
	githubComWzshimingJumpwayAppWebServicesConfigs "github.com/wzshiming/jumpway/app/web/services/configs"
	githubComWzshimingJumpwayConfig "github.com/wzshiming/jumpway/config"
)

// notFoundHandler Is the not found of handler
func notFoundHandler(w http.ResponseWriter, r *http.Request) {

	err := fmt.Errorf("Not found '%s %s'", r.Method, r.URL.Path)

	http.Error(w, err.Error(), 404)

}

// Router is all routing for package
// generated do not edit.
func Router() http.Handler {
	router := mux.NewRouter()

	// ConfigsService Define the method scope
	var _configsService githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService
	RouteConfigsService(router, &_configsService)

	router.NotFoundHandler = http.HandlerFunc(notFoundHandler)
	return router
}

// RouteConfigsService is routing for ConfigsService
func RouteConfigsService(router *mux.Router, _configsService *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, fs ...mux.MiddlewareFunc) *mux.Router {
	if router == nil {
		router = mux.NewRouter()
	}

	_routeConfigs := router.PathPrefix("/configs").Subrouter()
	if len(fs) != 0 {
		_routeConfigs.Use(fs...)
	}

	// Registered routing GET /configs
	var __operationGetConfigs http.Handler
	__operationGetConfigs = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationGetConfigs(_configsService, w, r)
	})
	_routeConfigs.Methods("GET").Path("").Handler(__operationGetConfigs)

	// Registered routing PUT /configs
	var __operationPutConfigs http.Handler
	__operationPutConfigs = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationPutConfigs(_configsService, w, r)
	})
	_routeConfigs.Methods("PUT").Path("").Handler(__operationPutConfigs)

	if router.NotFoundHandler == nil {
		router.NotFoundHandler = http.HandlerFunc(notFoundHandler)
	}
	return router
}

// _requestBodyConf Parsing the body for of conf
func _requestBodyConf(w http.ResponseWriter, r *http.Request) (_conf *githubComWzshimingJumpwayConfig.Config, err error) {

	defer r.Body.Close()

	var __conf []byte
	__conf, err = ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 400)

		return
	}

	err = json.Unmarshal(__conf, &_conf)
	if err != nil {
		http.Error(w, err.Error(), 400)

		return
	}

	return
}

// _operationPutConfigs Is the route of Update
func _operationPutConfigs(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// requests github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.Update.conf
	var _conf *githubComWzshimingJumpwayConfig.Config
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.Update.err
	var _err error

	// Parsing conf.
	_conf, _err = _requestBodyConf(w, r)
	if _err != nil {
		return
	}

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.Update.
	_err = s.Update(_conf)

	// Response code 400 Bad Request for err.
	if _err != nil {
		http.Error(w, _err.Error(), 400)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("null"))

	return
}

// _operationGetConfigs Is the route of Get
func _operationGetConfigs(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.Get.conf
	var _conf_1 *githubComWzshimingJumpwayConfig.Config
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.Get.err
	var _err error

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.Get.
	_conf_1, _err = s.Get()

	// Response code 200 OK for conf.
	if _conf_1 != nil {
		var __conf_1 []byte
		__conf_1, _err = json.Marshal(_conf_1)
		if _err != nil {
			http.Error(w, _err.Error(), 500)

			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		w.Write(__conf_1)
		return
	}

	// Response code 400 Bad Request for err.
	if _err != nil {
		http.Error(w, _err.Error(), 400)
		return
	}

	var __conf_1 []byte
	__conf_1, _err = json.Marshal(_conf_1)
	if _err != nil {
		http.Error(w, _err.Error(), 500)

		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	w.Write(__conf_1)

	return
}
