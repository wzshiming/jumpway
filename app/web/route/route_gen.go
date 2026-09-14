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

	// Registered routing GET /configs/web-ui
	var __operationGetConfigsWebUI http.Handler
	__operationGetConfigsWebUI = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationGetConfigsWebUI(_configsService, w, r)
	})
	_routeConfigs.Methods("GET").Path("/web-ui").Handler(__operationGetConfigsWebUI)

	// Registered routing PUT /configs/web-ui
	var __operationPutConfigsWebUI http.Handler
	__operationPutConfigsWebUI = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationPutConfigsWebUI(_configsService, w, r)
	})
	_routeConfigs.Methods("PUT").Path("/web-ui").Handler(__operationPutConfigsWebUI)

	// Registered routing GET /configs/status
	var __operationGetConfigsStatus http.Handler
	__operationGetConfigsStatus = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationGetConfigsStatus(_configsService, w, r)
	})
	_routeConfigs.Methods("GET").Path("/status").Handler(__operationGetConfigsStatus)

	// Registered routing POST /configs/rules
	var __operationPostConfigsRules http.Handler
	__operationPostConfigsRules = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationPostConfigsRules(_configsService, w, r)
	})
	_routeConfigs.Methods("POST").Path("/rules").Handler(__operationPostConfigsRules)

	// Registered routing GET /configs/rules
	var __operationGetConfigsRules http.Handler
	__operationGetConfigsRules = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationGetConfigsRules(_configsService, w, r)
	})
	_routeConfigs.Methods("GET").Path("/rules").Handler(__operationGetConfigsRules)

	// Registered routing PUT /configs/raw
	var __operationPutConfigsRaw http.Handler
	__operationPutConfigsRaw = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationPutConfigsRaw(_configsService, w, r)
	})
	_routeConfigs.Methods("PUT").Path("/raw").Handler(__operationPutConfigsRaw)

	// Registered routing GET /configs/raw
	var __operationGetConfigsRaw http.Handler
	__operationGetConfigsRaw = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationGetConfigsRaw(_configsService, w, r)
	})
	_routeConfigs.Methods("GET").Path("/raw").Handler(__operationGetConfigsRaw)

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

	// Registered routing PUT /configs/rules/{name}
	var __operationPutConfigsRulesName http.Handler
	__operationPutConfigsRulesName = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationPutConfigsRulesName(_configsService, w, r)
	})
	_routeConfigs.Methods("PUT").Path("/rules/{name}").Handler(__operationPutConfigsRulesName)

	// Registered routing DELETE /configs/rules/{name}
	var __operationDeleteConfigsRulesName http.Handler
	__operationDeleteConfigsRulesName = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationDeleteConfigsRulesName(_configsService, w, r)
	})
	_routeConfigs.Methods("DELETE").Path("/rules/{name}").Handler(__operationDeleteConfigsRulesName)

	// Registered routing GET /configs/rules/{name}
	var __operationGetConfigsRulesName http.Handler
	__operationGetConfigsRulesName = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_operationGetConfigsRulesName(_configsService, w, r)
	})
	_routeConfigs.Methods("GET").Path("/rules/{name}").Handler(__operationGetConfigsRulesName)

	if router.NotFoundHandler == nil {
		router.NotFoundHandler = http.HandlerFunc(notFoundHandler)
	}
	return router
}

// _requestBodyAddress Parsing the body for of address
func _requestBodyAddress(w http.ResponseWriter, r *http.Request) (_address *githubComWzshimingJumpwayConfig.Address, err error) {

	defer r.Body.Close()

	var __address []byte
	__address, err = ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 400)

		return
	}

	err = json.Unmarshal(__address, &_address)
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

// _requestBodyR Parsing the body for of r
func _requestBodyR(w http.ResponseWriter, r *http.Request) (_r *githubComWzshimingJumpwayConfig.Rule, err error) {

	defer r.Body.Close()

	var __r []byte
	__r, err = ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 400)

		return
	}

	err = json.Unmarshal(__r, &_r)
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

