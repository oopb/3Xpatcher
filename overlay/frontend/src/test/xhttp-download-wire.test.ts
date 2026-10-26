import { describe, expect, it } from 'vitest';

import { rawInboundToFormValues, formValuesToWirePayload } from '@/lib/xray/inbound-form-adapter';
import { normalizeXhttpForWire } from '@/lib/xray/stream-wire-normalize';
import { InboundFormSchema } from '@/schemas/forms/inbound-form';
import { XHttpStreamSettingsSchema } from '@/schemas/protocols/stream/xhttp';

function savedRow() {
  return rawInboundToFormValues({
    port: 443,
    protocol: 'vless',
    listen: '0.0.0.0',
    settings: { clients: [], decryption: 'none', encryption: 'none', fallbacks: [] },
    streamSettings: {
      network: 'xhttp',
      security: 'none',
      xhttpSettings: {
        path: '/shared',
        mode: 'auto',
        downloadSettings: {
          address: 'cdn.example.com', port: 443, security: 'tls', network: 'xhttp',
          xhttpSettings: { host: 'cdn.example.com' },
          tlsSettings: { serverName: 'cdn.example.com', fingerprint: 'chrome', alpn: ['h2'] },
        },
      },
    },
  });
}

describe('XHTTP downloadSettings save lifecycle', () => {
  it('re-enables the UI switch when editing a previously saved download configuration', () => {
    const values = savedRow();
    if (values.streamSettings?.network !== 'xhttp') throw new Error('expected XHTTP');
    expect(values.streamSettings.xhttpSettings.enableDownloadSettings).toBe(true);
    expect(values.streamSettings.xhttpSettings.downloadSettings?.address).toBe('cdn.example.com');
  });

  it('saving an OFF switch deletes the client download object even if child fields remain registered', () => {
    const values = savedRow();
    if (values.streamSettings?.network !== 'xhttp') throw new Error('expected XHTTP');
    values.streamSettings.xhttpSettings.enableDownloadSettings = false;
    const parsed = InboundFormSchema.safeParse(values);
    expect(parsed.success).toBe(true);
    if (!parsed.success) return;
    const payload = formValuesToWirePayload(parsed.data);
    const stream = JSON.parse(payload.streamSettings) as Record<string, unknown>;
    const xhttp = stream.xhttpSettings as Record<string, unknown>;
    expect(xhttp).not.toHaveProperty('downloadSettings');
    expect(xhttp).not.toHaveProperty('enableDownloadSettings');
  });

  it('disabled invalid download data never prevents schema validation/save', () => {
    const parsed = XHttpStreamSettingsSchema.safeParse({
      path: '/shared',
      mode: 'auto',
      enableDownloadSettings: false,
      downloadSettings: { address: 'invalid', port: 'not-a-number', security: 'nonsense' },
    });
    expect(parsed.success).toBe(true);
    if (!parsed.success) return;
    expect(parsed.data.downloadSettings).toBeUndefined();
    expect(normalizeXhttpForWire(parsed.data, 'inbound')).not.toHaveProperty('downloadSettings');
  });

  it('saving an ON switch retains nested settings but never serializes the toggle', () => {
    const values = savedRow();
    const parsed = InboundFormSchema.safeParse(values);
    expect(parsed.success).toBe(true);
    if (!parsed.success) return;
    const payload = formValuesToWirePayload(parsed.data);
    const stream = JSON.parse(payload.streamSettings) as Record<string, unknown>;
    const xhttp = stream.xhttpSettings as Record<string, unknown>;
    expect(xhttp).not.toHaveProperty('enableDownloadSettings');
    expect((xhttp.downloadSettings as Record<string, unknown>).address).toBe('cdn.example.com');
  });
});
