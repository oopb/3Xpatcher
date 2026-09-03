#!/usr/bin/env python3
import sys
from v2_patchlib import Patcher

rep = Patcher(sys.argv[1]).rep

rep(
    'frontend/src/pages/inbounds/list/helpers.ts',
    """    case 'amneziawg':\n      return true;""",
    """    case 'amneziawg':\n    case 'tuic':\n    case 'anytls':\n    case 'shadowtls':\n    case 'naive':\n      return true;""",
)

rep(
    'frontend/src/pages/inbounds/list/RowActions.tsx',
    """      <Dropdown\n        trigger={['click']}""",
    """      {isInboundMultiUser(record) && (\n        <Button type=\"text\" size=\"small\" style={{ fontSize: 16 }} icon={<UsergroupAddOutlined />} aria-label={t('pages.inbounds.attachExistingClients')} onClick={() => onClick('attachExisting')} />\n      )}\n      {isInboundMultiUser(record) && hasClients && (\n        <Button type=\"text\" size=\"small\" style={{ fontSize: 16 }} icon={<UsergroupDeleteOutlined />} aria-label={t('pages.inbounds.detachClients')} onClick={() => onClick('detachClients')} />\n      )}\n      <Dropdown\n        trigger={['click']}""",
)

rep(
    'internal/singbox/integrated.go',
    """\ttlsIn, _ := stream[\"tlsSettings\"].(map[string]any)\n\tif tlsIn == nil {\n\t\treturn errors.New(\"TLS settings are missing\")\n\t}\n\ttlsOut := map[string]any{\"enabled\": true}""",
    """\ttlsIn, _ := stream[\"tlsSettings\"].(map[string]any)\n\tif tlsIn == nil {\n\t\treturn errors.New(\"TLS settings are missing\")\n\t}\n\tmode, _ := tlsIn[\"certificateMode\"].(string)\n\tif mode == \"\" {\n\t\tmode, _ = settings[\"tlsMode\"].(string) // 0.6 compatibility\n\t}\n\tif mode == \"self_signed_sni\" {\n\t\treturn installSelfSignedTLS(settings, tlsIn)\n\t}\n\ttlsOut := map[string]any{\"enabled\": true}\n\tcopyNativeTLSOptions(tlsIn, tlsOut)""",
)

rep(
    'frontend/src/schemas/protocols/security/tls.ts',
    """  echSockopt: SockoptStreamSettingsSchema.optional(),\n  settings: TlsClientSettingsSchema.default({""",
    """  echSockopt: SockoptStreamSettingsSchema.optional(),\n  certificateMode: z.enum(['native', 'self_signed_sni']).optional(),\n  selfSignedValidityDays: z.number().int().min(1).max(3650).optional(),\n  selfSignedServerName: z.string().optional(),\n  selfSignedCertificatePath: z.string().optional(),\n  selfSignedKeyPath: z.string().optional(),\n  selfSignedNotAfter: z.string().optional(),\n  settings: TlsClientSettingsSchema.default({""",
)

rep(
    'internal/web/controller/server.go',
    """\tg.POST(\"/getNewEchCert\", a.getNewEchCert)""",
    """\tg.POST(\"/getNewEchCert\", a.getNewEchCert)\n\tg.POST(\"/generateSingboxSniCert\", a.generateSingboxSniCert)""",
)

