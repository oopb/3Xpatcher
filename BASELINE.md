# Baseline and compatibility

This V1 UI alpha was aligned against the current upstream 3x-ui `main` observed on 2026-08-31:

- Repository: `MHSanaei/3x-ui`
- Upstream version/commit observed: `v3.7.0` / `f727d04f6522bb94a8fb52e8352fdcafb51c11e1`
- Go module: `github.com/mhsanaei/3x-ui/v3`
- Backend patch points:
  - `internal/database/db.go` → `allModels()`
  - `internal/web/controller/api.go` → `/panel/api` controller initialization
- Frontend patch points:
  - `frontend/src/routes.tsx` → lazy page declarations + panel child routes
  - `frontend/src/layouts/AppSidebar.tsx` → sidebar tabs

`apply-overlay.sh` validates all four patch points before copying or modifying files. If a future upstream release moves them, it fails closed rather than producing a partial guessed patch.

## sing-box channel policy

The installer uses the official GitHub `releases/latest` endpoint and accepts only stable semantic-version tags matching `vMAJOR.MINOR.PATCH`, with `draft=false` and `prerelease=false`.

V1 user-visible protocols remain intentionally fixed to:

- TUIC
- AnyTLS
- ShadowTLS v3
- Naive

Snell remains disabled in V1 even when the installed stable binary contains it. Enabling Snell requires an explicit application update, not merely a core binary update.

## Isolation contract

The V1 patch must not:

- replace the Xray binary;
- change the Xray update endpoint/workflow;
- restart or stop Xray while applying sing-box configuration;
- store supplemental protocols in the original Xray `inbounds` table;
- expose duplicate VLESS/VMess/Trojan/Shadowsocks/Hysteria2 management through sing-box.

## Verification limitation

The artifact environment used to assemble this alpha does not meet the current upstream full build toolchain requirements and cannot directly download/run the release binary from shell. Full `make verify` against an unmodified upstream checkout plus real-binary four-protocol integration checks therefore remain required before production deployment.
