# 3Xpatcher — 3x-ui Integrated Multi-Core Patch

3Xpatcher 是面向官方 **3x-ui** 的多内核集成补丁。

目标是在尽量保持 3x-ui 原生 **Inbounds / Clients / Subscription / Traffic / Online / Nodes** 工作流不变的前提下，保留官方 Xray，同时增加彼此隔离的 sing-box 与 Mieru 运行时，使一个 3x-ui 面板能够统一管理更多协议。

当前版本：`0.12.0-integrated-alpha`

当前兼容上游：`3x-ui v3.7.0`

固定补充运行时：

- sing-box `v1.14.0`，使用官方默认构建标签并额外启用 `with_v2ray_api`
- Mieru / official `mita` `v3.36.0`

> 3Xpatcher 不替换 3x-ui 自带的 Xray。Xray 与 supplemental runtimes 相互隔离。

## 架构

```text
                   3x-ui
        UI / DB / Clients / Subscription
          Traffic / Online / Nodes
                     │
        ┌────────────┼────────────┐
        │            │            │
        ▼            ▼            ▼
      Xray        sing-box      Mieru
   官方原生内核    supplemental   official mita
                    runtime       per inbound
        │            │            │
        │      x-ui-singbox   x-ui-mieru@<id>
        │
        └── 官方 Xray 升级与运行方式保持不变
```

补充协议与普通 3x-ui 入站共用原生数据库、ClientRecord、ClientInbound、流量统计和订阅体系，不维护第二套用户数据库。

## 支持协议

| 协议 | 运行时 | 用户模型 | 原生 Traffic / Online | 主要客户端导出 |
| --- | --- | --- | --- | --- |
| 3x-ui 原生 Xray 协议 | Xray | 原生 | 原生 | 原生 |
| TUIC | sing-box | 多用户 | Yes | Shadowrocket / Mihomo |
| AnyTLS | sing-box | 多用户 | Yes | Shadowrocket / Mihomo |
| ShadowTLS v3 | sing-box | 多用户 | Yes | Shadowrocket / Mihomo |
| Naive TCP / HTTP2 | sing-box | 多用户 | Yes | Shadowrocket / native Naive |
| Naive UDP / QUIC | sing-box | 多用户 | Yes | Shadowrocket HTTP3 / native Naive QUIC |
| Snell v5 | sing-box | 单活动客户端 | Yes | Shadowrocket / Mihomo |
| Mieru | official `mita` | 多用户 | Yes | Mieru compatible clients |

## 3x-ui 原生集成

Supplemental protocols 直接进入 3x-ui 原生数据模型，因此可以继续使用：

- Inbound 创建、编辑、启用、禁用
- Clients Attach / Detach
- Group / bulk add / delete
- Client enable / disable
- Expiry / traffic limit / reset traffic
- Client subscription ID
- Inbound / Client traffic
- Online / Last Online
- Inbound Export / Client Info / QR
- 原生订阅入口
- Dashboard 在线状态与流量展示

Xray 配置生成时会过滤 supplemental protocols，它们不会被错误写入 Xray JSON。

## 运行时隔离

### Xray

3Xpatcher 不替换：

```text
/usr/local/x-ui/bin/xray-*
```

安装过程中会记录并校验 Xray SHA256，补丁面板不会把 sing-box / Mieru 注入 Xray binary。

### sing-box

TUIC / AnyTLS / ShadowTLS / Naive / Snell 共用独立 sing-box sidecar：

```text
/usr/local/x-ui-singbox/bin/sing-box
/usr/local/x-ui-singbox/config/config.json
/usr/local/x-ui-singbox/certs/
/etc/3xpatcher/singbox-stats.addr
/etc/systemd/system/x-ui-singbox.service
```

stats API 仅监听 loopback，并由 panel 与 collector 使用同一持久化地址。

### Mieru

每个启用的 Mieru inbound 使用独立 official `mita` 实例：

```text
/usr/local/x-ui-mieru/bin/mita
/usr/local/x-ui-mieru/config/<inbound-id>.json
/etc/systemd/system/x-ui-mieru@.service
/run/x-ui-mieru/<inbound-id>.sock
/var/lib/x-ui-mieru/<inbound-id>/metrics.pb
```

