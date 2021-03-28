package config

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/skratchdot/open-golang/open"
	"github.com/wzshiming/jumpway"
	"github.com/wzshiming/logger"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Ways  jumpway.Ways
	Proxy Proxy
}

type Proxy struct {
	Port uint32
}

var (
	configDir  = ""
	configPath = ""

	//go:embed config.yaml
	defaultConfig string
)

func init() {
	var err error
	configDir, err = os.UserHomeDir()
	if err != nil {
		logger.Log.Error(err, "get hostname")
		os.Exit(2)
	}
	configDir = filepath.Join(configDir, ".jumpway")
	configPath = filepath.Join(configDir, "config.yaml")
}

func GetConfigDir() string {
	return configDir
}

func InitConfig() error {
	fi, err := os.Stat(configPath)
	if err == nil && fi.Size() != 0 {
		return nil
	}
	os.MkdirAll(filepath.Dir(configPath), 0755)
	return os.WriteFile(configPath, []byte(defaultConfig), 0644)
}

func LoadConfig() (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	conf := Config{}
	err = yaml.Unmarshal(data, &conf)
	if err != nil {
		return nil, err
	}
	return &conf, nil
}

func EditConfig() error {
	return open.Run(configPath)
}
