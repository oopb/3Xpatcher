# 3Xpatcher — 3x-ui Integrated Dual-Core Patch

3Xpatcher 在 **保留 3x-ui 原生 Inbounds / Clients / Subscription 体验** 的前提下，为官方 3x-ui 增加独立 sing-box supplemental core。

当前版本：`0.5.0-integrated-alpha`

## 当前协议

新增到原生 **Inbounds → Add Inbound → Protocol**：

- TUIC
- AnyTLS
- ShadowTLS v3
- Naive

VLESS / VMess / Trojan / Shadowsocks / Hysteria / WireGuard 等仍由原 3x-ui / Xray 管理，不重复实现。

## V2：原生管理界面，双 Core 运行时

```text
3x-ui native UI / DB / Client / Subscription
                    │
                    ├── Xray protocols
                    │       └── Xray
                    │
                    └── TUIC / AnyTLS / ShadowTLS / Naive
                            └── x-ui-singbox.service
```

V2 不再增加独立 `Sing-box` 侧边栏页面，也不再使用单独的 `singbox_inbounds` 作为运行数据源。

### 直接复用 3x-ui

- 原生 `inbounds` 表和 Inbounds 列表
- 原生 Add / Edit / Delete / Enable / Disable
- 原生 `clients` + `client_inbounds` 关联
- Client email / subId / UUID / Password / Enable / Expiry / Total 等身份与策略字段
- 原生 Client Attach / Detach、多入站关联
- 原生订阅排序与分享地址解析
- 原生 `/sub/:subId` 订阅链路
- 原生 TLS 表单、证书内容/证书路径、默认面板证书选择
- 原生 Hosts/share endpoint 机制（订阅端）

### Core 严格隔离

数据库和 UI 是统一的，但运行时最后一跳分流：

```text
vmess/vless/trojan/... -> Xray runtime

tuic/anytls/shadowtls/naive -> sing-box runtime
```

补丁会在 Xray 全量配置生成时显式过滤 supplemental protocols，避免 TUIC/AnyTLS 等进入 Xray config。

修改 supplemental inbound/client 时，sing-box 从原生 `inbounds + clients + client_inbounds` 重新渲染，先执行 `sing-box check`，再原子替换配置并只重启 `x-ui-singbox.service`。配置内容没有变化时不会重启 sing-box。

新建 supplemental inbound 时可以先不绑定 Client；0 active-client inbound 会保留在 3x-ui 中，但不会实际绑定监听端口，直到至少绑定一个有效 Client。

## Client credential 映射

V2 不维护第二套用户：

| Protocol | sing-box credential | 3x-ui Client source |
| --- | --- | --- |
| TUIC | name / uuid / password | email / UUID / Password |
| AnyTLS | name / password | email / Password |
| ShadowTLS v3 | name / password | email / Password |
| Naive | username / password | email / Password |

因此同一 Client 可以同时 attach 到 VLESS、TUIC、AnyTLS 等多个 inbound，并继续使用同一个 `subId`。

## Subscription

原生 raw subscription `/sub/:subId` 已扩展为同时检索 Xray 与 supplemental inbounds。

- TUIC：输出 TUIC share link
- AnyTLS：输出 AnyTLS share link
- ShadowTLS v3：输出 3Xpatcher 扩展 share link，并在 Mihomo Clash 输出中映射为 Shadowsocks + `shadow-tls` plugin
- Naive：输出 `naive+https` raw link

Mihomo/Clash 输出目前支持 TUIC、AnyTLS、ShadowTLS；Naive 暂不写入 Mihomo YAML，因为当前没有可安全依赖的原生 Naive proxy 类型。

> 当前 alpha 尚未把 sing-box 的实时字节统计合并回 3x-ui/Xray traffic counters。Client enable/expiry/attachment/订阅身份已统一，但 supplemental traffic accounting 是后续工作。

## 安装

要求：

- 已安装官方 3x-ui
- Debian / Ubuntu / Armbian
- systemd
- amd64 / arm64

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/install.sh)
```

安装器读取当前 `/usr/local/x-ui/x-ui -v`，下载与当前官方版本匹配的 GitHub Actions 预编译 patched panel，校验 release SHA256、版本和架构后才替换 `/usr/local/x-ui/x-ui`。

目标 VPS 不需要 Go / Node.js / npm / CGO 编译环境。

当前兼容上游版本见：

```text
UPSTREAM_COMPAT
```

## Xray 安全边界

安装前会备份当前 panel binary、`/etc/x-ui`（若存在）并记录 `/usr/local/x-ui/bin/xray-*` SHA256。

安装过程中：

- 不替换 Xray binary
- 不修改 Xray updater
- patched `x-ui` 激活后再次验证所有 Xray binary SHA256
- 激活失败自动恢复原 panel binary

安装 patched panel 会重启一次 `x-ui.service`，因此其 Xray 子进程会短暂重连一次；之后 supplemental core 与 Xray 独立运行。

## sing-box runtime

```text
/usr/local/x-ui-singbox/bin/sing-box
/usr/local/x-ui-singbox/config/config.json
/etc/systemd/system/x-ui-singbox.service
```

更新 sing-box：

```bash
/usr/local/share/3xpatcher/current/scripts/update-singbox.sh
```

## 回滚

只恢复原 3x-ui panel binary，默认保留 sing-box runtime：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/rollback.sh)
```

同时清理 sing-box runtime：

```bash
PURGE_SINGBOX=1 bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/rollback.sh)
```

历史 V1 的 `singbox_inbounds` 表不会自动 DROP，作为回滚/数据恢复遗留保留；V2 runtime 不再读取它。

## 开发 / CI

GitHub Actions：

1. 下载 `UPSTREAM_COMPAT` 指定的官方 3x-ui 源码；
2. `scripts/apply-overlay.sh` 将 V2 integration overlay 应用到原生 Inbounds / Clients / Subscription / runtime；
3. 运行 supplemental renderer tests；
4. `npm ci && npm run build`；
5. 静态构建 amd64 / arm64 patched `x-ui`；
6. 发布 rolling `prebuilt-vX.Y.Z` release。

本地静态 smoke：

```bash
./tests/smoke.sh
```

## 3x-ui 升级

官方 Panel 升级仍会用官方 `x-ui` 覆盖 patched binary。升级官方 3x-ui 后，需要等待/生成对应版本的 `prebuilt-vX.Y.Z` 兼容包，然后重新运行一键安装器。

Xray core 单独更新和 sing-box core 单独更新不需要重新 patch panel。
