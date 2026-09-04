# 3Xpatcher — 3x-ui Integrated Multi-Core Patch

3Xpatcher 在保留 **3x-ui 原生 Inbounds / Clients / Subscription / Traffic / Online** 体验的前提下，为官方 3x-ui 增加彼此隔离的 supplemental cores。

当前版本：`0.10.0-integrated-alpha`

兼容上游：`3x-ui v3.7.0`

固定运行时：

- sing-box `v1.14.0`（由 CI 使用 `with_v2ray_api` 构建，用于原生流量统计接入）
- Mieru / mita `v3.36.0`

## 协议

直接加入 **Inbounds → Add Inbound → Protocol**：

- TUIC
- AnyTLS
- ShadowTLS v3
- Naive
- Mieru

运行时保持隔离：

- 原生 Xray 协议 → 3x-ui 自带 Xray
- TUIC / AnyTLS / ShadowTLS / Naive → `x-ui-singbox.service`
- Mieru → 官方 `mita`，每个 Inbound 一个 `x-ui-mieru@<inbound-id>.service`

Mieru 没有被伪装成 sing-box inbound；它始终由官方 `mita` 核心运行。

## 原生 Client / Traffic / Online 复用

所有 supplemental inbound 都复用 3x-ui 原生 Client / ClientInbound / client_traffics 数据模型，因此继续支持：

- Attach existing clients
- Detach clients
- Attach clients from another inbound
- Client groups
- Enable / Disable
- Expiry
- Traffic limit / traffic disable state
- `subId`
- 原生 subscription
- 原生 Traffic 统计
- 原生 Online / Last Online

不维护第二套用户数据库。

| Protocol | Runtime credential | 3x-ui Client |
| --- | --- | --- |
| TUIC | UUID / password | UUID / Password |
| AnyTLS | name / password | email / Password |
| ShadowTLS v3 | name / password | email / Password |
| Naive | username / password | email / Password |
| Mieru | username / password | email / Password |

同一个 Client 可以同时附加到 VLESS、TUIC、AnyTLS、Mieru 等不同 Inbound，并继续使用同一个 `subId`。

### Supplemental Traffic

sing-box 使用仅监听 `127.0.0.1` 的 V2Ray-compatible stats API；Mieru 使用官方 `mita get metrics`。两者都由独立的 5 秒 collector 采集增量，再写入 3x-ui 原生 `InboundService.AddTraffic`。

因此站内 Inbound 流量、Client 上传/下载、流量限额与自动禁用都与 Xray 使用同一套数据库逻辑。

### Supplemental Online

supplemental online 状态不再依赖 `xray.Process`。即使面板完全没有活动 Xray inbound，sing-box / Mieru 用户仍会进入：

- `/client/onlines`
- per-guid online map
- active inbound map
- Dashboard Online
- Client Last Online

在线缓存使用 20 秒 grace window，以覆盖 5 秒采样间隔和持续连接中暂时没有字节增量的情况。

## Mieru

### 独立实例模型

官方 mita 的 `users` 属于整个服务端实例，而 3x-ui Client 是按 Inbound 关联。为了避免一个 Mieru Inbound 的用户意外获得另一个 Mieru Inbound 的认证权限，3Xpatcher 不把多个 Mieru Inbound 合并到同一个 mita daemon。

每个启用的 Mieru Inbound 对应：

```text
/usr/local/x-ui-mieru/config/<inbound-id>.json
x-ui-mieru@<inbound-id>.service
/run/x-ui-mieru/<inbound-id>.sock
/var/lib/x-ui-mieru/<inbound-id>/metrics.pb
```

新增、修改、删除 Mieru Inbound 或 Client 时只 reconcile Mieru 实例，不会把 Mieru 配置写入 Xray 或 sing-box。

### Mieru v3.36 ServerConfig 全覆盖

3Xpatcher 当前覆盖官方 Mieru v3.36.0 `ServerConfig` 的全部服务端顶层能力。

#### Port Bindings

- 主 3x-ui `Port` 作为 primary binding
- TCP / UDP
- 单端口
- 端口范围
- 任意数量 Additional Port Bindings
- 同一个 Inbound 可混合多个 TCP / UDP 单端口与端口范围

所有 bindings 共用该 Inbound 关联的原生 3x-ui Clients / quotas / policy。

#### Server / User Policy

- MTU
- Logging level
- Allow private IP destination
- Allow loopback destination
- User quota: rolling days + megabytes
- Metrics logging interval
- Mandatory user hint

#### DNS

支持官方全部 DualStack 策略：

- `USE_FIRST_IP`
- `PREFER_IPv4`
- `PREFER_IPv6`
- `ONLY_IPv4`
- `ONLY_IPv6`

并支持任意数量静态 `hosts` 域名 → IPv4 / IPv6 映射。

#### Egress / Routing

支持 Mieru v3.36 当前官方 egress 能力：