Mieru 实例之间的配置、socket 与 metrics state 相互隔离。

## 协议说明

### TUIC

服务端由 sing-box 运行。

Dedicated Clash / Mihomo subscription 保留 TUIC UDP，并输出当前客户端兼容所需的 TLS 参数，包括：

```yaml
- name: example
  type: tuic
  server: example.com
  port: 443
  uuid: <uuid>
  password: <password>
  sni: example.com
  skip-cert-verify: true
  congestion-controller: bbr
  udp: true
```

如果服务端配置了 ALPN，则按服务端配置原样导出；不会人为覆盖不存在的 ALPN。

### AnyTLS

普通 TLS AnyTLS 可生成 Mihomo `type: anytls` 节点。

AnyTLS + Reality 可以作为 sing-box 服务端运行，但如果目标客户端本身不支持对应组合，dedicated Clash subscription 会跳过无法正确表达的节点，而不是生成伪配置。

### ShadowTLS v3

服务端结构保持 sing-box 的 ShadowTLS + Shadowsocks detour 模型：

```text
public ShadowTLS v3 inbound
          │
          ▼
hidden Shadowsocks inbound
```

客户端导出按客户端能力生成：

- Shadowrocket raw subscription / QR：Shadowrocket 可识别的 ShadowTLS descriptor
- Mihomo / Clash Verge：`type: ss` + `plugin: shadow-tls` + `plugin-opts`
- 其他 raw consumer：仅输出其能够明确表达的 URI 形式

### Naive

Naive 支持 TCP 与 UDP 两种服务端 network：

```text
TCP  -> HTTP/2 CONNECT
UDP  -> QUIC / HTTP/3
```

Shadowrocket：

```text
TCP -> http2://...
UDP -> http3://...
```

UDP / QUIC 导出会包含 `alpn=h3`，TLS SNI 使用 `peer` 参数。

Native Naive links：

```text
TCP -> naive+https://...
UDP -> naive+quic://...
```

当前 Mihomo 没有可直接对应的 Naive proxy type，因此 dedicated Clash subscription 不会伪造 Naive 节点。

### Snell v5

Snell 使用兼容客户端最稳定的单活动客户端模型：

```text
3x-ui ClientRecord.Password == Snell PSK
```

每个 Snell inbound 最多一个活动客户端。

Shadowrocket raw subscription / QR 使用：

```text
snell://<base64(chacha20-ietf-poly1305:PSK@host:port)>?version=5&tfo=...
```

启用 HTTP obfs 时会附加：

```text
obfs=http
obfs-host=<host>
```

Mihomo dedicated subscription 使用 `type: snell`、`version: 5`、`psk` 与 `udp: true`。

### Mieru

Mieru 使用 official `mita`，支持当前集成字段，包括：

- TCP / UDP
- primary / additional port bindings
- DNS
- DNS dual stack / hosts
- SOCKS5 egress
- egress rules
- multiplexing
- traffic pattern
- handshake mode

## Subscription / QR / Export

推荐入口：

```text
Shadowrocket        -> 普通 /sub/:subId
Clash Verge/Mihomo  -> dedicated /clash/:subId
```

不同协议会根据客户端 User-Agent 与实际能力输出对应格式，不会为了“看起来有节点”而伪造客户端不支持的协议类型。

## Nodes / 多面板部署

3Xpatcher 不会从主面板远程安装 sing-box 或 Mieru runtime。

因此：

- 主面板需要管理 supplemental protocols 时，应安装 3Xpatcher；
- **实际运行 TUIC / AnyTLS / ShadowTLS / Naive / Snell / Mieru 的远程面板必须安装兼容版本的 3Xpatcher**；
- 只运行官方 Xray 协议的节点可以继续使用官方 3x-ui；
- 多级节点或希望 Traffic / Online / quota 行为完全一致时，建议相关面板统一使用同一版本 3Xpatcher；
- 不能假设只在主面板安装补丁，就能让未安装 supplemental runtimes 的远端节点运行这些协议。

3x-ui 原生 Node / traffic / online 机制仍由各面板本地运行内核并同步状态，3Xpatcher 不改变这一基本模型。

## 系统要求

快速安装器当前面向：

- Debian
- Ubuntu
- Armbian
- systemd
- amd64 / x86_64
- arm64 / aarch64
- 已安装可正常运行的官方 3x-ui

