#!/usr/bin/env python3
"""Add client-side XHTTP downloadSettings to the native 3x-ui inbound editor/exporters.

This deliberately stores settings on the inbound row for subscription generation
but strips them when producing the Xray *server* listener. A split XHTTP client
MUST terminate both HTTP connections in the same backend listener/session.
"""
from pathlib import Path
import sys

root = Path(sys.argv[1])

def patch(path, old, new):
    target = root / path
    raw = target.read_text()
    if new in raw:
        return
    if old not in raw:
        raise SystemExit(f"XHTTP split patch anchor missing: {path}: {old[:100]!r}")
    target.write_text(raw.replace(old, new, 1))

schema = 'frontend/src/schemas/protocols/stream/xhttp.ts'
patch(schema,
    'export const XHttpStreamSettingsSchema = z.preprocess(',
    '''// Client outbound-only preferences. Stored on the inbound to generate
// subscriptions; NEVER passed to the Xray server's XHTTP listener.
export const XHttpDownloadSettingsSchema = z.object({
  address: z.string().default(''),
  port: z.number().int().min(0).max(65535).default(0),
  security: z.enum(['tls', 'reality']).default('tls'),
  network: z.literal('xhttp').default('xhttp'),
  xhttpSettings: z.object({ host: z.string().default('') }).default({ host: '' }),
  tlsSettings: z.object({
    serverName: z.string().default(''),
    fingerprint: z.string().default('chrome'),
    alpn: z.array(z.string()).default(['h2']),
  }).default({ serverName: '', fingerprint: 'chrome', alpn: ['h2'] }),
  realitySettings: z.object({
    serverName: z.string().default(''),
    fingerprint: z.string().default('chrome'),
    publicKey: z.string().default(''),
    shortId: z.string().default(''),
  }).default({ serverName: '', fingerprint: 'chrome', publicKey: '', shortId: '' }),
});
export type XHttpDownloadSettings = z.infer<typeof XHttpDownloadSettingsSchema>;

export const XHttpStreamSettingsSchema = z.preprocess(''')
patch(schema,
    '    enableXmux: z.boolean().default(false),',
    '''    enableXmux: z.boolean().default(false),
    // UI-only switch: do not infer its state from nested Controller fields.
    enableDownloadSettings: z.boolean().default(false),
    downloadSettings: XHttpDownloadSettingsSchema.optional(),''')

# The enable flag is UI-only. Removing a disabled subobject before validation
# makes it possible to switch OFF even after entering invalid download fields.
patch(schema,
    '  delete o.sessionKey;\n  return o;',
    '''  delete o.sessionKey;
  if (o.enableDownloadSettings === false) delete o.downloadSettings;
  return o;''')

ui = 'frontend/src/pages/inbounds/form/transport/xhttp.tsx'
patch(ui,
    "import { AutoComplete, Input, InputNumber, Select, Switch } from 'antd';",
    "import { Alert, AutoComplete, Divider, Input, InputNumber, Select, Switch } from 'antd';")
patch(ui,
    "import { int32RangeUpper } from '@/lib/xray/stream-wire-normalize';",
    """import { int32RangeUpper } from '@/lib/xray/stream-wire-normalize';
import { UTLS_FINGERPRINT } from '@/schemas/primitives';""")
patch(ui,
    "  const enableXmux = !!useWatch({ control, name: 'streamSettings.xhttpSettings.enableXmux' });",
    """  const enableXmux = !!useWatch({ control, name: 'streamSettings.xhttpSettings.enableXmux' });
  // A separate flag prevents still-mounted child FormField controllers from
  // resurrecting the downloadSettings object immediately after switch-off.
  const clientDownload = !!useWatch({
    control, name: 'streamSettings.xhttpSettings.enableDownloadSettings',
  });
  const downloadSecurity = useWatch({
    control, name: 'streamSettings.xhttpSettings.downloadSettings.security',
  });
  const protocol = useWatch({ control, name: 'protocol' });

  function onDownloadToggle(checked: boolean) {
    const base = 'streamSettings.xhttpSettings.downloadSettings';
    setValue('streamSettings.xhttpSettings.enableDownloadSettings', checked, {
      shouldDirty: true,
      shouldTouch: true,
    });
    // Keep the nested data temporarily so React Hook Form can unmount the
    // child controllers cleanly. The schema/wire serializer deletes it when
    // the switch is off, including any malformed unsaved values.
    if (!checked) return;
    if (getValues(base)) return;
    const primarySecurity = getValues('streamSettings.security');
    const primaryReality = getValues('streamSettings.realitySettings') ?? {};
    const primaryTls = getValues('streamSettings.tlsSettings') ?? {};
    const serverName =
      primarySecurity === 'reality'
        ? (primaryReality.settings?.serverName || primaryReality.serverNames?.[0] || '')
        : (primaryTls.serverName || '');
    const configuredAddress = getValues('shareAddr') || getValues('listen') || '';
    const address = ['0.0.0.0', '::', '127.0.0.1', 'localhost'].includes(configuredAddress)
      ? '' : configuredAddress;
    setValue(base, {
      address,
      port: Number(getValues('port')) || 443,
      security: primarySecurity === 'reality' ? 'reality' : 'tls',
      network: 'xhttp',
      xhttpSettings: { host: getValues('streamSettings.xhttpSettings.host') || '' },
      tlsSettings: {
        serverName, fingerprint: primaryTls.settings?.fingerprint || 'chrome',
        alpn: primaryTls.alpn?.length ? primaryTls.alpn : ['h2'],
      },
      realitySettings: {
        serverName,
        fingerprint: primaryReality.settings?.fingerprint || 'chrome',
        publicKey: primaryReality.settings?.publicKey || '',
        shortId: primaryReality.shortIds?.[0] || '',
      },
    }, { shouldDirty: true });
  }""")
