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
        - [x] Shell git（`nc`）
        - [x] Cmd
        - [x] Cmd git（Git for Windows 自带的 `ssh` 和 `connect.exe`）
        - [x] PowerShell
        - [x] PowerShell git（Git for Windows 自带的 `ssh` 和 `connect.exe`）
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
（或直接浏览器打开 `http://127.0.0.1:1088/`）在浏览器中管理。侧边栏（窄屏下为抽屉）
显示运行状态、切换页面，并提供 English/中文 与跟随系统/浅色/深色主题切换。
桌面侧边栏可折叠为图标栏并记住选择，语言与主题仍可通过“偏好设置”调整。

- `概览`：运行中的规则数（其余按重试中、已停止、已禁用分别计数）、活动连接数与
  统计起始以来的累计连接数、当前速率与累计流量，以及每条规则一张卡片
  （状态、地址、目标、出口链路与速率）。当前速率旁的折线为页面打开期间最近约一分钟的采样，
  重置统计或暂停后重新开始。卡片上的 `启用` 开关单独停止或重启该规则
  （向文件写入 `disabled` 并应用），铅笔和图表图标分别打开规则编辑器与流量统计，
  垃圾桶图标在确认后删除规则。`新建规则` 与规则名打开编辑器。一条规则由**入口**（`listen`）和
  **出口**（`forward`）组成：
    - `listen`：客户端连接的主机与端口，可选代理凭据。`protocols` 列出该端口提供的
      代理协议（`http`、`socks5`、`socks4`、`ssh`、`ss`），每项可单独设置 `username`、
      `password`，`ss` 还可设置 `cipher`；条目中留空的字段继承共享的 `username`/`password`
      （`ss` 也继承 `cipher`）。不写 `protocols` 时端口提供 HTTP、SOCKS5、SOCKS4 和 SSH，
      设置 `cipher` 时再加 Shadowsocks。Shadowsocks（仅 TCP）以 cipher 与 password 认证，
      SOCKS4 只校验用户名；仅有共享 password 不能认证 HTTP、SOCKS5 或 SSH 客户端，
      它们仍需要 `username`。`经由监听` 留空表示在本机
      开放端口；否则由第一跳在它那一侧绑定端口（`ssh://` 跳板使用 SSH 远程转发，
      远端 `sshd` 默认只绑定回环地址，除非启用 `GatewayPorts`），最后一跳由本机
      拨号。
    - `forward`：`代理` 模式下由客户端通过入口提供的任一协议自行选择目标；`端口转发` 模式下每条连接
      都被转到目标主机与端口，目标从出口节点访问（主机留空表示出口节点上的 `127.0.0.1`）。
      `出口链路` 由本机拨号：跳板 1 是出口节点，最后一跳最先被拨号；留空表示本机直连。
    - 跳板是代理 URL，每跳可填多条用于负载均衡；`拼装…` 按协议、主机、端口和凭据
      拼出 URL。表单上方按从客户端到目标的顺序画出整条链路。
    - 任一侧也可改用 `virtual` 指定进程内通道（显示为 `virtual://<通道>`，与该侧的
      主机、端口和链路互斥）：`forward.virtual` 与另一条已启用规则的 `listen.virtual`
      同名时，连接不经套接字直接交给那条规则。
- `全局设置`：网页配置的地址（本页面与 REST API）以及绕过代理规则的主机、环境变量和文件列表，两部分分别保存
- `配置文件`：直接编辑 YAML 文件

规则编辑、全局设置与配置文件统一使用 `保存并应用` 按钮和未保存提示；全局设置的两个表单仍分别保存。
规则编辑页的 `取消` 不会保存，返回概览前若有未保存的修改，会先要求确认。
页面直接使用当前地址，不再维护旧页面的兼容别名。

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
      protocols:             # 只提供这些；不写则提供 HTTP、SOCKS5、SOCKS4 和 SSH
        - type: http
        - type: socks5
          username: bob      # 继承共享的 password
        - type: ss
          cipher: aes-256-gcm
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

