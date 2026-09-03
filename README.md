# 3Xpatcher — 3x-ui Integrated Dual-Core Patch

3Xpatcher 在保留 **3x-ui 原生 Inbounds / Clients / Subscription** 体验的前提下，为官方 3x-ui 增加独立 sing-box supplemental core。

当前版本：`0.8.0-integrated-alpha`

## 协议

直接加入 **Inbounds → Add Inbound → Protocol**：

- TUIC
- AnyTLS
- ShadowTLS v3
- Naive

Xray 协议仍由原 3x-ui/Xray 管理；supplemental 协议由独立 `x-ui-singbox.service` 管理。

## 原生 Client 复用

四种 supplemental inbound 被标记为 3x-ui 原生 multi-user inbound，因此每个 Inbound 行可以直接：

- Attach existing clients
- Detach clients
- Attach clients from another inbound
- Add clients to groups

不维护第二套用户。身份映射：

| Protocol | sing-box credential | 3x-ui Client |
| --- | --- | --- |
| TUIC | name / uuid / password | email / UUID / Password |
| AnyTLS | name / password | email / Password |
| ShadowTLS v3 | name / password | email / Password |
| Naive | username / password | email / Password |

同一个 Client 可以同时附加到 VLESS、TUIC、AnyTLS 等入站，并继续使用同一个 `subId`。

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

点击按钮会立即强制重新生成 ECDSA P-256 自签证书；保存节点时如果证书不存在或临近过期，也会自动生成/复用。

TLS 页面会显示 SAN/SNI、expiration time、certificate path、key path，以及当前 SNI 是否和已生成证书一致。

生成文件：

```text
/usr/local/x-ui-singbox/certs/<sni-hash>/cert.pem
/usr/local/x-ui-singbox/certs/<sni-hash>/key.pem
```

自签模式下原生证书导入/路径区域会隐藏，避免同时出现两套证书来源。

这不是 REALITY，也不会获得第三方域名的公有 CA 信任。raw subscription 会自动加入 `insecure=1`，Mihomo 会自动加入 `skip-cert-verify: true`。

### Native 3x-ui Reality reuse

AnyTLS 直接复用 3x-ui 已有的 **Security → Reality** UI 和操作：

- Target / target scanner
- SNI / serverNames
- X25519 keypair generation
- public/private key
- short IDs
- max time diff
- existing Reality target probing flow

3Xpatcher 在保存后把 3x-ui 原生 `streamSettings.realitySettings` 映射为 sing-box 1.14 `tls.reality`：

```text
3x-ui target              -> reality.handshake.server/server_port
3x-ui serverNames[0]      -> tls.server_name
3x-ui privateKey          -> reality.private_key
3x-ui shortIds            -> reality.short_id
3x-ui maxTimediff (ms)    -> reality.max_time_difference
```

当前 supplemental 协议的 Reality 支持边界：

| Protocol | Reality | 原因 |
| --- | --- | --- |
| AnyTLS | Yes | TCP + sing-box inbound/outbound TLS Reality 均可用 |
| TUIC | No | QUIC custom TLS 仅支持 ECH，不支持 Reality |
| ShadowTLS v3 | No | 协议自身不是 `InboundTLSOptions` Reality 模型 |
| Naive | No | inbound 虽复用 TLS 容器，但 sing-box 1.14 Naive outbound 明确拒绝 Reality |

因此不会为了“界面上能选”而生成实际上无法端到端连接的 TUIC/Naive Reality 节点。

### 0.6 compatibility

旧版存放在协议 settings 中的：

```text
tlsMode
camouflageSNI
selfSignedValidityDays
```

仍可被后端读取；打开旧节点编辑时，前端会把它们迁移到 `streamSettings.tlsSettings` 的新位置，并复用 TLS 原生 `serverName`。

## sing-box 1.14 option coverage

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
- QUIC: idle_timeout / keep_alive_period / stream_receive_window / connection_receive_window / max_concurrent_streams / initial_packet_size / disable_path_mtu_discovery

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
- wildcard_sni: off / authed / all
- automatically managed hidden Shadowsocks carrier inbound

### Naive

- network
- users
- quic_congestion_control: bbr / cubic / reno
- TLS
- common Listen options

`bbr2` 不暴露给 Naive inbound：sing-box v1.14.0 inbound enum 是 `bbr,cubic,reno`，`bbr2` 只存在于 outbound。

## Subscription

原生 `/sub/:subId` 会同时包含 Xray 与 supplemental inbounds：

- TUIC raw link
- AnyTLS raw link
- ShadowTLS v3 raw link
- Naive `naive+https` raw link
- Mihomo: TUIC / AnyTLS(TLS) / ShadowTLS

AnyTLS + Reality raw link 会输出 Reality 常用查询参数：

```text
security=reality
sni=...
pbk=...
sid=...
fp=...
```

Mihomo 当前明确不支持 **AnyTLS + Reality**，因此 Clash/Mihomo subscription 会跳过这一组合，而不是输出一个必定连接失败的 proxy。使用 AnyTLS + Reality 时客户端需支持 sing-box AnyTLS Reality。

自签 SNI 模式会自动输出对应的跳过证书验证参数。

3x-ui 的 Xray JSON subscription (`/json`) 无法表达 sing-box-only 协议，因此会跳过这些协议，而不是生成无效 Xray 配置。

## Core isolation

```text
3x-ui UI / DB / Client / Subscription
              │
      ┌───────┴────────┐
      ▼                ▼
 Xray protocols    supplemental protocols
      │                │
     Xray          x-ui-singbox.service
```

Xray 全量配置会过滤 TUIC/AnyTLS/ShadowTLS/Naive。修改 supplemental inbound/client 只 reconcile sing-box；配置无变化时不重启 sing-box。

## 安装 / 升级

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/install.sh)
```

安装器下载与当前官方 3x-ui 版本匹配的 GitHub Actions 预编译 panel，验证 SHA256、patch version、upstream version 和架构。目标 VPS 不需要 Go/Node/npm 编译环境。

安装 patched panel 时 `x-ui.service` 会重启一次；`/usr/local/x-ui/bin/xray-*` 安装前后进行 SHA256 校验且不会被替换。

## Runtime

```text
/usr/local/x-ui-singbox/bin/sing-box
/usr/local/x-ui-singbox/config/config.json
/usr/local/x-ui-singbox/certs/
/etc/systemd/system/x-ui-singbox.service
```

更新 sing-box：

```bash
/usr/local/share/3xpatcher/current/scripts/update-singbox.sh
```

## 回滚

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/rollback.sh)
```

同时清理 sing-box runtime：

```bash
PURGE_SINGBOX=1 bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/rollback.sh)
```

历史 `singbox_inbounds` 表和生成的证书不会自动 DROP/删除，以避免破坏回滚与数据恢复。
