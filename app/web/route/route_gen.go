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

	// Registered routing GET /configs/proxy
	var __operationGetConfigsProxy http.Handler
	__operationGetConfigsProxy = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationGetConfigsProxy(_configsService, w, r)
	})
	_routeConfigs.Methods("GET").Path("/proxy").Handler(__operationGetConfigsProxy)

	// Registered routing PUT /configs/proxy
	var __operationPutConfigsProxy http.Handler
	__operationPutConfigsProxy = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationPutConfigsProxy(_configsService, w, r)
	})
	_routeConfigs.Methods("PUT").Path("/proxy").Handler(__operationPutConfigsProxy)

	// Registered routing GET /configs/no-proxy
	var __operationGetConfigsNoProxy http.Handler
	__operationGetConfigsNoProxy = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationGetConfigsNoProxy(_configsService, w, r)
	})
	_routeConfigs.Methods("GET").Path("/no-proxy").Handler(__operationGetConfigsNoProxy)

	// Registered routing PUT /configs/no-proxy
	var __operationPutConfigsNoProxy http.Handler
	__operationPutConfigsNoProxy = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationPutConfigsNoProxy(_configsService, w, r)
	})
	_routeConfigs.Methods("PUT").Path("/no-proxy").Handler(__operationPutConfigsNoProxy)

	// Registered routing GET /configs/current-context
	var __operationGetConfigsCurrentContext http.Handler
	__operationGetConfigsCurrentContext = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationGetConfigsCurrentContext(_configsService, w, r)
	})
	_routeConfigs.Methods("GET").Path("/current-context").Handler(__operationGetConfigsCurrentContext)

	// Registered routing PUT /configs/current-context
	var __operationPutConfigsCurrentContext http.Handler
	__operationPutConfigsCurrentContext = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationPutConfigsCurrentContext(_configsService, w, r)
	})
	_routeConfigs.Methods("PUT").Path("/current-context").Handler(__operationPutConfigsCurrentContext)

	// Registered routing GET /configs/contexts
	var __operationGetConfigsContexts http.Handler
	__operationGetConfigsContexts = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationGetConfigsContexts(_configsService, w, r)
	})
	_routeConfigs.Methods("GET").Path("/contexts").Handler(__operationGetConfigsContexts)

	// Registered routing POST /configs/contexts
	var __operationPostConfigsContexts http.Handler
	__operationPostConfigsContexts = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationPostConfigsContexts(_configsService, w, r)
	})
	_routeConfigs.Methods("POST").Path("/contexts").Handler(__operationPostConfigsContexts)

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

	// Registered routing PUT /configs/contexts/{name}
	var __operationPutConfigsContextsName http.Handler
	__operationPutConfigsContextsName = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationPutConfigsContextsName(_configsService, w, r)
	})
	_routeConfigs.Methods("PUT").Path("/contexts/{name}").Handler(__operationPutConfigsContextsName)

	// Registered routing DELETE /configs/contexts/{name}
	var __operationDeleteConfigsContextsName http.Handler
	__operationDeleteConfigsContextsName = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationDeleteConfigsContextsName(_configsService, w, r)
	})
	_routeConfigs.Methods("DELETE").Path("/contexts/{name}").Handler(__operationDeleteConfigsContextsName)

	// Registered routing GET /configs/contexts/{name}
	var __operationGetConfigsContextsName http.Handler
	__operationGetConfigsContextsName = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationGetConfigsContextsName(_configsService, w, r)
	})
	_routeConfigs.Methods("GET").Path("/contexts/{name}").Handler(__operationGetConfigsContextsName)

	if router.NotFoundHandler == nil {
		router.NotFoundHandler = http.HandlerFunc(notFoundHandler)
	}
	return router
}

// _requestBodyC Parsing the body for of c
func _requestBodyC(w http.ResponseWriter, r *http.Request) (_c *githubComWzshimingJumpwayConfig.Context, err error) {

	defer r.Body.Close()

	var __c []byte
	__c, err = ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 400)

		return
	}

	err = json.Unmarshal(__c, &_c)
	if err != nil {
		http.Error(w, err.Error(), 400)

		return
	}

	return
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

// _requestBodyCurrent Parsing the body for of current
func _requestBodyCurrent(w http.ResponseWriter, r *http.Request) (_current *githubComWzshimingJumpwayAppWebServicesConfigs.CurrentContext, err error) {

	defer r.Body.Close()

	var __current []byte
	__current, err = ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 400)

		return
	}

	err = json.Unmarshal(__current, &_current)
	if err != nil {
		http.Error(w, err.Error(), 400)

		return
	}

	return
}

