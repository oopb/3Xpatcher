# 3Xpatcher — 3x-ui Integrated Multi-Core Patch

3Xpatcher 在保留 **3x-ui 原生 Inbounds / Clients / Subscription** 体验的前提下，为官方 3x-ui 增加隔离的 supplemental cores。

当前版本：`0.9.0-integrated-alpha`

兼容上游：`3x-ui v3.7.0`

固定 Mieru 核心：`mita v3.36.0`

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

Mieru 没有被伪装成 sing-box inbound。3Xpatcher 使用官方 Mieru 服务端核心 `mita`。

## 原生 Client 复用

所有 supplemental inbound 都复用 3x-ui 原生 Client/ClientInbound 数据模型，因此每个 Inbound 可以继续使用：

- Attach existing clients
- Detach clients
- Attach clients from another inbound
- Client groups
- Enable / Disable
- Expiry
- Traffic disable state
- `subId`
- 原生 subscription 入口

不维护第二套用户数据库。

| Protocol | Runtime credential | 3x-ui Client |
| --- | --- | --- |
| TUIC | UUID / password | UUID / Password |
| AnyTLS | name / password | email / Password |
| ShadowTLS v3 | name / password | email / Password |
| Naive | username / password | email / Password |
| Mieru | username / password | email / Password |

同一个 Client 可以同时附加到 VLESS、TUIC、AnyTLS、Mieru 等不同 Inbound，并继续使用同一个 `subId`。

## Mieru

### 独立实例模型

官方 mita 的 server config 中 `users` 属于整个服务端实例，而 3x-ui 的 Client 语义是按 Inbound 关联。为了避免一个 Mieru Inbound 的用户意外获得另一个 Mieru Inbound 的认证权限，3Xpatcher 不把多个 Mieru Inbound 合并进同一个 mita daemon。

每个启用的 Mieru Inbound 对应：

```text
/usr/local/x-ui-mieru/config/<inbound-id>.json
x-ui-mieru@<inbound-id>.service
/run/x-ui-mieru/<inbound-id>.sock
```

新增/修改/删除 Mieru Inbound 或 Client 时，只 reconcile Mieru 实例，不会把 Mieru 配置写入 Xray 或 sing-box。

### Mieru Server 配置

当前 UI 支持 Mieru v3.36.0 的主要 server-side 配置：

- Transport: TCP / UDP
- Single port / port range
- MTU
- Logging level
- Allow private IP destination
- Allow loopback destination
- User quota: rolling days + megabytes
- Metrics logging interval
- Mandatory user hint

端口范围使用 3x-ui Inbound 的 `Port` 作为起始端口，再填写 `Port Range End`。

### Traffic Pattern

Mieru Traffic Pattern 提供明确的结构化 UI：

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
  - Mask rotation

如果 **Enable custom Traffic Pattern** 关闭，3Xpatcher 不会写入 `trafficPattern` 字段，让官方 mita 自己生成/使用其隐式默认 Traffic Pattern，而不是人为写一组看似“默认”的固定参数。

### Mieru 密码落盘

3x-ui Client 数据仍需要保存 Mieru 客户端的原始 Password，因为 subscription 客户端连接需要它。

但写给 mita 的运行配置不会重复保存明文 Password。3Xpatcher 使用 Mieru 官方算法生成：

```text
SHA256(password || 0x00 || username)
```

并只写入 `hashedPassword`。

### Mieru 安装方式

安装器从 `enfein/mieru` 官方 GitHub Release 下载锁定版本的：

```text
mita_<version>_<arch>.deb
```

然后：

1. 读取 GitHub Release asset digest；
2. 本地重新计算 SHA256 并严格比对；
3. 使用 `dpkg-deb -x` 解包；
4. 只提取官方 `mita` 二进制；
5. 不执行 `dpkg -i`，不安装上游 Debian service/package；
6. 创建官方 mita 运行所需要的 `mita` system user/group；
7. 使用 3Xpatcher 自己的 per-inbound systemd template。

这样既使用官方核心，又避免与系统已有服务布局发生冲突。

## Security / TLS / Reality

### Native TLS

TUIC / AnyTLS / Naive 的证书来源统一放在原生 **Security → TLS**，不在协议表单重复维护 SNI/证书字段。

