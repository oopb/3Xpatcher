#!/usr/bin/env python3
"""Opt-in paired REALITY listener for XHTTP download, patched on top of v26.

The original public TLS endpoint is replaced by a TLS/RAW gateway with
fallback to the same abstract Unix-socket XHTTP inbound as the generated
REALITY/RAW gateway. No extra independent XHTTP session is created.
"""
from pathlib import Path
import sys
root=Path(sys.argv[1])

def patch(path,old,new):
    p=root/path
    raw=p.read_text()
    if old not in raw:
        raise SystemExit(f"Missing managed XHTTP patch anchor {path}: {old[:85]!r}")
    p.write_text(raw.replace(old,new,1))

p="frontend/src/schemas/protocols/stream/xhttp.ts"
patch(p,
"""  realitySettings: z.object({
    serverName: z.string().default(''),
    fingerprint: z.string().default('chrome'),
    publicKey: z.string().default(''),
    shortId: z.string().default(''),
  }).default({ serverName: '', fingerprint: 'chrome', publicKey: '', shortId: '' }),
});
export type XHttpDownloadSettings""",
"""  realitySettings: z.object({
    serverName: z.string().default(''),
    fingerprint: z.string().default('chrome'),
    publicKey: z.string().default(''),
    shortId: z.string().default(''),
  }).default({ serverName: '', fingerprint: 'chrome', publicKey: '', shortId: '' }),
  // Private server listener parameters: never export into client subscriptions.
  managedListener: z.object({
    enabled: z.boolean().default(false),
    listen: z.string().default('0.0.0.0'),
    realitySettings: z.object({
      target: z.string().default(''),
      serverNames: z.array(z.string()).default([]),
      shortIds: z.array(z.string()).default([]),
      privateKey: z.string().default(''),
      publicKey: z.string().default(''),
    }).default({ target: '', serverNames: [], shortIds: [], privateKey: '', publicKey: '' }),
  }).optional(),
});
export type XHttpDownloadSettings""")

p="frontend/src/pages/inbounds/form/transport/xhttp.tsx"
patch(p,
"import { Alert, AutoComplete, Divider, Input, InputNumber, Select, Switch } from 'antd';",
"import { Alert, AutoComplete, Button, Divider, Input, InputNumber, Select, Switch } from 'antd';")
patch(p,
"import { UTLS_FINGERPRINT } from '@/schemas/primitives';",
"import { UTLS_FINGERPRINT } from '@/schemas/primitives';\nimport { HttpUtil } from '@/utils';")
patch(p,
"  const protocol = useWatch({ control, name: 'protocol' });",
"""  const protocol = useWatch({ control, name: 'protocol' });
  const autoReality = !!useWatch({
    control,
    name: 'streamSettings.xhttpSettings.downloadSettings.managedListener.enabled',
  });
  const uplinkSecurity = useWatch({ control, name: 'streamSettings.security' });
  const managedSecurityAllowed = protocol === 'vless' && uplinkSecurity === 'tls';
  async function genManagedRealityKeypair() {
    const res = await HttpUtil.get('/panel/api/server/getNewX25519Cert');
    if (!res?.success || !res.obj) return;
    const { privateKey, publicKey } = res.obj as { privateKey: string; publicKey: string };
    setValue('streamSettings.xhttpSettings.downloadSettings.managedListener.realitySettings.privateKey',
      privateKey, { shouldDirty: true });
    setValue('streamSettings.xhttpSettings.downloadSettings.managedListener.realitySettings.publicKey',
      publicKey, { shouldDirty: true });
    setValue('streamSettings.xhttpSettings.downloadSettings.realitySettings.publicKey',
      publicKey, { shouldDirty: true });
  }
  async function enableManagedReality(enabled: boolean) {
    const base = 'streamSettings.xhttpSettings.downloadSettings';
    if (!enabled) {
      setValue(`${base}.managedListener.enabled`, false, { shouldDirty: true });
      return;
    }
    const existing = getValues(`${base}.managedListener`);
    setValue(`${base}.managedListener.listen`,
      existing?.listen || '0.0.0.0', { shouldDirty: true });
    const currentPort = Number(getValues(`${base}.port`));
    const uplinkPort = Number(getValues('port'));
    if (!currentPort || currentPort === uplinkPort) {
      // An additional listener cannot bind to an occupied TLS port.
      setValue(`${base}.port`, 9443, { shouldDirty: true });
    }
    const reality = getValues(`${base}.realitySettings`) ?? {};
    const names = getValues(`${base}.managedListener.realitySettings.serverNames`) ?? [];
    if (!names.length && reality.serverName) {
      setValue(`${base}.managedListener.realitySettings.serverNames`,
        [reality.serverName], { shouldDirty: true });
    }
    const target = (getValues(`${base}.managedListener.realitySettings.target`) || '').trim();
    if (!target || target === reality.serverName + ':' + Number(getValues(`${base}.port`))) {
      setValue(`${base}.managedListener.realitySettings.target`, reality.serverName + ':443', { shouldDirty: true });
    }
    const shorts = getValues(`${base}.managedListener.realitySettings.shortIds`) ?? [];
    if (!shorts.length) {
      setValue(`${base}.managedListener.realitySettings.shortIds`,
        [reality.shortId || ''], { shouldDirty: true });
    }
    if (!getValues(`${base}.managedListener.realitySettings.privateKey`)) {
      await genManagedRealityKeypair();
      if (!getValues(`${base}.managedListener.realitySettings.privateKey`)) return;
    }
    setValue(`${base}.managedListener.enabled`, true, { shouldDirty: true });
  }""")
