package configs

import (
	"github.com/wzshiming/jumpway/config"
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
	return config.SaveConfig(conf)
}

// Get the Config
// #route:"GET /"#
func (s *ConfigsService) Get() (conf *config.Config, err error) {
	return config.LoadConfig()
}
