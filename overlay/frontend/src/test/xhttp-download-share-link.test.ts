import { describe, expect, it } from 'vitest';

import { genVlessLink } from '@/lib/xray/inbound-link';
import type { Inbound } from '@/schemas/api/inbound';

function fixture(withDownload: boolean, security: 'tls' | 'reality' = 'tls'): Inbound {
  const xhttp: Record<string, unknown> = {
    host: 'cdn03.ijj.kdns.fr',
    path: '/cfw45fc89qy34d',
    mode: 'auto',
    xPaddingBytes: '100-1000',
  };
  if (withDownload) {
    xhttp.downloadSettings = {
      address: security === 'reality' ? '203.0.113.8' : 'cdn03.ijj.kdns.fr',
      port: security === 'reality' ? 9443 : 2083,
      security,
      xhttpSettings: { host: 'cdn03.ijj.kdns.fr' },
      tlsSettings: { serverName: 'cdn03.ijj.kdns.fr', fingerprint: 'chrome', alpn: ['h2'] },
      realitySettings: {
        serverName: 'aws.amazon.com',
        fingerprint: 'chrome',
        publicKey: 'REALITY_DOWNLOAD_PUBLIC_KEY',
        shortId: 'aabbccdd',
      },
    };
  }
  return {
    protocol: 'vless',
    port: 2083,
    settings: { encryption: 'mlkem768x25519plus.native.0rtt.EXAMPLE' },
    streamSettings: {
      network: 'xhttp',
      security: 'tls',
      tlsSettings: {
        serverName: 'cdn03.ijj.kdns.fr',
        alpn: ['h2', 'http/1.1'],
        settings: {
          fingerprint: 'chrome',
          echConfigList: '',
          verifyPeerCertByName: '',
          pinnedPeerCertSha256: [],
        },
      },
      xhttpSettings: xhttp,
    },
  } as unknown as Inbound;
}

function params(withDownload: boolean, security: 'tls' | 'reality' = 'tls'): URLSearchParams {
  const link = genVlessLink({
    inbound: fixture(withDownload, security),
    address: 'v2rayssr.com',
    clientId: '12a1882c-5813-4a0d-9758-fc418ff2db92',
    flow: 'xtls-rprx-vision',
  });
  const url = new URL(link);
  expect(url.host).toBe('v2rayssr.com:2083');
  expect(url.username).toBe('12a1882c-5813-4a0d-9758-fc418ff2db92');
  return url.searchParams;
}

describe('XHTTP split links remain Xray-native', () => {
  it('exports standard TLS/TLS XHTTP downlink without client-specific parameters', () => {
    const query = params(true);
    expect(query.get('type')).toBe('xhttp');
    expect(query.get('host')).toBe('cdn03.ijj.kdns.fr');
    expect(query.get('security')).toBe('tls');
    expect(query.get('encryption')).toBe('mlkem768x25519plus.native.0rtt.EXAMPLE');
    expect(query.has('obfs')).toBe(false);
    expect(query.has('obfsParam')).toBe(false);
    expect(query.has('pbk')).toBe(false);
    expect(query.has('sid')).toBe(false);
    expect(JSON.parse(query.get('extra')!).downloadSettings).toMatchObject({
      address: 'cdn03.ijj.kdns.fr',
      port: 2083,
      security: 'tls',
    });
  });

  it('keeps REALITY downlink credentials nested under Xray extra only', () => {
    const query = params(true, 'reality');
    expect(query.get('type')).toBe('xhttp');
    expect(query.get('security')).toBe('tls');
    expect(query.get('sni')).toBe('cdn03.ijj.kdns.fr');
    expect(query.get('host')).toBe('cdn03.ijj.kdns.fr');
    expect(query.has('pbk')).toBe(false);
    expect(query.has('sid')).toBe(false);
    expect(query.has('obfsParam')).toBe(false);
    expect(query.has('obfs')).toBe(false);
    const down = JSON.parse(query.get('extra')!).downloadSettings;
    expect(down).toMatchObject({
      address: '203.0.113.8',
      port: 9443,
      security: 'reality',
      realitySettings: { publicKey: 'REALITY_DOWNLOAD_PUBLIC_KEY', shortId: 'aabbccdd' },
    });
    expect(down.xhttpSettings.path).toBe('/cfw45fc89qy34d');
  });

  it('does not add client-specific split parameters to ordinary XHTTP', () => {
    const query = params(false);
    expect(query.get('type')).toBe('xhttp');
    expect(query.get('host')).toBe('cdn03.ijj.kdns.fr');
    expect(query.has('obfs')).toBe(false);
    expect(query.has('obfsParam')).toBe(false);
  });
});
