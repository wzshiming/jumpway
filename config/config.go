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
	"unicode"

	"github.com/pkg/browser"
	bridgeconfig "github.com/wzshiming/bridge/config"
	"github.com/wzshiming/httpcache"
	"github.com/wzshiming/jumpway/i18n"
	"github.com/wzshiming/jumpway/log"
	"github.com/wzshiming/shadowsocks"
	_ "github.com/wzshiming/shadowsocks/init"
	"gopkg.in/yaml.v3"
)

type Config struct {
	WebUI   Address `yaml:"web_ui" json:"web_ui"`
	Rules   []Rule  `yaml:"rules" json:"rules"`
	NoProxy NoProxy `yaml:"no_proxy" json:"no_proxy"`
}

// Address is a local TCP listen address; an empty Host means 127.0.0.1.
type Address struct {
	Host string `yaml:"host" json:"host"`
	Port uint32 `yaml:"port" json:"port"`
}

func (a Address) String() string {
	if a.Host == "" {
		a.Host = "127.0.0.1"
	}
	return net.JoinHostPort(a.Host, fmt.Sprint(a.Port))
}

// Rule is one entry: where connections come in and where they go out.
type Rule struct {
	Name     string  `yaml:"name" json:"name"`
	Disabled bool    `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	Listen   Listen  `yaml:"listen,omitempty" json:"listen"`
	Forward  Forward `yaml:"forward,omitempty" json:"forward"`
}

// Listen is the entry: Host:Port is bound by the first hop of Way, or by this machine when Way is empty; Virtual names an in-process channel instead.
type Listen struct {
	Host     string              `yaml:"host,omitempty" json:"host"`
	Port     uint32              `yaml:"port" json:"port"`
	Virtual  string              `yaml:"virtual,omitempty" json:"virtual,omitempty"`
	Way      []bridgeconfig.Node `yaml:"way,omitempty" json:"way,omitempty"`
	Username string              `yaml:"username,omitempty" json:"username,omitempty"`
	Password string              `yaml:"password,omitempty" json:"password,omitempty"`
	Cipher   string              `yaml:"cipher,omitempty" json:"cipher,omitempty"`
}

func (l Listen) Address() string {
	if l.Virtual != "" {
		return VirtualScheme + l.Virtual
	}
	return (Address{Host: l.Host, Port: l.Port}).String()
}

func (l Listen) Remote() bool {
	return len(l.Way) > 0
}

func (l Listen) User() *url.Userinfo {
	if l.Username == "" {
		return nil
	}
	if l.Password != "" {
		return url.UserPassword(l.Username, l.Password)
	}
	return url.User(l.Username)
}

// Shadowsocks is the cipher:password pair served as ss:// on the entry, nil when no cipher is set.
func (l Listen) Shadowsocks() *url.Userinfo {
	if l.Cipher == "" {
		return nil
	}
	return url.UserPassword(l.Cipher, l.Password)
}

// Forward is the exit: connections are dialed through Way (empty = from this machine) to Host:Port, to the in-process channel Virtual, or to the proxy client's own target when both are empty.
type Forward struct {
	Host    string              `yaml:"host,omitempty" json:"host,omitempty"`
	Port    uint32              `yaml:"port,omitempty" json:"port,omitempty"`
	Virtual string              `yaml:"virtual,omitempty" json:"virtual,omitempty"`
	Way     []bridgeconfig.Node `yaml:"way,omitempty" json:"way,omitempty"`
}

// IsProxy reports whether the rule serves proxy protocols instead of forwarding to a fixed target.
func (f Forward) IsProxy() bool { return f.Port == 0 && f.Virtual == "" }

// Target is the fixed destination, empty in proxy mode; an empty Host means 127.0.0.1 on the exit node.
func (f Forward) Target() string {
	if f.Virtual != "" {
		return VirtualScheme + f.Virtual
	}
	if f.IsProxy() {
		return ""
	}
	return (Address{Host: f.Host, Port: f.Port}).String()
}

// VirtualScheme prefixes the display form of an in-process channel endpoint.
const VirtualScheme = "virtual://"

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
	if conf.WebUI.Port > 65535 {
		return fmt.Errorf("web_ui.port %d is out of range (0-65535)", conf.WebUI.Port)
	}
	names := make(map[string]struct{}, len(conf.Rules))
	addresses := make(map[string]string, len(conf.Rules))
	for ruleIndex, rule := range conf.Rules {
		if strings.TrimSpace(rule.Name) == "" {
			return fmt.Errorf("rules[%d].name is empty", ruleIndex)
		}
		if strings.Contains(rule.Name, "/") {
			return fmt.Errorf("rules[%d].name %q must not contain \"/\"", ruleIndex, rule.Name)
		}
		if _, ok := names[rule.Name]; ok {
			return fmt.Errorf("duplicate rule name %q", rule.Name)
		}
		names[rule.Name] = struct{}{}
		if rule.Listen.Port > 65535 {
			return fmt.Errorf("rules[%d].listen.port %d is out of range (0-65535)", ruleIndex, rule.Listen.Port)
		}
		if rule.Forward.Port > 65535 {
			return fmt.Errorf("rules[%d].forward.port %d is out of range (0-65535)", ruleIndex, rule.Forward.Port)
		}
		if rule.Forward.Host != "" && rule.Forward.IsProxy() {
			return fmt.Errorf("rules[%d].forward.host is set but port is 0", ruleIndex)
		}
		if rule.Listen.Cipher != "" && rule.Listen.Password == "" {
			return fmt.Errorf("rules[%d].listen.cipher is set but password is empty", ruleIndex)
		}
		if rule.Listen.Password != "" && rule.Listen.Username == "" && rule.Listen.Cipher == "" {
			return fmt.Errorf("rules[%d].listen.password is set but username is empty", ruleIndex)
		}
		if strings.Contains(rule.Listen.Username, ":") {
			return fmt.Errorf("rules[%d].listen.username %q must not contain \":\"", ruleIndex, rule.Listen.Username)
		}
		if !rule.Forward.IsProxy() && rule.Listen.Username != "" {
			return fmt.Errorf("rules[%d].listen.username is only used by proxy rules", ruleIndex)
		}
		if !rule.Forward.IsProxy() && rule.Listen.Cipher != "" {
			return fmt.Errorf("rules[%d].listen.cipher is only used by proxy rules", ruleIndex)
		}
		if rule.Listen.Cipher != "" && !shadowsocks.IsCipher(rule.Listen.Cipher) {
			return fmt.Errorf("rules[%d].listen.cipher %q is unsupported", ruleIndex, rule.Listen.Cipher)
		}
		if err := validateVirtual(fmt.Sprintf("rules[%d].listen", ruleIndex), rule.Listen.Virtual, rule.Listen.Host, rule.Listen.Port, rule.Listen.Way); err != nil {
			return err
		}
		if err := validateVirtual(fmt.Sprintf("rules[%d].forward", ruleIndex), rule.Forward.Virtual, rule.Forward.Host, rule.Forward.Port, rule.Forward.Way); err != nil {
			return err
		}
		if rule.Listen.Virtual != "" && rule.Listen.Virtual == rule.Forward.Virtual {
			return fmt.Errorf("rules[%d].forward.virtual %q loops back to its own listener", ruleIndex, rule.Forward.Virtual)
		}
		if err := validateWay(fmt.Sprintf("rules[%d].listen.way", ruleIndex), rule.Listen.Way); err != nil {
			return err
		}
		if err := validateWay(fmt.Sprintf("rules[%d].forward.way", ruleIndex), rule.Forward.Way); err != nil {
			return err
		}
		if rule.Disabled || rule.Listen.Remote() || (rule.Listen.Port == 0 && rule.Listen.Virtual == "") {
			continue
		}
		address := rule.Listen.Address()
		if name, ok := addresses[address]; ok {
			return fmt.Errorf("rules[%d].listen address %s is already used by rule %q", ruleIndex, address, name)
		}
		if conf.WebUI.Port != 0 && address == conf.WebUI.String() {
			return fmt.Errorf("rules[%d].listen address %s is already used by web_ui", ruleIndex, address)
		}
		addresses[address] = rule.Name
	}
	return nil
}

func validateVirtual(prefix, virtual, host string, port uint32, way []bridgeconfig.Node) error {
	if virtual == "" {
		return nil
	}
	if strings.ContainsFunc(virtual, unicode.IsSpace) || strings.Contains(virtual, "/") {
		return fmt.Errorf("%s.virtual %q must not contain whitespace or \"/\"", prefix, virtual)
	}
	if host != "" {
		return fmt.Errorf("%s.virtual must not be combined with host", prefix)
	}
	if port != 0 {
		return fmt.Errorf("%s.virtual must not be combined with port", prefix)
	}
	if len(way) != 0 {
		return fmt.Errorf("%s.virtual must not be combined with way", prefix)
	}
	return nil
}

func validateWay(prefix string, way []bridgeconfig.Node) error {
	for nodeIndex, node := range way {
		if len(node.LB) == 0 {
			return fmt.Errorf("%s[%d] has no proxy URL", prefix, nodeIndex)
		}
		for _, proxyURL := range node.LB {
			if strings.TrimSpace(proxyURL) == "" {
				return fmt.Errorf("%s[%d] contains an empty proxy URL", prefix, nodeIndex)
			}
			parsedURL, err := url.Parse(proxyURL)
			if err != nil {
				return fmt.Errorf("%s[%d]: invalid proxy URL %q: %w (e.g. socks5://host:1080)", prefix, nodeIndex, proxyURL, err)
			}
			if parsedURL.Scheme == "" {
				return fmt.Errorf("%s[%d]: proxy URL %q has no scheme (e.g. socks5://host:1080)", prefix, nodeIndex, proxyURL)
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
