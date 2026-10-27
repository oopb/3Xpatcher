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
      minClientVer: z.string().default('1.8.2'),
    }).default({ target: '', serverNames: [], shortIds: [], privateKey: '', publicKey: '', minClientVer: '1.8.2' }),
  }).optional(),
});
export type XHttpDownloadSettings""")

p="frontend/src/pages/inbounds/form/transport/xhttp.tsx"
patch(p,
"import { Alert, AutoComplete, Divider, Input, InputNumber, Select, Switch } from 'antd';",
"import { Alert, AutoComplete, Button, Divider, Form, Input, InputNumber, Select, Space, Switch, message } from 'antd';")
patch(p,
"import { UTLS_FINGERPRINT } from '@/schemas/primitives';",
"import { UTLS_FINGERPRINT } from '@/schemas/primitives';\nimport { HttpUtil, RandomUtil } from '@/utils';\nimport { useState } from 'react';\nimport { SearchOutlined } from '@ant-design/icons';\nimport type { RealityScanResult } from '@/generated/types';\nimport { validateRealityTarget } from '@/lib/xray/stream-wire-normalize';\nimport RealityTargetScannerModal from '../security/RealityTargetScannerModal';")
patch(p,
"  const protocol = useWatch({ control, name: 'protocol' });",
"""  const protocol = useWatch({ control, name: 'protocol' });
  const [scannerOpen, setScannerOpen] = useState(false);
  async function scanManagedRealityCandidates(targets?: string): Promise<RealityScanResult[]> {
    const response = await HttpUtil.post<RealityScanResult[]>(
      '/panel/api/server/scanRealityTargets',
      targets ? { targets } : {},
      { silent: true },
    );
    if (!response?.success || !Array.isArray(response.obj)) {
      message.error(response?.msg || 'REALITY 伪装目标扫描失败');
      return [];
    }
    return response.obj;
  }

  function applyManagedRealityScanResult(result: RealityScanResult) {
    // Mirror 3x-ui's native onPick logic: a new target replaces its allowed SNI
    // list. Never keep an SNI from an unrelated previous camouflage target.
    const base = 'streamSettings.xhttpSettings.downloadSettings';
    const names = (result.serverNames ?? []).filter(
      (name): name is string => typeof name === 'string' && name.trim() !== '',
    );
    setValue(base + '.managedListener.realitySettings.target', result.target, { shouldDirty: true });
    setValue(base + '.managedListener.realitySettings.serverNames', names, { shouldDirty: true });
    const current = getValues(base + '.realitySettings.serverName');
    const preferred = names.includes(current) ? current
      : names.includes(result.host) ? result.host : (names[0] || '');
    setValue(base + '.realitySettings.serverName', preferred, { shouldDirty: true });
    if (!result.feasible) {
      message.warning(result.reason || '该 REALITY 目标未通过可用性检查，请确认后再使用');
    }
    if (!names.length) message.warning('扫描结果没有可用 SNI，请更换目标');
  }
  function syncManagedRealitySNI(value: unknown) {
    const base = 'streamSettings.xhttpSettings.downloadSettings';
    const sni = typeof value === 'string' ? value.trim() : '';
    const names = getValues(base + '.managedListener.realitySettings.serverNames') ?? [];
    if (!sni) {
      setValue(base + '.managedListener.realitySettings.serverNames', [], { shouldDirty: true });
    } else if (!Array.isArray(names) || !names.includes(sni)) {
      // A manual SNI edit must not leave a disabled client/server SNI pair.
      setValue(base + '.managedListener.realitySettings.serverNames', [sni], { shouldDirty: true });
    }
  }
  const autoReality = !!useWatch({
    control,
    name: 'streamSettings.xhttpSettings.downloadSettings.managedListener.enabled',
  });
  const uplinkSecurity = useWatch({ control, name: 'streamSettings.security' });
  const managedSecurityAllowed = protocol === 'vless' && uplinkSecurity === 'tls';
  async function genManagedRealityKeypair() {
    const res = await HttpUtil.get('/panel/api/server/getNewX25519Cert');
    if (!res?.success || !res.obj) {
      message.error(res?.msg || 'REALITY 密钥生成失败，请重试');
      return;
    }
    const { privateKey, publicKey } = res.obj as { privateKey: string; publicKey: string };
    if (!privateKey || !publicKey) {
      message.error('密钥生成接口没有返回完整的密钥对');
      return;
    }
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
      setValue(base + '.managedListener.enabled', false, { shouldDirty: true });
      return;
    }
    if (!managedSecurityAllowed || getValues(base + '.security') !== 'reality') {
      message.warning('服务端自动监听仅支持 VLESS + XHTTP + TLS 上行 / REALITY 下行');
      return;
    }
    const portValue = getValues(base + '.port');
    const currentPort = portValue == null || portValue === '' ? NaN : Number(portValue);
    const uplinkPort = Number(getValues('port'));
    if (!Number.isInteger(currentPort) || currentPort < 1 || currentPort > 65535) {
      message.warning('请先明确填写有效的下行监听端口（1–65535）');
      return;
    }
    if (currentPort === uplinkPort) {
      message.warning('下行 REALITY 端口不能与上行 TLS 端口相同');
      return;
    }
    const sni = String(getValues(base + '.realitySettings.serverName') || '').trim();
    if (!sni) {
      message.warning('请先填写 REALITY SNI，或通过右侧原生选择器自动填充');
      return;
    }
    syncManagedRealitySNI(sni);
    const reality = getValues(base + '.realitySettings') ?? {};
    const shorts = getValues(base + '.managedListener.realitySettings.shortIds') ?? [];
    let shortId = typeof reality.shortId === 'string' ? reality.shortId.trim() : '';
    if (!shortId && Array.isArray(shorts)) {
      shortId = shorts.find((id: unknown) => typeof id === 'string' && id !== '') || '';
    }
    if (!shortId) {
      shortId = RandomUtil.randomShortIds().split(',').map((id) => id.trim()).find(Boolean) || '';
    }
    if (!Array.isArray(shorts) || !shorts.includes(shortId)) {
      setValue(base + '.managedListener.realitySettings.shortIds', [shortId], { shouldDirty: true });
    }
    if (shortId !== reality.shortId) {
      setValue(base + '.realitySettings.shortId', shortId, { shouldDirty: true });
    }
    const privateKey = getValues(base + '.managedListener.realitySettings.privateKey');
    const publicKey = getValues(base + '.managedListener.realitySettings.publicKey');
    if (!privateKey || !publicKey) {
      // Incomplete/empty key material cannot be used. Explicit opt-in repairs
      // it atomically using 3x-ui's native X25519 certificate endpoint.
      await genManagedRealityKeypair();
      if (!getValues(base + '.managedListener.realitySettings.privateKey') ||
          !getValues(base + '.managedListener.realitySettings.publicKey')) return;
    } else if (reality.publicKey !== publicKey) {
      setValue(base + '.realitySettings.publicKey', publicKey, { shouldDirty: true });
    }
    if (!getValues(base + '.managedListener.realitySettings.minClientVer')) {
      setValue(base + '.managedListener.realitySettings.minClientVer', '1.8.2', { shouldDirty: true });
    }
    setValue(base + '.managedListener.enabled', true, { shouldDirty: true });
    const target = getValues(base + '.managedListener.realitySettings.target');
    if (!target) {
      message.info('请在下方填写 REALITY Target，或使用 SNI 旁的目标选择器自动填充');
    }
  }""")

# Put the exact upstream RealityTargetScannerModal next to the REALITY SNI,
# not inside a second advanced-settings editor.
patch(p,
"""                  <FormField
                    name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'realitySettings', 'serverName']}
                    label="REALITY SNI"
                    rules={{ required: 'REALITY 需要 SNI' }}
                  ><Input placeholder="reality.example.com" /></FormField>""",
"""                   <Form.Item label="REALITY SNI">
                     <Space.Compact block style={{ display: 'flex' }}>
                       <FormField
                         noStyle
                         name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'realitySettings', 'serverName']}
                         rules={{ required: 'REALITY 需要 SNI' }}
                         onAfterChange={syncManagedRealitySNI}
                       ><Input style={{ flex: 1 }} placeholder="选择或输入 REALITY SNI" /></FormField>
                       <Button
                         icon={<SearchOutlined aria-hidden />}
                         aria-label="选择 REALITY 伪装目标"
                         onClick={() => setScannerOpen(true)}
                       >选取目标</Button>
                     </Space.Compact>
                   </Form.Item>
                   <RealityTargetScannerModal
                     open={scannerOpen}
                     onClose={() => setScannerOpen(false)}
                     scanRealityCandidates={scanManagedRealityCandidates}
                     onPick={applyManagedRealityScanResult}
                   />""")
# Hide automatically managed key material, but retain editable fields for
# client-only download to an independently configured REALITY server.
patch(p,
"""                  <FormField
                    name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'realitySettings', 'publicKey']}""",
"""                   {!autoReality && (<>
                   <FormField
                     name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'realitySettings', 'publicKey']}""")
patch(p,
"""                  <FormField
                    name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'realitySettings', 'fingerprint']}""",
"""                   </>)}
                   <FormField
                     name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'realitySettings', 'fingerprint']}""")
anchor="""              <FormField
                name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'xhttpSettings', 'host']}
                label="下行 HTTP Host（可留空）"
              ><Input placeholder="留空时按 SNI 自动填充，REALITY 下行可留空" /></FormField>
