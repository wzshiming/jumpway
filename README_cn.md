# Jump Way

一个跨平台的 GUI 客户端

[![Build Darwin](https://github.com/wzshiming/jumpway/actions/workflows/build_darwin.yml/badge.svg)](https://github.com/wzshiming/jumpway/actions/workflows/build_darwin.yml)
[![Build Windows](https://github.com/wzshiming/jumpway/actions/workflows/build_windows.yml/badge.svg)](https://github.com/wzshiming/jumpway/actions/workflows/build_windows.yml)
[![Build Linux](https://github.com/wzshiming/jumpway/actions/workflows/build_linux.yml/badge.svg)](https://github.com/wzshiming/jumpway/actions/workflows/build_linux.yml)

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
- [x] 图形界面配置代理（网页配置，见下文）
    - [x] 配置多级代理
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

## 配置

配置文件位于 `~/.jumpway/config.yaml`。通过托盘菜单 `配置` → `网页配置`
（或直接浏览器打开 `http://127.0.0.1:1087/`，即代理自身的监听地址）在浏览器中管理：

- `当前上下文`：选择并切换生效的上下文
- `上下文管理`：每个上下文一个标签页，可重命名、编辑跳板节点与代理 URL（带 URL 拼装器）或删除；`+` 新建上下文。跳板节点 1 是最靠近目标的出口节点，最后一个节点由本机直接连接
- `监听地址`：代理和本页面监听的主机与端口
- `不走代理`：绕过代理的主机、环境变量和文件列表
- `高级 YAML`：直接编辑文件

每次保存都会校验、写入文件并重载代理；修改监听端口后页面会给出新地址的链接。
同样的操作也提供 REST API（`/apis/configs/`，文档见 `/swaggerui/`）。

## 构建

### MacOS

`./tools/build_darwin.sh`

### Windows

`.\tools\build_windows.bat`

### Linux

`./tools/build_linux.sh`

## License

软件包根据 MIT License 许可。有关完整的许可证文本，请参阅[LICENSE](https://github.com/wzshiming/jumpway/blob/master/LICENSE)。  