继续复用 3x-ui 原生 TLS 编辑器：

- SNI (`serverName`)
- certificate file / inline certificate
- panel default certificate
- ALPN
- TLS min/max version
- cipher suites
- curve preferences

### Generate self-signed certificate from SNI

在 **Security → TLS**：

```text
SNI              -> www.microsoft.com
Certificate Mode -> Generate self-signed certificate from SNI
Validity          -> 3650
[ Generate / Regenerate ]
```

生成 ECDSA P-256 自签证书，SAN/CN 使用填写的 SNI。生成文件：

```text
/usr/local/x-ui-singbox/certs/<sni-hash>/cert.pem
/usr/local/x-ui-singbox/certs/<sni-hash>/key.pem
```

这不是 REALITY，也不会获得第三方域名的公有 CA 信任。raw subscription 会自动加入 `insecure=1`，Mihomo 会自动加入 `skip-cert-verify: true`。

### Native 3x-ui Reality reuse

AnyTLS 复用 3x-ui 已有的 **Security → Reality** UI：

- Target / target scanner
- SNI / serverNames
- X25519 keypair generation
- public/private key
- short IDs
- max time diff

3Xpatcher 将原生 `streamSettings.realitySettings` 映射到 sing-box Reality。

当前边界：

| Protocol | Reality |
| --- | --- |
| AnyTLS | Yes |
| TUIC | No |
| ShadowTLS v3 | No |
| Naive | No |
| Mieru | No |

Mieru 是独立协议/核心，不经过 3x-ui TLS/Reality tab。

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
- udp_fragment（Protocol default / Enabled / Disabled）
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
- handshake_for_server_name (JSON)
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

原生 `/sub/:subId` 会同时包含 Xray 与 supplemental inbounds。

sing-box supplemental：

- TUIC raw link
- AnyTLS raw link
- ShadowTLS v3 raw link
- Naive `naive+https` raw link
- Mihomo 原生支持的对应 proxy

Mieru raw subscription 使用官方 human-readable simple sharing link：

```text
mierus://username:password@server?profile=...&mtu=1400&multiplexing=...&handshake-mode=...&port=...&protocol=...
```

自定义客户端 Traffic Pattern 时还会输出：

```text
traffic-pattern=...
```

Mihomo 使用原生：

```yaml
- name: example
  type: mieru
  server: 1.2.3.4
  port: 5000
  # 或 port-range: 5000-5010
  transport: TCP
  udp: true
  username: user@example.com
  password: password
  multiplexing: MULTIPLEXING_LOW
```

3x-ui 的 Xray JSON subscription (`/json`) 无法表达这些 supplemental-only 协议，因此会跳过它们，而不是生成无效 Xray 配置。

## Core isolation

```text
3x-ui UI / DB / Clients / Subscription
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

已有 3Xpatcher 用户不需要卸载旧版本，直接重新执行：

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
```

安装完成但尚未创建/启用 Mieru Inbound 时，没有活动的 `x-ui-mieru@*.service` 是正常的。

## 回滚

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/rollback.sh)
```

默认回滚行为：

- 恢复原始 x-ui panel binary；
- 停止并禁用 Mieru instance services；
- 保留 Mieru per-inbound configs；
- 不自动删除 supplemental DB rows。

同时清理 sing-box runtime：

```bash
PURGE_SINGBOX=1 bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/rollback.sh)
```

同时清理 Mieru runtime 和配置：

```bash
PURGE_MIERU=1 bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/rollback.sh)
```

全部清理：

```bash
PURGE_SINGBOX=1 PURGE_MIERU=1 bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/rollback.sh)
```

## 当前边界

- Supplemental protocols 的 Client identity、Enable、Expiry、attach/detach、groups、`subId` 和 subscription 已与原生 3x-ui 统一。
- **sing-box / Mieru 的实时字节计数目前没有合并进 3x-ui 原生 Xray traffic counters。** 不应把原生 Xray 流量统计理解为所有 core 的统一计数器。
- 历史旧版独立 `singbox_inbounds` 表不会自动迁移或 DROP，以避免破坏旧安装的回滚/数据恢复。
- Mieru 是官方 `mita v3.36.0` 独立核心，不是 sing-box inbound，也不支持 TLS/Reality。
