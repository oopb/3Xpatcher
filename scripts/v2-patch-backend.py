#!/usr/bin/env python3
import sys
from v2_patchlib import Patcher

rep = Patcher(sys.argv[1]).rep

# ---------------------------------------------------------------------------
# Backend: model enum acceptance + Xray/sing-box runtime isolation
# ---------------------------------------------------------------------------
rep(
    'internal/database/model/model.go',
    'validate:"required,oneof=vmess vless trojan shadowsocks wireguard hysteria http mixed tunnel tun mtproto amneziawg"',
    'validate:"required,oneof=vmess vless trojan shadowsocks wireguard hysteria http mixed tunnel tun mtproto amneziawg tuic anytls shadowtls naive"',
)

# Keep supplemental streamSettings because TLS fields reuse the native 3x-ui
# TLS editor. They are not sent to Xray; integrated.go translates them to
# sing-box TLS fields.
rep(
    'internal/web/service/inbound.go',
    '''\t\tmodel.WireGuard:   true,\n\t\tmodel.Tunnel:      true,\n\t}''',
    '''\t\tmodel.WireGuard:   true,\n\t\tmodel.Tunnel:      true,\n\t\tmodel.TUIC:        true,\n\t\tmodel.AnyTLS:      true,\n\t\tmodel.ShadowTLS:   true,\n\t\tmodel.Naive:       true,\n\t}''',
)

# Native AddInbound accepts imported/seeded clients for the supplemental
# protocols using the credentials each protocol actually consumes. Without
# this, the upstream default branch requires UUID/ID for AnyTLS/ShadowTLS/Naive.
rep(
    'internal/web/service/inbound.go',
    '''\t\tcase "hysteria":
\t\t\tif client.Auth == "" {
\t\t\t\treturn inbound, false, common.NewError("empty client ID")
\t\t\t}
\t\tcase "wireguard", "amneziawg":''',
    '''\t\tcase "hysteria":
\t\t\tif client.Auth == "" {
\t\t\t\treturn inbound, false, common.NewError("empty client ID")
\t\t\t}
\t\tcase "tuic":
\t\t\tif client.ID == "" || client.Password == "" {
\t\t\t\treturn inbound, false, common.NewError("TUIC client requires UUID and password")
\t\t\t}
\t\tcase "anytls", "shadowtls", "naive":
\t\t\tif client.Password == "" {
\t\t\t\treturn inbound, false, common.NewError("client password is required")
\t\t\t}
\t\tcase "wireguard", "amneziawg":''',
)

# Global ClientRecord credential defaults for supplemental protocols.
rep(
    'internal/web/service/client_crud.go',
    '''\tcase model.Hysteria:\n\t\tif c.Auth == "" {\n\t\t\tc.Auth = strings.ReplaceAll(uuid.NewString(), "-", "")\n\t\t}\n\tcase model.MTProto:''',
    '''\tcase model.Hysteria:\n\t\tif c.Auth == "" {\n\t\t\tc.Auth = strings.ReplaceAll(uuid.NewString(), "-", "")\n\t\t}\n\tcase model.TUIC:\n\t\tif c.ID == "" {\n\t\t\tc.ID = uuid.NewString()\n\t\t}\n\t\tif c.Password == "" {\n\t\t\tc.Password = strings.ReplaceAll(uuid.NewString(), "-", "")\n\t\t}\n\tcase model.AnyTLS, model.ShadowTLS, model.Naive:\n\t\tif c.Password == "" {\n\t\t\tc.Password = strings.ReplaceAll(uuid.NewString(), "-", "")\n\t\t}\n\tcase model.MTProto:''',
)

# Per-inbound client attach/update validation must mirror the credential model
# above. This path is used by the native Clients page, bulk add, attach/detach,
# and compatibility APIs.
rep(
    'internal/web/service/client_inbound_apply.go',
    '''\t\tcase "hysteria":
\t\t\tif client.Auth == "" {
\t\t\t\treturn false, common.NewError("empty client ID")
\t\t\t}
\t\tcase "wireguard", "amneziawg":''',
    '''\t\tcase "hysteria":
\t\t\tif client.Auth == "" {
\t\t\t\treturn false, common.NewError("empty client ID")
\t\t\t}
\t\tcase "tuic":
\t\t\tif client.ID == "" || client.Password == "" {
\t\t\t\treturn false, common.NewError("TUIC client requires UUID and password")
\t\t\t}
\t\tcase "anytls", "shadowtls", "naive":
\t\t\tif client.Password == "" {
\t\t\t\treturn false, common.NewError("client password is required")
\t\t\t}
\t\tcase "wireguard", "amneziawg":''',
)
rep(
    'internal/web/service/client_inbound_apply.go',
    '''\tcase "hysteria":
\t\tnewClientId = clients[0].Auth
\tcase "wireguard", "amneziawg":''',
    '''\tcase "hysteria":
\t\tnewClientId = clients[0].Auth
\tcase "tuic":
\t\tnewClientId = clients[0].ID
\t\tif clients[0].Password == "" {
\t\t\treturn false, common.NewError("TUIC client requires UUID and password")
\t\t}
\tcase "anytls", "shadowtls", "naive":
\t\tnewClientId = clients[0].Password
\tcase "wireguard", "amneziawg":''',
)

