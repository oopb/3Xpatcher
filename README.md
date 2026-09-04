# 3Xpatcher — 3x-ui Integrated Multi-Core Patch

3Xpatcher 在尽量保持 **3x-ui 原生 Inbounds / Clients / Subscription / Traffic / Online** 工作流不变的前提下，为官方 3x-ui 增加彼此隔离的 supplemental cores。

当前版本：`0.11.1-integrated-alpha`

兼容上游：`3x-ui v3.7.0`

固定运行时：

- sing-box `v1.14.0`（CI 使用 `with_v2ray_api` 构建）
- Mieru / mita `v3.36.0`

## 协议与核心

新增协议直接出现在 **Inbounds → Add Inbound → Protocol**：

| Protocol | Runtime | Multi-user | Native traffic / online |
| --- | --- | ---: | ---: |
| TUIC | sing-box | Yes | Yes |
| AnyTLS | sing-box | Yes | Yes |
| ShadowTLS v3 | sing-box | Yes | Yes |
| Naive | sing-box | Yes | Yes |
| Mieru | official `mita` | Yes | Yes |

运行时彼此隔离：

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

Xray 全量配置会过滤 supplemental protocols；3Xpatcher 不替换 3x-ui 自带的 Xray binary，安装前后还会校验 Xray SHA256。

## V11：原生操作完整度

V11 补齐了 V10 中遗漏的客户端计数与 UI capability 链路。TUIC / AnyTLS / ShadowTLS / Naive / Mieru 现在进入 3x-ui 原生 `clientCount` rollup，因此会按原生多用户 Inbound 规则显示并执行：

- Attach existing clients
- Attach clients from another inbound
- Detach clients
- Add clients to group
- Delete all clients
- Client edit / bulk add
- Enable / Disable
- Expiry
- Traffic limit
- Reset traffic
- Clone inbound
- Export inbound JSON
- Export client subscription URLs

所有 supplemental inbound 复用 3x-ui 原生 Client / ClientInbound / `client_traffics` 数据模型，不维护第二套用户数据库。

| Protocol | Runtime credential | 3x-ui Client fields |
| --- | --- | --- |
| TUIC | UUID / password | UUID / Password |
| AnyTLS | name / password | Email / Password |
| ShadowTLS v3 | name / password | Email / Password |
| Naive | username / password | Email / Password |
| Mieru | username / password | Email / Password |

同一个 Client 可以同时附加到 VLESS、TUIC、AnyTLS、ShadowTLS、Naive、Mieru 等不同 Inbound，并继续使用同一个 `subId`。

> Remote Nodes 的“Deploy To”没有强行开放给 supplemental protocols。远端节点是否安装 sing-box / mita 属于独立部署能力；在没有远端 runtime provisioning 前伪装为可部署会得到不可运行的配置。本地 3x-ui 的原生操作不受此限制。

## V11.1：TUIC / AnyTLS / TLS runtime 安装修复

V10 的 `x-ui-singbox.service` 使用了 `ProtectHome=true`。当 3x-ui TLS 证书位于常见的 `/root/...` 路径时：

- 面板进程执行 `sing-box check` 可以读取证书；
- systemd 启动的 sing-box 却看不到 `/root`；
- 因而 TUIC / AnyTLS / Naive 等 TLS inbound 会在真正启动时失败。

V11 最初改成了 `ProtectHome=read-only`。这能让普通 systemd 主机读取 `/root`，但部分受限 LXC/NAT VPS 对 `ProtectHome` 所需的 mount namespace 本身不兼容。因此 V11.1 明确使用：

```ini
ProtectHome=false
```

即不隐藏、也不重新挂载 `/root`。`x-ui-singbox` 仍保持独立 systemd unit 与 `NoNewPrivileges=true`，但不会因为 home namespace 阻断 3x-ui 已有 TLS 证书。

V11.1 同时修复安装器事务边界：`Type=simple` 的 `systemctl restart` 可能先返回成功、随后 sing-box 在启动后立即退出。安装器现在要求服务通过连续启动稳定性检查；若失败，会自动打印 `systemctl status` 与最近 100 行 journal，并恢复之前的 sing-box binary 与 systemd unit，而不是静默留下一个 dead service。

