# 3Xpatcher — 3x-ui Integrated Dual-Core Patch

3Xpatcher 在保留 **3x-ui 原生 Inbounds / Clients / Subscription** 体验的前提下，为官方 3x-ui 增加独立 sing-box supplemental core。

当前版本：`0.6.0-integrated-alpha`

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

## TLS certificate modes

TUIC / AnyTLS / Naive 支持两种证书方式：

### Native 3x-ui TLS certificate

继续复用 3x-ui 原生 TLS 编辑器：证书文件、内联证书、SNI、ALPN、TLS 版本、cipher suites、curve preferences。

### Generated self-signed SNI certificate

协议表单选择：

```text
Certificate Mode -> Generated self-signed SNI certificate
Camouflage SNI   -> www.microsoft.com
```

3Xpatcher 使用 Go `crypto/x509` 自动生成 ECDSA P-256 自签证书，SAN/CN 为填写的 SNI，并保存到：

```text
/usr/local/x-ui-singbox/certs/<sni-hash>/cert.pem
/usr/local/x-ui-singbox/certs/<sni-hash>/key.pem
```

这不是 REALITY，也不会获得第三方域名的公有 CA 信任。raw subscription 会自动加入 `insecure=1`，Mihomo 会自动加入 `skip-cert-verify: true`，从而使生成的节点与自签证书配置保持一致。

如果需要公有 CA 信任证书，必须使用自己可控制的域名和 ACME/CA；不能为不受控制的第三方域名合法获取受信任证书。

## sing-box 1.14 option coverage

V3 对照 sing-box v1.14.0 和 S-UI 补充：

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
- QUIC: idle_timeout / keep_alive_period / stream_receive_window / connection_receive_window / max_concurrent_streams / initial_packet_size / disable_path_mtu_discovery

### AnyTLS

- users
- padding_scheme
- TLS
- common Listen options

### ShadowTLS v3

- users
- handshake server/port
- handshake_for_server_name (JSON; values may include sing-box Dial Fields)
- strict_mode
- wildcard_sni: off / authed / all
- automatically managed hidden Shadowsocks carrier inbound

### Naive

- network
- users
- quic_congestion_control: bbr / cubic / reno
- TLS
- common Listen options

`bbr2` is intentionally not exposed for Naive **inbound**: sing-box v1.14.0 source only includes `bbr2` in Naive outbound, while inbound enum is `bbr,cubic,reno`.

Not exposed as ordinary per-inbound toggles: arbitrary `detour` (ShadowTLS detour is internally managed), mTLS client-auth, ECH server keys, kernel TLS, and top-level certificate providers. These require additional lifecycle/trust semantics and are not necessary for the current integrated client/subscription model.

## Subscription

原生 `/sub/:subId` 会同时包含 Xray 与 supplemental inbounds：

- TUIC raw link
- AnyTLS raw link
- ShadowTLS v3 raw link
- Naive `naive+https` raw link
- Mihomo: TUIC / AnyTLS / ShadowTLS

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
