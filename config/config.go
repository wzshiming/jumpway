package config

import (
	"fmt"
	"net"
	"net/url"

	bridgeconfig "github.com/wzshiming/bridge/config"
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
	Host      string              `yaml:"host,omitempty" json:"host"`
	Port      uint32              `yaml:"port" json:"port"`
	Virtual   string              `yaml:"virtual,omitempty" json:"virtual,omitempty"`
	Way       []bridgeconfig.Node `yaml:"way,omitempty" json:"way,omitempty"`
	Username  string              `yaml:"username,omitempty" json:"username,omitempty"`
	Password  string              `yaml:"password,omitempty" json:"password,omitempty"`
	Cipher    string              `yaml:"cipher,omitempty" json:"cipher,omitempty"`
	Protocols []Protocol          `yaml:"protocols,omitempty" json:"protocols,omitempty"`
}

// Protocol is one proxy scheme served on the entry; empty credentials inherit the shared Listen ones.
type Protocol struct {
	Type     string `yaml:"type" json:"type"`
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
	Cipher   string `yaml:"cipher,omitempty" json:"cipher,omitempty"`
}

const (
	ProtocolHTTP        = "http"
	ProtocolSOCKS5      = "socks5"
	ProtocolSOCKS4      = "socks4"
	ProtocolSSH         = "ssh"
	ProtocolShadowsocks = "ss"
)

// DefaultProtocols is served when Protocols is empty, plus Shadowsocks when Cipher is set.
var DefaultProtocols = []string{ProtocolHTTP, ProtocolSOCKS5, ProtocolSOCKS4, ProtocolSSH}

func (l Listen) Address() string {
	if l.Virtual != "" {
		return VirtualScheme + l.Virtual
	}
	return (Address{Host: l.Host, Port: l.Port}).String()
}

func (l Listen) Remote() bool {
	return len(l.Way) > 0
}

// Schemes is the served protocol list with shared credentials filled in; the receiver's slices are left untouched.
func (l Listen) Schemes() []Protocol {
	protocols := l.Protocols
	if len(protocols) == 0 {
		protocols = make([]Protocol, 0, len(DefaultProtocols)+1)
		for _, typ := range DefaultProtocols {
			protocols = append(protocols, Protocol{Type: typ})
		}
		if l.Cipher != "" {
			protocols = append(protocols, Protocol{Type: ProtocolShadowsocks})
		}
	}
	schemes := make([]Protocol, len(protocols))
	for index, protocol := range protocols {
		if protocol.Username == "" {
			protocol.Username = l.Username
		}
		if protocol.Password == "" {
			protocol.Password = l.Password
		}
		if protocol.Type == ProtocolShadowsocks && protocol.Cipher == "" {
			protocol.Cipher = l.Cipher
		}
		schemes[index] = protocol
	}
	return schemes
}

// User is the proxy URL userinfo: cipher:password for ss, username only for socks4, nil when unauthenticated.
func (p Protocol) User() *url.Userinfo {
	switch {
	case p.Type == ProtocolShadowsocks:
		if p.Cipher == "" {
			return nil
		}
		return url.UserPassword(p.Cipher, p.Password)
	case p.Username == "":
		return nil
	case p.Password == "" || p.Type == ProtocolSOCKS4:
		return url.User(p.Username)
	}
	return url.UserPassword(p.Username, p.Password)
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
