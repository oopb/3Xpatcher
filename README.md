# 3x-ui Dual Core Patch — V1 UI alpha

目标：在 **不替换、不接管 Xray core 生命周期** 的前提下，为 3x-ui 增加一个独立的 sing-box supplemental core，并在现有 3x-ui React 面板里管理它。

当前版本：`0.2.0-ui-alpha`

## V1 协议范围

用户可见协议固定为：

- TUIC
- AnyTLS
- ShadowTLS v3
- Naive

V1 **不开放 Snell**，即使安装到的 stable sing-box binary 已经支持 Snell；也不重复开放 VLESS / VMess / Trojan / Shadowsocks / Hysteria2 等继续由 Xray 管理的协议。

## 隔离原则

```text
3x-ui
├── 原有 Xray core               # 原升级、重启、配置逻辑保持原样
└── sing-box supplemental core   # /usr/local/x-ui-singbox
    ├── TUIC
    ├── AnyTLS
    ├── ShadowTLS v3
    └── Naive
```

本补丁：

- 不替换 Xray binary；
- 不调用 Xray update / restart / stop；
- 不把 supplemental 协议写进原 Xray `inbounds` 表；
- sing-box 配置失败不会用错误配置覆盖当前可运行配置；
- 源码 overlay 可回滚；
- 卸载 sing-box core 不会删除或停止 Xray。

## 已实现

### 1. 独立数据库模型

新增 `singbox_inbounds`：

- `internal/database/model/singbox.go`
- `scripts/apply-overlay.sh` 将 `&model.SingboxInbound{}` 加入当前 3x-ui v3 的 `allModels()`，由上游 GORM migration 创建表。

原 `Inbound`/`inbounds` 继续只归 Xray 使用。

### 2. 四协议配置生成器

`internal/singbox/config.go`

- 对 TUIC / AnyTLS / ShadowTLS v3 / Naive 做协议级校验；
- 仅生成 enabled rows；
- 明确拒绝 Snell 和 Xray 重复协议；
- TUIC 默认 congestion control 为 `cubic`；
- TLS 协议要求真实 certificate/key path；
- ShadowTLS 固定 v3。

ShadowTLS v3 会自动生成隐藏的 injectable Shadowsocks 2022 inbound：

```text
shadowtls:<tag>
    detour -> shadowsocks:<tag>-inner
```

隐藏 Shadowsocks 只是 carrier 注入实现，不会出现在用户协议选择器里。

### 3. 安全运行时

`internal/singbox/runtime.go`

每次配置变化：

1. 生成临时配置；
2. `sing-box check -c <temp>`；
3. 校验成功后原子替换 `/usr/local/x-ui-singbox/config/config.json`；
4. 只重启 `x-ui-singbox.service`；
5. 如果新配置启动失败，恢复旧配置并重启旧配置。

### 4. 独立 API

- `GET  /panel/api/singbox/list`
- `GET  /panel/api/singbox/get/:id`
- `GET  /panel/api/singbox/status`
- `POST /panel/api/singbox/add`
- `POST /panel/api/singbox/update/:id`
- `POST /panel/api/singbox/del/:id`
- `POST /panel/api/singbox/setEnable/:id`
- `POST /panel/api/singbox/check`
- `POST /panel/api/singbox/restart`

CRUD 变更会根据 DB 中全部 enabled supplemental inbounds 重建一份完整 sing-box config。

### 5. React 面板

新增：

`frontend/src/pages/singbox/SingboxInboundsPage.tsx`

源码 overlay 会同时注入：

- `frontend/src/routes.tsx` → `/panel/singbox`
- `frontend/src/layouts/AppSidebar.tsx` → `Sing-box`

页面目前支持：

- core status / version；
- Refresh；
- `sing-box check`；
- restart supplemental core；
- inbound list；
- create / edit / delete；
- enable / disable；
- TUIC / AnyTLS / ShadowTLS v3 / Naive 专用表单；
- TUIC UUID/password generator；
- ShadowTLS hidden inner Shadowsocks key generator。

如果 sing-box binary/service 尚未安装，status 会显示 unavailable，但数据库列表仍可读取，不会因为 status endpoint 失败而把管理页面一起打坏。

### 6. stable sing-box 安装/更新

`scripts/install-singbox.sh`