目标 VPS 不需要预装 Go / Node.js / npm。

## 安装 / 更新

已有 3x-ui 用户执行：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/install.sh)
```

安装器会：

1. 识别当前稳定版 3x-ui；
2. 获取与该上游版本匹配的 prebuilt patched panel；
3. 校验 GitHub Release digest；
4. 校验 patch version / upstream version / CPU architecture；
5. 安装或更新 sing-box runtime；
6. 安装或更新 official Mieru `mita`；
7. 备份当前 panel 与 `/etc/x-ui`；
8. 替换 panel binary；
9. 校验 Xray binary SHA256 未发生变化；
10. 启动失败时自动恢复安装前状态。

## 完全卸载

完全卸载使用：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/uninstall.sh)
```

卸载脚本采用 **database guard**。

在任何停止服务、替换 binary 或删除文件之前，会先以只读方式检查数据库是否仍存在以下协议：

```text
tuic
anytls
shadowtls
naive
snell
mieru
```

如果发现任意 supplemental inbound：

```text
立即拒绝卸载
不修改数据库
不停止 x-ui
不停止 supplemental runtimes
不替换 panel binary
不删除任何 3Xpatcher 文件
```

需要先从面板中自行删除这些 supplemental inbounds，再重新运行卸载脚本。

### 仅检查是否可以卸载

```bash
CHECK_ONLY=1 \
bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/uninstall.sh)
```

`CHECK_ONLY=1` 只检查数据库，不执行文件或服务操作。

### 数据库保护

SQLite 使用只读连接：

```text
mode=ro
PRAGMA query_only = ON
```

PostgreSQL 使用只读事务。

卸载器不会对 3x-ui 数据库执行：

```text
INSERT
UPDATE
DELETE
ALTER
DROP
```

数据库检查通过后，卸载器会：

1. 根据当前 3x-ui 版本从 `MHSanaei/3x-ui` 官方 GitHub Release 下载对应原版；
2. 校验官方 Release SHA256 digest；
3. 使用官方 `x-ui` 替换 patched panel binary；
4. 确认官方 `x-ui.service` 可以正常运行；
5. 删除 sing-box / Mieru sidecars；
6. 删除 3Xpatcher systemd units；
7. 删除 3Xpatcher runtime / state / backup / install files；
8. 保留 `/etc/x-ui`、数据库以及官方 3x-ui 数据与配置。

如果官方面板替换后无法正常启动，卸载器会恢复卸载前的 panel binary，并保留 3Xpatcher 文件，不继续清理。

完整卸载不依赖历史 `/var/lib/3xpatcher/backups/.../x-ui` 作为官方 binary 来源，而是重新获取并验证对应版本的官方 Release。

## 完全卸载会清理的路径

在数据库 guard 通过并成功恢复官方 panel 后，主要清理：

```text
/usr/local/x-ui-singbox
/usr/local/x-ui-mieru
/var/lib/x-ui-mieru
/run/x-ui-mieru
/usr/local/share/3xpatcher
/etc/3xpatcher
/var/lib/3xpatcher
```

以及：

```text
/etc/systemd/system/x-ui-singbox.service
/etc/systemd/system/x-ui-mieru@.service
x-ui-mieru@*.service
```

不会删除：

```text
/etc/x-ui
/etc/x-ui/x-ui.db
/usr/local/x-ui/bin/xray-*
```

## 当前边界

- 当前项目为 alpha，建议在重要机器上保留可用备份；
- 3Xpatcher 不修改数据库来完成卸载；存在 supplemental inbound 时必须先由用户自行删除；
- Naive 当前没有 Mihomo proxy type；
- AnyTLS + Reality 无法在当前 Mihomo dedicated Clash 格式中无损表达；
- supplemental runtime 不会自动安装到未打补丁的远程 Node；
- 客户端是否支持某个导出格式最终取决于对应客户端自身实现。

## License / Upstream

3Xpatcher 是对 3x-ui 的补充集成工程。

上游项目：

- 3x-ui: `MHSanaei/3x-ui`
- sing-box: `SagerNet/sing-box`
- Mieru: `enfein/mieru`

使用时请同时遵守各上游项目的许可证与使用条款。