"""
append=anchor.replace('留空时按 SNI 自动填充，REALITY 下行可留空', '留空时继承上行 XHTTP Host（与服务端匹配）')+r"""              {downloadSecurity === 'reality' && (
                <>
                  <div style={{ marginBottom: 12 }}>
                    <span style={{ marginRight: 12 }}>REALITY 服务端自动监听</span>
                    <Switch
                      aria-label="REALITY 服务端自动监听"
                      checked={autoReality}
                      disabled={!managedSecurityAllowed}
                      onChange={(enabled) => void enableManagedReality(enabled)}
                    />
                  </div>
                  {autoReality && (
                    <>
                      <Alert
                        type="info"
                        showIcon
                        message="自动生成监听、密钥、Short ID 和 SNI 列表。下行端口请在上方填写；Target 可由 SNI 旁的原生扫描器选取。"
                        style={{ marginBottom: 12 }}
                      />
                      <FormField
                        name={['streamSettings', 'xhttpSettings', 'downloadSettings', 'managedListener', 'realitySettings', 'target']}
                        label="REALITY Target（远端伪装目标）"
                        rules={{
                          required: '请先选取或填写 REALITY Target',
                          validate: (value) => validateRealityTarget(typeof value === 'string' ? value : '') ?? true,
                        }}
                      ><Input placeholder="通过 SNI 旁的选取目标自动填充，或手动填写 host:port" /></FormField>
                    </>
                  )}
                </>
              )}
"""
patch(p,anchor,append)
# A managed listener is not client-only. Describe the real effect of the toggle.
patch(p,
'''message="仅用于客户端订阅/分享链接，不会写入 Xray 服务端 listener。上下行必须汇聚到同一个 XHTTP 入站与 Path；CDN 回源、TLS/REALITY 分流需事先配置。"''',
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
