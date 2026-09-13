package configs

import (
	"errors"
	"strings"

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
}

// NewConfigsService Create a new ConfigsService
func NewConfigsService(store *config.Store, runtime Runtime) *ConfigsService {
	return &ConfigsService{store: store, runtime: runtime}
}

// Update the Config
// #route:"PUT /"#
func (s *ConfigsService) Update(conf *config.Config) (err error) {
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