rep(
    'frontend/src/pages/inbounds/form/security/tls.tsx',
    """import { useTranslation } from 'react-i18next';""",
    """import { useEffect, useState } from 'react';\nimport { useTranslation } from 'react-i18next';""",
)
rep(
    'frontend/src/pages/inbounds/form/security/tls.tsx',
    """import { Button, Form, Input, InputNumber, Radio, Select, Space, Switch } from 'antd';""",
    """import { Alert, Button, Divider, Form, Input, InputNumber, Radio, Select, Space, Switch } from 'antd';""",
)
rep(
    'frontend/src/pages/inbounds/form/security/tls.tsx',
    """import { FormField } from '@/components/form/rhf';""",
    """import { FormField } from '@/components/form/rhf';\nimport { HttpUtil } from '@/utils';""",
)
rep(
    'frontend/src/pages/inbounds/form/security/tls.tsx',
    """  const { control } = useFormContext();\n  const { fields, append, remove } = useFieldArray({""",
    """  const { control, setValue } = useFormContext();\n  const protocol = useWatch({ control, name: 'protocol' }) as string | undefined;\n  const rawCertificateMode = useWatch({ control, name: 'streamSettings.tlsSettings.certificateMode' }) as string | undefined;\n  const certificateMode = rawCertificateMode || 'native';\n  const sni = (useWatch({ control, name: 'streamSettings.tlsSettings.serverName' }) as string | undefined) || '';\n  const validityDays = (useWatch({ control, name: 'streamSettings.tlsSettings.selfSignedValidityDays' }) as number | undefined) || 3650;\n  const persistedSni = (useWatch({ control, name: 'streamSettings.tlsSettings.selfSignedServerName' }) as string | undefined) || '';\n  const persistedCert = (useWatch({ control, name: 'streamSettings.tlsSettings.selfSignedCertificatePath' }) as string | undefined) || '';\n  const persistedKey = (useWatch({ control, name: 'streamSettings.tlsSettings.selfSignedKeyPath' }) as string | undefined) || '';\n  const persistedNotAfter = (useWatch({ control, name: 'streamSettings.tlsSettings.selfSignedNotAfter' }) as string | undefined) || '';\n  const legacyMode = useWatch({ control, name: 'settings.tlsMode' }) as string | undefined;\n  const legacySni = (useWatch({ control, name: 'settings.camouflageSNI' }) as string | undefined) || '';\n  const legacyDays = useWatch({ control, name: 'settings.selfSignedValidityDays' }) as number | undefined;\n  const isSupplementalTLS = ['tuic', 'anytls', 'naive'].includes(protocol || '');\n  const [generatingSniCert, setGeneratingSniCert] = useState(false);\n  const [generatedInfo, setGeneratedInfo] = useState<{ serverName: string; certificatePath: string; keyPath: string; notAfter: string; created: boolean } | null>(null);\n  useEffect(() => {\n    if (!isSupplementalTLS || rawCertificateMode || legacyMode !== 'self_signed_sni') return;\n    setValue('streamSettings.tlsSettings.certificateMode', 'self_signed_sni', { shouldDirty: true });\n    if (!sni.trim() && legacySni.trim()) setValue('streamSettings.tlsSettings.serverName', legacySni.trim(), { shouldDirty: true });\n    if (legacyDays) setValue('streamSettings.tlsSettings.selfSignedValidityDays', legacyDays, { shouldDirty: true });\n    setValue('settings.tlsMode', undefined, { shouldDirty: true });\n    setValue('settings.camouflageSNI', undefined, { shouldDirty: true });\n    setValue('settings.selfSignedValidityDays', undefined, { shouldDirty: true });\n  }, [isSupplementalTLS, rawCertificateMode, legacyMode, legacySni, legacyDays, sni, setValue]);\n  const generateSniCert = async () => {\n    if (!sni.trim()) return;\n    setGeneratingSniCert(true);\n    try {\n      const msg = await HttpUtil.post('/panel/api/server/generateSingboxSniCert', { sni: sni.trim(), validityDays });\n      if (msg?.success && msg.obj) {\n        const info = msg.obj as { serverName: string; certificatePath: string; keyPath: string; notAfter: string; created: boolean };\n        setGeneratedInfo(info);\n        setValue('streamSettings.tlsSettings.selfSignedServerName', info.serverName, { shouldDirty: true });\n        setValue('streamSettings.tlsSettings.selfSignedCertificatePath', info.certificatePath, { shouldDirty: true });\n        setValue('streamSettings.tlsSettings.selfSignedKeyPath', info.keyPath, { shouldDirty: true });\n        setValue('streamSettings.tlsSettings.selfSignedNotAfter', info.notAfter, { shouldDirty: true });\n      }\n    } finally {\n      setGeneratingSniCert(false);\n    }\n  };\n  const displayedInfo = generatedInfo || (persistedCert && persistedKey && persistedNotAfter ? { serverName: persistedSni, certificatePath: persistedCert, keyPath: persistedKey, notAfter: persistedNotAfter, created: false } : null);\n  const generatedForCurrentSni = !!displayedInfo && displayedInfo.serverName === sni.trim();\n  const { fields, append, remove } = useFieldArray({""",
)
rep(
    'frontend/src/pages/inbounds/form/security/tls.tsx',
    """      <FormField name={['streamSettings', 'tlsSettings', 'serverName']} label=\"SNI\">\n        <Input placeholder={t('pages.inbounds.form.serverNameIndication')} />\n      </FormField>""",
    """      <FormField name={['streamSettings', 'tlsSettings', 'serverName']} label=\"SNI\">\n        <Input placeholder={t('pages.inbounds.form.serverNameIndication')} />\n      </FormField>\n      {isSupplementalTLS && (\n        <>\n          <Divider orientation=\"left\" plain>Certificate Source</Divider>\n          <FormField label=\"Certificate Mode\" name={['streamSettings', 'tlsSettings', 'certificateMode']}>\n            <Select placeholder=\"Native 3x-ui certificate\" options={[\n              { value: 'native', label: 'Native / imported certificate' },\n              { value: 'self_signed_sni', label: 'Generate self-signed certificate from SNI' },\n            ]} />\n          </FormField>\n          {certificateMode === 'self_signed_sni' && (\n            <>\n              <Alert type=\"warning\" showIcon title=\"Self-signed SNI certificate (not REALITY)\" description=\"The SNI above is written into the certificate SAN. Public CA trust is not available for domains you do not control, so generated subscriptions automatically use insecure / skip-cert-verify.\" style={{ marginBottom: 12 }} />\n              <FormField label=\"Validity (days)\" name={['streamSettings', 'tlsSettings', 'selfSignedValidityDays']}>\n                <InputNumber min={1} max={3650} style={{ width: '100%' }} placeholder=\"3650\" />\n              </FormField>\n              <Form.Item label=\"Generated Certificate\">\n                <Button type=\"primary\" loading={generatingSniCert} disabled={!sni.trim()} onClick={generateSniCert}>Generate / Regenerate</Button>\n              </Form.Item>\n              {displayedInfo && (\n                <Alert\n                  type={generatedForCurrentSni ? 'success' : 'warning'}\n                  showIcon\n                  title={generatedForCurrentSni ? (displayedInfo.created ? 'Certificate generated' : 'Generated certificate ready') : 'SNI changed — regenerate certificate'}\n                  description={`SAN: ${displayedInfo.serverName || '(unknown)'} | Expires: ${displayedInfo.notAfter} | Cert: ${displayedInfo.certificatePath} | Key: ${displayedInfo.keyPath}`}\n                  style={{ marginBottom: 12 }}\n                />\n              )}\n            </>\n          )}\n        </>\n      )}""",
)
