# Jump Way

A cross-platform proxy GUI client

[![Build Darwin](https://github.com/wzshiming/jumpway/actions/workflows/build_darwin.yml/badge.svg)](https://github.com/wzshiming/jumpway/actions/workflows/build_darwin.yml)
[![Build Windows](https://github.com/wzshiming/jumpway/actions/workflows/build_windows.yml/badge.svg)](https://github.com/wzshiming/jumpway/actions/workflows/build_windows.yml)
[![Build Linux](https://github.com/wzshiming/jumpway/actions/workflows/build_linux.yml/badge.svg)](https://github.com/wzshiming/jumpway/actions/workflows/build_linux.yml)

- [English](https://github.com/wzshiming/jumpway/blob/master/README.md)
- [简体中文](https://github.com/wzshiming/jumpway/blob/master/README_cn.md)

## Feature

- [ ] I18n
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
        - AEAD
            - [x] aes-128-gcm
            - [x] aes-256-gcm
            - [x] chacha20-ietf-poly1305
        - Stream
            - [x] aes-128-cfb
            - [x] aes-192-cfb
            - [x] aes-256-cfb
            - [x] aes-128-ctr
            - [x] aes-192-ctr
            - [x] aes-256-ctr
            - [x] des-cfb
            - [x] bf-cfb
            - [x] cast5-cfb
            - [x] rc4-md5
            - [x] chacha20
            - [x] chacha20-ietf
            - [x] salsa20

## Configuration

The configuration lives in `~/.jumpway/config.yaml`. Open the tray menu
`Config` → `Web UI` (or browse to `http://127.0.0.1:1088/`) to manage it in the
browser. The sidebar (a drawer on narrow screens) shows the runtime state,
switches between the pages and holds the English/中文 and system/light/dark
theme switches. On desktop it can collapse into an icon rail and remembers
that choice; language and theme remain available from Preferences.

- `Overview`: how many rules are running, the active connections, current rates
  and total traffic, and one card per rule with its state, address, target,
  exit chain and rates. The card's `Enabled` switch stops or restarts that rule
  alone (it writes `disabled` to the file and applies it). Pencil and chart icons
  open the rule editor and its traffic statistics; the trash icon deletes the
  rule after confirmation. `New rule` and the rule names open the editor. A
  rule is an **entry** (`listen`) and an **exit** (`forward`):
    - `listen`: the host and port clients connect to, with optional proxy
      credentials. Leave `Listen through` empty to open the port on this
      machine; otherwise the first hop binds the port on its side (an `ssh://`
      hop uses SSH remote forwarding, and the remote `sshd` binds loopback
      unless `GatewayPorts` is enabled) and the last hop is dialed from this
      machine.
    - `forward`: in `Proxy` mode clients pick their own destination
      (HTTP, SOCKS4, SOCKS5 and SSH are served on the entry); in `Port forward`
      mode every connection is piped to the target host and port, reached from
      the exit node (an empty host means `127.0.0.1` on the exit node). The
      `Exit chain` is dialed from this machine: hop 1 is the exit node, the last
      hop is the first one dialed; leave it empty to connect directly.
    - Hops are proxy URLs, one or more per hop for load balancing; `Build…`
      assembles one from the protocol, host, port and credentials. The chain is
      drawn above the form from the clients to the target.
    - Either side may instead name an in-process channel with `virtual`
      (shown as `virtual://<channel>`, exclusive with host, port and the chain on
      that side): a rule whose `forward.virtual` matches another enabled rule's
      `listen.virtual` hands its connections to that rule without a socket.
- `Global Settings`: the Web UI address (this page and the REST API) and the hosts,
  environment variables and files that bypass proxy rules, each saved on its own
- `Configuration File`: edit the YAML file directly

Rule editing, Global Settings and Configuration File use the same `Save & Apply`
control and unsaved-change indicator. The two Global Settings forms still save
independently. In the rule editor, `Cancel` returns to Overview without saving;
unsaved changes require confirmation before leaving. Current page URLs are used
directly; retired page aliases are not maintained.

```yaml
web_ui:
  host: 127.0.0.1
  port: 1088
rules:
  - name: default            # HTTP/SOCKS proxy on this machine, leaving through a SOCKS5 server
    listen:
      host: 127.0.0.1
      port: 1087
    forward:
      way:
        - lb:
            - socks5://exit.example:1080
  - name: office             # proxy entry bound on a VPS over SSH, with credentials
    listen:
      port: 1080
      username: alice
      password: secret
      way:
        - lb:
            - ssh://alice@vps.example:22?identity_file=~/.ssh/id_ed25519
  - name: db                 # port forward to a database behind a bastion
    listen:
      port: 15432
    forward:
      host: 10.0.0.5
      port: 5432
      way:
        - lb:
            - ssh://alice@bastion.example:22
no_proxy:
  list: [127.0.0.1, localhost, 10.0.0.0/8]
  from_env: [NO_PROXY, no_proxy]
```

Saving from the Web UI or selecting `Reload Config` restarts only rules whose
definition changed (or whose `no_proxy` lists changed, for proxy rules); other
rules keep their listeners and connections.

Every save validates the change, writes the file and reloads the rules; a rule
whose port cannot be bound keeps retrying with backoff and the page and tray
show its state. The same operations are available as a REST API under
`/apis/configs/` (see `/swaggerui/`). Configurations written for earlier
releases (`contexts`, `proxy`) are not migrated: recreate the rules in the
Web UI.

## Statistics

Three pages show the live numbers (bandwidth over the last second and its peak,
total bytes and when the last byte went up or down, current and cumulative
connections, dial latency and failures); the overview shows the per-rule rates
as well. All three use the same list layout with aligned metric columns. Each
entry shows its full statistics without expansion; circled question marks
explain the concepts on hover, keyboard focus or click. `Rule Traffic` lists
one entry per rule with its state and address. Expansion adds the whole chain
in traffic order: clients, the entry (or the hops that bind a
remote entry), this machine, every exit hop with its URLs and the hop it is
reached through, and the targets — each stage with the same numbers; the
connection count links to the connections of that rule. `Proxy Hosts` aggregates
every hop URL of every rule by host, so a jump server shared by several rules
is one entry with its aggregate statistics visible, expandable into its endpoints.
An entry with one endpoint does not repeat the same statistics in its expansion;
multiple endpoints retain their individual breakdowns.
`Live Connections` lists every current connection with its client IP, rule (the hops
it actually went through are shown on hover), target, current and peak rates,
total bytes, last-transfer times and duration, sortable and filterable. The
full client address and port are visible. Expansion adds only the start time
and path, without repeating the statistics; the `Disconnect` icon closes the
connection. For loopback clients of rules that listen locally, the client
also shows the local process that opened the connection (name and PID),
resolved through `lsof` on macOS, `/proc` on Linux and the IP Helper API on
Windows, limited to processes jumpway is allowed to inspect. Rule names on
these pages open the rule's entry in `Rule Traffic`; editing starts from
`Overview`. Proxy URLs are shown as `scheme://host:port`.
The counters live in memory since the process started, survive reloads for
unchanged rule names and for URLs that keep their position in a chain, and keep
at most 1000 targets per rule. A reset also restarts the bytes, rates and
duration of connections still open and drops the targets that no longer have
an open connection. Aggregate active/total counts restart at zero; surviving
connections stay in `Live Connections` but are not included in those counts.
Peaks are the highest one-second rate since start
or reset; a hop's peak is the peak of its URLs combined, while `Proxy Hosts`, which
only sums per-rule numbers, shows the sum of its endpoints' peaks as an upper
bound. Hops behind a connection-multiplexing hop (SSH) count transports rather
than client connections. The same data is served as JSON at `/apis/stats`
(`DELETE` resets it, `DELETE /apis/stats/connections/{id}` closes one
connection) and in Prometheus text format at `/metrics` (hop series are per URL,
so the combined hop peak exists only in the JSON).

## Build

### MacOS

`./tools/build_darwin.sh`

### Windows

`.\tools\build_windows.bat`

### Linux

`./tools/build_linux.sh`

### Web UI

The browser UI is a Svelte app in `app/web/ui`; its build output in
`app/web/statics` is committed and embedded into the binary, so the Go builds
above need no Node. Changing the UI needs Node (the versions listed under
`engines` in `app/web/ui/package.json`; `.nvmrc` picks 24) and pnpm 12.4.2:

- `make -C app/web ui`: frozen install and production build into
  `app/web/statics`; commit the result together with the source change
- `make -C app/web ui-dev`: dev server with hot reload; `/apis`, `/metrics`,
  `/swaggerui` and `/debug` are proxied to a running jumpway
  (`http://127.0.0.1:1088`, or `JUMPWAY_URL`)
- `make -C app/web ui-check`: type checks, formatting and unit tests
- `pnpm --dir app/web/ui test:e2e`: browser tests against a mocked API (once:
  `pnpm --dir app/web/ui exec playwright install chromium`; `PW_CHANNEL=chrome`
  uses an installed Google Chrome instead)

`pnpm --dir app/web/ui test:e2e:real` runs a read-only check against the
jumpway at `JUMPWAY_URL`; the tests that create rules, rewrite the YAML file and
open connections stay skipped unless opted in with `JUMPWAY_E2E_WRITE=1` (plus
`JUMPWAY_E2E_ISOLATED=1` for the YAML file and `JUMPWAY_TARGET_URL` for
traffic), and are meant for a disposable instance, not a personal
configuration.

## License

Licensed under the MIT License. See [LICENSE](https://github.com/wzshiming/jumpway/blob/master/LICENSE) for the full license text.
