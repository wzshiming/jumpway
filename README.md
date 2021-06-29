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
- [ ] Configure the proxy with GUI
    - [ ] Configure the multi-level proxy
    - [ ] Support to get SSH proxy configuration from `~/.ssh/config`
- [x] Multi-level proxy
- [x] Proxy protocol
    - [x] SSH
    - [x] [Http Proxy](https://github.com/wzshiming/httpproxy)
    - [x] [Socks4](https://github.com/wzshiming/socks4)
    - [x] [Socks5](https://github.com/wzshiming/socks5)
    - [x] [Shadowsocks](https://github.com/wzshiming/shadowsocks)
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

## Build

### MacOS

`./tools/build_darwin.sh`

### Windows

`.\tools\build_windows.bat`

## License

Licensed under the MIT License. See [LICENSE](https://github.com/wzshiming/jumpway/blob/master/LICENSE) for the full license text.
