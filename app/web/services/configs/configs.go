package configs

import (
	"errors"
	"strings"

	"github.com/wzshiming/jumpway/app/web/runtime"
	"github.com/wzshiming/jumpway/config"
	"gopkg.in/yaml.v3"
)

// ConfigsService
// #path:"/configs/"#
type ConfigsService struct {
}

// NewConfigsService Create a new ConfigsService
func NewConfigsService() (*ConfigsService, error) {
	return &ConfigsService{}, nil
}

// Update the Config
// #route:"PUT /"#
func (s *ConfigsService) Update(conf *config.Config) (err error) {
	if err := config.Validate(conf); err != nil {
		return err
	}
	if err := config.SaveConfig(conf); err != nil {
		return err
	}
	return runtime.Get().Reload()
}

// Get the Config
// #route:"GET /"#
func (s *ConfigsService) Get() (conf *config.Config, err error) {
	return config.LoadConfig()
}

// RawConfig carries config.yaml verbatim.
type RawConfig struct {
	YAML string `json:"yaml"`
}

// GetRaw the config.yaml text
// #route:"GET /raw"#
func (s *ConfigsService) GetRaw() (raw *RawConfig, err error) {
	data, err := config.LoadRawConfig()
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
	if err := config.SaveRawConfig(data); err != nil {
		return err
	}
	return runtime.Get().Reload()
}

// Status of the running proxy
// #route:"GET /status"#
func (s *ConfigsService) Status() (status *runtime.Status, err error) {
	st := runtime.Get().Status()
	return &st, nil
}
