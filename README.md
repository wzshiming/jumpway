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
    - [x] TUN entry: route this machine's traffic through a rule via a virtual network interface (macOS, Linux and Windows, see [TUN entry](#tun-entry))
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

Like the System Proxy, the TUN entry is chosen at runtime from the tray (macOS, Linux and Windows): this machine's traffic enters a TUN virtual network interface and TCP connections go to their destination through the selected rule's way.
The TUN implementation lives in [github.com/wzshiming/tun](https://github.com/wzshiming/tun).

- Select: the tray's TUN submenu, next to System Proxy, has an Off entry and one entry per enabled rule; rules with a remote listen (`listen.way`) or a virtual listen (`listen.virtual`) are not offered. The submenu appears on macOS, Linux and Windows. Turning the TUN entry on or off (Off to a rule or back) restarts every enabled rule, because their bypass routing changes; switching from one rule to another restarts only those two rules, and the previous rule's device closes. Like the System Proxy choice, the TUN entry is Off when JumpWay starts and is not saved to the configuration.
- Authorization: unless JumpWay runs as root (elevated on Windows), opening the TUN asks for administrator authorization (an `osascript` prompt on macOS, polkit's `pkexec` on Linux, a UAC prompt on Windows through [github.com/wzshiming/elevate](https://github.com/wzshiming/elevate)). On macOS and Linux it is used only to run `jumpway tun-helper` as root, which creates and configures the interface, hands it to JumpWay and exits; nothing is installed. On Windows the UAC prompt runs `jumpway tun-helper`, which creates a Wintun adapter and relays packets to JumpWay over a loopback connection while the TUN is open; an elevated JumpWay opens the adapter directly. So the prompt appears every time the TUN entry is opened, not only the first.
- Capture: JumpWay routes all IPv4 and IPv6 destinations into the TUN except the IP and CIDR entries of `no_proxy` (`list`, the variables named in `from_env` such as `NO_PROXY`, and `from_file`), and loopback, link-local, multicast and broadcast (255.255.255.255) addresses; JumpWay's own connections, including those to the rule's hops, escape the TUN by binding the physical interface (see Limits). Host and domain entries of `no_proxy` cannot become routes and do not apply to TUN traffic. The interface and its routes disappear when you choose Off, select another rule, or JumpWay exits.
- DNS: JumpWay does not change the system DNS. DNS queries over UDP to port 53 of a captured address are relayed over TCP port 53 through the rule's way, so set the system DNS to a public resolver for remote resolution; a resolver inside a `no_proxy` network, such as a router on the LAN, is still queried directly.
- Other UDP: only DNS is relayed. Other UDP to a captured address is refused with ICMP port unreachable; applications typically fall back to TCP. Ping to a captured address gets no reply.
- Status and failures: the Web UI shows a TUN badge on the rule currently carrying the TUN entry. If the prompt is denied or the TUN cannot be opened, the rule shows as stopped with a TUN error; nothing retries on its own until you choose Off, select the rule again in the TUN submenu, or reload the config. IPv6 setup failures next to an IPv4 address are only logged as warnings.
- Limits:
    - While the TUN entry is on, JumpWay's own outgoing connections bypass the TUN by binding to the interface that holds the default route (`IP_UNICAST_IF` on Windows); on Linux this needs kernel 5.7 or later unless JumpWay runs as root. On macOS the helper also adds an interface-scoped default route for the physical default interface when it is missing, so JumpWay's own connections can leave through it while capture routes point at the TUN; that route only steers sockets bound to the interface and remains after the TUN closes.
    - Hops that run a command on this machine (`cmd:` and `nc:` hops) connect from a child process whose sockets are not bound, so the TUN captures them like any other program's; list the addresses those commands connect to, such as that of `jump` in `cmd:ssh -W %h:%p jump`, in `no_proxy` as IPs or CIDRs.
    - `no_proxy` sources fetched over http or https (`from_file` URLs) are not bound either: a reload fetches them before it restarts any rule, so they go through the running TUN and its rule's way; keep them reachable that way or use local files.
    - Linux needs iproute2 (`ip`) and polkit (`/usr/bin/pkexec`).
    - Windows needs Windows 10 or later. The Wintun 0.14.1 DLL (by WireGuard LLC, license in `embedded/LICENSE.txt` of [github.com/wzshiming/wintun](https://github.com/wzshiming/wintun/blob/master/embedded/LICENSE.txt)) is embedded in JumpWay and loaded in memory by github.com/wzshiming/wintun, so nothing is written to disk; Windows installs the Wintun driver the first time the TUN entry is opened.

## License

Licensed under the MIT License. See [LICENSE](https://github.com/wzshiming/jumpway/blob/master/LICENSE) for the full license text.
