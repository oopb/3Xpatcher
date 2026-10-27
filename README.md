# 3Xpatcher — 3x-ui Integrated Multi-Core Patch

3Xpatcher 是面向官方 **3x-ui** 的多内核集成补丁。

目标是在尽量保持 3x-ui 原生 **Inbounds / Clients / Subscription / Traffic / Online / Nodes** 工作流不变的前提下，保留官方 Xray，同时增加彼此隔离的 sing-box 与 Mieru 运行时，使一个 3x-ui 面板能够统一管理更多协议。

当前版本：`0.17.10-integrated-alpha`

当前兼容上游：`3x-ui v3.11.0`（保留 v3.7.0、v3.8.5、v3.9.0、v3.10.0 的预构建支持）

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
| TUIC | Native / sing-box 可选 | 多用户 | Yes | Shadowrocket / Mihomo |
| AnyTLS | sing-box | 多用户 | Yes | Shadowrocket / Mihomo |
| ShadowTLS v3 | sing-box | 多用户 | Yes | Shadowrocket / Mihomo |
| Naive TCP / HTTP2 | sing-box | 多用户 | Yes | Shadowrocket / native Naive |
| Naive UDP / QUIC | sing-box | 多用户 | Yes | Shadowrocket HTTP3 / native Naive QUIC |
| Snell v5 | sing-box | 单活动客户端 | Yes | Shadowrocket / Mihomo |
| Mieru | official `mita` | 多用户 | Yes | Mieru compatible clients |

## 内置去广告（3x-ui v3.9.0 起）

从 `0.15.1-integrated-alpha` 开始，新构建的 3Xpatcher 面板默认移除 3x-ui 自 v3.9.0 引入的 Sponsors 赞助商广告：Dashboard、登录页、旧版侧边栏广告位、Sponsors 导航页与命令面板入口均不再显示。

补丁还让面板公开的赞助商 API 返回空列表、Logo API 返回 404，避免通过面板 HTTP 请求远程获取赞助商数据。**不修改 Xray、sing-box、Mieru、节点/用户配置，也不删除数据库记录。**

- 在 v3.7.0 / v3.8.5 等未包含 Sponsors 模块的上游版本，构建脚本自动跳过。
- 在 v3.9.0 / v3.10.0 / v3.11.0，构建脚本自动应用 `scripts/disable-sponsors.py`，无需在浏览器中单独安装去广告插件。
- 已发布的 v3.9.0 / v3.10.0 旧预构建包**不会自动变成去广告版本**；需重新构建这些上游版本才能应用新补丁。当前更新发布主要针对 v3.11.0。
- 本功能只对采用新补丁构建的 patched panel 生效；更新为官方原版 3x-ui 将恢复其原有显示行为.

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

选择 `sing-box` runtime 的 TUIC 与 AnyTLS / ShadowTLS / Naive / Snell 共用独立 sing-box sidecar：

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

TUIC 每个 inbound 可选择 3x-ui 原生 TUIC runtime 或 sing-box runtime；两者复用同一套客户端、证书与订阅数据。

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

## XHTTP 自动 REALITY 下行入口（0.17.0 实验性）

在 VLESS + XHTTP + TLS 入站的 **Streaming → Download Settings** 中，将下行安全类型设为 REALITY 后，可选择 **自动创建 REALITY 入口**。面板通过原生密钥生成 API 创建 REALITY X25519 密钥对，可填写独立下行监听 IP/端口、Target、SNI 列表及 Short ID；客户端公钥与服务端私钥分离保存。下行端口不提供默认候选值，必须手动选择有效且未被占用的独立监听端口，不能与原 TLS 上行端口冲突；该端口需要自行在防火墙开放。

启用后运行配置将原 TLS/XHTTP 入口替换为 TLS/RAW 回落前端，并添加 REALITY/RAW 回落前端，两者回落到**同一个内部 Unix 抽象套接字 XHTTP 入站**；原用户、Path 和入站流量统计保留在内部共享入站。关闭时恢复原始 TLS/XHTTP 入站结构。配置生成阶段会拒绝重复端口、缺失密钥、SNI/Short ID 不一致。

