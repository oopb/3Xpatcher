#!/usr/bin/env python3
from pathlib import Path
import sys
from v2_patchlib import Patcher

root = Path(sys.argv[1]).resolve()
rep = Patcher(sys.argv[1]).rep

# 3x-ui v3.8.5 validates every native TLS certificate row before the
# supplemental self-signed-SNI renderer gets a chance to generate/use its own
# certificate. Keep the strict native path unchanged, but let the explicit
# self_signed_sni mode ignore the hidden empty native certificate editor row.
path = root / 'frontend/src/schemas/forms/inbound-form.ts'
text = path.read_text(encoding='utf-8')
patched = False
for issue_key in ('message', 'error'):
    old = f'''const InboundTlsSettingsSchema = TlsStreamSettingsSchema.extend({{
  certificates: z
    .array(InboundTlsCertSchema)
    .default([])
    .refine((certificates) => certificates.some((cert) => cert.usage !== 'verify'), {{
      {issue_key}: 'pages.inbounds.form.tlsServerCertificateRequired',
    }}),
}});'''
    if old not in text:
        continue
    new = f'''const NativeInboundTlsSettingsSchema = TlsStreamSettingsSchema.extend({{
  certificates: z
    .array(InboundTlsCertSchema)
    .default([])
    .refine((certificates) => certificates.some((cert) => cert.usage !== 'verify'), {{
      {issue_key}: 'pages.inbounds.form.tlsServerCertificateRequired',
    }}),
}});

const SelfSignedInboundTlsSettingsSchema = TlsStreamSettingsSchema.extend({{
  certificateMode: z.literal('self_signed_sni'),
  certificates: z.array(InboundTlsCertFieldsSchema).default([]),
}}).transform((tls) => ({{
  ...tls,
  // The native certificate editor is hidden in this mode, but react-hook-form
  // can keep its default empty row alive. Preserve any genuinely populated
  // native rows and discard only rows that fail the native certificate parser.
  certificates: tls.certificates.flatMap((cert) => {{
    const parsed = InboundTlsCertSchema.safeParse(cert);
    return parsed.success ? [parsed.data] : [];
  }}),
}}));

const InboundTlsSettingsSchema = z.union([
  SelfSignedInboundTlsSettingsSchema,
  NativeInboundTlsSettingsSchema,
]);'''
    text = text.replace(old, new, 1)
    patched = True
    break
if not patched and 'const NativeInboundTlsSettingsSchema' not in text:
    raise SystemExit('v16 TLS self-signed validation: no supported 3x-ui TLS schema anchor found')
path.write_text(text, encoding='utf-8')

# Match 3x-ui's native protocol column style with useful transport/security
# badges, but do not add protocol-version badges (for example v3/v5).
rep(
    'frontend/src/pages/inbounds/list/useInboundColumns.tsx',
    '''          if (record.isWireguard || record.isAmneziawg || record.isHysteria || record.isTuic) {
            tags.push(
              <Tag key="n" color="green">
                UDP
              </Tag>,
            );
          } else if (record.isSS) {''',
    '''          const supplementalSettings = coerceInboundJsonField(record.settings) as Record<string, unknown>;
          if (record.protocol === 'tuic') {
            tags.push(
              <Tag key="n" color="green">UDP</Tag>,
              <Tag key="tls" color="blue">TLS</Tag>,
            );
          } else if (record.protocol === 'anytls') {
            tags.push(
              <Tag key="n" color="green">TCP</Tag>,
              <Tag key="tls" color="blue">TLS</Tag>,
            );
          } else if (record.protocol === 'shadowtls') {
            tags.push(
              <Tag key="n" color="green">TCP</Tag>,
              <Tag key="tls" color="blue">TLS</Tag>,
            );
          } else if (record.protocol === 'naive') {
            tags.push(
              <Tag key="n" color="green">TCP</Tag>,
              <Tag key="tls" color="blue">TLS</Tag>,
            );
          } else if (record.protocol === 'snell') {
            tags.push(
              <Tag key="n" color="green">TCP</Tag>,
            );
          } else if (record.protocol === 'mieru') {
            const transport = String(supplementalSettings.transport || 'TCP').toUpperCase();
            tags.push(
              <Tag key="n" color="green">{transport === 'UDP' ? 'UDP' : 'TCP'}</Tag>,
            );
          } else if (record.isWireguard || record.isAmneziawg || record.isHysteria || record.isTuic) {
            tags.push(
              <Tag key="n" color="green">
                UDP
              </Tag>,
            );
          } else if (record.isSS) {''',
)

print('V16 TLS self-signed validation + supplemental protocol badge hotfix applied.')