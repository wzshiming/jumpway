package configs

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/wzshiming/jumpway/config"
	"gopkg.in/yaml.v3"
)

// Status is what the web UI shows about the running app: the web UI listener and every enabled rule.
type Status struct {
	Address string       `json:"address"`
	Running bool         `json:"running"`
	Error   string       `json:"error,omitempty"`
	Rules   []RuleStatus `json:"rules"`
}

// RuleStatus is one rule's listener state; Attempt counts consecutive failed listen attempts.
type RuleStatus struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	// Target is the fixed destination for port-forward rules; empty for proxy rules.
	Target  string `json:"target,omitempty"`
	Remote  bool   `json:"remote"`
	Running bool   `json:"running"`
	Attempt int    `json:"attempt,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Runtime is the running proxy the service reloads and inspects.
type Runtime interface {
	Reload() error
	Status() Status
}

// ConfigsService
// #path:"/configs/"#
type ConfigsService struct {
	store   *config.Store
	runtime Runtime
	mu      sync.Mutex
}

// NewConfigsService Create a new ConfigsService
func NewConfigsService(store *config.Store, runtime Runtime) *ConfigsService {
	return &ConfigsService{store: store, runtime: runtime}
}

// modify applies fn to the stored config, then validates, saves and reloads.
func (s *ConfigsService) modify(fn func(conf *config.Config) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	conf, err := s.store.Load()
	if err != nil {
		return err
	}
	if err := fn(conf); err != nil {
		return err
	}
	if err := config.Validate(conf); err != nil {
		return err
	}
	if err := s.store.Save(conf); err != nil {
		return err
	}
	return s.runtime.Reload()
}

// Update the Config
// #route:"PUT /"#
func (s *ConfigsService) Update(conf *config.Config) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := config.Validate(conf); err != nil {
		return err
	}
	if err := s.store.Save(conf); err != nil {
		return err
	}
	return s.runtime.Reload()
}

// Get the Config
// #route:"GET /"#
func (s *ConfigsService) Get() (conf *config.Config, err error) {
	return s.store.Load()
}

// GetWebUI returns the web UI listen address
// #route:"GET /web-ui"#
func (s *ConfigsService) GetWebUI() (address *config.Address, err error) {
	conf, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	return &conf.WebUI, nil
}

// UpdateWebUI changes the web UI listen address
// #route:"PUT /web-ui"#
func (s *ConfigsService) UpdateWebUI(address *config.Address) (err error) {
	if address == nil {
		return errors.New("web_ui is nil")
	}
	return s.modify(func(conf *config.Config) error {
		conf.WebUI = *address
		return nil
	})
}

// ListRules lists the rules
// #route:"GET /rules"#
func (s *ConfigsService) ListRules() (rules []config.Rule, err error) {
	conf, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	if conf.Rules == nil {
		return []config.Rule{}, nil
	}
	return conf.Rules, nil
}

// CreateRule adds a rule
// #route:"POST /rules"#
func (s *ConfigsService) CreateRule(r *config.Rule) (err error) {
	if r == nil || strings.TrimSpace(r.Name) == "" {
		return errors.New("rule name is empty")
	}
	return s.modify(func(conf *config.Config) error {
		for _, candidate := range conf.Rules {
			if candidate.Name == r.Name {
				return fmt.Errorf("rule %q already exists", r.Name)
			}
		}
		conf.Rules = append(conf.Rules, *r)
		return nil
	})
}

// GetRule returns one rule
// #route:"GET /rules/{name}"#
func (s *ConfigsService) GetRule(name string /* #name:"name"# */) (rule *config.Rule, err error) {
	conf, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	for index := range conf.Rules {
		if conf.Rules[index].Name == name {
			return &conf.Rules[index], nil
		}
	}
	return nil, fmt.Errorf("rule %q not found", name)
}

// UpdateRule replaces one rule (renames it when the body name differs)
// #route:"PUT /rules/{name}"#
func (s *ConfigsService) UpdateRule(name string /* #name:"name"# */, r *config.Rule) (err error) {
	if r == nil {
		return errors.New("rule is nil")
	}
	return s.modify(func(conf *config.Config) error {
		index := slices.IndexFunc(conf.Rules, func(candidate config.Rule) bool {
			return candidate.Name == name
		})
		if index == -1 {
			return fmt.Errorf("rule %q not found", name)
		}
		replacement := *r
		if replacement.Name == "" {
			replacement.Name = name
		}
		if replacement.Name != name {
			for _, candidate := range conf.Rules {
				if candidate.Name == replacement.Name {
					return fmt.Errorf("rule %q already exists", replacement.Name)
				}
			}
		}
		conf.Rules[index] = replacement
		return nil
	})
}

// DeleteRule removes one rule
// #route:"DELETE /rules/{name}"#
func (s *ConfigsService) DeleteRule(name string /* #name:"name"# */) (err error) {
	return s.modify(func(conf *config.Config) error {
		index := slices.IndexFunc(conf.Rules, func(candidate config.Rule) bool {
			return candidate.Name == name
		})
		if index == -1 {
			return fmt.Errorf("rule %q not found", name)
		}
		conf.Rules = slices.Delete(conf.Rules, index, index+1)
		return nil
	})
}

// GetNoProxy returns the bypass lists
// #route:"GET /no-proxy"#
func (s *ConfigsService) GetNoProxy() (noProxy *config.NoProxy, err error) {
	conf, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	return &conf.NoProxy, nil
}

// UpdateNoProxy replaces the bypass lists
// #route:"PUT /no-proxy"#
func (s *ConfigsService) UpdateNoProxy(noProxy *config.NoProxy) (err error) {
	if noProxy == nil {
		return errors.New("no-proxy is nil")
	}
	return s.modify(func(conf *config.Config) error {
		conf.NoProxy = *noProxy
		return nil
	})
}

// RawConfig carries config.yaml verbatim.
type RawConfig struct {
	YAML string `json:"yaml"`
}

// GetRaw the config.yaml text
// #route:"GET /raw"#
func (s *ConfigsService) GetRaw() (raw *RawConfig, err error) {
	data, err := s.store.LoadRaw()
	if err != nil {
		return nil, err
	}
	return &RawConfig{YAML: string(data)}, nil
}

// UpdateRaw replaces config.yaml with the given text
// #route:"PUT /raw"#
func (s *ConfigsService) UpdateRaw(raw *RawConfig) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if raw == nil || strings.TrimSpace(raw.YAML) == "" {
		return errors.New("config is empty")
	}
	data := []byte(raw.YAML)
	var conf config.Config
	if err := yaml.Unmarshal(data, &conf); err != nil {
		return err
	}
	if err := config.Validate(&conf); err != nil {
		return err
	}
	if err := s.store.SaveRaw(data); err != nil {
		return err
	}
	return s.runtime.Reload()
}

// Status of the running proxy
// #route:"GET /status"#
func (s *ConfigsService) Status() (status *Status, err error) {
	st := s.runtime.Status()
	return &st, nil
}
