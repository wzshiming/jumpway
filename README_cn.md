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
    - [x] 多条规则同时启用，各用独立端口
    - [x] 远端入口：通过 SSH 把端口绑定在其他机器上（bridge bind）
    - [x] 端口转发到固定目标
    - [x] 代理认证（用户名/密码）
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
（或直接浏览器打开 `http://127.0.0.1:1088/`）在浏览器中管理：

- `规则`：每条规则一个标签页，`+` 新建。一条规则由**入口**（`listen`）和
  **出口**（`forward`）组成：
    - `listen`：客户端连接的主机与端口，可选代理凭据。`经由监听` 留空表示在本机
      开放端口；否则由第一跳在它那一侧绑定端口（`ssh://` 跳板使用 SSH 远程转发，
      远端 `sshd` 默认只绑定回环地址，除非启用 `GatewayPorts`），最后一跳由本机
      拨号。
    - `forward`：`代理` 模式下由客户端自行选择目标（入口上同时提供 HTTP、SOCKS4、
      SOCKS5 和 SSH 代理）；`端口转发` 模式下每条连接都被转到目标主机与端口，目标
      从出口节点访问（主机留空表示出口节点上的 `127.0.0.1`）。`出口链路` 由本机
      拨号：跳板 1 是出口节点，最后一跳最先被拨号；留空表示本机直连。
- `设置`：网页配置的地址（本页面与 REST API）以及绕过代理规则的主机、环境变量和文件列表
- `高级 YAML`：直接编辑文件

```yaml
web_ui:
  host: 127.0.0.1
  port: 1088
rules:
  - name: default            # 本机上的 HTTP/SOCKS 代理，经 SOCKS5 服务器出去
    listen:
      host: 127.0.0.1
      port: 1087
    forward:
      way:
        - lb:
            - socks5://exit.example:1080
  - name: office             # 通过 SSH 绑定在 VPS 上的代理入口，带凭据
    listen:
      port: 1080
      username: alice
      password: secret
      way:
        - lb:
            - ssh://alice@vps.example:22?identity_file=~/.ssh/id_ed25519
  - name: db                 # 经堡垒机转发到数据库的端口转发
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

每次保存都会校验、写入文件并重载规则；端口绑定失败的规则会按退避持续重试，页面和
托盘都会显示其状态。同样的操作也提供 REST API（`/apis/configs/`，文档见
`/swaggerui/`）。旧版本的配置（`contexts`、`proxy`）不会自动迁移：请在网页配置中
重新创建规则。

## 构建

### MacOS

`./tools/build_darwin.sh`

### Windows

`.\tools\build_windows.bat`

### Linux

`./tools/build_linux.sh`

## License

软件包根据 MIT License 许可。有关完整的许可证文本，请参阅[LICENSE](https://github.com/wzshiming/jumpway/blob/master/LICENSE)。  