# Form inserted at the end of XHTTP's transport card, after native XMUX controls.
needle='''      {enableXmux && (
        <>'''
assert needle in (root/ui).read_text()
raw=(root/ui).read_text()
anchor='''      )}
    </>
  );
}
'''
assert raw.endswith(anchor), 'unexpected end of upstream xhttp form'
extra='''      {protocol === 'vless' && (
        <>
          <Divider>Download Settings (XHTTP 上下行分离)</Divider>
          <FormField
            name={['streamSettings', 'xhttpSettings', 'downloadSettings']}
            label="启用独立下行连接"
          >
            <Switch
              checked={!!clientDownload}
              onChange={onDownloadToggle}
            />
          </FormField>
          {!!clientDownload && (
            <>
              <Alert
                type="info"
                showIcon
                message="仅用于客户端订阅/分享链接，不会写入 Xray 服务端 listener。上下行必须汇聚到同一个 XHTTP 入站与 Path；CDN 回源、TLS/REALITY 分流需事先配置。"
                style={{ marginBottom: 12 }}
              />
              <FormField
                name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'address']}
                label="下行连接地址 (IP / 域名)"
                rules={{ required: '请填写下行连接地址' }}
              >
                <Input placeholder="VPS_IP 或 cdn.example.com" />
              </FormField>
              <FormField
                name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'port']}
                label="下行端口"
                rules={{ min: { value: 1, message: '端口必须大于 0' } }}
              >
                <InputNumber min={1} max={65535} style={{ width: '100%' }} />
              </FormField>
              <FormField
                name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'security']}
                label="下行安全层"
                onAfterChange={(value) => {
                  // A TLS download uses the existing upstream TLS listener.
                  // Any previously managed REALITY sidecar must be disabled.
                  if (value === 'tls') {
                    setValue('streamSettings.xhttpSettings.downloadSettings.managedListener.enabled',
                      false, { shouldDirty: true });
                  }
                }}
              >
                <Select
                  options={[{value:'tls',label:'TLS (支持 CDN)'},{value:'reality',label:'REALITY (源站直连)'}]}
                />
              </FormField>
              {downloadSecurity === 'reality' ? (
                <>
                  <Alert
                    type="warning"
                    showIcon
                    message="兼容性：Mihomo 官方不支持 Xray-core v26.7.11+ 的 REALITY 变更。Clash Verge 请优先选择 TLS 直连下行（源站 IP + 原 TLS 端口与证书 SNI），不需要额外 REALITY 入口；Xray 客户端可使用 REALITY 分离。"
                    style={{ marginBottom: 12 }}
                  />
                  <FormField
                    name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'realitySettings', 'serverName']}
                    label="REALITY SNI"
                    rules={{ required: 'REALITY 需要 SNI' }}
                  ><Input placeholder="reality.example.com" /></FormField>
                  <FormField
                    name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'realitySettings', 'publicKey']}
                    label="REALITY Public Key"
                    rules={{ required: 'REALITY 需要公钥' }}
                  ><Input /></FormField>
                  <FormField
                    name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'realitySettings', 'shortId']}
                    label="REALITY Short ID"
                  ><Input /></FormField>
                  <FormField
                    name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'realitySettings', 'fingerprint']}
                    label="REALITY uTLS"
                  ><Select options={Object.values(UTLS_FINGERPRINT).map(fp=>({value:fp,label:fp}))} /></FormField>
                </>
              ) : (
                <>
                  <FormField
                    name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'tlsSettings', 'serverName']}
                    label="TLS SNI"
                    rules={{ required: 'TLS 需要证书域名 / SNI' }}
                  ><Input placeholder="cdn.example.com" /></FormField>
                  <FormField
                    name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'tlsSettings', 'fingerprint']}
                    label="TLS uTLS"
                  ><Select options={Object.values(UTLS_FINGERPRINT).map(fp=>({value:fp,label:fp}))} /></FormField>
                  <FormField
                    name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'tlsSettings', 'alpn']}
                    label="TLS ALPN"
                  ><Select mode="multiple" options={['h2','h3','http/1.1'].map(v=>({value:v,label:v}))} /></FormField>
                </>
              )}
              <FormField
                name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'xhttpSettings', 'host']}
                label="下行 HTTP Host（可留空）"
              ><Input placeholder="留空时按 SNI 自动填充，REALITY 下行可留空" /></FormField>
              <Alert
                type="warning"
                showIcon
                message="下行 Path 与 mode 自动继承上行；两种安全层共用同一服务端 XHTTP 实例。Mihomo 对新 Xray REALITY 的兼容性需按客户端版本实测。"
              />
            </>
          )}
        </>
      )}
'''
# Switch itself must not be wrapped in FormField controlled same name as object,
# otherwise RHF would write a boolean over the downloadSettings object.
extra=extra.replace("""          <FormField
            name={['streamSettings', 'xhttpSettings', 'downloadSettings']}
            label="启用独立下行连接"
          >
            <Switch
              checked={!!clientDownload}
              onChange={onDownloadToggle}
            />
          </FormField>""", """          <div style={{ marginBottom: 12 }}>
            <span style={{ marginRight: 12 }}>启用独立下行连接</span>
            <Switch aria-label="启用独立下行连接" checked={clientDownload} onChange={onDownloadToggle} />
          </div>""")
