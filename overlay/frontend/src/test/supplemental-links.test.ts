import { describe, expect, it } from 'vitest';

import type { Inbound } from '@/schemas/api/inbound';
import { genSupplementalLinks, getSupplementalClients } from '@/lib/xray/supplemental-links';
import { Base64 } from '@/utils';

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
  it('exports TUIC without server heartbeat leakage', () => {
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
    expect(url.searchParams.get('heartbeat')).toBeNull();
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

  it('exports Shadowrocket descriptor ShadowTLS instead of SIP003', () => {
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
    expect(variants[0].label).toBe('ShadowTLS / Shadowrocket');
    const url = new URL(variants[0].link);
    expect(url.searchParams.get('plugin')).toBeNull();
    const descriptorRaw = url.searchParams.get('shadow-tls');
    expect(descriptorRaw).toBeTruthy();
    const descriptor = JSON.parse(Base64.decode(descriptorRaw!));
    expect(descriptor).toMatchObject({
      version: '3',
      password: 'outer-pass',
      host: 'www.cloudflare.com',
      address: 'shadow.example',
      port: '443',
    });
  });

  it('exports S-UI compatible Naive HTTP2 plus native link', () => {
    const ib = inbound(
      'naive',
      {
        clients: [{ email: 'naive-user', password: 'naive-pass' }],
        network: 'tcp',
        tcpFastOpen: true,
      },
      {
        security: 'tls',
        tlsSettings: {
          serverName: 'naive.example',
          alpn: ['h2'],
          certificateMode: 'self_signed_sni',
        },
      },
    );
    const client = getSupplementalClients(ib)![0];
    const variants = genSupplementalLinks({ inbound: ib, address: '203.0.113.40', port: 443, client });
    expect(variants).toHaveLength(2);
    const http2 = new URL(variants[0].link);
    expect(http2.protocol).toBe('http2:');
    expect(Base64.decode(http2.host)).toBe('naive-user:naive-pass@203.0.113.40:443');
    expect(http2.searchParams.get('padding')).toBe('1');
    expect(http2.searchParams.get('peer')).toBe('naive.example');
    expect(http2.searchParams.get('sni')).toBeNull();
    expect(http2.searchParams.get('alpn')).toBe('h2');
    expect(http2.searchParams.get('insecure')).toBe('1');
    expect(http2.searchParams.get('tfo')).toBe('1');
    expect(variants[1].link).toContain('naive+https://naive-user:naive-pass@203.0.113.40:443');
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