// _requestPathName Parsing the path for of name
func _requestPathName(w http.ResponseWriter, r *http.Request) (_name string, err error) {

	var _rawName = mux.Vars(r)["name"]
	_name = string(_rawName)
	if err != nil {
		http.Error(w, err.Error(), 400)

		return
	}

	return
}

// _requestBodyNoProxy Parsing the body for of noProxy
func _requestBodyNoProxy(w http.ResponseWriter, r *http.Request) (_noProxy *githubComWzshimingJumpwayConfig.NoProxy, err error) {

	defer r.Body.Close()

	var __noProxy []byte
	__noProxy, err = ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 400)

		return
	}

	err = json.Unmarshal(__noProxy, &_noProxy)
	if err != nil {
		http.Error(w, err.Error(), 400)

		return
	}

	return
}

// _requestBodyProxy Parsing the body for of proxy
func _requestBodyProxy(w http.ResponseWriter, r *http.Request) (_proxy *githubComWzshimingJumpwayConfig.Proxy, err error) {

	defer r.Body.Close()

	var __proxy []byte
	__proxy, err = ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 400)

		return
	}

	err = json.Unmarshal(__proxy, &_proxy)
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

// _operationPutConfigsProxy Is the route of UpdateProxy
func _operationPutConfigsProxy(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// requests github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateProxy.proxy
	var _proxy *githubComWzshimingJumpwayConfig.Proxy
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateProxy.err
	var _err error

	// Parsing proxy.
	_proxy, _err = _requestBodyProxy(w, r)
	if _err != nil {
		return
	}

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateProxy.
	_err = s.UpdateProxy(_proxy)

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

// _operationPutConfigsNoProxy Is the route of UpdateNoProxy
func _operationPutConfigsNoProxy(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// requests github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateNoProxy.noProxy
	var _noProxy *githubComWzshimingJumpwayConfig.NoProxy
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateNoProxy.err
	var _err error

	// Parsing noProxy.
	_noProxy, _err = _requestBodyNoProxy(w, r)
	if _err != nil {
		return
	}

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateNoProxy.
	_err = s.UpdateNoProxy(_noProxy)

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

// _operationPutConfigsContextsName Is the route of UpdateContext
func _operationPutConfigsContextsName(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// requests github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateContext.name
	var _name string
	// requests github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateContext.c
	var _c *githubComWzshimingJumpwayConfig.Context
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateContext.err
	var _err error

	// Parsing name.
	_name, _err = _requestPathName(w, r)
	if _err != nil {
		return
	}

	// Parsing c.
	_c, _err = _requestBodyC(w, r)
	if _err != nil {
		return
	}

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateContext.
	_err = s.UpdateContext(_name, _c)

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

// _operationPutConfigsCurrentContext Is the route of SetCurrentContext
func _operationPutConfigsCurrentContext(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// requests github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.SetCurrentContext.current
	var _current *githubComWzshimingJumpwayAppWebServicesConfigs.CurrentContext
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.SetCurrentContext.err
	var _err error

	// Parsing current.
	_current, _err = _requestBodyCurrent(w, r)
	if _err != nil {
		return
	}

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.SetCurrentContext.
	_err = s.SetCurrentContext(_current)

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

// _operationGetConfigsContexts Is the route of ListContexts
func _operationGetConfigsContexts(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.ListContexts.contexts
	var _contexts []githubComWzshimingJumpwayConfig.Context
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.ListContexts.err
	var _err error

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.ListContexts.
	_contexts, _err = s.ListContexts()

	// Response code 200 OK for contexts.
	if _contexts != nil {
		var __contexts []byte
		__contexts, _err = json.Marshal(_contexts)
		if _err != nil {
			http.Error(w, _err.Error(), 500)

			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		w.Write(__contexts)
		return
	}

	// Response code 400 Bad Request for err.
	if _err != nil {
		http.Error(w, _err.Error(), 400)
		return
	}

	var __contexts []byte
	__contexts, _err = json.Marshal(_contexts)
	if _err != nil {
		http.Error(w, _err.Error(), 500)

		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	w.Write(__contexts)

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

// _operationGetConfigsProxy Is the route of GetProxy
func _operationGetConfigsProxy(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetProxy.proxy
	var _proxy_1 *githubComWzshimingJumpwayConfig.Proxy
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetProxy.err
	var _err error

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetProxy.
	_proxy_1, _err = s.GetProxy()

	// Response code 200 OK for proxy.
	if _proxy_1 != nil {
		var __proxy_1 []byte
		__proxy_1, _err = json.Marshal(_proxy_1)
		if _err != nil {
			http.Error(w, _err.Error(), 500)

			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		w.Write(__proxy_1)
		return
	}

	// Response code 400 Bad Request for err.
	if _err != nil {
		http.Error(w, _err.Error(), 400)
		return
	}

	var __proxy_1 []byte
	__proxy_1, _err = json.Marshal(_proxy_1)
	if _err != nil {
		http.Error(w, _err.Error(), 500)

		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	w.Write(__proxy_1)

	return
}

// _operationGetConfigsNoProxy Is the route of GetNoProxy
func _operationGetConfigsNoProxy(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetNoProxy.noProxy
	var _noProxy_1 *githubComWzshimingJumpwayConfig.NoProxy
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetNoProxy.err
	var _err error

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetNoProxy.
	_noProxy_1, _err = s.GetNoProxy()

	// Response code 200 OK for noProxy.
	if _noProxy_1 != nil {
		var __noProxy_1 []byte
		__noProxy_1, _err = json.Marshal(_noProxy_1)
		if _err != nil {
			http.Error(w, _err.Error(), 500)

			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		w.Write(__noProxy_1)
		return
	}

	// Response code 400 Bad Request for err.
	if _err != nil {
		http.Error(w, _err.Error(), 400)
		return
	}

	var __noProxy_1 []byte
	__noProxy_1, _err = json.Marshal(_noProxy_1)
	if _err != nil {
		http.Error(w, _err.Error(), 500)

		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	w.Write(__noProxy_1)

	return
}

// _operationGetConfigsCurrentContext Is the route of GetCurrentContext
func _operationGetConfigsCurrentContext(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetCurrentContext.current
	var _current_1 *githubComWzshimingJumpwayAppWebServicesConfigs.CurrentContext
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetCurrentContext.err
	var _err error

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetCurrentContext.
	_current_1, _err = s.GetCurrentContext()

	// Response code 200 OK for current.
	if _current_1 != nil {
		var __current_1 []byte
		__current_1, _err = json.Marshal(_current_1)
		if _err != nil {
			http.Error(w, _err.Error(), 500)

			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		w.Write(__current_1)
		return
	}

	// Response code 400 Bad Request for err.
	if _err != nil {
		http.Error(w, _err.Error(), 400)
		return
	}

	var __current_1 []byte
	__current_1, _err = json.Marshal(_current_1)
	if _err != nil {
		http.Error(w, _err.Error(), 500)

		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	w.Write(__current_1)

	return
}

// _operationGetConfigsContextsName Is the route of GetContext
func _operationGetConfigsContextsName(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// requests github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetContext.name
	var _name string
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetContext.c
	var _c_1 *githubComWzshimingJumpwayConfig.Context
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetContext.err
	var _err error

	// Parsing name.
	_name, _err = _requestPathName(w, r)
	if _err != nil {
		return
	}

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetContext.
	_c_1, _err = s.GetContext(_name)

	// Response code 200 OK for c.
	if _c_1 != nil {
		var __c_1 []byte
		__c_1, _err = json.Marshal(_c_1)
		if _err != nil {
			http.Error(w, _err.Error(), 500)

			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		w.Write(__c_1)
		return
	}

	// Response code 400 Bad Request for err.
	if _err != nil {
		http.Error(w, _err.Error(), 400)
		return
	}

	var __c_1 []byte
	__c_1, _err = json.Marshal(_c_1)
	if _err != nil {
		http.Error(w, _err.Error(), 500)

		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	w.Write(__c_1)

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

// _operationDeleteConfigsContextsName Is the route of DeleteContext
func _operationDeleteConfigsContextsName(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// requests github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.DeleteContext.name
	var _name string
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.DeleteContext.err
	var _err error

	// Parsing name.
	_name, _err = _requestPathName(w, r)
	if _err != nil {
		return
	}

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.DeleteContext.
	_err = s.DeleteContext(_name)

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

// _operationPostConfigsContexts Is the route of CreateContext
func _operationPostConfigsContexts(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// requests github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.CreateContext.c
	var _c *githubComWzshimingJumpwayConfig.Context
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.CreateContext.err
	var _err error

	// Parsing c.
	_c, _err = _requestBodyC(w, r)
	if _err != nil {
		return
	}

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.CreateContext.
	_err = s.CreateContext(_c)

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