(root/ui).write_text(raw[:-len(anchor)] + '''      )}
''' + extra + '''    </>
  );
}
''')

# Inbound link preview / QR is generated client side as well; emit raw share-link
# extra in Xray's documented nested downloadSettings structure.
link='frontend/src/lib/xray/inbound-link.ts'
patch(link,
    '  return Object.keys(extra).length > 0 ? extra : null;\n}',
    '''  const download = xhttp.downloadSettings;
  if (download?.address && download.port > 0) {
    const reality = download.security === 'reality';
    extra.downloadSettings = {
      address: download.address,
      port: download.port,
      network: 'xhttp',
      security: download.security,
      ...(reality ? {
        realitySettings: {
          serverName: download.realitySettings.serverName,
          fingerprint: download.realitySettings.fingerprint || 'chrome',
          publicKey: download.realitySettings.publicKey,
          shortId: download.realitySettings.shortId,
        },
      } : {
        tlsSettings: {
          serverName: download.tlsSettings.serverName,
          fingerprint: download.tlsSettings.fingerprint || 'chrome',
          alpn: download.tlsSettings.alpn?.length ? download.tlsSettings.alpn : ['h2'],
        },
      }),
      xhttpSettings: {
        path: xhttp.path,
        mode: xhttp.mode,
        host: download.xhttpSettings.host || (reality ? '' : download.tlsSettings.serverName),
      },
    };
  }
  return Object.keys(extra).length > 0 ? extra : null;
}''')


# The edit modal loads a saved inbound without a UI-only flag. Restore it
# from the persisted client download configuration rather than default false.
patch('frontend/src/lib/xray/inbound-form-adapter.ts',
    '      streamRecord.xhttpSettings = xhttp;\n      const xmux = xhttp.xmux;',
    '''      streamRecord.xhttpSettings = xhttp;
      xhttp.enableDownloadSettings = !!xhttp.downloadSettings;
      const xmux = xhttp.xmux;''')

# On every save, omit the UI-only switch. A disabled switch removes even a
# stale subtree resurrected by an unmounted child Controller.
patch('frontend/src/lib/xray/stream-wire-normalize.ts',
    '  const enableXmux = out.enableXmux === true;\n  delete out.enableXmux;',
    '''  const enableXmux = out.enableXmux === true;
  delete out.enableXmux;
  const enableDownloadSettings = out.enableDownloadSettings === true;
  delete out.enableDownloadSettings;
  if (side === 'inbound' && !enableDownloadSettings) delete out.downloadSettings;''')

# A downloadSettings field on the DB row must never become server listener
# configuration. The existing JSON subscription path will rehydrate it.
server='internal/web/service/xray.go'
patch(server,
    '\t\t\tliftXhttpSessionIDKeys(stream)\n',
    '''\t\t\tliftXhttpSessionIDKeys(stream)
\t\t\t// Client-only downloadSettings is persisted for subscriptions, not the listener.
\t\t\tif xhttp, ok := stream["xhttpSettings"].(map[string]any); ok {
\t\t\t\tdelete(xhttp, "downloadSettings")
\t\t\t}
''')

