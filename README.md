# Jump Way

A cross-platform proxy GUI client

[![Build Darwin](https://github.com/wzshiming/jumpway/actions/workflows/build_darwin.yml/badge.svg)](https://github.com/wzshiming/jumpway/actions/workflows/build_darwin.yml)
[![Build Windows](https://github.com/wzshiming/jumpway/actions/workflows/build_windows.yml/badge.svg)](https://github.com/wzshiming/jumpway/actions/workflows/build_windows.yml)
[![Build Linux](https://github.com/wzshiming/jumpway/actions/workflows/build_linux.yml/badge.svg)](https://github.com/wzshiming/jumpway/actions/workflows/build_linux.yml)

## Feature

- [x] I18n
- [x] System Tray
    - [x] Power on
    - [x] System proxy
    - [x] Proxy export line to clipboard
        - [x] Shell
        - [x] Shell git (`nc`)
        - [x] Cmd
        - [x] Cmd git (Git for Windows bundled `ssh` and `connect.exe`)
        - [x] PowerShell
        - [x] PowerShell git (Git for Windows bundled `ssh` and `connect.exe`)
- [x] Configure the proxy with GUI (Web UI, see below)
    - [x] Configure the multi-level proxy
    - [ ] Support to get SSH proxy configuration from `~/.ssh/config`
- [x] Multi-level proxy [Bridge](https://github.com/wzshiming/bridge)
    - [x] Several rules at once, each on its own port
    - [x] Remote entry: bind the port on another machine over SSH (bridge bind)
    - [x] TUN entry: capture this machine's TCP traffic through a virtual network interface (macOS and Linux, see [TUN entry](#tun-entry))
    - [x] Port forwarding to a fixed target
    - [x] Proxy authentication (username/password)
- [x] Traffic statistics per rule, hop, proxy URL and target
    - [x] Live bandwidth, totals, connections, latency and parent hop in the Web UI
    - [x] Prometheus exposition at `/metrics`
- [x] Support multiple proxy protocols on a port [Any Proxy](https://github.com/wzshiming/anyproxy)
- [x] Proxy protocol
    - [x] [SSH Proxy](https://github.com/wzshiming/sshproxy)
    - [x] [Http Proxy](https://github.com/wzshiming/httpproxy)
    - [x] [Socks4](https://github.com/wzshiming/socks4)
    - [x] [Socks5](https://github.com/wzshiming/socks5)
    - [x] [Shadow Socks](https://github.com/wzshiming/shadowsocks)

## TUN entry

A proxy rule can also capture this machine's traffic through a TUN virtual network interface (macOS and Linux): TCP connections that enter the interface go to their destination through the rule's exit chain.

- Enable: in the Web UI rule editor, tick TUN under Protocols; or in `~/.jumpway/config.yaml`, add a `tun` entry to the rule's `listen.protocols`, for example `protocols: [{type: http}, {type: tun}]`. Setting `protocols` replaces the default proxy protocols, so list the ones the port should still serve; with `tun` alone the rule opens no proxy port. TUN is not available with a virtual listen or a listen-through chain.
- Options (both optional): `address: [CIDR, ...]` with host bits, at most one IPv4 and one IPv6 (default `198.18.0.1/30` and `fd00:6a77:7761::1/64`; choose addresses that do not collide with your LAN), and `mtu`, 1280 to 65535 (default 1500).
- Authorization: unless JumpWay runs as root, starting the TUN asks for administrator authorization (a dialog on macOS, polkit's `pkexec` on Linux). It is used only to run `jumpway tun-helper` as root, which creates and configures the interface, hands it to JumpWay and exits; nothing is installed. So this repeats each time the TUN starts: when JumpWay starts, and when a change to the rule or to `no_proxy` restarts the rule.
- Capture: JumpWay routes all IPv4 and IPv6 destinations into the TUN except the IP and CIDR entries of `no_proxy` (`list`, the variables named in `from_env` such as `NO_PROXY`, and `from_file`) and loopback, link-local, multicast and broadcast (255.255.255.255) addresses; a family without a TUN address is not captured. Host and domain entries of `no_proxy` cannot become routes and do not apply to TUN traffic. The interface and its routes disappear when the rule stops or JumpWay exits.
- DNS: JumpWay does not change the system DNS. DNS queries over UDP to port 53 of a captured address are relayed over TCP port 53 through the rule's exit chain, so set the system DNS to a public resolver for remote resolution; a resolver inside a `no_proxy` network, such as a router on the LAN, is still queried directly.
- Other UDP: only DNS is relayed. Other UDP to a captured address is refused with ICMP port unreachable; applications typically fall back to TCP. Ping to a captured address gets no reply.
- Status and failures: while the TUN runs, the rule's card in the Web UI overview shows the interface name (`utunN` on macOS, `jumpwayN` on Linux). A denied prompt or any TUN error stops the whole rule, its proxy port included, and the rule's status shows why ("TUN denied" or "TUN unavailable"); reload the configuration (the tray's Reload Config) to try again. IPv6 setup failures next to an IPv4 address are only logged as warnings.
- Limits:
    - Only one enabled rule can use TUN.
    - Windows is not supported yet.
    - While a TUN rule exists, JumpWay's own outgoing connections bypass the TUN by binding to the interface that holds the default route; on Linux this needs kernel 5.7 or later unless JumpWay runs as root.
    - Linux needs iproute2 (`ip`) and polkit (`/usr/bin/pkexec`).
    - Proxy rules with an exit chain that connect directly because of the `NO_PROXY`/`ONLY_PROXY` environment variables of the JumpWay process do not bind those connections, so they may enter the TUN.

## License

Licensed under the MIT License. See [LICENSE](https://github.com/wzshiming/jumpway/blob/master/LICENSE) for the full license text.