## Native Traffic / Online

sing-box 使用仅监听 `127.0.0.1` 的 V2Ray-compatible stats API；Mieru 使用官方 `mita get metrics`。独立 collector 采集增量后写回 3x-ui 原生 `InboundService.AddTraffic`。

因此 supplemental protocols 会进入：

- Inbound 上传 / 下载
- Client 上传 / 下载
- 流量限额与自动禁用
- `/client/onlines`
- per-guid online map
- active inbound map
- Dashboard Online
- Client Last Online

Online cache 使用 20 秒 grace window，以覆盖采样间隔和持续连接暂时无字节增量的情况。

## Subscription / QR / Export

V11 将 supplemental protocols 同时接入两套 3x-ui 链路：

1. Go 后端 subscription provider；
2. 浏览器侧原生 `inbound-link.ts`，供 Inbound Export / Client Info / QR 使用。

因此不会再出现“订阅里有节点，但面板二维码/导出为空”或反过来的情况。

### Raw subscription

原生 `/sub/:subId` 可以输出：

- TUIC `tuic://`
- AnyTLS `anytls://`
- ShadowTLS v3（见下方兼容格式）
- Naive `naive+https://`
- Mieru 官方 `mierus://`

### ShadowTLS v3

V10 的自定义 `shadowtls://` 已移除。V11 按真实协议栈导出为 **Shadowsocks + ShadowTLS v3**，并提供两种兼容表示：

1. SIP002/SIP003：`ss://...?...plugin=shadow-tls...`
2. Shadowrocket 兼容的 `ss://` + `shadow-tls=<descriptor>`

Mihomo / Clash Verge 的专用 YAML 使用：

```yaml
- type: ss
  cipher: 2022-blake3-aes-128-gcm
  password: <inner-password>
  plugin: shadow-tls
  client-fingerprint: chrome
  plugin-opts:
    version: 3
    host: <handshake-host>
    password: <shadowtls-user-password>
```

### Clash / Mihomo 专用订阅

3x-ui 的 dedicated Clash subscription URI 现在不仅出现在 Clients 页面，也贯通到：

- Inbound subscription export
- Inbound QR
- Inbound client info

Clash Verge 推荐使用该专用 Clash / Mihomo URL，而不是依赖普通 `/sub/` 的 User-Agent 自动识别。

### Naive 客户端边界

服务器端 Naive 与 `naive+https://` raw link 均保留。

当前 Mihomo / Clash Verge 没有 Naive proxy type，因此 3Xpatcher **不会**把 Naive 伪造成 HTTP/HTTPS/SS 节点；那样只能“显示”，协议握手仍然错误。Shadowrocket 同样没有可直接表达 sing-box Naive inbound 的通用节点类型。

### Mieru raw

使用官方 human-readable simple sharing link：

```text
mierus://username:password@server?profile=...&mtu=1400&multiplexing=...&handshake-mode=...&port=5000&protocol=TCP
```

多个 server bindings 会重复输出 `port` / `protocol`：

```text
...&port=5000-5010&protocol=TCP&port=6000&protocol=UDP
```

Mihomo 一个 Mieru proxy 只能表达一个单端口或一个范围，因此多 binding Inbound 会展开为多个 Mihomo proxy。

### Xray JSON

3x-ui 的 Xray JSON subscription (`/json`) 无法表达 supplemental-only protocols，因此会跳过这些协议，而不是生成不可用的伪 Xray 配置。

## TLS / Reality

TUIC / AnyTLS / Naive 复用原生 **Security → TLS**：

- SNI
- certificate file / inline certificate
- panel default certificate
- ALPN
- TLS min/max version
- cipher suites
- curve preferences

还支持从 SNI 生成 ECDSA P-256 自签证书：

```text
/usr/local/x-ui-singbox/certs/<sni-hash>/cert.pem
/usr/local/x-ui-singbox/certs/<sni-hash>/key.pem
```

