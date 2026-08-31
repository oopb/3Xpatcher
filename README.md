# 3Xpatcher — 3x-ui Dual Core Patch

在**不替换、不接管 Xray core 生命周期**的前提下，为 3x-ui 增加独立的 sing-box supplemental core，并在现有 3x-ui React 面板中管理它。

当前版本：`0.4.1-prebuilt-alpha`

## V1 协议

- TUIC
- AnyTLS
- ShadowTLS v3
- Naive

V1 不开放 Snell，也不重复开放 VLESS / VMess / Trojan / Shadowsocks / Hysteria2 等继续由 Xray 管理的协议。

## 架构

```text
3x-ui
├── 原有 Xray core               # 原配置、更新、重启逻辑保持原样
└── sing-box supplemental core   # /usr/local/x-ui-singbox
    ├── TUIC
    ├── AnyTLS
    ├── ShadowTLS v3
    └── Naive
```

补丁新增独立 `singbox_inbounds` 模型、`/panel/api/singbox/*` API、`/panel/singbox` React 页面以及独立 `x-ui-singbox.service`。原 Xray `inbounds` 表和 `/usr/local/x-ui/bin/xray-*` 不由 3Xpatcher 修改。

## 快速安装

支持 Debian / Ubuntu / Armbian、systemd、amd64 / arm64，并要求机器上已经安装官方 3x-ui。

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/install.sh)
```

### 0.4.0 起不再默认在 VPS 上编译

旧版一键脚本会在目标 VPS 下载 3x-ui 源码、Node、Go 和大量依赖，然后本地构建。小磁盘/小内存机器容易耗时较长，并可能在 Go/CGO 编译阶段占满根分区。

现在改为：

1. 读取当前 `/usr/local/x-ui/x-ui -v`；
2. 按当前 3x-ui 版本寻找 `prebuilt-vX.Y.Z` 兼容构建；
3. 校验 GitHub release asset SHA256 digest；
4. 校验包内 `PATCH_VERSION / UPSTREAM_REF / ARCH`；
5. 备份当前 `x-ui`、`/etc/x-ui` 和所有 Xray binary SHA256；
6. 安装/更新 stable sing-box；
7. 只替换 `/usr/local/x-ui/x-ui`；
8. 重启一次 `x-ui.service` 并验证状态；
9. 再次校验 Xray binary SHA256；
10. 激活失败则自动恢复原 panel binary。

因此目标 VPS **不需要 Node.js、Go、npm install、Go module download 或本地 CGO 编译**。正常安装只需要下载一个预编译 patched panel 和 sing-box runtime。

当前预编译兼容的官方 3x-ui 版本写在：

```text
UPSTREAM_COMPAT
```

如果当前 3x-ui 版本还没有对应预编译包，安装器会快速退出，不会自动回退到耗时源码编译。

## 为什么仍然属于“源码 Patch 架构”

预编译并没有改变补丁结构。GitHub Actions 仍然执行：

```text
官方 3x-ui 对应版本源码
        ↓
scripts/apply-overlay.sh
        ↓
加入 Sing-box React 页面 / API / DB model
        ↓
npm build
        ↓
Go + CGO static build
        ↓
预编译 patched x-ui
```

区别只是把耗时构建从 VPS 移到了 GitHub Actions。

## 前端修改

Overlay 新增：

```text
frontend/src/pages/singbox/SingboxInboundsPage.tsx
```

并对官方源码做两个小型注入：

```text
frontend/src/routes.tsx
  + /panel/singbox

frontend/src/layouts/AppSidebar.tsx
  + Sing-box
```

原 Dashboard / Inbounds / Clients / Settings / Xray 页面不被替换。

## sing-box runtime

安装目录：

```text
/usr/local/x-ui-singbox/
├── bin/
│   ├── sing-box
│   └── *.so
├── config/
│   └── config.json
└── backup/
```

systemd：

```text
x-ui-singbox.service
```

配置更新流程：

```text
DB
 → render temp config
 → sing-box check
 → atomic replace
 → restart x-ui-singbox.service only
 → failed: restore previous config
```

## 一键回滚

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/rollback.sh)
```

默认恢复安装前的 3x-ui panel binary，同时保留 sing-box runtime/config 和 `singbox_inbounds` 数据。

完全清除 supplemental runtime：

```bash
PURGE_SINGBOX=1 bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/rollback.sh)
```

安装状态：

```text
/etc/3xpatcher/install.env
```

备份：

```text
/var/lib/3xpatcher/backups/
```

## 3x-ui 升级

Xray Core 单独升级不需要重新安装 3Xpatcher。

如果升级 **3x-ui Panel 本体**，官方 updater 会替换 patched `/usr/local/x-ui/x-ui`，因此 Sing-box 页面/API 会暂时消失，但 `/usr/local/x-ui-singbox` 和 supplemental 配置不会被官方 updater 删除。

新 3x-ui 版本需要先由 3Xpatcher CI 生成对应兼容预编译包，然后重新运行：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/oopb/3Xpatcher/main/install.sh)
```

如果上游 route/sidebar/API patch point 已变化，`apply-overlay.sh` 会 fail closed，CI 不会发布该版本的预编译 patched panel。

## 开发 / 源码 Overlay

```bash
./scripts/apply-overlay.sh /path/to/3x-ui
```

它会先验证 patch points，再复制 supplemental backend/frontend 代码并注入 DB/API/route/sidebar。

源码恢复：

```bash
./scripts/revert-overlay.sh /path/to/3x-ui /path/to/3x-ui/.dualcore-backup-YYYYMMDD-HHMMSS
```

回滚现在按备份文件**原样恢复**，不会在恢复后再次 `gofmt`，因此可做字节级 hash 对比。

## 测试

```bash
./tests/smoke.sh
```

覆盖：renderer unit tests、shell syntax、V1 protocol allowlist、Xray isolation、overlay、前端 route/sidebar 注入以及字节级 source rollback。