anchor="""              <FormField
                name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'xhttpSettings', 'host']}
                label="下行 HTTP Host（可留空）"
              ><Input placeholder="留空时按 SNI 自动填充，REALITY 下行可留空" /></FormField>
"""
append=anchor+"""              {downloadSecurity === 'reality' && (
                <>
                  <Divider>REALITY 服务端自动监听（可选）</Divider>
                  <div style={{ marginBottom: 12 }}>
                    <span style={{ marginRight: 12 }}>自动创建 REALITY 入口并与 TLS 上行共用 XHTTP 会话</span>
                    <Switch
                      aria-label="自动创建 REALITY 入口"
                      checked={autoReality}
                      disabled={!managedSecurityAllowed}
                      onChange={(enabled) => void enableManagedReality(enabled)}
                    />
                  </div>
                  {autoReality && (
                    <>
                      <Alert
                        type="warning"
                        showIcon
                        message="此模式将自动把原 TLS/XHTTP 监听改为 TLS/RAW 回落入口，并创建共享的内部 XHTTP 会话与独立 REALITY/RAW 端口。必须使用不同的公网端口（例如 9443），并在防火墙开放；仅在确知服务端无端口冲突时启用。TLS 证书沿用原入站证书。"
                        style={{ marginBottom: 12 }}
                      />
                      <details>
                        <summary style={{ cursor: 'pointer', marginBottom: 12 }}>高级设置（通常无需修改）</summary>
                      <FormField
                        name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'managedListener', 'listen']}
                        label="REALITY 监听 IP"
                      ><Input placeholder="0.0.0.0" /></FormField>
                      <FormField
                        name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'managedListener', 'realitySettings', 'target']}
                        label="REALITY Target (伪装目标 host:port)"
                        rules={{required:'REALITY target 不能为空'}}
                      ><Input placeholder="www.example.com:443" /></FormField>
                      <FormField
                        name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'managedListener', 'realitySettings', 'serverNames']}
                        label="REALITY serverNames (SNI)"
                        rules={{required:'至少一个 REALITY SNI'}}
                        onAfterChange={(value) => {
                          const names = Array.isArray(value) ? value as string[] : [];
                          const current = getValues('streamSettings.xhttpSettings.downloadSettings.realitySettings.serverName');
                          if (!names.includes(current)) {
                            setValue('streamSettings.xhttpSettings.downloadSettings.realitySettings.serverName',
                              names[0] || '', { shouldDirty: true });
                          }
                        }}
                      ><Select mode="tags" tokenSeparators={[',']} placeholder="例如 www.example.com" /></FormField>
                      <FormField
                        name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'managedListener', 'realitySettings', 'shortIds']}
                        label="REALITY shortIds"
                        rules={{required:'至少一个 shortId'}}
                        onAfterChange={(value) => {
                          const shorts = Array.isArray(value) ? value as string[] : [];
                          const current = getValues('streamSettings.xhttpSettings.downloadSettings.realitySettings.shortId');
                          if (!shorts.includes(current)) {
                            setValue('streamSettings.xhttpSettings.downloadSettings.realitySettings.shortId',
                              shorts[0] || '', { shouldDirty: true });
                          }
                        }}
                      ><Select mode="tags" tokenSeparators={[',']} placeholder="十六进制字符串" /></FormField>
                      <Button onClick={() => {
                        const bytes = new Uint8Array(8);
                        crypto.getRandomValues(bytes);
                        const shortId = Array.from(bytes).map(v => v.toString(16).padStart(2, '0')).join('');
                        setValue('streamSettings.xhttpSettings.downloadSettings.managedListener.realitySettings.shortIds',
                          [shortId], { shouldDirty: true });
                        setValue('streamSettings.xhttpSettings.downloadSettings.realitySettings.shortId',
                          shortId, { shouldDirty: true });
                      }}>生成 Short ID</Button>
                      <FormField
                        name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'managedListener', 'realitySettings', 'privateKey']}
                        label="REALITY 私钥（仅服务端保存）"
                        rules={{required:'必须生成 REALITY 私钥'}}
                      ><Input.Password autoComplete="off" /></FormField>
                      <FormField
                        name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'managedListener', 'realitySettings', 'publicKey']}
                        label="REALITY 公钥（自动同步到客户端下载设置）"
                        rules={{required:'必须生成 REALITY 公钥'}}
                      ><Input readOnly /></FormField>
                      <Button onClick={() => void genManagedRealityKeypair()}>
                        重新生成 REALITY 密钥对（会同步更新客户端公钥）
                      </Button>
                      </details>
                    </>
                  )}
                </>
              )}
"""
patch(p,anchor,append)
# A managed listener is not client-only. Describe the real effect of the toggle.
patch(p,
'''message="仅用于客户端订阅/分享链接，不会写入 Xray 服务端 listener。上下行必须汇聚到同一个 XHTTP 入站与 Path；CDN 回源、TLS/REALITY 分流需事先配置."''',
'''message={autoReality
                  ? '已启用服务端托管：保存后会生成独立 REALITY 端口，并让 TLS/REALITY 两条路径汇聚到同一 XHTTP 后端。'
                  : '当前仅生成客户端的下行配置；若需自动启动 REALITY 下行监听，请开启下方服务端托管开关。'}''')

# The new metadata is stored on the raw inbound exclusively. xhttpClientDownload
# already constructs an allowlisted outbound, so the privateKey is never emitted.
p="internal/web/service/xray.go"
patch(p,
"""		xrayConfig.InboundConfigs = append(xrayConfig.InboundConfigs, *inboundConfig)
	}
	xrayConfig.InboundConfigs = dropInboundsRefusedByXDrive(xrayConfig.InboundConfigs)""",
"""		xrayConfig.InboundConfigs = append(xrayConfig.InboundConfigs, *inboundConfig)
	}
	// Opt-in XHTTP gateways share a single internal session/listener. Reject
	// incomplete keypairs or duplicate ports instead of producing a broken
	// or insecure second standalone VLESS/XHTTP inbound.
	if err := injectManagedXHTTPGateways(xrayConfig, inbounds); err != nil {
		return nil, err
	}
	xrayConfig.InboundConfigs = dropInboundsRefusedByXDrive(xrayConfig.InboundConfigs)""")
print("Managed paired XHTTP gateway patch applied")