# Copy-inbound creates a new identity; give supplemental target protocols the
# credentials their runtime consumes. Normal attach/detach continues to reuse
# the existing global ClientRecord unchanged.
rep(
    'internal/web/service/inbound_clients.go',
    """\tcase model.Hysteria:
\t\ttarget.Auth = s.generateRandomCredential(targetProtocol)
\tcase model.MTProto:""",
    """\tcase model.Hysteria:
\t\ttarget.Auth = s.generateRandomCredential(targetProtocol)
\tcase model.TUIC:
\t\ttarget.ID = uuid.NewString()
\t\ttarget.Password = s.generateRandomCredential(targetProtocol)
\tcase model.AnyTLS, model.ShadowTLS, model.Naive:
\t\ttarget.Password = s.generateRandomCredential(targetProtocol)
\tcase model.MTProto:""",
)

# Xray config generation must never see a supplemental protocol. Also reconcile
# sing-box from canonical DB state whenever the panel rebuilds its local core.
rep(
    'internal/web/service/xray.go',
    '"github.com/mhsanaei/3x-ui/v3/internal/logger"\n',
    '"github.com/mhsanaei/3x-ui/v3/internal/logger"\n\tsbox "github.com/mhsanaei/3x-ui/v3/internal/singbox"\n',
)
rep(
    'internal/web/service/xray.go',
    '''\tinbounds, err := s.inboundService.GetAllInbounds()\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tfor _, inbound := range inbounds {''',
    '''\tinbounds, err := s.inboundService.GetAllInbounds()\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tif err := sbox.Reconcile(); err != nil {\n\t\tlogger.Warning("supplemental sing-box reconcile failed:", err)\n\t}\n\tfor _, inbound := range inbounds {''',
)
rep(
    'internal/web/service/xray.go',
    '''\t\tif inbound.Protocol == model.MTProto || inbound.Protocol == model.AmneziaWG {\n\t\t\tcontinue\n\t\t}\n''',
    '''\t\tif inbound.Protocol == model.MTProto || inbound.Protocol == model.AmneziaWG || model.IsSingboxProtocol(inbound.Protocol) {\n\t\t\tcontinue\n\t\t}\n''',
)

# Local runtime dispatch: DB/UI stays unified; only this last hop chooses core.
rep(
    'internal/web/runtime/local.go',
    '"github.com/mhsanaei/3x-ui/v3/internal/mtproto"\n',
    '"github.com/mhsanaei/3x-ui/v3/internal/mtproto"\n\tsbox "github.com/mhsanaei/3x-ui/v3/internal/singbox"\n',
)
rep(
    'internal/web/runtime/local.go',
    '''func (l *Local) AddInbound(_ context.Context, ib *model.Inbound) error {\n\tif ib.Protocol == model.MTProto {''',
    '''func (l *Local) AddInbound(_ context.Context, ib *model.Inbound) error {\n\tif model.IsSingboxProtocol(ib.Protocol) {\n\t\treturn sbox.Reconcile()\n\t}\n\tif ib.Protocol == model.MTProto {''',
)
rep(
    'internal/web/runtime/local.go',
    '''func (l *Local) DelInbound(_ context.Context, ib *model.Inbound) error {\n\tif ib.Protocol == model.MTProto {''',
    '''func (l *Local) DelInbound(_ context.Context, ib *model.Inbound) error {\n\tif model.IsSingboxProtocol(ib.Protocol) {\n\t\treturn sbox.Reconcile()\n\t}\n\tif ib.Protocol == model.MTProto {''',
)
rep(
    'internal/web/runtime/local.go',
    '''func (l *Local) UpdateInbound(ctx context.Context, oldIb, newIb *model.Inbound) error {\n\tif oldIb.Protocol == model.MTProto || newIb.Protocol == model.MTProto {''',
    '''func (l *Local) UpdateInbound(ctx context.Context, oldIb, newIb *model.Inbound) error {\n\toldSingbox := model.IsSingboxProtocol(oldIb.Protocol)\n\tnewSingbox := model.IsSingboxProtocol(newIb.Protocol)\n\tif oldSingbox || newSingbox {\n\t\tif oldSingbox && newSingbox {\n\t\t\treturn sbox.Reconcile()\n\t\t}\n\t\tif oldSingbox {\n\t\t\tif err := sbox.Reconcile(); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n\t\t\tif !newIb.Enable {\n\t\t\t\treturn nil\n\t\t\t}\n\t\t\treturn l.AddInbound(ctx, newIb)\n\t\t}\n\t\t_ = l.DelInbound(ctx, oldIb)\n\t\treturn sbox.Reconcile()\n\t}\n\tif oldIb.Protocol == model.MTProto || newIb.Protocol == model.MTProto {''',
)
rep(
    'internal/web/runtime/local.go',
    '''func (l *Local) AddUser(_ context.Context, ib *model.Inbound, userMap map[string]any) error {\n\tif ib.Protocol == model.MTProto || ib.Protocol == model.AmneziaWG {''',
    '''func (l *Local) AddUser(_ context.Context, ib *model.Inbound, userMap map[string]any) error {\n\tif model.IsSingboxProtocol(ib.Protocol) {\n\t\treturn sbox.Reconcile()\n\t}\n\tif ib.Protocol == model.MTProto || ib.Protocol == model.AmneziaWG {''',
)
rep(
    'internal/web/runtime/local.go',
    '''func (l *Local) RemoveUser(_ context.Context, ib *model.Inbound, email string) error {\n\tif ib.Protocol == model.MTProto || ib.Protocol == model.AmneziaWG {''',
    '''func (l *Local) RemoveUser(_ context.Context, ib *model.Inbound, email string) error {\n\tif model.IsSingboxProtocol(ib.Protocol) {\n\t\treturn sbox.Reconcile()\n\t}\n\tif ib.Protocol == model.MTProto || ib.Protocol == model.AmneziaWG {''',
)