- 读取 SagerNet/sing-box 官方 GitHub `releases/latest`；
- 只接受 `vMAJOR.MINOR.PATCH` 且 `draft=false`、`prerelease=false`；
- Linux amd64 / arm64；
- 使用 GitHub release asset 提供的 SHA256 digest 校验 tarball；
- 新 binary 替换前先用当前 config 执行 `check`；
- runtime 独立安装到 `/usr/local/x-ui-singbox`；
- 若新 binary 启动失败，回滚上一套 binary directory。

目录：

```text
/usr/local/x-ui-singbox/
├── bin/
│   ├── sing-box
│   └── *.so               # 官方 archive 有则保留
├── config/
│   └── config.json
└── backup/
```

systemd unit：`x-ui-singbox.service`

## 应用到 3x-ui 源码

```bash
./scripts/apply-overlay.sh /path/to/3x-ui
```

脚本会先验证当前上游 patch points，验证失败则 fail closed，不猜测修改位置。随后：

1. 备份 `db.go` / `api.go` / `routes.tsx` / `AppSidebar.tsx`；
2. 复制 supplemental backend 与 React 页面；
3. 注入 DB model；
4. 注入 `/panel/api/singbox`；
5. 注入 `/panel/singbox` 前端 route；
6. 注入 sidebar item；
7. `gofmt` Go 文件。

之后使用 **3x-ui 上游自己的构建流程**：

```bash
make typecheck
make test
make build
```

正式安装时应在满足上游当前 Go/Node 版本要求的构建环境执行。

### 回滚源码 overlay

```bash
./scripts/revert-overlay.sh /path/to/3x-ui /path/to/3x-ui/.dualcore-backup-YYYYMMDD-HHMMSS
```

回滚不会 DROP `singbox_inbounds`，防止误删配置。

## 安装 / 更新 sing-box core

```bash
sudo ./scripts/install-singbox.sh
```

更新到最新 stable：

```bash
sudo ./scripts/update-singbox.sh
```

卸载 binary/service、保留 config/backups：

```bash
sudo ./scripts/uninstall-singbox.sh
```

连 supplemental core 数据目录一起删：

```bash
sudo PURGE=1 ./scripts/uninstall-singbox.sh
```

这些脚本都不会删除或停止 Xray。

## API payload 示例

见 `examples/`。

TUIC 示例默认使用 `cubic`：

```json
{
  "users": [{"name":"alice","uuid":"550e8400-e29b-41d4-a716-446655440000","password":"..."}],
  "congestionControl": "cubic",
  "heartbeat": "10s",
  "zeroRTTHandshake": false,
  "tls": {
    "enabled": true,
    "certificatePath": "/path/fullchain.pem",
    "keyPath": "/path/privkey.pem"
  }
}
```

ShadowTLS hidden inner 支持：

- `2022-blake3-aes-128-gcm`：16-byte 标准 Base64 key；
- `2022-blake3-aes-256-gcm`：32-byte 标准 Base64 key；
- `2022-blake3-chacha20-poly1305`：32-byte 标准 Base64 key。

## Smoke test

```bash
./tests/smoke.sh
```

当前 smoke 覆盖：

- Go renderer unit tests；
- shell syntax；
- example JSON / V1 protocol allowlist；
- supplemental 源码 Xray lifecycle 静态隔离 guard；
- backend + frontend overlay 注入；
- overlay rollback 后原文件 hash 恢复。

## 仍未完成

`0.2.0-ui-alpha` 还不是最终的一键生产版。下一阶段主要是：

1. subscription URI / sing-box JSON / Clash Meta 输出；
2. sing-box user traffic stats、quota、expiry；
3. 端口冲突检查（Xray 与 sing-box 共用宿主端口空间）；
4. 更完整的 certificate 复用/选择 UI；
5. patch-aware 3x-ui Panel Updater；
6. 在真实当前 3x-ui full source 上跑完整 `make verify`；
7. 在真实 stable sing-box binary 上对四协议生成结果跑 integration `sing-box check`。

## 当前验证边界

本源码包已经完成 renderer/unit/static/overlay/rollback 测试，但当前执行环境无法完成最终上游集成验证：本地 Go/Node 版本低于当前 3x-ui upstream 构建要求，且 shell 环境不能直接下载官方 release binary。因此目前不能把它标记为 production-ready。
