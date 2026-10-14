#!/usr/bin/env python3
"""v3.10 MASQUE-safe normalization around legacy exact-match anchors.
The pre pass touches only the downloaded build tree; the post pass restores
the original MASQUE functionality. Older releases skip both passes.
"""
from pathlib import Path
import sys
root=Path(sys.argv[1]).resolve()
stage=sys.argv[2]
flag=root/".3xpatcher-upstream-310"

def edit(rel,old,new):
    path=root/rel
    value=path.read_text(encoding="utf-8")
    n=value.count(old)
    if n!=1: raise SystemExit(f"v24 {stage}: {rel} anchor {old[:65]!r}: {n} matches")
    path.write_text(value.replace(old,new,1),encoding="utf-8")

if stage=="pre":
    edit("internal/database/model/model.go",'amneziawg tuic masque"','amneziawg tuic"')
    edit("internal/sub/service.go","'mtproto','tuic','masque')","'mtproto','tuic')")
    edit("frontend/src/schemas/primitives/protocol.ts","  'tuic',\n  'masque',\n]);","  'tuic',\n]);")
    edit("frontend/src/schemas/primitives/protocol.ts","  TUIC: 'tuic',\n  MASQUE: 'masque',\n});","  TUIC: 'tuic',\n});")
    edit("frontend/src/schemas/protocols/inbound/index.ts","  z.object({ protocol: z.literal('masque'), settings: MasqueInboundSettingsSchema }),\n","")
    edit("frontend/src/lib/xray/inbound-defaults.ts","  | TuicInboundSettings\n  | MasqueInboundSettings;","  | TuicInboundSettings;")
    edit("frontend/src/lib/xray/inbound-defaults.ts","    case 'masque':\n      return createDefaultMasqueInboundSettings();\n","")
    edit("frontend/src/pages/inbounds/form/InboundFormModal.tsx","                    Protocols.TUIC,\n                    Protocols.MASQUE,\n","                    Protocols.TUIC,\n")
    edit("frontend/src/pages/inbounds/list/helpers.ts","    case 'tuic':\n    case 'masque':\n","    case 'tuic':\n")
    edit("frontend/src/pages/inbounds/useInbounds.ts","  Protocols.TUIC,\n  Protocols.MASQUE,\n];","  Protocols.TUIC,\n];")
    for name in ("ClientFormModal.tsx","ClientBulkAddModal.tsx"):
        edit("frontend/src/pages/clients/"+name,"  'tuic',\n  'masque',\n]);","  'tuic',\n]);")
    edit("frontend/src/lib/xray/protocol-capabilities.ts",
         "if (values.protocol === 'hysteria' || values.protocol === 'masque') return true;",
         "if (values.protocol === 'hysteria') return true;")
    edit("internal/web/service/inbound.go",
         "\t\tmodel.Tunnel:      true,\n\t\tmodel.MASQUE:      true,\n\t}",
         "\t\tmodel.Tunnel:      true,\n\t}")
    flag.write_text("v3.10.0\n")
elif stage=="post":
    if not flag.exists(): raise SystemExit("v24 post without pre")
    edit("internal/database/model/model.go",
         'amneziawg tuic anytls shadowtls naive snell mieru"',
         'amneziawg tuic masque anytls shadowtls naive snell mieru"')
    edit("internal/sub/service.go",
         "'mtproto','tuic','anytls','shadowtls','naive','snell','mieru')",
         "'mtproto','tuic','masque','anytls','shadowtls','naive','snell','mieru')")
    edit("frontend/src/schemas/primitives/protocol.ts",
         "  'tuic',\n  'anytls',","  'tuic',\n  'masque',\n  'anytls',")
    edit("frontend/src/schemas/primitives/protocol.ts",
         "  TUIC: 'tuic',\n  ANYTLS:","  TUIC: 'tuic',\n  MASQUE: 'masque',\n  ANYTLS:")
    edit("frontend/src/schemas/protocols/inbound/index.ts",
         "  z.object({ protocol: z.literal('tuic'), settings: TuicInboundSettingsSchema }),\n",
         "  z.object({ protocol: z.literal('tuic'), settings: TuicInboundSettingsSchema }),\n  z.object({ protocol: z.literal('masque'), settings: MasqueInboundSettingsSchema }),\n")
    edit("frontend/src/lib/xray/inbound-defaults.ts",
         ";\n\nexport function createDefaultInboundSettings(",
         "\n  | MasqueInboundSettings;\n\nexport function createDefaultInboundSettings(")
    edit("frontend/src/lib/xray/inbound-defaults.ts",
         "    default:\n      return null;\n  }\n}",
         "    case 'masque':\n      return createDefaultMasqueInboundSettings();\n    default:\n      return null;\n  }\n}")
    edit("frontend/src/pages/inbounds/form/InboundFormModal.tsx",
         "                    Protocols.TUIC,\n                    Protocols.ANYTLS,",
         "                    Protocols.TUIC,\n                    Protocols.MASQUE,\n                    Protocols.ANYTLS,")
    edit("frontend/src/pages/inbounds/list/helpers.ts",
         "    case 'tuic':\n","    case 'tuic':\n    case 'masque':\n")
    edit("frontend/src/pages/inbounds/useInbounds.ts",
         "  Protocols.TUIC,\n  Protocols.ANYTLS,",
         "  Protocols.TUIC,\n  Protocols.MASQUE,\n  Protocols.ANYTLS,")
    for name in ("ClientFormModal.tsx","ClientBulkAddModal.tsx"):
        edit("frontend/src/pages/clients/"+name,
             "  'tuic',\n  'anytls',", "  'tuic',\n  'masque',\n  'anytls',")
    edit("frontend/src/lib/xray/protocol-capabilities.ts",
         "if (values.protocol === 'hysteria'",
         "if (values.protocol === 'hysteria' || values.protocol === 'masque'")
    edit("internal/web/service/inbound.go",
         "\t\tmodel.Tunnel:      true,\n\t\tmodel.TUIC:        true,",
         "\t\tmodel.Tunnel:      true,\n\t\tmodel.MASQUE:      true,\n\t\tmodel.TUIC:        true,")
    flag.unlink()
else:
    raise SystemExit("usage: v24-310-compat.py SOURCE pre|post")
print(f"3x-ui v3.10 MASQUE-native compatibility {stage}: OK")
