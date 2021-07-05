package config

import (
	"bufio"
	"crypto/tls"
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

	"github.com/skratchdot/open-golang/open"
	"github.com/wzshiming/bridge/config"
	"github.com/wzshiming/httpcache"
	"github.com/wzshiming/jumpway/log"
	"gopkg.in/yaml.v3"
)

type Config struct {
	CurrentContext string    `yaml:"current_context"`
	Contexts       []Context `yaml:"contexts"`
	Proxy          Proxy     `yaml:"proxy"`
	NoProxy        NoProxy   `yaml:"no_proxy"`
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
	Name string        `yaml:"name"`
	Way  []config.Node `yaml:"way"`
}

type Proxy struct {
	Host string `yaml:"host"`
	Port uint32 `yaml:"port"`
}

type NoProxy struct {
	List     []string `yaml:"list"`
	FromEnv  []string `yaml:"from_env"`
	FromFile []string `yaml:"from_file"`
}

func (n *NoProxy) GetList() []string {
	set := map[string]struct{}{}
	for _, item := range n.List {
		setEnv(set, item)
	}
	for _, env := range n.FromEnv {
		setEnv(set, os.Getenv(env))
	}
	for _, file := range n.FromFile {
		f, err := getFile(file)
		if err != nil {
			log.Error(err, "Open file", "file", file)
			continue
		}
		reader := bufio.NewReader(f)
		for i := 0; ; i++ {
			line, _, err := reader.ReadLine()
			if err != nil {
				if err != io.EOF {
					log.Error(err, "Open file", "file", file, "line", i+1)
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

var (
	homeDir    = ""
	configDir  = ""
	configPath = ""

	//go:embed config.yaml
	defaultConfig string
)

func init() {
	var err error
	homeDir, err = os.UserHomeDir()
	if err != nil {
		log.Error(err, "get hostname")
		os.Exit(2)
	}
	configDir = filepath.Join(homeDir, ".jumpway")
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

func SaveConfig(conf *Config) error {
	out, err := yaml.Marshal(conf)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, out, 0644)
}

func EditConfig() error {
	return open.Run(configPath)
}

func getFile(filpath string) (io.ReadCloser, error) {
	u, err := url.Parse(filpath)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "http", "https":
		req, err := http.NewRequest(http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		resp, err := httpCli.Do(req)
		if err != nil {
			return nil, err
		}
		return resp.Body, nil
	case "file", "":
		file := u.Path
		if strings.HasPrefix(file, "~") {
			file = filepath.Join(homeDir, file[1:])
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

var httpCli *http.Client

func init() {
	httpCli = &http.Client{
		Transport: httpcache.NewRoundTripper(&http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 5 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}, httpcache.WithStorer(
			httpcache.DirectoryStorer(filepath.Join(configDir, "cache")),
		)),
	}
}
