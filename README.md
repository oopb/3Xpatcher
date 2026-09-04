# 3Xpatcher — 3x-ui Integrated Multi-Core Patch

3Xpatcher 在尽量保持 **3x-ui 原生 Inbounds / Clients / Subscription / Traffic / Online** 工作流不变的前提下，为官方 3x-ui 增加彼此隔离的 supplemental cores。

当前版本：`0.11.6-integrated-alpha`

兼容上游：`3x-ui v3.7.0`

固定运行时：

- sing-box `v1.14.0`（基于官方 `DEFAULT_BUILD_TAGS_OTHERS`，额外启用 `with_v2ray_api`）
- Mieru / mita `v3.36.0`

## 协议与核心

| Protocol | Runtime | Multi-user | Native traffic / online |
| --- | --- | ---: | ---: |
| TUIC | sing-box | Yes | Yes |
| AnyTLS | sing-box | Yes | Yes |
| ShadowTLS v3 | sing-box | Yes | Yes |
| Naive | sing-box | Yes | Yes |
| Mieru | official `mita` | Yes | Yes |

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

Xray 全量配置会过滤 supplemental protocols；3Xpatcher 不替换 3x-ui 自带的 Xray binary，安装前后会校验 Xray SHA256。

## 原生 3x-ui 集成

TUIC / AnyTLS / ShadowTLS / Naive / Mieru 进入 3x-ui 原生 Clients / ClientInbound / `client_traffics` 数据模型，不维护第二套用户数据库。因此可以继续使用：

- Attach / Detach clients
- Group / bulk add / delete all
- Enable / Disable
- Expiry / traffic limit / reset traffic
- Client subscription IDs
- Inbound / Client traffic
- Online / Last Online
- Inbound Export / Client Info / QR

Remote Node `Deploy To` 暂不对 supplemental protocols 自动开放，因为远端 runtime provisioning 尚未实现。

## V11.1 / V11.2 runtime 修复

### TLS 文件可见性

`x-ui-singbox.service` 使用：

```ini
ProtectHome=false
```

避免 3x-ui 的 TLS 证书位于 `/root/...` 时被 systemd home namespace 隐藏，同时保留独立 unit 与 `NoNewPrivileges=true`。

### stats API 端口

sing-box V2Ray-compatible stats 地址持久化到：

```text
/etc/3xpatcher/singbox-stats.addr
```

如默认端口冲突，会在 loopback `62000-62999` 范围选择空闲地址，panel renderer、collector 与 runtime 始终读取同一个地址。

## V11.4 / V11.5 / V11.6：客户端兼容修正

V11.3 对 Shadowrocket / Mihomo 的几项兼容判断是错误的。V11.4 修正 ShadowTLS/Naive；V11.5 撤销了错误的 TUIC `alpn: [h3]` 强制覆盖；V11.6 进一步以一个在 Clash Verge 中确认可用的 TUIC v5 节点为兼容基线，并用官方 Mihomo 做 TCP + UDP 双路径 E2E。

### ShadowTLS v3

sing-box 官方的协议结构是：

```text
Shadowsocks outbound
        │ detour
        ▼
ShadowTLS v3 outbound
        │
        ▼
server ShadowTLS inbound
        │ detour
        ▼
server Shadowsocks inbound
```

3Xpatcher 服务端继续按这一结构运行：公开 ShadowTLS v3 inbound detour 到隐藏 Shadowsocks inbound。

S-UI 当前没有把 ShadowTLS 放进通用 raw `LinkGenerator`，因此不存在一个可假定所有客户端都支持的“官方 ShadowTLS URI”。3Xpatcher 按目标客户端区分：

- **Shadowrocket raw `/sub`**：使用其既有 `ss://...?...shadow-tls=<base64 JSON>` descriptor 表示；
- **面板 QR / Export**：默认输出同一 Shadowrocket descriptor 表示；
- **通用非 Shadowrocket raw**：保留 SIP003 `plugin=shadow-tls` 表示，仅供明确支持该插件 URI 的客户端；
- **Mihomo / Clash Verge `/clash`**：继续输出 `type: ss` + `plugin: shadow-tls` + `plugin-opts`，不依赖 raw URI。

Shadowrocket descriptor 包含：

```json
{
  "version": "3",
  "password": "<shadowtls-user-password>",
  "host": "<handshake-host>",
  "address": "<outer-server>",
  "port": "<outer-port>"
}
```

### Naive

sing-box `v1.14.0` Naive inbound 在 TCP 模式使用 HTTP/2 CONNECT，并要求 `Padding` 与 Basic Proxy Authorization；其 TLS listener 会在需要时自动加入 `h2` ALPN。

S-UI 当前客户端分享格式包含兼容 `http2://` 表示。3Xpatcher 对 Shadowrocket raw `/sub` 使用同样的形式：

