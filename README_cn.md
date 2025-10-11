# Jump Way

一个跨平台的 GUI 客户端

[![Build Darwin](https://github.com/wzshiming/jumpway/actions/workflows/build_darwin.yml/badge.svg)](https://github.com/wzshiming/jumpway/actions/workflows/build_darwin.yml)
[![Build Windows](https://github.com/wzshiming/jumpway/actions/workflows/build_windows.yml/badge.svg)](https://github.com/wzshiming/jumpway/actions/workflows/build_windows.yml)
[![Build Linux gtk3](https://github.com/wzshiming/jumpway/actions/workflows/build_linux_gtk3.yaml/badge.svg)](https://github.com/wzshiming/jumpway/actions/workflows/build_linux_gtk3.yaml)

- [English](https://github.com/wzshiming/jumpway/blob/master/README.md)
- [简体中文](https://github.com/wzshiming/jumpway/blob/master/README_cn.md)

## 特性

- [ ] 国际化
- [x] 系统托盘
    - [x] 开机启动
    - [x] 系统代理
    - [x] 导出终端代理命令到剪切板
        - [x] Shell
        - [x] Cmd
        - [x] PowerShell
- [x] Web 界面配置代理
    - [x] 通过 React 网页界面编辑配置
    - [x] 管理上下文和路由
    - [x] 配置代理设置
    - [ ] 配置多级代理
    - [ ] 支持从 `~/.ssh/config` 获取 SSH 代理
- [x] 多级代理 [Bridge](https://github.com/wzshiming/bridge)
- [x] 单端口支持多代理协议 [Any Proxy](https://github.com/wzshiming/anyproxy)
- [x] 代理协议
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

## Web 界面

Jump Way 包含一个基于 React 的网页配置管理界面。

当应用程序运行时，在浏览器中访问 `http://localhost:8080/config/` 即可使用。

### 功能
- 实时查看和编辑配置
- 管理上下文和路由配置
- 配置代理和 no-proxy 设置
- 添加/编辑/删除上下文和路由节点

### 构建 Web 界面

Web 界面使用 React 构建。修改后重新构建：

```bash
cd app/web/webui
npm install
npm run build
```

或使用构建脚本：

```bash
cd app/web
./build_webui.sh
```

## 构建

### MacOS

`./tools/build_darwin.sh`

### Windows

`.\tools\build_windows.bat`

### Linux gtk3

`./tools/build_linux_gtk3.sh`

## License

软件包根据 MIT License 许可。有关完整的许可证文本，请参阅[LICENSE](https://github.com/wzshiming/jumpway/blob/master/LICENSE)。  
