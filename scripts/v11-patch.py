#!/usr/bin/env python3
import sys
from v2_patchlib import Patcher

rep = Patcher(sys.argv[1]).rep

# ---------------------------------------------------------------------------
# Native inbound list parity: every supplemental multi-user protocol must
# participate in the same client rollup used by the upstream row-action menu.
# ---------------------------------------------------------------------------
rep(
    'frontend/src/pages/inbounds/useInbounds.ts',
    '''  Protocols.AMNEZIAWG,\n];''',
    '''  Protocols.AMNEZIAWG,\n  Protocols.TUIC,\n  Protocols.ANYTLS,\n  Protocols.SHADOWTLS,\n  Protocols.NAIVE,\n  Protocols.MIERU,\n];''',
)
rep(
    'frontend/src/pages/inbounds/list/useInboundColumns.tsx',
    '''            hasClients={(clientCount[record.id]?.clients || 0) > 0}''',
    '''            hasClients={clientTotal(record) > 0}''',
)
rep(
    'frontend/src/pages/inbounds/list/types.ts',
    '''  | 'delAllClients'\n  | 'clone';''',
    '''  | 'delAllClients'\n  | 'attachClients'\n  | 'attachExisting'\n  | 'detachClients'\n  | 'addToGroup'\n  | 'clone';''',
)

# Zod defaults are part of the inferred settings type. Factory defaults must
# explicitly provide every field that has a schema .default().
rep(
    'frontend/src/lib/xray/inbound-defaults.ts',
    '''  return { clients: [], congestionControl: 'cubic', authTimeout: '3s', zeroRTTHandshake: false, heartbeat: '10s' };''',
    '''  return {\n    clients: [],\n    congestionControl: 'cubic',\n    authTimeout: '3s',\n    zeroRTTHandshake: false,\n    heartbeat: '10s',\n    bindInterface: '',\n    routingMark: 0,\n    reuseAddr: false,\n    netns: '',\n    tcpFastOpen: false,\n    tcpMultiPath: false,\n    disableTCPKeepAlive: false,\n    tcpKeepAlive: '',\n    tcpKeepAliveInterval: '',\n    udpTimeout: '',\n    idleTimeout: '',\n    keepAlivePeriod: '',\n    maxConcurrentStreams: 0,\n    initialPacketSize: 0,\n    disablePathMTUDiscovery: false,\n  };''',
)
rep(
    'frontend/src/lib/xray/inbound-defaults.ts',
    '''  return { clients: [], paddingScheme: [] };''',
    '''  return {\n    clients: [],\n    paddingScheme: [],\n    bindInterface: '',\n    routingMark: 0,\n    reuseAddr: false,\n    netns: '',\n    tcpFastOpen: false,\n    tcpMultiPath: false,\n    disableTCPKeepAlive: false,\n    tcpKeepAlive: '',\n    tcpKeepAliveInterval: '',\n    udpTimeout: '',\n  };''',
)
rep(
    'frontend/src/lib/xray/inbound-defaults.ts',
    '''    innerPassword: RandomUtil.randomShadowsocksPassword('2022-blake3-aes-128-gcm'),\n  };''',
    '''    innerPassword: RandomUtil.randomShadowsocksPassword('2022-blake3-aes-128-gcm'),\n    handshakeForServerNameJson: '',\n    bindInterface: '',\n    routingMark: 0,\n    reuseAddr: false,\n    netns: '',\n    tcpFastOpen: false,\n    tcpMultiPath: false,\n    disableTCPKeepAlive: false,\n    tcpKeepAlive: '',\n    tcpKeepAliveInterval: '',\n    udpTimeout: '',\n  };''',
)
rep(
    'frontend/src/lib/xray/inbound-defaults.ts',
    '''  return { clients: [], network: '', quicCongestionControl: 'bbr' };''',
    '''  return {\n    clients: [],\n    network: '',\n    quicCongestionControl: 'bbr',\n    bindInterface: '',\n    routingMark: 0,\n    reuseAddr: false,\n    netns: '',\n    tcpFastOpen: false,\n    tcpMultiPath: false,\n    disableTCPKeepAlive: false,\n    tcpKeepAlive: '',\n    tcpKeepAliveInterval: '',\n    udpTimeout: '',\n  };''',
)

# Ant Design 6 renamed Divider's left/right values to start/end.
rep('frontend/src/pages/inbounds/form/protocols/singbox.tsx', 'orientation="left"', 'orientation="start"', count=50)
rep('frontend/src/pages/inbounds/form/security/tls.tsx', 'orientation="left"', 'orientation="start"', count=50)

