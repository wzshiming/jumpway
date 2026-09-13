package config

import (
	"bufio"
	_ "embed"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkg/browser"
	"github.com/wzshiming/bridge/config"
	"github.com/wzshiming/httpcache"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
	"gopkg.in/yaml.v3"
)

type Config struct {
	CurrentContext string    `yaml:"current_context" json:"current_context"`
	Contexts       []Context `yaml:"contexts" json:"contexts"`
	Proxy          Proxy     `yaml:"proxy" json:"proxy"`
	NoProxy        NoProxy   `yaml:"no_proxy" json:"no_proxy"`
}

func (c Config) GetWay() []config.Node {
	for _, ctx := range c.Contexts {
		if ctx.Name == c.CurrentContext {
			return ctx.Way
		}
	}
	return nil
}

type Context struct {
	Name string        `yaml:"name" json:"name"`
	Way  []config.Node `yaml:"way" json:"way"`
}

type Proxy struct {
	Host string `yaml:"host" json:"host"`
	Port uint32 `yaml:"port" json:"port"`
}

type NoProxy struct {
	List     []string `yaml:"list" json:"list"`
	FromEnv  []string `yaml:"from_env" json:"from_env"`
	FromFile []string `yaml:"from_file" json:"from_file"`
}

func (n *NoProxy) GetList(configDir string) []string {
	set := map[string]struct{}{}
	for _, item := range n.List {
		setEnv(set, item)
	}
	for _, env := range n.FromEnv {
		setEnv(set, os.Getenv(env))
	}
	for _, file := range n.FromFile {
		f, err := getFile(configDir, file)
		if err != nil {
			log.Error(err, i18n.OpenFile(), "file", file)
			continue
		}
		reader := bufio.NewReader(f)
		for i := 0; ; i++ {
			line, _, err := reader.ReadLine()
			if err != nil {
				if err != io.EOF {
					log.Error(err, i18n.OpenFile(), "file", file, "line", i+1)
				}
				break
			}
			setEnv(set, string(line))
		}
		f.Close()
	}
	list := make([]string, 0, len(set))
	for item := range set {
		list = append(list, item)
	}
	sort.Strings(list)
	return list
}

func setEnv(set map[string]struct{}, val string) {
	val = strings.TrimSpace(val)
	if val == "" {
		return
	}
	for _, m := range strings.Split(val, ",") {
		m = strings.TrimSpace(m)
		set[m] = struct{}{}
	}
}

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

func Validate(conf *Config) error {
	if conf == nil {
		return fmt.Errorf("config is nil")
	}
	if conf.Proxy.Port > 65535 {
		return fmt.Errorf("proxy.port %d is out of range (0-65535)", conf.Proxy.Port)
	}
	names := make(map[string]struct{}, len(conf.Contexts))
	for contextIndex, ctx := range conf.Contexts {
		if strings.TrimSpace(ctx.Name) == "" {
			return fmt.Errorf("contexts[%d].name is empty", contextIndex)
		}
		if _, ok := names[ctx.Name]; ok {
			return fmt.Errorf("duplicate context name %q", ctx.Name)
		}
		names[ctx.Name] = struct{}{}
	}
	if conf.CurrentContext != "" || len(conf.Contexts) > 0 {
		if _, ok := names[conf.CurrentContext]; !ok {
			return fmt.Errorf("current_context %q does not match any context", conf.CurrentContext)
		}
	}
	for contextIndex, ctx := range conf.Contexts {
		for nodeIndex, node := range ctx.Way {
			if len(node.LB) == 0 {
				return fmt.Errorf("contexts[%d].way[%d] has no proxy URL", contextIndex, nodeIndex)
			}
			for _, proxyURL := range node.LB {
				if strings.TrimSpace(proxyURL) == "" {
					return fmt.Errorf("contexts[%d].way[%d] contains an empty proxy URL", contextIndex, nodeIndex)
				}
				parsedURL, err := url.Parse(proxyURL)
				if err != nil {
					return fmt.Errorf("contexts[%d].way[%d]: invalid proxy URL %q: %w (e.g. socks5://host:1080)", contextIndex, nodeIndex, proxyURL, err)
				}
				if parsedURL.Scheme == "" {
					return fmt.Errorf("contexts[%d].way[%d]: proxy URL %q has no scheme (e.g. socks5://host:1080)", contextIndex, nodeIndex, proxyURL)
				}
			}
		}
	}
	return nil
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

func getFile(configDir, filePath string) (io.ReadCloser, error) {
	u, err := url.Parse(filePath)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "http", "https":
		req, err := http.NewRequest(http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		client := *httpCli
		client.Transport = httpcache.NewRoundTripper(client.Transport, httpcache.WithStorer(
			httpcache.DirectoryStorer(filepath.Join(configDir, "cache")),
		))
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("unexpected status %s for %s", resp.Status, u.String())
		}
		return resp.Body, nil
	case "file", "":
		file := u.Path
		if strings.HasPrefix(file, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			file = filepath.Join(home, file[1:])
		} else if strings.HasPrefix(file, ".") {
			file = filepath.Join(configDir, file[1:])
		}
		body, err := os.OpenFile(file, os.O_RDONLY, 0)
		if err != nil {
			return nil, err
		}
		return body, nil
	default:
		return nil, fmt.Errorf("unknown scheme %v", u.Scheme)
	}
}

var httpCli = &http.Client{
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 5 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}
