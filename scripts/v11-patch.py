#!/usr/bin/env python3
import sys
from v2_patchlib import Patcher

rep = Patcher(sys.argv[1]).rep

# ---------------------------------------------------------------------------
# Native inbound list parity: every supplemental multi-user protocol must
# participate in the same client rollup used by the upstream row-action menu.
# Without this, hasClients remains false and native detach/group/delete-all
# actions disappear even though client_inbounds contains attached clients.
# ---------------------------------------------------------------------------
rep(
    'frontend/src/pages/inbounds/useInbounds.ts',
    '''  Protocols.AMNEZIAWG,\n];''',
    '''  Protocols.AMNEZIAWG,\n  Protocols.TUIC,\n  Protocols.ANYTLS,\n  Protocols.SHADOWTLS,\n  Protocols.NAIVE,\n  Protocols.MIERU,\n];''',
)

# Use the column's existing fallbackClientCount() as well as the computed map.
# This keeps the menu correct during slim-query hydration / transient rollup
# gaps instead of hiding actions until another client-count refresh completes.
rep(
    'frontend/src/pages/inbounds/list/useInboundColumns.tsx',
    '''            hasClients={(clientCount[record.id]?.clients || 0) > 0}''',
    '''            hasClients={clientTotal(record) > 0}''',
)

# ---------------------------------------------------------------------------
# Native QR / row-export / client-info parity.
# The upstream browser-side share-link dispatcher is a separate implementation
# from the backend subscription service. Supplemental protocols therefore need
# to be wired into it explicitly; otherwise /sub works while the native panel's
# QR and "Export links" controls silently produce empty links.
# ---------------------------------------------------------------------------
rep(
    'frontend/src/lib/xray/inbound-link.ts',
    "import { deriveSpiderX } from './spider-x';\n",
    "import { deriveSpiderX } from './spider-x';\nimport { genSupplementalLinks, getSupplementalClients } from './supplemental-links';\n",
)

rep(
    'frontend/src/lib/xray/inbound-link.ts',
    '''    default:\n      return null;\n  }\n}\n\nexport interface GenLinkInput {''',
    '''    default:\n      return getSupplementalClients(inbound) as ClientShape[] | null;\n  }\n}\n\nexport interface GenLinkInput {''',
)

rep(
    'frontend/src/lib/xray/inbound-link.ts',
    '''    case 'mtproto':\n      return genMtprotoLink({ inbound, address, port, clientSecret: client.secret ?? '' });\n    default:\n      return '';\n  }\n}\n''',
    '''    case 'mtproto':\n      return genMtprotoLink({ inbound, address, port, clientSecret: client.secret ?? '' });\n    default:\n      return (\n        genSupplementalLinks({\n          inbound,\n          address,\n          port,\n          remark,\n          client,\n          externalProxy,\n        })[0]?.link ?? ''\n      );\n  }\n}\n''',
)

rep(
    'frontend/src/lib/xray/inbound-link.ts',
    '''  const externals = inbound.streamSettings?.externalProxy;\n  if (!externals || externals.length === 0) {\n    const r = composeRemark('');\n    return [\n      {\n        remark: r,\n        link: genLink({ inbound, address: addr, port, forceTls: 'same', remark: r, client }),\n      },\n    ];\n  }\n  return externals.map((ep) => {\n    const r = composeRemark(ep.remark);\n    return {\n      remark: r,\n      link: genLink({\n        inbound,\n        address: ep.dest,\n        port: ep.port,\n        forceTls: ep.forceTls,\n        remark: r,\n        client,\n        externalProxy: ep,\n      }),\n    };\n  });''',
    '''  const renderEndpoint = (\n    address: string,\n    endpointPort: number,\n    forceTls: ForceTls,\n    r: string,\n    ep: ExternalProxyEntry | null,\n  ): GenAllLinksEntry[] => {\n    const supplemental = genSupplementalLinks({\n      inbound,\n      address,\n      port: endpointPort,\n      remark: r,\n      client,\n      externalProxy: ep,\n    });\n    if (supplemental.length > 0) {\n      return supplemental.map((variant) => ({\n        remark: variant.label ? `${r} — ${variant.label}` : r,\n        link: variant.link,\n      }));\n    }\n    const link = genLink({\n      inbound,\n      address,\n      port: endpointPort,\n      forceTls,\n      remark: r,\n      client,\n      externalProxy: ep,\n    });\n    return link ? [{ remark: r, link }] : [];\n  };\n\n  const externals = inbound.streamSettings?.externalProxy;\n  if (!externals || externals.length === 0) {\n    const r = composeRemark('');\n    return renderEndpoint(addr, port, 'same', r, null);\n  }\n  return externals.flatMap((ep) => {\n    const r = composeRemark(ep.remark);\n    return renderEndpoint(ep.dest, ep.port, ep.forceTls, r, ep);\n  });''',
)