**实验性注意：** 这是运行配置拓扑变更，首次启用时可能重启 Xray。建议先在测试 VPS 或非主力入站测试，并备份面板配置。静态测试不能替代真实 TLS/REALITY 握手和 CDN 兼容性测试。现阶段自动监听针对 **TLS 上行 + REALITY 下行**；其他模式仍为客户端订阅导出，不会自动启动相应服务端入口。TLS 证书仍复用原入站的 TLS 设置，尚未实现完整的独立证书向导。Shadowrocket 可能在 URI 导入时丢弃 downloadSettings，不能保证自动识别。

## XHTTP 上下行分离（3x-ui 3.11.0）

> 0.16.1 修复「启用独立下行连接」打开后无法关闭的问题：开关状态独立管理，关闭后保存会删除下载配置且重新编辑保持关闭，原有启用节点仍能正确加载。


原生 **Inbounds → Streaming → XHTTP → Download Settings** 可为 VLESS/XHTTP 节点增加一个客户端专用的下行连接地址。UI 使用原来的 `FormField`、uTLS 指纹选项和 TLS/REALITY 语义；打开开关时预填现有入站端口、地址、SNI、Reality 公钥与 Short ID，未设置前不会改变任何节点。客户端仍可根据需要修改下行地址、端口、TLS/REALITY SNI、指纹与 Host。

- 数据保存在入站的 `streamSettings.xhttpSettings.downloadSettings`，**Xray 服务端配置生成时剔除**；3x-ui 不会把客户端出站选项交给服务器监听器。
- Xray JSON subscription 与 VLESS share-link 的 `extra.downloadSettings` 使用 Xray 的嵌套 `address/port/network/security/{tlsSettings|realitySettings}/xhttpSettings` 格式；模式和 path 继承上行，避免因路径不匹配而断连。
- Dedicated Mihomo/Clash subscription 使用 `xhttp-opts.download-settings` 的扁平 `server/port/tls/servername/client-fingerprint/reality-opts` 格式，并在下行 TLS 上清空上行 REALITY public key，避免意外携带 REALITY 握手。
- 不完整的下行地址、端口或必需 SNI/Reality 公钥不会被导出；不启用时，旧版节点的输出不变。
- **服务端前提：** CDN/TLS 与 REALITY 两个入口必须汇聚到**同一 XHTTP 服务端实例和相同 path**。本功能不自动配置 Caddy、Nginx、CDN 或防火墙，也不能使已封锁的源站直连下行恢复。
- **兼容性：** Mihomo 文档警告其 Reality 与较新 Xray-core 存在协议差异。启用混合 TLS/REALITY 后，请先针对实际客户端版本进行导入与握手测试；对于无法表达该组合的客户端，优先使用原生 Xray JSON 配置。