这不是 REALITY；raw subscription 会加入 `insecure=1`，Mihomo 会加入 `skip-cert-verify: true`。

Reality 支持矩阵：

| Protocol | Reality |
| --- | --- |
| AnyTLS | Yes |
| TUIC | No |
| ShadowTLS v3 | No |
| Naive | No |
| Mieru | No |

## Shadowsocks 2022 / Generate

- 原生 SS2022 server/client key 自动生成
- 空值、错误长度、非法 Base64 保存前自动 healing
- 旧数据库坏 key 自动修复并持久化
- ShadowTLS hidden Shadowsocks carrier 使用同一套 16/32-byte key 规则
- Generate 使用正确的 RHF 绑定结构
- Generate button 为 `htmlType="button"`
- method 改变后自动生成匹配长度的新 key

## sing-box option coverage

Common Listen：

- `bind_interface`
- `routing_mark`
- `reuse_addr`
- `netns`
- `tcp_fast_open`
- `tcp_multi_path`
- `disable_tcp_keep_alive`
- `tcp_keep_alive`
- `tcp_keep_alive_interval`
- `udp_timeout`

TUIC 还支持 QUIC advanced options、auth timeout、0-RTT、heartbeat、congestion control；AnyTLS 支持 padding scheme；ShadowTLS v3 支持 handshake routing、strict mode、wildcard SNI 和 hidden Shadowsocks carrier；Naive 支持 network 与 QUIC congestion control。

## Mieru v3.36

每个启用的 Mieru Inbound 使用独立 official mita 实例：

```text
/usr/local/x-ui-mieru/config/<inbound-id>.json
x-ui-mieru@<inbound-id>.service
/run/x-ui-mieru/<inbound-id>.sock
/var/lib/x-ui-mieru/<inbound-id>/metrics.pb
```

覆盖 Mieru v3.36 服务端能力：

- primary + arbitrary additional port bindings
- TCP / UDP single port / port range
- MTU / logging
- private / loopback policy
- rolling user quota
- metrics interval / mandatory hint
- all dual-stack DNS strategies
- static DNS hosts
- SOCKS5 egress proxies + optional auth
- ordered DIRECT / PROXY / REJECT rules
- full Traffic Pattern editor
- client multiplexing / handshake mode / encoded traffic-pattern

运行配置只写 Mieru 官方：

```text
SHA256(password || 0x00 || username)
```

得到的 `hashedPassword`；明文 Password 只保留在 3x-ui Client DB 中，以便生成客户端订阅。

## 安装 / 升级

已有用户直接重新执行：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/install.sh)
```

安装器会：

- 识别当前稳定 3x-ui 版本；
- 下载对应 rolling prebuilt patched panel；
- 校验 Release digest / patch version / upstream version / architecture；
- 安装或更新 sing-box；
- 安装或更新锁定的 official mita；
- 备份原 x-ui binary 与 `/etc/x-ui`；
- 校验 Xray binaries SHA256 不变；
- sing-box 新 runtime 必须通过启动稳定性检查；
- sing-box 启动失败时自动打印 status/journal 并恢复旧 runtime/unit；
- panel 激活失败时恢复原 panel，并清理本次首次引入的 runtime。

目标 VPS 不需要 Go / Node / npm。

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

没有启用的 Mieru Inbound 时，没有活动的 `x-ui-mieru@*.service` 是正常状态。

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

V11 的 source overlay / revert 列表保持对称，包含新增的 `inbound-link.ts`、Inbound Info、QR、client count/action type 等原生文件。

## 当前边界

- 历史旧版独立 `singbox_inbounds` 表不会自动迁移或 DROP，以保护回滚与数据恢复。
- Mieru 是 official `mita v3.36.0` 独立核心，不支持 TLS/Reality。
- Xray JSON subscription 无法表达 supplemental-only protocols。
- Host-level external endpoint override 是显式对外映射；不会猜测额外 Mieru port bindings 的 NAT 映射。
- Supplemental protocols 暂不自动部署到远端 3x-ui Nodes，直到远端 runtime provisioning 一并实现。
