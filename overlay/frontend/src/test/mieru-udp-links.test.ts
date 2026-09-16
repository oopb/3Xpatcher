import { describe, expect, it } from 'vitest';

import type { Inbound } from '@/schemas/api/inbound';
import { genSupplementalLinks, getSupplementalClients } from '@/lib/xray/supplemental-links';

function mieruInbound(transport: 'TCP' | 'UDP'): Inbound {
  return {
    id: 991,
    up: 0,
    down: 0,
    total: 0,
    remark: 'mieru-primary-udp',
    enable: true,
    expiryTime: 0,
    listen: '0.0.0.0',
    port: 45678,
    protocol: 'mieru',
    settings: {
      clients: [{ email: 'udp-user', password: 'udp-pass' }],
      transport,
      portRangeEnd: 0,
      additionalPortBindings: [],
      mtu: 1400,
      clientMultiplexing: 'MULTIPLEXING_LOW',
      clientHandshakeMode: 'HANDSHAKE_STANDARD',
    },
    streamSettings: {},
    tag: 'inbound-991',
    sniffing: {},
  } as unknown as Inbound;
}

describe('Mieru primary UDP browser export', () => {
  it('exports protocol=UDP rather than silently falling back to TCP', () => {
    const ib = mieruInbound('UDP');
    const client = getSupplementalClients(ib)![0];
    const link = genSupplementalLinks({
      inbound: ib,
      address: 'mieru.example',
      port: 45678,
      remark: 'Mieru UDP',
      client,
    })[0].link;
    const url = new URL(link);
    expect(url.protocol).toBe('mierus:');
    expect(url.searchParams.getAll('port')).toEqual(['45678']);
    expect(url.searchParams.getAll('protocol')).toEqual(['UDP']);
  });
});