```text
http2://BASE64(username:password@server:port)?padding=1&peer=<SNI>&alpn=...&insecure=...&tfo=...
```

关键兼容参数：

- `peer`：TLS SNI
- `padding=1`
- `alpn`
- `insecure=1`（仅自签/允许不安全时）
- `tfo=0|1`

通用导出继续按 network 输出：

- TCP → `naive+https://`
- UDP → `naive+quic://`
- network 未限制 → 两种 native link

面板 QR / Export 会同时给出 Shadowrocket HTTP2 与相应 native Naive link。

Mihomo 当前没有 Naive proxy type，因此 `/clash` 不伪造 Naive 节点。

### TUIC / Clash Verge

V11.6 的 dedicated Clash TUIC 以确认可用的 Mihomo / Clash Verge TUIC v5 配置形状为基线：

```yaml
- name: <name>
  type: tuic
  server: <server>
  port: <port>
  uuid: <uuid>
  password: <password>
  sni: <server-name>
  skip-cert-verify: true   # 仅自签/允许不安全时
  congestion-controller: <cubic|bbr|new_reno>
  udp-relay-mode: native
  alpn:
    - h3
    - h2
    - http/1.1
```

具体规则：

- `udp-relay-mode: native` 显式输出，不依赖 Mihomo 版本默认值；
- TUIC 本身默认支持 UDP，因此不再附加冗余的通用 `udp: true`；
- TLS 配置有 ALPN 时，按原顺序完整保留，例如 `h3, h2, http/1.1`；
- TLS 没有 ALPN 时，不凭空生成 `h3`；
- `reduce-rtt: true` 仅在服务端启用 0-RTT 时输出；
- `heartbeat-interval`、`max-open-streams`、`disable-mtu-discovery`、`disable-sni` 等不会仅因服务端存在相似字段就被错误投影到客户端。

CI 使用官方 Mihomo v1.19.30 和与正式发布相同构建形状的 sing-box v1.14.0。测试先锁定上述 Clash 配置字段，再验证：

1. HTTP/TCP 经过 Mihomo → TUIC → sing-box 成功；
2. SOCKS5 UDP ASSOCIATE 经过 Mihomo `udp-relay-mode: native` → TUIC → sing-box → UDP echo 成功。

### AnyTLS

普通 TLS AnyTLS 可以输出 Mihomo `type: anytls`。AnyTLS + Reality 在 sing-box 服务端可运行，但当前 Mihomo 不支持该组合，所以 dedicated Clash subscription 会跳过。

## Subscription / QR / Export

推荐：

- Shadowrocket：使用普通 `/sub/:subId`
- Clash Verge / Mihomo：使用 dedicated `/clash/:subId`

原生 `/sub` 可包含 TUIC / AnyTLS / ShadowTLS / Naive / Mieru；实际 URI 会按客户端能力进行上述兼容处理。

## Mieru

每个启用的 Mieru Inbound 使用独立 official mita 实例：

```text
/usr/local/x-ui-mieru/config/<inbound-id>.json
x-ui-mieru@<inbound-id>.service
/run/x-ui-mieru/<inbound-id>.sock
/var/lib/x-ui-mieru/<inbound-id>/metrics.pb
```

支持 primary + additional port bindings、TCP/UDP、DNS、SOCKS5 egress、rules、traffic pattern、multiplexing、handshake mode 等当前集成字段。

## 安装 / 升级

已有 3x-ui 用户直接执行：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/install.sh)
```

安装器会：

- 识别当前 3x-ui stable version；
- 下载对应 rolling prebuilt panel；
- 校验 GitHub Release digest、patch version、upstream version、architecture；
- 安装/更新 sing-box 与 official mita；
- 备份当前 panel 与 `/etc/x-ui`；
- 校验 Xray binaries SHA256 不变；
- runtime / panel 启动失败时自动恢复。

目标 VPS 不需要 Go / Node / npm。

## Runtime paths

sing-box：

```text
/usr/local/x-ui-singbox/bin/sing-box
/usr/local/x-ui-singbox/config/config.json
/usr/local/x-ui-singbox/certs/
/etc/3xpatcher/singbox-stats.addr
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

## 回滚

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/rollback.sh)
```

可选清理 runtime：

```bash
PURGE_SINGBOX=1 bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/rollback.sh)
PURGE_MIERU=1 bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/rollback.sh)
PURGE_SINGBOX=1 PURGE_MIERU=1 bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/rollback.sh)
```

## 当前边界

- 历史 `singbox_inbounds` 表不会自动 DROP，以保护回滚与数据恢复。
- Xray JSON subscription 无法表达 supplemental-only protocols。
- Naive 没有 Mihomo proxy type。
- AnyTLS + Reality 没有当前 Mihomo dedicated Clash 表示。
- Supplemental protocols 暂不自动部署到远端 3x-ui Nodes。