- 任意数量 SOCKS5 egress proxies
- 可选 SOCKS5 username/password authentication
- 按顺序执行的 egress rules
- IP CIDR / `*`
- Domain suffix / `*`
- `DIRECT`
- `PROXY`
- `REJECT`
- 一个 PROXY rule 可引用一个或多个已定义 proxy names

规则按官方语义从上到下首条匹配生效；没有规则匹配时默认 DIRECT。

### Traffic Pattern

Mieru Traffic Pattern 提供结构化 UI：

- Enable custom Traffic Pattern
- Seed
- Unlock all implicit options
- TCP Fragment
  - Enable
  - Max sleep milliseconds
- Nonce
  - RANDOM
  - PRINTABLE
  - PRINTABLE_SUBSET
  - FIXED
  - Apply to all UDP packets
  - Min / Max length
  - Custom fixed hex prefixes
- Padding
  - Maximum middle padding length
  - Maximum end padding length
- Low Entropy
  - OFF / 32 / 40 / 48 / 56
  - 全部左右 mask rotation

如果 **Enable custom Traffic Pattern** 关闭，3Xpatcher 不会写入 `trafficPattern`，让官方 mita 使用自己的隐式/default pattern。

### Client Share Defaults

官方 simple sharing link 支持的客户端字段均可配置：

- MTU
- Multiplexing: OFF / LOW / MIDDLE / HIGH
- Handshake mode: STANDARD / NO_WAIT
- 官方 encoded `traffic-pattern`
- 多组 port / protocol bindings

### Mieru 密码落盘

3x-ui Client 数据仍需保存客户端原始 Password，因为订阅客户端连接需要它。

但写给 mita 的运行配置不会重复保存明文 Password。3Xpatcher 使用 Mieru 官方算法：

```text
SHA256(password || 0x00 || username)
```

并只写入 `hashedPassword`。

### Mieru 安装方式

安装器从 `enfein/mieru` 官方 GitHub Release 下载锁定版本：

```text
mita_<version>_<arch>.deb
```

安装过程：

1. 读取 GitHub Release asset digest；
2. 本地重新计算 SHA256 并严格比对；
3. 使用 `dpkg-deb -x` 解包；
4. 只提取官方 `mita` 二进制；
5. 不执行 `dpkg -i`，不安装上游 service/package；
6. 创建官方 mita 所需的 `mita` system user/group；
7. 使用 3Xpatcher 自己的 per-inbound systemd template。

## Shadowsocks 2022 / Generate

3Xpatcher 保留并扩展 3x-ui 原生 SS2022 密钥逻辑：

- 原生 Shadowsocks 2022 server/client key 自动生成
- 空值、错误长度、非法 Base64 会在保存前自动 healing
- 旧数据库中的坏 key 会自动修复并持久化
- ShadowTLS v3 hidden Shadowsocks carrier 使用相同 16/32-byte key 规则
- ShadowTLS `Generate` 按原生 `Form.Item → Space.Compact → noStyle FormField → Input` 结构绑定 RHF
- Generate 按钮为 `htmlType="button"`，不会误提交父表单
- method 改变后会自动生成匹配长度的新 key

## Security / TLS / Reality

### Native TLS

TUIC / AnyTLS / Naive 的证书来源统一放在原生 **Security → TLS**，继续复用 3x-ui TLS 编辑器：

- SNI
- certificate file / inline certificate
- panel default certificate
- ALPN
- TLS min/max version
- cipher suites
- curve preferences

### Generate self-signed certificate from SNI

在 **Security → TLS** 可选择：

```text
SNI              -> www.microsoft.com
Certificate Mode -> Generate self-signed certificate from SNI
Validity          -> 3650
[ Generate / Regenerate ]
```

生成 ECDSA P-256 自签证书：

```text
/usr/local/x-ui-singbox/certs/<sni-hash>/cert.pem
/usr/local/x-ui-singbox/certs/<sni-hash>/key.pem
```

这不是 REALITY，也不会获得第三方域名的公有 CA 信任。raw subscription 自动加入 `insecure=1`，Mihomo 自动加入 `skip-cert-verify: true`。

### Native 3x-ui Reality reuse

AnyTLS 复用 3x-ui 已有 **Security → Reality** UI，并映射为 sing-box Reality。

| Protocol | Reality |
| --- | --- |
| AnyTLS | Yes |
| TUIC | No |
| ShadowTLS v3 | No |
| Naive | No |
| Mieru | No |

Mieru 是独立协议/核心，不经过 TLS/Reality tab。

## sing-box option coverage

### Common Listen options

- bind_interface
- routing_mark
- reuse_addr
- netns
- tcp_fast_open
- tcp_multi_path
- disable_tcp_keep_alive
- tcp_keep_alive
- tcp_keep_alive_interval
- udp_fragment
- udp_timeout

### TUIC

- users / UUID / password
- congestion_control
- auth_timeout
- zero_rtt_handshake
- heartbeat
- TLS
- QUIC advanced options

### AnyTLS

- users
- padding_scheme
- TLS
- Reality
- common Listen options

### ShadowTLS v3

