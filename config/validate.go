package config

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"unicode"

	bridgeconfig "github.com/wzshiming/bridge/config"
	"github.com/wzshiming/shadowsocks"
	_ "github.com/wzshiming/shadowsocks/init"
)

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
		if err := validateListenAuth(fmt.Sprintf("rules[%d].listen", ruleIndex), rule.Listen, rule.Forward.IsProxy()); err != nil {
			return err
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
	tunOwner := ""
	for ruleIndex, rule := range conf.Rules {
		protocolIndex := slices.IndexFunc(rule.Listen.Protocols, func(protocol Protocol) bool { return protocol.Type == ProtocolTUN })
		if rule.Disabled || protocolIndex < 0 {
			continue
		}
		if tunOwner != "" {
			return fmt.Errorf("rules[%d].listen.protocols[%d].type %q is already used by rule %q", ruleIndex, protocolIndex, ProtocolTUN, tunOwner)
		}
		tunOwner = rule.Name
	}
	return nil
}

func validateListenAuth(prefix string, listen Listen, proxy bool) error {
	if strings.Contains(listen.Username, ":") {
		return fmt.Errorf("%s.username %q must not contain \":\"", prefix, listen.Username)
	}
	if listen.Cipher != "" && !shadowsocks.IsCipher(listen.Cipher) {
		return fmt.Errorf("%s.cipher %q is unsupported", prefix, listen.Cipher)
	}
	if !proxy {
		switch {
		case listen.Username != "":
			return fmt.Errorf("%s.username is only used by proxy rules", prefix)
		case listen.Cipher != "":
			return fmt.Errorf("%s.cipher is only used by proxy rules", prefix)
		case len(listen.Protocols) != 0:
			return fmt.Errorf("%s.protocols is only used by proxy rules", prefix)
		}
	}
	explicit := len(listen.Protocols) != 0
	// The shared password may serve ss alone, leaving the other schemes unauthenticated as before.
	sharedForSS := !explicit && listen.Cipher != ""
	types := make(map[string]bool, len(listen.Protocols))
	for index, protocol := range listen.Protocols {
		field := fmt.Sprintf("%s.protocols[%d]", prefix, index)
		switch protocol.Type {
		case ProtocolHTTP, ProtocolSOCKS5, ProtocolSOCKS4, ProtocolSSH, ProtocolShadowsocks, ProtocolTUN:
		default:
			return fmt.Errorf("%s.type %q is unknown", field, protocol.Type)
		}
		if types[protocol.Type] {
			return fmt.Errorf("%s.type %q is duplicated", field, protocol.Type)
		}
		types[protocol.Type] = true
		if err := validateTUN(field, protocol, listen); err != nil {
			return err
		}
		if strings.Contains(protocol.Username, ":") {
			return fmt.Errorf("%s.username %q must not contain \":\"", field, protocol.Username)
		}
		if protocol.Cipher != "" && protocol.Type != ProtocolShadowsocks {
			return fmt.Errorf("%s.cipher is only used by ss", field)
		}
		if protocol.Cipher != "" && !shadowsocks.IsCipher(protocol.Cipher) {
			return fmt.Errorf("%s.cipher %q is unsupported", field, protocol.Cipher)
		}
		if protocol.Type == ProtocolShadowsocks && protocol.Password == "" {
			sharedForSS = true
		}
	}
	if explicit && listen.Cipher != "" && !types[ProtocolShadowsocks] {
		return fmt.Errorf("%s.cipher is set but protocols has no ss", prefix)
	}
	index := 0
	for _, scheme := range listen.Schemes() {
		var raw Protocol
		if explicit {
			// Schemes leaves tun rows out; skipping them keeps index on the raw row.
			for listen.Protocols[index].Type == ProtocolTUN {
				index++
			}
			raw = listen.Protocols[index]
		}
		field := fmt.Sprintf("%s.protocols[%d]", prefix, index)
		index++
		if scheme.Type == ProtocolShadowsocks {
			switch {
			case scheme.Cipher == "":
				return fmt.Errorf("%s.cipher is empty", field)
			case scheme.Password != "":
			case raw.Cipher != "":
				return fmt.Errorf("%s.cipher is set but password is empty", field)
			default:
				return fmt.Errorf("%s.cipher is set but password is empty", prefix)
			}
			continue
		}
		switch {
		case scheme.Password == "" || scheme.Username != "":
		case raw.Password != "":
			return fmt.Errorf("%s.password is set but username is empty", field)
		case !sharedForSS:
			return fmt.Errorf("%s.password is set but username is empty", prefix)
		}
	}
	return nil
}

func validateTUN(field string, protocol Protocol, listen Listen) error {
	if protocol.Type != ProtocolTUN {
		switch {
		case len(protocol.Address) != 0:
			return fmt.Errorf("%s.address is only used by tun", field)
		case protocol.MTU != 0:
			return fmt.Errorf("%s.mtu is only used by tun", field)
		}
		return nil
	}
	switch {
	case protocol.Username != "":
		return fmt.Errorf("%s.username is not used by tun", field)
	case protocol.Password != "":
		return fmt.Errorf("%s.password is not used by tun", field)
	case protocol.Cipher != "":
		return fmt.Errorf("%s.cipher is not used by tun", field)
	}
	families := make(map[string]bool, 2)
	for addressIndex, address := range protocol.Address {
		entry := fmt.Sprintf("%s.address[%d] %q", field, addressIndex, address)
		prefix, ok := parseTUNAddress(address)
		if !ok {
			return fmt.Errorf("%s is not a CIDR (e.g. 198.18.0.1/30)", entry)
		}
		if prefix.Bits() == 0 {
			return fmt.Errorf("%s has prefix length 0", entry)
		}
		if addr := prefix.Addr(); addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsMulticast() || addr.IsUnspecified() {
			return fmt.Errorf("%s is not a usable interface address", entry)
		}
		if prefix.Bits() < prefix.Addr().BitLen() && prefix.Addr() == prefix.Masked().Addr() {
			return fmt.Errorf("%s has no host bits (e.g. 198.18.0.1/30)", entry)
		}
		family := "IPv6"
		if prefix.Addr().Is4() {
			family = "IPv4"
		}
		if families[family] {
			return fmt.Errorf("%s is a second %s address", entry, family)
		}
		families[family] = true
	}
	if protocol.MTU != 0 && (protocol.MTU < 1280 || protocol.MTU > 65535) {
		return fmt.Errorf("%s.mtu %d is out of range (1280-65535)", field, protocol.MTU)
	}
	switch {
	case listen.Virtual != "":
		return fmt.Errorf("%s.type %q must not be combined with virtual", field, protocol.Type)
	case len(listen.Way) != 0:
		return fmt.Errorf("%s.type %q must not be combined with way", field, protocol.Type)
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
