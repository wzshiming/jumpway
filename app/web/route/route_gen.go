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

	// Registered routing GET /configs/status
	var __operationGetConfigsStatus http.Handler
	__operationGetConfigsStatus = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationGetConfigsStatus(_configsService, w, r)
	})
	_routeConfigs.Methods("GET").Path("/status").Handler(__operationGetConfigsStatus)

	// Registered routing GET /configs/raw
	var __operationGetConfigsRaw http.Handler
	__operationGetConfigsRaw = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationGetConfigsRaw(_configsService, w, r)
	})
	_routeConfigs.Methods("GET").Path("/raw").Handler(__operationGetConfigsRaw)

	// Registered routing PUT /configs/raw
	var __operationPutConfigsRaw http.Handler
	__operationPutConfigsRaw = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationPutConfigsRaw(_configsService, w, r)
	})
	_routeConfigs.Methods("PUT").Path("/raw").Handler(__operationPutConfigsRaw)

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

// _requestBodyRaw Parsing the body for of raw
func _requestBodyRaw(w http.ResponseWriter, r *http.Request) (_raw *githubComWzshimingJumpwayAppWebServicesConfigs.RawConfig, err error) {

	defer r.Body.Close()

	var __raw []byte
	__raw, err = ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 400)

		return
	}

	err = json.Unmarshal(__raw, &_raw)
	if err != nil {
		http.Error(w, err.Error(), 400)

		return
	}

	return
}

// _operationPutConfigsRaw Is the route of UpdateRaw
func _operationPutConfigsRaw(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// requests github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateRaw.raw
	var _raw *githubComWzshimingJumpwayAppWebServicesConfigs.RawConfig
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateRaw.err
	var _err error

	// Parsing raw.
	_raw, _err = _requestBodyRaw(w, r)
	if _err != nil {
		return
	}

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateRaw.
	_err = s.UpdateRaw(_raw)

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

// _operationGetConfigsStatus Is the route of Status
func _operationGetConfigsStatus(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.Status.status
	var _status *githubComWzshimingJumpwayAppWebServicesConfigs.Status
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.Status.err
	var _err error

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.Status.
	_status, _err = s.Status()

	// Response code 200 OK for status.
	if _status != nil {
		var __status []byte
		__status, _err = json.Marshal(_status)
		if _err != nil {
			http.Error(w, _err.Error(), 500)

			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		w.Write(__status)
		return
	}

	// Response code 400 Bad Request for err.
	if _err != nil {
		http.Error(w, _err.Error(), 400)
		return
	}

	var __status []byte
	__status, _err = json.Marshal(_status)
	if _err != nil {
		http.Error(w, _err.Error(), 500)

		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	w.Write(__status)

	return
}

// _operationGetConfigsRaw Is the route of GetRaw
func _operationGetConfigsRaw(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetRaw.raw
	var _raw_1 *githubComWzshimingJumpwayAppWebServicesConfigs.RawConfig
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetRaw.err
	var _err error

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetRaw.
	_raw_1, _err = s.GetRaw()

	// Response code 200 OK for raw.
	if _raw_1 != nil {
		var __raw_1 []byte
		__raw_1, _err = json.Marshal(_raw_1)
		if _err != nil {
			http.Error(w, _err.Error(), 500)

			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		w.Write(__raw_1)
		return
	}

	// Response code 400 Bad Request for err.
	if _err != nil {
		http.Error(w, _err.Error(), 400)
		return
	}

	var __raw_1 []byte
	__raw_1, _err = json.Marshal(_raw_1)
	if _err != nil {
		http.Error(w, _err.Error(), 500)

		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	w.Write(__raw_1)

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
