package config

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/pkg/browser"
	"gopkg.in/yaml.v3"
)

//go:embed config.yaml
var defaultConfig string

type Store struct {
	dir, path string
}

func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".jumpway"), nil
}

func NewStore(dir string) *Store {
	return &Store{dir: dir, path: filepath.Join(dir, "config.yaml")}
}

func (s *Store) Dir() string {
	return s.dir
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Init() error {
	fi, err := os.Stat(s.path)
	if err == nil && fi.Size() != 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	return os.WriteFile(s.path, []byte(defaultConfig), 0644)
}

func (s *Store) Load() (*Config, error) {
	data, err := os.ReadFile(s.path)
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

func (s *Store) Save(conf *Config) error {
	out, err := yaml.Marshal(conf)
	if err != nil {
		return err
	}
	return s.SaveRaw(out)
}

func (s *Store) LoadRaw() ([]byte, error) {
	return os.ReadFile(s.path)
}

func (s *Store) SaveRaw(data []byte) error {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp(s.dir, "config.yaml.*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Chmod(0644); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	return os.Rename(tmpFile.Name(), s.path)
}

func (s *Store) Edit() error {
	return browser.OpenFile(s.path)
}
