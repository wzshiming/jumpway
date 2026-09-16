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
- [x] 按规则、跳板、代理 URL 和访问目标统计流量
    - [x] 网页配置实时显示带宽、累计流量、连接数、延迟与上一级
    - [x] Prometheus 指标 `/metrics`
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

- 每条规则是页面顶部的一个标签页，`+` 新建。一条规则由**入口**（`listen`）和
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

从网页配置保存或选择托盘菜单的 `重载配置` 时，只会重启定义发生变化的规则（代理规则
还会在 `no_proxy` 列表变化时重启），其他规则保留监听器和现有连接。

每次保存都会校验、写入文件并重载规则；端口绑定失败的规则会按退避持续重试，页面和
托盘都会显示其状态。同样的操作也提供 REST API（`/apis/configs/`，文档见
`/swaggerui/`）。旧版本的配置（`contexts`、`proxy`）不会自动迁移：请在网页配置中
重新创建规则。

## 统计

三个标签页展示实时数据（最近 1 秒带宽及其峰值、累计流量及最后一次上传/下载时间、当前与累计连接数、拨号延迟与失败次数）。
`统计` 每条规则一行，可展开为按流量方向排列的完整链路——客户端、入口（或绑定远端入口的跳板）、
本机、每个出口跳板及其 URL 与上一级、目标，每一环节都带同样的数据；连接数可点击跳转到该规则的
连接列表。`主机` 把所有规则的所有跳板 URL 按主机聚合，多条规则共用的跳板机只显示一行，可展开
查看各端点及使用它们的规则。`连接` 列出每一条当前连接：客户端 IP、规则（悬停可查看实际经过的
跳板）、目标、流量、速率与持续时间，支持排序与筛选，并可点击 `断开` 关闭它。这些页面上的规则名会跳转到
`统计` 中该规则所在行，编辑仍通过上方的规则标签进行。代理 URL 只显示
`scheme://host:port`。计数保存在内存中，自进程启动起累计，重载后同名规则与链路中位置不变的
URL 的计数保留，每条规则最多记录 1000 个目标。重置同时会把仍然打开的连接的流量、速率与持续时间归零重新计算，
并丢弃没有打开连接的目标。峰值是自启动或重置以来最高的 1 秒速率；跳板的峰值按其
所有 URL 合计后取峰值，而 `主机` 页只能汇总各规则的数字，显示的是各端点峰值之和（上界）。位于多路复用跳板（SSH）之后的跳板统计的是传输
连接而非客户端连接。同样的数据以 JSON 提供于 `/apis/stats`（`DELETE` 重置，
`DELETE /apis/stats/connections/{id}` 关闭一条连接），并以 Prometheus 文本格式提供于 `/metrics`
（跳板序列按 URL 区分，合计的跳板峰值只在 JSON 中提供）。

## 构建

### MacOS

`./tools/build_darwin.sh`

### Windows

`.\tools\build_windows.bat`

### Linux

`./tools/build_linux.sh`

## License

软件包根据 MIT License 许可。有关完整的许可证文本，请参阅[LICENSE](https://github.com/wzshiming/jumpway/blob/master/LICENSE)。  
