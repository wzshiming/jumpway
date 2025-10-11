# Jump Way

A cross-platform proxy GUI client

[![Build Darwin](https://github.com/wzshiming/jumpway/actions/workflows/build_darwin.yml/badge.svg)](https://github.com/wzshiming/jumpway/actions/workflows/build_darwin.yml)
[![Build Windows](https://github.com/wzshiming/jumpway/actions/workflows/build_windows.yml/badge.svg)](https://github.com/wzshiming/jumpway/actions/workflows/build_windows.yml)
[![Build Linux gtk3](https://github.com/wzshiming/jumpway/actions/workflows/build_linux_gtk3.yaml/badge.svg)](https://github.com/wzshiming/jumpway/actions/workflows/build_linux_gtk3.yaml)

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
- [x] Configure the proxy with WebUI
    - [x] Edit configuration via React-based web interface
    - [x] Manage contexts and routing
    - [x] Configure proxy settings
    - [ ] Configure the multi-level proxy
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

## WebUI

Jump Way includes a React + TypeScript based web interface for configuration management. 

Access the WebUI at `http://localhost:8080/config/` when the application is running.

### Features
- View and edit configuration in real-time
- Manage contexts and routing configurations
- Configure proxy and no-proxy settings
- Add/edit/delete contexts and way nodes

### Building the WebUI

The WebUI is built using React with TypeScript. To rebuild after making changes:

```bash
cd app/web/webui
npm install
npm run build
```

Or use the build script:

```bash
cd app/web
./build_webui.sh
```

## Build

### MacOS

`./tools/build_darwin.sh`

### Windows

`.\tools\build_windows.bat`

### Linux gtk3

`./tools/build_linux_gtk3.sh`

## License

Licensed under the MIT License. See [LICENSE](https://github.com/wzshiming/jumpway/blob/master/LICENSE) for the full license text.