三个页面展示实时数据（最近 1 秒带宽及其峰值、累计流量及最后一次上传/下载时间、当前与累计连接数、拨号延迟与失败次数），概览页也显示各规则的速率，每条规则的当前速率还带有最近一分钟的折线。
三个页面采用一致的列表布局和对齐的指标列，每条记录的完整统计直接显示，无需展开；概念后的带圈问号可悬停、聚焦或点击查看说明。
`规则流量` 每条规则一项，显示状态、地址与全部统计，展开后补充按流量方向排列的完整链路——客户端、入口（或绑定远端入口的跳板）、
本机、每个出口跳板及其 URL 与上一级、目标，每一环节都带同样的数据；连接数可点击跳转到该规则的
连接列表。`跳板主机` 把所有规则的所有跳板 URL 按主机聚合，多条规则共用的跳板机只显示一项，汇总统计直接可见，展开查看各端点。
仅有一个端点时，展开区不重复汇总统计；多个端点时保留各自的分项统计。
`活动连接` 列出每一条当前连接：客户端完整地址与端口、规则（悬停可查看实际经过的跳板）、目标、当前与峰值速率、累计流量、最后传输时间和持续时间，支持排序与筛选；列表每次显示 200 条，`再显示` 按钮展开下一批。
展开只补充开始时间与路径，不重复统计，并可点击 `断开` 图标关闭连接。当本地监听规则的客户端为回环地址时，
客户端旁会标出发起连接的本地进程（名称与 PID），通过 macOS 的 `lsof`、Linux 的 `/proc`
和 Windows 的 IP Helper API 查找，仅解析 jumpway 有权查看的进程。这些页面上的规则名会跳转到
`规则流量` 中该规则所在项，编辑从 `概览` 进入。代理 URL 只显示
`scheme://host:port`。计数保存在内存中，自进程启动起累计，重载后同名规则与链路中位置不变的
URL 的计数保留，每条规则最多记录 1000 个目标。重置同时会把仍然打开的连接的流量、速率与持续时间归零重新计算，
并丢弃没有打开连接的目标。汇总的当前/累计连接数从零重新计数，重置前已打开的连接仍列在 `活动连接` 中，但不计入这些汇总计数。峰值是自启动或重置以来最高的 1 秒速率；跳板的峰值按其
所有 URL 合计后取峰值，而 `跳板主机` 页只能汇总各规则的数字，显示的是各端点峰值之和（上界）。位于多路复用跳板（SSH）之后的跳板统计的是传输
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

### 网页界面

浏览器界面是 `app/web/ui` 下的 Svelte 应用，构建产物 `app/web/statics` 已提交并嵌入二进制，
因此上面的 Go 构建不需要 Node。修改界面需要 Node（`app/web/ui/package.json` 中 `engines`
所列版本，`.nvmrc` 为 24）和 pnpm 12.4.2：

- `make -C app/web ui`：按锁文件安装并构建到 `app/web/statics`，产物与源码改动一起提交
- `make -C app/web ui-dev`：热更新开发服务器，`/apis`、`/metrics`、`/swaggerui` 与 `/debug`
  转发到正在运行的 jumpway（`http://127.0.0.1:1088`，或 `JUMPWAY_URL`）
- `make -C app/web ui-check`：类型检查、格式检查与单元测试
- `pnpm --dir app/web/ui test:e2e`：基于模拟 API 的浏览器测试（首次需
  `pnpm --dir app/web/ui exec playwright install chromium`；设置 `PW_CHANNEL=chrome`
  则使用本机已安装的 Google Chrome）

`pnpm --dir app/web/ui test:e2e:real` 对 `JUMPWAY_URL` 指向的 jumpway 做只读检查；会创建规则、
改写 YAML 文件和建立连接的测试只在显式设置 `JUMPWAY_E2E_WRITE=1`（YAML 文件还需
`JUMPWAY_E2E_ISOLATED=1`，流量测试还需 `JUMPWAY_TARGET_URL`）时运行，只应针对一次性实例，
不要用于个人配置。

## License

软件包根据 MIT License 许可。有关完整的许可证文本，请参阅[LICENSE](https://github.com/wzshiming/jumpway/blob/master/LICENSE)。  