# Preserve only client-relevant keys; xray's raw VLESS link generator already
# includes downloadSettings, but use a sanitized, fully populated object.
subservice='internal/sub/service.go'
patch(subservice,
    '''\tfor _, field := range []string{"xmux", "downloadSettings"} {''',
    '''\tif download := xhttpClientDownload(xhttp); download != nil {
\t\textra["downloadSettings"] = download
\t}
\tfor _, field := range []string{"xmux"} {''')
jsonservice='internal/sub/json_service.go'
patch(jsonservice,
    '''\t\tif xhttp, ok := streamSettings["xhttpSettings"].(map[string]any); ok {
\t\t\tdelete(xhttp, "noSSEHeader")''',
    '''\t\tif xhttp, ok := streamSettings["xhttpSettings"].(map[string]any); ok {
\t\t\tif download := xhttpClientDownload(xhttp); download != nil {
\t\t\t\textra, _ := xhttp["extra"].(map[string]any)
\t\t\t\tif extra == nil { extra = map[string]any{} }
\t\t\t\textra["downloadSettings"] = download
\t\t\t\txhttp["extra"] = extra
\t\t\t}
\t\t\tdelete(xhttp, "downloadSettings")
\t\t\tdelete(xhttp, "noSSEHeader")''')
clash='internal/sub/clash_service.go'
patch(clash,
    '''\t// Direct fields: path, mode
\tif v, ok := xhttp["path"].(string); ok && v != "" {''',
    '''\tif download := xhttpClientDownload(xhttp); download != nil {
\t\tif converted := xhttpDownloadMihomo(download); len(converted) > 0 {
\t\t\topts["download-settings"] = converted
\t\t}
\t}
\t// Direct fields: path, mode
\tif v, ok := xhttp["path"].(string); ok && v != "" {''')


# Hosts may override the uplink path per endpoint. Keep the nested download path
# in sync for both raw URI exports and Xray JSON. The XHTTP session will fail
# if the two HTTP connections do not target the same listener path.
hostsub='internal/sub/host_sub.go'
patch(hostsub,
    '''\tif p, ok := e.ep["path"].(string); ok && p != "" {
\t\tif _, exists := params["path"]; exists {
\t\t\tparams["path"] = p
\t\t}
\t}
}''',
    '''\tif p, ok := e.ep["path"].(string); ok && p != "" {
\t\tif _, exists := params["path"]; exists {
\t\t\tparams["path"] = p
\t\t}
\t\t// Raw VLESS extra embeds the XHTTP download transport's path.
\t\tif original := params["extra"]; original != "" {
\t\t\tvar extra map[string]any
\t\t\tif json.Unmarshal([]byte(original), &extra) == nil {
\t\t\t\tif download, ok := extra["downloadSettings"].(map[string]any); ok {
\t\t\t\t\tif settings, ok := download["xhttpSettings"].(map[string]any); ok {
\t\t\t\t\t\tsettings["path"] = p
\t\t\t\t\t\tif updated, err := json.Marshal(extra); err == nil {
\t\t\t\t\t\t\tparams["extra"] = string(updated)
\t\t\t\t\t\t}
\t\t\t\t\t}
\t\t\t\t}
\t\t\t}
\t\t}
\t}
}''')
patch(hostsub,
    '''\tif p, ok := ep["path"].(string); ok && p != "" {
\t\tfor _, key := range []string{"wsSettings", "httpupgradeSettings", "xhttpSettings"} {
\t\t\tif ts, ok := stream[key].(map[string]any); ok && ts != nil {
\t\t\t\tts["path"] = p
\t\t\t}
\t\t}
\t}''',
    '''\tif p, ok := ep["path"].(string); ok && p != "" {
\t\tfor _, key := range []string{"wsSettings", "httpupgradeSettings", "xhttpSettings"} {
\t\t\tif ts, ok := stream[key].(map[string]any); ok && ts != nil {
\t\t\t\tts["path"] = p
\t\t\t\tif key == "xhttpSettings" {
\t\t\t\t\tif extra, ok := ts["extra"].(map[string]any); ok {
\t\t\t\t\t\tif download, ok := extra["downloadSettings"].(map[string]any); ok {
\t\t\t\t\t\t\tif downXhttp, ok := download["xhttpSettings"].(map[string]any); ok {
\t\t\t\t\t\t\t\tdownXhttp["path"] = p
\t\t\t\t\t\t\t}
\t\t\t\t\t\t}
\t\t\t\t\t}
\t\t\t\t}
\t\t\t}
\t\t}
\t}''')

print("XHTTP client download settings patch applied")
