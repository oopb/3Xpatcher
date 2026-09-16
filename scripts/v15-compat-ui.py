#!/usr/bin/env python3
from pathlib import Path
import sys

if len(sys.argv) != 2:
    raise SystemExit(f"Usage: {sys.argv[0]} /path/to/3x-ui-source")

root = Path(sys.argv[1]).resolve()


def replace_once(rel: str, old: str, new: str, label: str) -> None:
    path = root / rel
    text = path.read_text(encoding="utf-8")
    if new in text and old not in text:
        return
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"v15 ui {label}: expected exactly one anchor, found {count}")
    path.write_text(text.replace(old, new, 1), encoding="utf-8")


def remove_once(rel: str, old: str, label: str) -> None:
    path = root / rel
    text = path.read_text(encoding="utf-8")
    count = text.count(old)
    if count == 0:
        return
    if count != 1:
        raise SystemExit(f"v15 ui {label}: expected at most one anchor, found {count}")
    path.write_text(text.replace(old, "", 1), encoding="utf-8")


# v3.8.x includes native TUIC among client-attachable protocols. Preserve TUIC
# and append only the additional supplemental protocols owned by 3Xpatcher.
for rel in (
    "frontend/src/pages/clients/ClientFormModal.tsx",
    "frontend/src/pages/clients/ClientBulkAddModal.tsx",
):
    replace_once(
        rel,
        "  'amneziawg',\n  'tuic',\n]);",
        "  'amneziawg',\n  'tuic',\n  'anytls',\n  'shadowtls',\n  'naive',\n]);",
        f"{rel} attachable protocols",
    )

# Keep the upstream TUIC client helper/type available, but detach the native
# TUIC inbound-settings type and factory. 3Xpatcher's TUIC runtime is sing-box,
# whose settings schema is intentionally different from the v3.8 sidecar shape.
defaults = "frontend/src/lib/xray/inbound-defaults.ts"
replace_once(
    defaults,
    "import type { TuicClient, TuicInboundSettings } from '@/schemas/protocols/inbound/tuic';\n",
    "import type { TuicClient } from '@/schemas/protocols/inbound/tuic';\n",
    "native TUIC inbound settings import",
)
remove_once(
    defaults,
    "export function createDefaultTuicInboundSettings(): TuicInboundSettings {\n  return {\n    server: {\n      certificate: '',\n      private_key: '',\n      congestion_control: 'bbr',\n      alpn: ['h3', 'spdy/3.1'],\n      udp_relay_mode: 'native',\n      zero_rtt_handshake: true,\n      log_level: 'info',\n      max_idle_time: 15,\n      authentication_timeout: 3,\n      max_udp_relay_packet_size: 1500,\n      sni: '',\n    },\n    clients: [],\n  };\n}\n\n",
    "native TUIC inbound defaults factory",
)
replace_once(
    defaults,
    "  | MtprotoInboundSettings\n  | AmneziawgInboundSettings\n  | TuicInboundSettings;",
    "  | MtprotoInboundSettings\n  | AmneziawgInboundSettings;",
    "native TUIC inbound settings union",
)
remove_once(
    defaults,
    "    case 'tuic':\n      return createDefaultTuicInboundSettings();\n",
    "native TUIC defaults dispatch",
)

# Native v3.8 TUIC is already listed in the protocol-tab whitelist. Extend that
# list with the other supplemental protocols instead of trying to add TUIC twice.
replace_once(
    "frontend/src/pages/inbounds/form/InboundFormModal.tsx",
    "                    Protocols.AMNEZIAWG,\n                    Protocols.TUIC,\n                  ] as string[]",
    "                    Protocols.AMNEZIAWG,\n                    Protocols.TUIC,\n                    Protocols.ANYTLS,\n                    Protocols.SHADOWTLS,\n                    Protocols.NAIVE,\n                  ] as string[]",
    "native TUIC protocol-tab whitelist",
)

# The list helper also treats native TUIC as a multi-user protocol in v3.8.
# Keep that behavior and append the remaining supplemental protocols.
replace_once(
    "frontend/src/pages/inbounds/list/helpers.ts",
    "    case 'amneziawg':\n    case 'tuic':\n      return true;",
    "    case 'amneziawg':\n    case 'tuic':\n    case 'anytls':\n    case 'shadowtls':\n    case 'naive':\n      return true;",
    "v3.8 multi-user protocol helper",
)

# v11 extends the protocol set used by the native inbound client roll-up.
# 3x-ui v3.8 already contributes TUIC, so normalize directly to v11's final
# set (including Mieru) and let the legacy v11 patch recognize it as applied.
replace_once(
    "frontend/src/pages/inbounds/useInbounds.ts",
    "  Protocols.AMNEZIAWG,\n  Protocols.TUIC,\n];",
    "  Protocols.AMNEZIAWG,\n  Protocols.TUIC,\n  Protocols.ANYTLS,\n  Protocols.SHADOWTLS,\n  Protocols.NAIVE,\n  Protocols.MIERU,\n];",
    "v3.8 inbound client roll-up protocols",
)

# v3.8's native share-link dispatcher gained its own TUIC case. 3Xpatcher TUIC
# uses the sing-box settings shape, so remove only that dispatch arm and let
# v11's supplemental fallback handle TUIC together with the other extra cores.
remove_once(
    "frontend/src/lib/xray/inbound-link.ts",
    "    case 'tuic':\n      return genTuicLink({\n        inbound,\n        address,\n        port,\n        remark,\n        clientUuid: client.uuid ?? client.id ?? '',\n        clientPassword: client.password ?? '',\n        externalProxy,\n      });\n",
    "native TUIC genLink dispatch",
)

# InboundInfoModal's share-link display whitelist gained native TUIC in v3.8.
# Normalize directly to the v11-final supplemental set so valid generated links
# remain visible for all extra runtimes.
replace_once(
    "frontend/src/pages/inbounds/info/helpers.ts",
    "  Protocols.HYSTERIA,\n  Protocols.MTPROTO,\n  Protocols.TUIC,\n]);",
    "  Protocols.HYSTERIA,\n  Protocols.MTPROTO,\n  Protocols.TUIC,\n  Protocols.ANYTLS,\n  Protocols.SHADOWTLS,\n  Protocols.NAIVE,\n  Protocols.MIERU,\n]);",
    "v3.8 inbound info share-link whitelist",
)

print("V15 frontend compatibility normalization applied.")