# ---------------------------------------------------------------------------
# Native QR / row-export / client-info share links.
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
    '''    case 'mtproto':\n      return genMtprotoLink({ inbound, address, port, clientSecret: client.secret ?? '' });\n    default:\n      return (\n        genSupplementalLinks({ inbound, address, port, remark, client, externalProxy })[0]?.link ?? ''\n      );\n  }\n}\n''',
)
rep(
    'frontend/src/lib/xray/inbound-link.ts',
    '''  const externals = inbound.streamSettings?.externalProxy;\n  if (!externals || externals.length === 0) {\n    const r = composeRemark('');\n    return [\n      {\n        remark: r,\n        link: genLink({ inbound, address: addr, port, forceTls: 'same', remark: r, client }),\n      },\n    ];\n  }\n  return externals.map((ep) => {\n    const r = composeRemark(ep.remark);\n    return {\n      remark: r,\n      link: genLink({\n        inbound,\n        address: ep.dest,\n        port: ep.port,\n        forceTls: ep.forceTls,\n        remark: r,\n        client,\n        externalProxy: ep,\n      }),\n    };\n  });''',
    '''  const renderEndpoint = (\n    address: string,\n    endpointPort: number,\n    forceTls: ForceTls,\n    r: string,\n    ep: ExternalProxyEntry | null,\n  ): GenAllLinksEntry[] => {\n    const supplemental = genSupplementalLinks({\n      inbound, address, port: endpointPort, remark: r, client, externalProxy: ep,\n    });\n    if (supplemental.length > 0) {\n      return supplemental.map((variant) => ({\n        remark: variant.label ? `${r} — ${variant.label}` : r,\n        link: variant.link,\n      }));\n    }\n    const link = genLink({\n      inbound, address, port: endpointPort, forceTls, remark: r, client, externalProxy: ep,\n    });\n    return link ? [{ remark: r, link }] : [];\n  };\n\n  const externals = inbound.streamSettings?.externalProxy;\n  if (!externals || externals.length === 0) {\n    const r = composeRemark('');\n    return renderEndpoint(addr, port, 'same', r, null);\n  }\n  return externals.flatMap((ep) => {\n    const r = composeRemark(ep.remark);\n    return renderEndpoint(ep.dest, ep.port, ep.forceTls, r, ep);\n  });''',
)

# ---------------------------------------------------------------------------
# Dedicated Clash/Mihomo subscription parity on the native Inbounds surfaces.
# Clients already exposes subClashURI; Inbounds previously dropped it, which
# made Clash Verge users discover only the raw subscription path.
# ---------------------------------------------------------------------------
rep(
    'frontend/src/pages/inbounds/useInbounds.ts',
    '''  subJsonURI: string;\n  subJsonEnable: boolean;''',
    '''  subJsonURI: string;\n  subJsonEnable: boolean;\n  subClashURI: string;\n  subClashEnable: boolean;''',
)
rep(
    'frontend/src/pages/inbounds/useInbounds.ts',
    '''      subJsonURI: defaults.subJsonURI || '',\n      subJsonEnable: !!defaults.subJsonEnable,\n      publicHost:''',
    '''      subJsonURI: defaults.subJsonURI || '',\n      subJsonEnable: !!defaults.subJsonEnable,\n      subClashURI: defaults.subClashURI || '',\n      subClashEnable: !!defaults.subClashEnable,\n      publicHost:''',
)
rep(
    'frontend/src/pages/inbounds/useInbounds.ts',
    '''      defaults.subJsonURI,\n      defaults.subJsonEnable,\n      defaults.subDomain,''',
    '''      defaults.subJsonURI,\n      defaults.subJsonEnable,\n      defaults.subClashURI,\n      defaults.subClashEnable,\n      defaults.subDomain,''',
)

# Export both standard and dedicated Clash subscription URLs from a row/all rows.
rep(
    'frontend/src/pages/inbounds/InboundsPage.tsx',
    '''        if (c.subId && subSettings.subURI) {\n          subLinks.push(subSettings.subURI + c.subId);\n        }''',
    '''        if (c.subId) {\n          if (subSettings.subURI) subLinks.push(subSettings.subURI + c.subId);\n          if (subSettings.subClashEnable && subSettings.subClashURI) {\n            subLinks.push(subSettings.subClashURI + c.subId);\n          }\n        }''',
    count=2,
)