依据：[Xray 五合一参考](https://github.com/XTLS/Xray-core/discussions/4118)、
[Mihomo XHTTP 参数](https://wiki.metacubex.one/config/proxies/transport/)、
[Mihomo TLS/Reality](https://wiki.metacubex.one/config/proxies/tls/)。

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

## Clash Verge / Mihomo 与 REALITY 分离兼容性（0.17.3）

**已找到并在 Xray 26.7.28 上复现、验证真正原因：REALITY `minClientVer` 默认限制。** Xray-core 自 v26.7.11 起，在省略 `minClientVer` 时默认使用 `26.3.27`；而 Mihomo v1.19.x 的 REALITY ClientHello 携带的版本为 `1.8.2`，导致 REALITY 下行握手返回 `authentication failed`。原补丁把所有 v26.7.11+ 标记为完全不能兼容，这是不准确的。

启用 **Download Settings → REALITY → 自动创建 REALITY 入口** 后，3Xpatcher 只在新生成的托管下行 REALITY 入站添加：

```json
"minClientVer": "1.8.2"
```

不会修改原 TLS 上行、普通 REALITY 入站或原 VLESS ML-KEM/ Vision 配置。用户仍使用 **TLS/CDN 上行 + REALITY/VPS IP 直连下行**；二者回落到同一个 XHTTP 后端和共享 Path/用户。

**真实网络测试**：官方 Xray-core `v26.7.28` + 官方 Mihomo `v1.19.33`，TLS 上行/REALITY 下行、内部 UDS XHTTP、VLESS native ML-KEM-768 Encryption 和 `xtls-rprx-vision`，通过实际 HTTPS 代理请求。证据：[工作流记录](https://github.com/oopb/3Xpatcher/actions/runs/40467983997)。兼容性参数放宽了 REALITY 所接受的最低客户端版本，可能降低使用版本约束进行异常客户端过滤的能力；目前只作用于主动开启的托管入口。

Mihomo 下行导出仍显式携带 `host: ""`、`alpn: [h2]` 以避免继承 CDN 上行 Host/ALPN。官方 Mihomo 对新版本 Xray REALITY 的其他变动不承诺持续支持；本测试精确针对 Xray `26.7.28`，不代表后续所有 Xray 版本必然兼容。

**Shadowrocket 仍是独立问题**：已有 VLESS 分享链接内 `extra.downloadSettings` 存在，但其导出 JSON 未出现独立下行内容；缺乏官方确认的 Shadowrocket 分享链接扩展格式，故不臆造转换或声称小火箭已修复。

资料：[MetaCubeX/Mihomo #3132](https://github.com/MetaCubeX/mihomo/issues/3132) · [Xray-core #6477](https://github.com/XTLS/Xray-core/issues/6477) · [Mihomo TLS 配置](https://wiki.metacubex.one/config/proxies/tls/)。


## XHTTP 上下行分离 Host 与 Shadowrocket 导入（0.17.4）

- 解决 **CDN/TLS 上行 + REALITY 直连下行** 的 Host 分离导致的 404：未设置独立下行 HTTP Host 时，客户端导出自动继承上行的 XHTTP HTTP Host（例如 cdn.example.com），**不会**错误地默认为 REALITY SNI（例如 reality.example.com）；TLS SNI / REALITY SNI 仍独立。
- 自动托管 REALITY 下行新增可编辑 `minClientVer`，默认 `1.8.2`；服务端验证 `x.y.z` 语法，已有旧配置自动回退默认值，避免 Mihomo REALITY 认证失败。
- 分享链接除了标准 Xray `extra.downloadSettings` 外，新增 Shadowrocket 的 `obfsParam` JSON（`downloadTargetHost`, `downloadTargetPort`, `downloadServerName`, `downloadHTTPHost`, `downloadPath`）。面板二维码预览和后端订阅统一。
- **限制**：这些 Shadowrocket 参数名由实际导出的内部节点结构与公开示例确认，但 Shadowrocket 是闭源客户端，尚无法确认它会从标准 URI `obfsParam` 载入全部字段；下行 REALITY 公钥和 Short ID 在 Shadowrocket 的 URL 导入映射仍无公开可验证依据。本次保留在 Xray `extra.downloadSettings.realitySettings`，**不宣称 iOS 端 REALITY 下行已实机成功**。不要把上行 TLS 改为 REALITY 来迎合 `pbk` 和 `sid`。


## XHTTP REALITY 托管入口表单自动设置修复（0.17.5）

- **不再自动指定下行监听端口**：创建独立下行时端口和地址均为空；启用托管 REALITY 时要求明确输入 1–65535 的下行端口，禁止与上行 TLS 入站端口冲突，不再擅自填充或覆盖为 9443。
- **REALITY Target 不再从 SNI 拼成固定 `:443`**。Target 是独立远端伪装目标，和本机 REALITY 下行监听端口属于不同含义，需手动填写或选取目标扫描结果。
- **复用官方 REALITY 目标扫描 UI**：托管高级设置内「选择 REALITY 伪装目标 / 自动填充 SNI」调用 3x-ui 自带的 `RealityTargetScannerModal` 和 `scanRealityTargets` API，选取的 Target 保留其自身端口，并同步候选 SNI 至服务端和客户端。
- **避免过度自动覆盖**：不从 CDN shareAddr 填充直连下行地址，不把原 TLS SNI 或旧公钥带入全新的 REALITY 下行配置；下行 HTTP Host 留空时动态继承上行 XHTTP Host，原先保存的端口、Target、SNI、Short ID 和可编辑的 `minClientVer` 尽量保留。密钥生成失败提供明确提示，已有密钥不完整/冲突时要求核对，不会偷偷替换密钥对。
- 已保存的有效托管配置不会因为本次更新而被自动重置。新建时需要自行填写端口、地址及 Target 后保存。

## XHTTP REALITY 极简配置（0.17.6）

- 「启用独立下行连接」的 **REALITY SNI** 旁直接使用 3x-ui v3.11.0 原生 `RealityTargetScannerModal`，通过官方 `scanRealityTargets` 扫描选择，自动同步选中的 Target、候选 SNI 和客户端 SNI；切换伪装目标不保留旧目标的 SNI。
- 「REALITY 服务端自动监听」仅保留一个开关，不再显示独立高级设置区；仅启用后显示必须明确选择的远端 REALITY Target，未扫描时允许手动填写，但必须通过表单校验，**不会猜测 Target:443**。
- 托管入口监听 IP 默认 `0.0.0.0`，`minClientVer` 默认 `1.8.2`；密钥对通过 3x-ui 原生接口自动生成，Short ID 自动产生，已保存的有效值保留。托管开启时自动同步客户端 Public Key、Short ID 和服务端允许的 SNI；无需在高级设置中重复输入。
- 未开启托管时仍支持连接外部 REALITY 下行服务器，用户可以在外层自行填写 Public Key 和 Short ID。所有模式均保留手动指定下行地址、端口和 uTLS 的能力。
- 该改动只精简表单展示，不改变已经生效的共享 XHTTP 会话、TLS 上行和 REALITY 下行的后端拓扑。

## XHTTP REALITY 可见参数与同步编辑（0.17.7）

- 保留 0.17.6 的外层原生 REALITY 目标扫描器与简洁的托管监听开关，但纠正“自动生成即隐藏字段”的问题。
- 下行 REALITY 客户端参数始终展示并可编辑：**SNI、Public Key、Short ID、uTLS、下行地址与端口、HTTP Host**；上行的 XHTTP Path/mode 保持原生表单配置。
- 启用「REALITY 服务端自动监听」后，在外层直接展开并可编辑全部托管参数：**监听 IP、远端 Target、允许的 SNI 列表（serverNames）、允许的 Short ID 列表（shortIds）、Private Key、服务端 Public Key、minClientVer**。不再引入“高级设置”。
- 启用托管时若缺少密钥对，通过 3x-ui 原生接口生成；显示「重新生成 REALITY 密钥对」与「重新生成 Short ID」按钮。客户端/服务端公钥、Short ID 与允许列表随用户修改同步；允许手动输入已有密钥对，但手动修改公钥或私钥后必须确保两者匹配。
- 保存的有效参数继续保留；不会预填下行端口、不会覆盖扫描得到的 REALITY Target 为固定 `SNI:443`。Private Key 只存储在服务端配置元数据，不进入任何客户端订阅或分享链接。

## XHTTP 管理监听端口冲突 / Mihomo TLS 下行配置（0.17.8）

- 托管 REALITY 下行作为额外 TCP 监听，会参与 3x-ui v3.11.0 保存、编辑和启用入站时的事务性端口冲突检查：上行与下行相撞、与其他入站主端口冲突、与其他托管下行冲突都会返回清晰错误；同时对新申请的托管 TCP 端口进行系统 socket 预检查。对于编辑已运行且未变动的托管监听端口，不会把自己的 Xray socket 误判为外部占用。不影响 TCP 与 UDP 同端口合法共存，也不会未经授权自动改端口。
- Mihomo 的 `xhttp-opts.download-settings` 根据上下行**实际安全类型**生成：TLS → TLS 不再生成 `reality-opts`；REALITY → TLS 仍可在下行清除继承的 REALITY 公钥；TLS → REALITY 保持公钥、Short ID 等设置。不使用最终 YAML 空字段清理或文本替换。
- 原有 3x-ui v3.7–v3.10 兼容映射保持原样；新增端口事务检查只适配提供 `internal/web/service/port_conflict.go` 的新版上游。
- 端口可用性预检查不是系统级原子锁：极短竞态仍须由最终 Xray bind 决定。升级不会改动现有节点端口，请自行核对实际监听和防火墙。

## XHTTP 三入站生命周期自动收敛（0.17.9）

- 修复修改 XHTTP 上行或托管 REALITY 下行端口后旧监听残留。源头是上游普通入站单 Tag 的 gRPC DelInbound/AddInbound 与 3Xpatcher 的 `共享 XHTTP + TLS 上行 + REALITY 下行` 三入站生成拓扑不匹配。
- 3x-ui 原生 `GetXrayConfig → RestartXray → ComputeHotDiff / stop-restart` 已能移除旧监听、接管新端口；现在对开启过托管的入站统一走这一机制，不再直接对单个数据库入站 Tag 做增删。
- 覆盖新建、编辑（包括上下行端口变化、切换托管开关）、启用/停用和删除。普通非托管 XHTTP 仍走原生快速单入站操作。更新时保留正常用户统计和共享会话拓扑。
- 测试覆盖：修改 REALITY 端口后新配置不再有旧端口，Xray 原生热更新规则会为 REALITY 端口变更触发完整重启，以及托管开启/关闭/删除状态过渡。

## Shadowrocket XHTTP 下行导入参数修复（0.17.10）

- **Xray/V2Ray 标准格式优先**：不修改 VLESS `UUID@地址:端口` 链接主体、顶层 `encryption=`、`type=xhttp`、TLS/REALITY 上行参数、Vision、`extra.downloadSettings` 和 Mihomo 输出。
- 依据 Shadowrocket 原生导出的 XHTTP 链接，保留其 `obfsParam` 下行目标地址/端口/SNI/HTTP Host；TLS 上行 + REALITY 下行场景增加 Shadowrocket 读取的顶层 `pbk` 和 `sid`。上行 REALITY 时绝不覆盖现有上行 `pbk`/`sid`，TLS→TLS 不注入 REALITY 选项。
- `obfsParam` 不再附加与主链接 `path` 重复、原生样本没有的 `downloadPath`；标准 `extra.downloadSettings.xhttpSettings.path` 仍原样导出。面板分享/二维码与订阅使用同样规则。
- 回归测试检查 TLS→REALITY、TLS→TLS、REALITY 上行密钥隔离和 Xray 标准字段不回退。

## Shadowrocket XHTTP Host 错误修复（0.17.11）

- 修复独立下行 VLESS/XHTTP 节点中 JSON `obfsParam` 被 Shadowrocket 错当作 HTTP Host 的问题。对于确实存在下行 `obfsParam` 的节点，额外添加小火箭原生传输标记 `obfs=xhttp`，同时保留 Xray 的 `type=xhttp`、`host=<domain>` 以及完整的 `extra.downloadSettings`。
- 不改变 VLESS UUID、顶层 `encryption=`、TLS/REALITY 上行参数、Vision、下行 REALITY 的 `pbk`/`sid` 或 Mihomo 输出；没有独立下行的普通 XHTTP 节点不添加 `obfs`。
- 同时修复订阅链接和面板复制/二维码导出。新增 Go 与真实前端链接生成回归测试，验证 Host 始终是纯域名，JSON 只在 `obfsParam` 中。
- 运行时连接行为仍需 Shadowrocket 真机导入验证；自动化测试不等同于小火箭解析器测试。
