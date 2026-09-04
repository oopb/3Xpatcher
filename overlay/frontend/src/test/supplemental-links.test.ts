import { describe, expect, it } from 'vitest';

import type { Inbound } from '@/schemas/api/inbound';
import { genSupplementalLinks, getSupplementalClients } from '@/lib/xray/supplemental-links';

function inbound(protocol: string, settings: Record<string, unknown>, streamSettings: Record<string, unknown> = {}): Inbound {
  return {
    id: 1,
    up: 0,
    down: 0,
    total: 0,
    remark: 'supp-test',
    enable: true,
    expiryTime: 0,
    listen: '0.0.0.0',
    port: 443,
    protocol,
    settings,
    streamSettings,
    tag: 'inbound-1',
    sniffing: {},
  } as unknown as Inbound;
}

describe('supplemental browser share links', () => {
  it('exports TUIC with native TLS and tuning params', () => {
    const ib = inbound(
      'tuic',
      {
        clients: [{ email: 'tuic@test', id: '11111111-1111-4111-8111-111111111111', password: 'secret' }],
        congestionControl: 'bbr',
        zeroRTTHandshake: true,
        heartbeat: '10s',
      },
      { security: 'tls', tlsSettings: { serverName: 'tuic.example', alpn: ['h3'] } },
    );
    const client = getSupplementalClients(ib)?.[0];
    expect(client).toBeTruthy();
    const link = genSupplementalLinks({
      inbound: ib,
      address: 'edge.example',
      port: 443,
      remark: 'TUIC test',
      client: client!,
    })[0]?.link;
    expect(link).toContain('tuic://11111111-1111-4111-8111-111111111111:secret@edge.example:443');
    const url = new URL(link!);
    expect(url.searchParams.get('sni')).toBe('tuic.example');
    expect(url.searchParams.get('alpn')).toBe('h3');
    expect(url.searchParams.get('congestion_control')).toBe('bbr');
    expect(url.searchParams.get('zero_rtt_handshake')).toBe('1');
  });

  it('exports AnyTLS and marks generated self-signed TLS insecure', () => {
    const ib = inbound(
      'anytls',
      { clients: [{ email: 'any@test', password: 'any-pass' }] },
      {
        security: 'tls',
        tlsSettings: {
          serverName: 'any.example',
          certificateMode: 'self_signed_sni',
        },
      },
    );
    const client = getSupplementalClients(ib)![0];
    const link = genSupplementalLinks({ inbound: ib, address: '1.2.3.4', port: 8443, client })[0].link;
    const url = new URL(link);
    expect(url.protocol).toBe('anytls:');
    expect(url.searchParams.get('sni')).toBe('any.example');
    expect(url.searchParams.get('insecure')).toBe('1');
  });

  it('exports one SIP003 ShadowTLS representation', () => {
    const ib = inbound('shadowtls', {
      clients: [{ email: 'shadow@test', password: 'outer-pass' }],
      handshakeServer: 'www.cloudflare.com',
      innerMethod: '2022-blake3-aes-128-gcm',
      innerPassword: 'inner-key',
    });
    const client = getSupplementalClients(ib)![0];
    const variants = genSupplementalLinks({
      inbound: ib,
      address: 'shadow.example',
      port: 443,
      remark: 'shadow',
      client,
    });
    expect(variants).toHaveLength(1);
    expect(variants[0].label).toBe('ShadowTLS');
    expect(variants[0].link).toContain('plugin=shadow-tls');
    expect(variants[0].link).not.toContain('shadow-tls=');
  });

  it('exports protocol-native Naive URI', () => {
    const ib = inbound(
      'naive',
      { clients: [{ email: 'naive-user', password: 'naive-pass' }] },
      { security: 'tls', tlsSettings: { serverName: 'naive.example' } },
    );
    const client = getSupplementalClients(ib)![0];
    const link = genSupplementalLinks({ inbound: ib, address: 'naive.example', port: 443, client })[0].link;
    expect(link).toContain('naive+https://naive-user:naive-pass@naive.example:443');
  });

  it('exports Mieru primary range and additional bindings', () => {
    const ib = inbound('mieru', {
      clients: [{ email: 'mieru-user', password: 'mieru-pass' }],
      transport: 'TCP',
      portRangeEnd: 445,
      additionalPortBindings: [{ port: 9000, portRangeEnd: 0, transport: 'UDP' }],
      mtu: 1400,
      clientMultiplexing: 'MULTIPLEXING_LOW',
      clientHandshakeMode: 'HANDSHAKE_STANDARD',
    });
    const client = getSupplementalClients(ib)![0];
    const link = genSupplementalLinks({ inbound: ib, address: 'mieru.example', port: 443, client })[0].link;
    const url = new URL(link);
    expect(url.protocol).toBe('mierus:');
    expect(url.searchParams.getAll('port')).toEqual(['443-445', '9000']);
    expect(url.searchParams.getAll('protocol')).toEqual(['TCP', 'UDP']);
    expect(url.searchParams.get('mtu')).toBe('1400');
  });
});