# QR modal: show Clash as its own scannable subscription entry.
rep(
    'frontend/src/pages/inbounds/qr/QrCodeModal.tsx',
    '''  const [subLink, setSubLink] = useState('');\n  const [subJsonLink, setSubJsonLink] = useState('');''',
    '''  const [subLink, setSubLink] = useState('');\n  const [subJsonLink, setSubJsonLink] = useState('');\n  const [subClashLink, setSubClashLink] = useState('');''',
)
rep(
    'frontend/src/pages/inbounds/qr/QrCodeModal.tsx',
    '''    let nextSub = '';\n    let nextSubJson = '';\n    if (subSettings?.enable && subId) {\n      nextSub = (subSettings.subURI || '') + subId;\n      nextSubJson = subSettings.subJsonEnable ? (subSettings.subJsonURI || '') + subId : '';\n    }\n    setSubLink(nextSub);\n    setSubJsonLink(nextSubJson);''',
    '''    let nextSub = '';\n    let nextSubJson = '';\n    let nextSubClash = '';\n    if (subSettings?.enable && subId) {\n      nextSub = (subSettings.subURI || '') + subId;\n      nextSubJson = subSettings.subJsonEnable ? (subSettings.subJsonURI || '') + subId : '';\n      nextSubClash = subSettings.subClashEnable ? (subSettings.subClashURI || '') + subId : '';\n    }\n    setSubLink(nextSub);\n    setSubJsonLink(nextSubJson);\n    setSubClashLink(nextSubClash);''',
)
rep(
    'frontend/src/pages/inbounds/qr/QrCodeModal.tsx',
    '''    if (subJsonLink) {\n      items.push({\n        key: 'sub-json',\n        header: `${t('subscription.title')} (JSON)`,\n        value: subJsonLink,\n      });\n    }''',
    '''    if (subJsonLink) {\n      items.push({\n        key: 'sub-json',\n        header: `${t('subscription.title')} (JSON)`,\n        value: subJsonLink,\n      });\n    }\n    if (subClashLink) {\n      items.push({\n        key: 'sub-clash',\n        header: `${t('subscription.title')} (Clash / Mihomo)`,\n        value: subClashLink,\n      });\n    }''',
)
rep(
    'frontend/src/pages/inbounds/qr/QrCodeModal.tsx',
    '''    subJsonLink,\n    links,''',
    '''    subJsonLink,\n    subClashLink,\n    links,''',
)

# Inbound client-info modal: mirror the Clients page's Clash URL card.
rep(
    'frontend/src/pages/inbounds/info/InboundInfoModal.tsx',
    '''  const [subLink, setSubLink] = useState('');\n  const [subJsonLink, setSubJsonLink] = useState('');''',
    '''  const [subLink, setSubLink] = useState('');\n  const [subJsonLink, setSubJsonLink] = useState('');\n  const [subClashLink, setSubClashLink] = useState('');''',
)
rep(
    'frontend/src/pages/inbounds/info/InboundInfoModal.tsx',
    '''      setSubJsonLink(\n        subSettings?.subJsonEnable ? (subSettings?.subJsonURI || '') + clientSet.subId : '',\n      );\n    } else {\n      setSubLink('');\n      setSubJsonLink('');\n    }''',
    '''      setSubJsonLink(\n        subSettings?.subJsonEnable ? (subSettings?.subJsonURI || '') + clientSet.subId : '',\n      );\n      setSubClashLink(\n        subSettings?.subClashEnable ? (subSettings?.subClashURI || '') + clientSet.subId : '',\n      );\n    } else {\n      setSubLink('');\n      setSubJsonLink('');\n      setSubClashLink('');\n    }''',
)
rep(
    'frontend/src/pages/inbounds/info/InboundInfoModal.tsx',
    '''          {subSettings?.subJsonEnable && subJsonLink && (\n            <div className="link-panel">\n              <div className="link-panel-header">\n                <Tag color="green">JSON</Tag>\n                <Tooltip title={t('copy')}>\n                  <Button\n                    size="small"\n                    icon={<CopyOutlined />}\n                    aria-label={t('copy')}\n                    onClick={() => copyText(subJsonLink, t)}\n                  />\n                </Tooltip>\n              </div>\n              <a\n                href={subJsonLink}\n                target="_blank"\n                rel="noopener noreferrer"\n                className="link-panel-anchor"\n              >\n                {subJsonLink}\n              </a>\n            </div>\n          )}''',
    '''          {subSettings?.subJsonEnable && subJsonLink && (\n            <div className="link-panel">\n              <div className="link-panel-header">\n                <Tag color="green">JSON</Tag>\n                <Tooltip title={t('copy')}>\n                  <Button size="small" icon={<CopyOutlined />} aria-label={t('copy')} onClick={() => copyText(subJsonLink, t)} />\n                </Tooltip>\n              </div>\n              <a href={subJsonLink} target="_blank" rel="noopener noreferrer" className="link-panel-anchor">\n                {subJsonLink}\n              </a>\n            </div>\n          )}\n          {subSettings?.subClashEnable && subClashLink && (\n            <div className="link-panel">\n              <div className="link-panel-header">\n                <Tag color="blue">Clash / Mihomo</Tag>\n                <Tooltip title={t('copy')}>\n                  <Button size="small" icon={<CopyOutlined />} aria-label={t('copy')} onClick={() => copyText(subClashLink, t)} />\n                </Tooltip>\n              </div>\n              <a href={subClashLink} target="_blank" rel="noopener noreferrer" className="link-panel-anchor">\n                {subClashLink}\n              </a>\n            </div>\n          )}''',
)