// _operationPutConfigsWebUI Is the route of UpdateWebUI
func _operationPutConfigsWebUI(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// requests github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateWebUI.address
	var _address *githubComWzshimingJumpwayConfig.Address
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateWebUI.err
	var _err error

	// Parsing address.
	_address, _err = _requestBodyAddress(w, r)
	if _err != nil {
		return
	}

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateWebUI.
	_err = s.UpdateWebUI(_address)

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

// _operationPutConfigsRulesName Is the route of UpdateRule
func _operationPutConfigsRulesName(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// requests github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateRule.name
	var _name string
	// requests github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateRule.r
	var _r *githubComWzshimingJumpwayConfig.Rule
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateRule.err
	var _err error

	// Parsing name.
	_name, _err = _requestPathName(w, r)
	if _err != nil {
		return
	}

	// Parsing r.
	_r, _err = _requestBodyR(w, r)
	if _err != nil {
		return
	}

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.UpdateRule.
	_err = s.UpdateRule(_name, _r)

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

// _operationGetConfigsRules Is the route of ListRules
func _operationGetConfigsRules(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.ListRules.rules
	var _rules []githubComWzshimingJumpwayConfig.Rule
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.ListRules.err
	var _err error

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.ListRules.
	_rules, _err = s.ListRules()

	// Response code 200 OK for rules.
	if _rules != nil {
		var __rules []byte
		__rules, _err = json.Marshal(_rules)
		if _err != nil {
			http.Error(w, _err.Error(), 500)

			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		w.Write(__rules)
		return
	}

	// Response code 400 Bad Request for err.
	if _err != nil {
		http.Error(w, _err.Error(), 400)
		return
	}

	var __rules []byte
	__rules, _err = json.Marshal(_rules)
	if _err != nil {
		http.Error(w, _err.Error(), 500)

		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	w.Write(__rules)

	return
}

// _operationGetConfigsWebUI Is the route of GetWebUI
func _operationGetConfigsWebUI(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetWebUI.address
	var _address_1 *githubComWzshimingJumpwayConfig.Address
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetWebUI.err
	var _err error

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetWebUI.
	_address_1, _err = s.GetWebUI()

	// Response code 200 OK for address.
	if _address_1 != nil {
		var __address_1 []byte
		__address_1, _err = json.Marshal(_address_1)
		if _err != nil {
			http.Error(w, _err.Error(), 500)

			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		w.Write(__address_1)
		return
	}

	// Response code 400 Bad Request for err.
	if _err != nil {
		http.Error(w, _err.Error(), 400)
		return
	}

	var __address_1 []byte
	__address_1, _err = json.Marshal(_address_1)
	if _err != nil {
		http.Error(w, _err.Error(), 500)

		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	w.Write(__address_1)

	return
}

// _operationGetConfigsRulesName Is the route of GetRule
func _operationGetConfigsRulesName(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// requests github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetRule.name
	var _name string
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetRule.rule
	var _rule *githubComWzshimingJumpwayConfig.Rule
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetRule.err
	var _err error

	// Parsing name.
	_name, _err = _requestPathName(w, r)
	if _err != nil {
		return
	}

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.GetRule.
	_rule, _err = s.GetRule(_name)

	// Response code 200 OK for rule.
	if _rule != nil {
		var __rule []byte
		__rule, _err = json.Marshal(_rule)
		if _err != nil {
			http.Error(w, _err.Error(), 500)

			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		w.Write(__rule)
		return
	}

	// Response code 400 Bad Request for err.
	if _err != nil {
		http.Error(w, _err.Error(), 400)
		return
	}

	var __rule []byte
	__rule, _err = json.Marshal(_rule)
	if _err != nil {
		http.Error(w, _err.Error(), 500)

		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	w.Write(__rule)

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

// _operationDeleteConfigsRulesName Is the route of DeleteRule
func _operationDeleteConfigsRulesName(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// requests github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.DeleteRule.name
	var _name string
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.DeleteRule.err
	var _err error

	// Parsing name.
	_name, _err = _requestPathName(w, r)
	if _err != nil {
		return
	}

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.DeleteRule.
	_err = s.DeleteRule(_name)

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

// _operationPostConfigsRules Is the route of CreateRule
func _operationPostConfigsRules(s *githubComWzshimingJumpwayAppWebServicesConfigs.ConfigsService, w http.ResponseWriter, r *http.Request) {
	// requests github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.CreateRule.r
	var _r *githubComWzshimingJumpwayConfig.Rule
	// responses github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.CreateRule.err
	var _err error

	// Parsing r.
	_r, _err = _requestBodyR(w, r)
	if _err != nil {
		return
	}

	// Call github.com/wzshiming/jumpway/app/web/services/configs ConfigsService.CreateRule.
	_err = s.CreateRule(_r)

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