- users
- handshake server/port
- handshake_for_server_name
- strict_mode
- wildcard_sni
- automatically managed hidden Shadowsocks carrier inbound

### Naive

- network
- users
- quic_congestion_control: bbr / cubic / reno
- TLS
- common Listen options

`bbr2` 不暴露给 Naive inbound，因为 sing-box v1.14.0 inbound enum 不支持它。

## Subscription

原生 `/sub/:subId` 同时包含 Xray 与 supplemental inbounds。

sing-box supplemental：

- TUIC raw link
- AnyTLS raw link
- ShadowTLS v3 raw link
- Naive `naive+https` raw link
- Mihomo 原生对应 proxy

### Mieru raw

使用官方 human-readable simple sharing link：

```text
mierus://username:password@server?profile=...&mtu=1400&multiplexing=...&handshake-mode=...&port=5000&protocol=TCP
```

多个 server bindings 会按官方格式重复输出对应的 `port` / `protocol`：

```text
...&port=5000-5010&protocol=TCP&port=6000&protocol=UDP
```

自定义客户端 Traffic Pattern 时还会输出：

```text
traffic-pattern=...
```

### Mieru Mihomo

Mihomo 一个 Mieru proxy 只能表达一个单端口或一个范围，因此 3Xpatcher 会把一个包含多个 Mieru `portBindings` 的 Inbound 自动展开成多个 proxy 条目，每个条目共享 username/password/client defaults，但保留自己的 transport 与 port/port-range。

```yaml
- name: example [TCP 5000-5010]
  type: mieru
  server: 1.2.3.4
  port-range: 5000-5010
  transport: TCP
  udp: true
  username: user@example.com
  password: password
  multiplexing: MULTIPLEXING_LOW
  handshake-mode: HANDSHAKE_STANDARD
```

3x-ui 的 Xray JSON subscription (`/json`) 无法表达 supplemental-only 协议，因此会跳过它们，而不是生成无效 Xray 配置。

## Core isolation

```text
3x-ui UI / DB / Clients / Traffic / Subscription
                 │
        ┌────────┼───────────────┐
        ▼        ▼               ▼
   Xray protocols  sing-box protocols   Mieru
        │        │               │
       Xray   x-ui-singbox   mita per inbound
                          x-ui-mieru@<id>
```

Xray 全量配置会过滤所有 supplemental protocol。Xray binary 在安装 patched panel 前后进行 SHA256 校验，不会被 3Xpatcher 替换。

## 安装 / 升级

已有 3Xpatcher 用户无需卸载旧版，重新执行：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/install.sh)
```

安装器会：

- 识别当前稳定 3x-ui 版本；
- 下载对应 GitHub Actions 预编译 patched panel；
- 验证 GitHub Release digest / patch version / upstream version / architecture；
- 安装或更新独立 sing-box runtime；
- 安装或更新锁定的官方 Mieru mita runtime；
- 备份原 x-ui binary 与 `/etc/x-ui`；
- 校验 Xray binaries 前后 SHA256 一致；
- 失败时自动恢复原 panel，并清理本次首次引入的 supplemental runtime。

目标 VPS 不需要 Go / Node / npm 编译环境。

## Runtime paths

sing-box：

```text
/usr/local/x-ui-singbox/bin/sing-box
/usr/local/x-ui-singbox/config/config.json
/usr/local/x-ui-singbox/certs/
/etc/systemd/system/x-ui-singbox.service
```

Mieru：

```text
/usr/local/x-ui-mieru/bin/mita
/usr/local/x-ui-mieru/config/<inbound-id>.json
/etc/systemd/system/x-ui-mieru@.service
/run/x-ui-mieru/<inbound-id>.sock
/var/lib/x-ui-mieru/<inbound-id>/metrics.pb
```

安装完成但尚未创建/启用 Mieru Inbound 时，没有活动的 `x-ui-mieru@*.service` 是正常的。

## 回滚

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/rollback.sh)
```

默认回滚：恢复原始 x-ui panel binary、停止并禁用 Mieru instance services、保留 per-inbound configs，并且不自动删除 supplemental DB rows。

同时清理 sing-box：

```bash
PURGE_SINGBOX=1 bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/rollback.sh)
```

同时清理 Mieru：

```bash
PURGE_MIERU=1 bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/rollback.sh)
```

全部清理：

```bash
PURGE_SINGBOX=1 PURGE_MIERU=1 bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/rollback.sh)
```

## 当前边界

- 历史旧版独立 `singbox_inbounds` 表不会自动迁移或 DROP，以避免破坏旧安装的回滚/数据恢复。
- Mieru 是官方 `mita v3.36.0` 独立核心，不支持 TLS/Reality；这属于协议本身的模型，不是 3Xpatcher 功能缺失。
- Xray JSON subscription 无法表达 supplemental-only 协议，因此 `/json` 会跳过它们；raw / Mihomo subscription 已覆盖。
- Host-level external endpoint override 表示一个显式对外映射地址/端口；在该模式下按配置的 external endpoints 输出，不会猜测额外 server portBindings 的 NAT 映射关系。
