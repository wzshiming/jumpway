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
        - [x] Cmd
        - [x] PowerShell
- [x] Configure the proxy with GUI (Web UI, see below)
    - [x] Configure the multi-level proxy
    - [ ] Support to get SSH proxy configuration from `~/.ssh/config`
- [x] Multi-level proxy [Bridge](https://github.com/wzshiming/bridge)
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
`Config` → `Web UI` (or browse to `http://127.0.0.1:1087/`, the proxy's own
listen address) to manage it in the browser:

- `Current`: pick the active context and switch to it
- `Contexts`: one tab per context to rename it, edit its proxy hops and URLs
  (with a URL builder), or delete it; `+` adds a context. Hop 1 is the exit
  node next to the target and the last hop is dialed from this machine
- `Listen address`: the host and port the proxy and this page listen on
- `No proxy`: hosts, environment variables and files that bypass the proxy
- `Advanced YAML`: edit the file directly

Every save validates the change, writes the file and reloads the proxy; when
the listen port changes, the page links to the new address. The same operations
are available as a REST API under `/apis/configs/` (see `/swaggerui/`).

## Build

### MacOS

`./tools/build_darwin.sh`

### Windows

`.\tools\build_windows.bat`

### Linux

`./tools/build_linux.sh`

## License

Licensed under the MIT License. See [LICENSE](https://github.com/wzshiming/jumpway/blob/master/LICENSE) for the full license text.
