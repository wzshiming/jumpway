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

// Status is what the web UI shows about the running proxy.
type Status struct {
	Address string `json:"address"`
	Running bool   `json:"running"`
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

type CurrentContext struct {
	Name string `json:"name"`
}

// GetCurrentContext returns the name of the active context
// #route:"GET /current-context"#
func (s *ConfigsService) GetCurrentContext() (current *CurrentContext, err error) {
	conf, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	return &CurrentContext{Name: conf.CurrentContext}, nil
}

// SetCurrentContext switches the active context
// #route:"PUT /current-context"#
func (s *ConfigsService) SetCurrentContext(current *CurrentContext) (err error) {
	if current == nil {
		return errors.New("current context is nil")
	}
	return s.modify(func(conf *config.Config) error {
		conf.CurrentContext = current.Name
		return nil
	})
}

// ListContexts lists the contexts
// #route:"GET /contexts"#
func (s *ConfigsService) ListContexts() (contexts []config.Context, err error) {
	conf, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	if conf.Contexts == nil {
		return []config.Context{}, nil
	}
	return conf.Contexts, nil
}

// CreateContext adds a context
// #route:"POST /contexts"#
func (s *ConfigsService) CreateContext(c *config.Context) (err error) {
	if c == nil || strings.TrimSpace(c.Name) == "" {
		return errors.New("context name is empty")
	}
	return s.modify(func(conf *config.Config) error {
		for _, candidate := range conf.Contexts {
			if candidate.Name == c.Name {
				return fmt.Errorf("context %q already exists", c.Name)
			}
		}
		if len(conf.Contexts) == 0 {
			conf.CurrentContext = c.Name
		}
		conf.Contexts = append(conf.Contexts, *c)
		return nil
	})
}

// GetContext returns one context
// #route:"GET /contexts/{name}"#
func (s *ConfigsService) GetContext(name string /* #name:"name"# */) (c *config.Context, err error) {
	conf, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	for index := range conf.Contexts {
		if conf.Contexts[index].Name == name {
			return &conf.Contexts[index], nil
		}
	}
	return nil, fmt.Errorf("context %q not found", name)
}

// UpdateContext replaces one context (renames it when the body name differs)
// #route:"PUT /contexts/{name}"#
func (s *ConfigsService) UpdateContext(name string /* #name:"name"# */, c *config.Context) (err error) {
	if c == nil {
		return errors.New("context is nil")
	}
	return s.modify(func(conf *config.Config) error {
		index := slices.IndexFunc(conf.Contexts, func(candidate config.Context) bool {
			return candidate.Name == name
		})
		if index == -1 {
			return fmt.Errorf("context %q not found", name)
		}
		replacement := *c
		if replacement.Name == "" {
			replacement.Name = name
		}
		if replacement.Name != name {
			for _, candidate := range conf.Contexts {
				if candidate.Name == replacement.Name {
					return fmt.Errorf("context %q already exists", replacement.Name)
				}
			}
		}
		if conf.CurrentContext == name {
			conf.CurrentContext = replacement.Name
		}
		conf.Contexts[index] = replacement
		return nil
	})
}

// DeleteContext removes one context
// #route:"DELETE /contexts/{name}"#
func (s *ConfigsService) DeleteContext(name string /* #name:"name"# */) (err error) {
	return s.modify(func(conf *config.Config) error {
		index := slices.IndexFunc(conf.Contexts, func(candidate config.Context) bool {
			return candidate.Name == name
		})
		if index == -1 {
			return fmt.Errorf("context %q not found", name)
		}
		if conf.CurrentContext == name {
			if len(conf.Contexts) > 1 {
				return fmt.Errorf("context %q is the current context; switch to another context first", name)
			}
			conf.CurrentContext = ""
		}
		conf.Contexts = slices.Delete(conf.Contexts, index, index+1)
		return nil
	})
}

// GetProxy returns the listen address
// #route:"GET /proxy"#
func (s *ConfigsService) GetProxy() (proxy *config.Proxy, err error) {
	conf, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	return &conf.Proxy, nil
}

// UpdateProxy changes the listen address
// #route:"PUT /proxy"#
func (s *ConfigsService) UpdateProxy(proxy *config.Proxy) (err error) {
	if proxy == nil {
		return errors.New("proxy is nil")
	}
	return s.modify(func(conf *config.Config) error {
		conf.Proxy = *proxy
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
