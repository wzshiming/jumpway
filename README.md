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
- TCP timeouts: JumpWay uses the defaults of github.com/wzshiming/tun and has no option to change them:
    - A captured TCP connection that the rule has not connected to its destination within 30 seconds is refused: the client gets a reset.
    - Once one side of a TCP connection has finished sending, the connection is closed when the other direction has carried no data for 60 seconds; data restarts the clock.
    - TCP keepalive runs towards the client: probes start after 60 seconds of idleness and repeat every 15 seconds, and after 9 unanswered probes, about 195 seconds in all, the connection is closed.
    - A connection open in both directions is not closed for being idle while its client still answers the probes.
- UDP and DNS options: a rule's `forward` can set how the TUN treats its UDP and its DNS, and with `udp` also how its listener serves SOCKS5 UDP (see SOCKS5 UDP on a rule's listener below). Every field is optional, an absent one keeps the behaviour the next three items describe, and changing one restarts the rule, keeping the TUN on it if it was.
    ```yaml
    rules:
      - name: default
        listen: {port: 1087}
        forward:
          way: [{lb: [socks5://proxy:1080]}]
          udp:
            mode: proxy         # proxy (default), direct or block
            nat: symmetric      # full-cone or symmetric; absent chooses as described below
            idle: 60            # seconds, 60 when absent or 0
          dns:
            mode: tcp           # tcp (default) or udp
            server: 1.1.1.1:53  # tcp mode only
            idle: 10            # seconds, 10 when absent or 0; tcp mode only
    ```
- DNS: JumpWay does not change the system DNS. With `dns.mode` absent or `tcp`, DNS queries over UDP to port 53 of a captured address are relayed as DNS over TCP to port 53 of the same address through the rule's way (directly for a rule without one), which therefore needs only TCP, whatever `udp.mode` says; with `dns.server` set, every such TCP connection goes through the way to that host:port instead, and the replies still come back from the address queried. A DNS session ends after `dns.idle` seconds (10 by default) without queries or replies. So set the system DNS to a public resolver for remote resolution, or set `dns.server`; a resolver inside a `no_proxy` network, such as a router on the LAN, is still queried directly. With `dns.mode: udp`, port 53 gets no special handling and is treated as Other UDP below, so `udp.mode: block` drops it and `udp.mode: direct` sends it from this machine.
- Other UDP: with `udp.mode` absent or `proxy`, a rule with a way sends each session (a client address and port) of other UDP to a captured address through the way, by default as one association relayed full cone, which sends its datagrams to every destination and returns replies from any source, when the way's last hop offers UDP that can address every destination, today a SOCKS5 last hop's UDP ASSOCIATE (`socks5://` or `socks5h://`). Its control connection goes through the earlier hops like TCP, while the datagrams go straight from this machine to the proxy's UDP relay, even when the SOCKS5 hop sits behind other hops; the socket to that relay escapes the TUN the way JumpWay's other connections do, bound to the physical interface when the relay is an address the TUN captures. A last hop that cannot carry UDP, such as an HTTP proxy, drops the datagrams without an ICMP error, so applications fall back to TCP only after their own timeout; a rule forwarding to a fixed or virtual target refuses UDP, dropping the datagrams the same way. A rule with a way that sends some destinations directly, through the `NO_PROXY` or `ONLY_PROXY` environment variables or a `no_proxy` entry of `*` or IP:port, instead dials each destination separately by default so that each follows those lists like TCP. A rule without a way sends each session directly over the physical interface from one socket, full cone by default, and those lists do not change that. `udp.mode: direct` sends a rule's UDP the same way from this machine whatever its way or target, so the way's proxies receive none of it, and `udp.mode: block` drops it without opening a socket, all but the DNS queries that `dns.mode` relays over TCP (see DNS). `udp.nat: full-cone` keeps one association or socket for the session without consulting those lists for each destination: only the session's first destination follows them, and when the connection it gets is tied to that single destination, as a direct one is, the session still uses one connection per destination; `udp.nat: symmetric` uses one per destination: through a SOCKS5 way an association each, and from this machine a connected socket each that escapes the TUN like JumpWay's direct TCP connections. A session ends after `udp.idle` seconds (60 by default) without datagrams, and a destination's own association or socket that has gone as long without datagrams to or from it is closed at the session's next datagram. Ping to a captured address gets no reply.
- SOCKS5 UDP on a rule's listener: the `udp` options also apply to the UDP ASSOCIATE requests a proxy rule's SOCKS5 listener serves, whether or not a TUN is selected. With `udp.mode` absent nothing changes: the listener's SOCKS5 server relays each association from its own plain socket, not through the rule's way, and while a TUN is selected that socket's datagrams to captured addresses enter the TUN like any other program's. With `udp.mode: proxy` the association's datagrams go through the rule's way as Other UDP describes (from this machine for a rule without a way); with `direct` they leave from this machine without the way, escaping a selected TUN like the rule's other direct connections; with `block` the ASSOCIATE request is refused. `udp.nat` chooses between one association or socket for all of an association's destinations and one per destination as above, and with `udp.idle` a destination's own association or socket that has gone as long without datagrams is closed at the association's next datagram; the association itself lasts as long as the client's control connection.
- Status and failures: the Web UI shows a TUN badge on the rule currently carrying the TUN entry. If the prompt is denied or the TUN cannot be opened, the rule shows as stopped with a TUN error; nothing retries on its own until you choose Off, select the rule again in the TUN submenu, or reload the config. IPv6 setup failures next to an IPv4 address are only logged as warnings.
- Limits:
    - While the TUN entry is on, JumpWay's own outgoing connections bypass the TUN by binding to the interface that holds the default route (`IP_UNICAST_IF` on Windows); on Linux this needs kernel 5.7 or later unless JumpWay runs as root. On macOS the helper also adds an interface-scoped default route for the physical default interface when it is missing, so JumpWay's own connections can leave through it while capture routes point at the TUN; that route only steers sockets bound to the interface and remains after the TUN closes.
    - Hops that run a command on this machine (`cmd:` and `nc:` hops) connect from a child process whose sockets are not bound, so the TUN captures them like any other program's; list the addresses those commands connect to, such as that of `jump` in `cmd:ssh -W %h:%p jump`, in `no_proxy` as IPs or CIDRs.
    - `no_proxy` sources fetched over http or https (`from_file` URLs) are not bound either: a reload fetches them before it restarts any rule, so they go through the running TUN and its rule's way; keep them reachable that way or use local files.
    - Linux needs iproute2 (`ip`) and polkit (`/usr/bin/pkexec`).
    - Windows needs Windows 10 or later. The Wintun 0.14.1 DLL (by WireGuard LLC, license in `embedded/LICENSE.txt` of [github.com/wzshiming/wintun](https://github.com/wzshiming/wintun/blob/master/embedded/LICENSE.txt)) is embedded in JumpWay and loaded in memory by github.com/wzshiming/wintun, so nothing is written to disk; Windows installs the Wintun driver the first time the TUN entry is opened.

## License

Licensed under the MIT License. See [LICENSE](https://github.com/wzshiming/jumpway/blob/master/LICENSE) for the full license text.
