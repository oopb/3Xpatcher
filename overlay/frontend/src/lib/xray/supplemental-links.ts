import { Base64 } from '@/utils';

import type { Inbound } from '@/schemas/api/inbound';
import type { ExternalProxyEntry } from '@/schemas/protocols/stream/external-proxy';

export type SupplementalClientShape = {
  id?: string;
  password?: string;
  email?: string;
  subId?: string;
};

export interface SupplementalLinkVariant {
  link: string;
  label?: string;
}

export interface GenSupplementalLinksInput {
  inbound: Inbound;
  address: string;
  port: number;
  remark?: string;
  client: SupplementalClientShape;
  externalProxy?: ExternalProxyEntry | null;
}

type AnyRecord = Record<string, any>;

function asRecord(value: unknown): AnyRecord {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as AnyRecord)
    : {};
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function asNumber(value: unknown, fallback = 0): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback;
}

function asBoolean(value: unknown): boolean {
  return value === true;
}

function asStringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((v): v is string => typeof v === 'string' && v.length > 0) : [];
}

function formatUrlHost(address: string): string {
  const bare = address.trim().replace(/^\[|\]$/g, '');
  return bare.includes(':') ? `[${bare}]` : bare;
}

function encodeUserinfo(value: string): string {
  return encodeURIComponent(value);
}

function buildLink(base: string, params: URLSearchParams, remark = ''): string {
  const query = params.toString();
  const fragment = remark ? `#${encodeURIComponent(remark)}` : '';
  return `${base}${query ? `?${query}` : ''}${fragment}`;
}

function applyTlsParams(
  inbound: Inbound,
  externalProxy: ExternalProxyEntry | null | undefined,
  params: URLSearchParams,
): void {
  const stream = asRecord(inbound.streamSettings);
  const settings = asRecord(inbound.settings);
  const security = asString(stream.security);

  if (security === 'reality') {
    const reality = asRecord(stream.realitySettings);
    params.set('security', 'reality');
    const names = asStringArray(reality.serverNames);
    const shortIds = asStringArray(reality.shortIds);
    if (names.length > 0) params.set('sni', names[0]);
    if (shortIds.length > 0) params.set('sid', shortIds[0]);
    const client = asRecord(reality.settings);
    const publicKey = asString(client.publicKey).trim();
    const fingerprint = asString(client.fingerprint).trim();
    if (publicKey) params.set('pbk', publicKey);
    if (fingerprint) params.set('fp', fingerprint);
  } else {
    const tls = asRecord(stream.tlsSettings);
    const sni = asString(tls.serverName).trim();
    const alpn = asStringArray(tls.alpn);
    if (sni) params.set('sni', sni);
    if (alpn.length > 0) params.set('alpn', alpn.join(','));

    let selfSigned = asString(tls.certificateMode) === 'self_signed_sni';
    if (!selfSigned && asString(settings.tlsMode) === 'self_signed_sni') {
      selfSigned = true;
      if (!params.has('sni')) {
        const legacySni = asString(settings.camouflageSNI).trim();
        if (legacySni) params.set('sni', legacySni);
      }
    }
    if (selfSigned) params.set('insecure', '1');
  }

  if (externalProxy) {
    const ep = asRecord(externalProxy);
    const sni = asString(ep.sni).trim();
    if (sni) params.set('sni', sni);
    if (asBoolean(ep.allowInsecure)) params.set('insecure', '1');
  }
}

function buildShadowTlsLinks(input: GenSupplementalLinksInput): SupplementalLinkVariant[] {
  const settings = asRecord(input.inbound.settings);
  const method = asString(settings.innerMethod) || '2022-blake3-aes-128-gcm';
  const innerPassword = asString(settings.innerPassword);
  const handshake = asString(settings.handshakeServer).trim();
  const shadowPassword = input.client.password || '';
  const host = formatUrlHost(input.address);
  if (!innerPassword || !handshake || !shadowPassword || !host || input.port < 1 || input.port > 65535) {
    return [];
  }

  const endpoint = `${host}:${input.port}`;
  const descriptor = Base64.encode(
    JSON.stringify({
      version: '3',
      password: shadowPassword,
      host: handshake,
      address: input.address.replace(/^\[|\]$/g, ''),
      port: String(input.port),
    }),
  );
  const params = new URLSearchParams();
  params.set('shadow-tls', descriptor);
  const authority = Base64.encode(`${method}:${innerPassword}@${endpoint}`);
  return [
    {
      link: buildLink(`ss://${authority}`, params, input.remark),
      label: 'ShadowTLS / Shadowrocket',
    },
  ];
}

function applySuiNaiveParams(settings: AnyRecord, params: URLSearchParams): void {
  const sni = params.get('sni');
  if (sni) {
    params.set('peer', sni);
    params.delete('sni');
  }
  params.set('padding', '1');
  params.set('tfo', asBoolean(settings.tcpFastOpen) ? '1' : '0');
}

function buildNaiveLinks(input: GenSupplementalLinksInput): SupplementalLinkVariant[] {
  const settings = asRecord(input.inbound.settings);
  const username = (input.client.email || '').trim();
  const password = input.client.password || '';
  const host = formatUrlHost(input.address);
  if (!username || !password || !host) return [];

  const params = new URLSearchParams();
  applyTlsParams(input.inbound, input.externalProxy, params);
  applySuiNaiveParams(settings, params);

  const rawAuthority = `${username}:${password}@${host}:${input.port}`;
  const variants: SupplementalLinkVariant[] = [
    {
      link: buildLink(`http2://${Base64.encode(rawAuthority)}`, params, input.remark),
      label: 'Naive / Shadowrocket HTTP2',
    },
  ];

  const network = asString(settings.network);
  const nativeSchemes = network === 'tcp'
    ? ['naive+https']
    : network === 'udp'
      ? ['naive+quic']
      : ['naive+https', 'naive+quic'];
  const encodedAuthority = `${encodeUserinfo(username)}:${encodeUserinfo(password)}@${host}:${input.port}`;
  for (const scheme of nativeSchemes) {
    variants.push({
      link: buildLink(`${scheme}://${encodedAuthority}`, params, input.remark),
      label: scheme === 'naive+quic' ? 'Naive / QUIC' : 'Naive / HTTPS',
    });
  }
  return variants;
}

function buildMieruLink(input: GenSupplementalLinksInput): SupplementalLinkVariant[] {
  const settings = asRecord(input.inbound.settings);
  const email = (input.client.email || '').trim();
  const password = input.client.password || '';
  if (!email || !password || !input.address.trim()) return [];

  const params = new URLSearchParams();
  const inboundRemark = asString((input.inbound as unknown as AnyRecord).remark).trim();
  params.set('profile', inboundRemark || input.remark?.trim() || 'default');
  params.set('mtu', String(asNumber(settings.mtu, 1400) || 1400));
  params.set('multiplexing', asString(settings.clientMultiplexing) || 'MULTIPLEXING_LOW');
  params.set('handshake-mode', asString(settings.clientHandshakeMode) || 'HANDSHAKE_STANDARD');
  const trafficPattern = asString(settings.clientTrafficPattern).trim();
  if (trafficPattern) params.set('traffic-pattern', trafficPattern);

  const normalizedTransport = (value: unknown): 'TCP' | 'UDP' =>
    asString(value).trim().toUpperCase() === 'UDP' ? 'UDP' : 'TCP';

  if (input.externalProxy) {
    params.append('port', String(input.port));
    params.append('protocol', normalizedTransport(settings.transport));
  } else {
    const primaryEnd = asNumber(settings.portRangeEnd, 0);
    params.append(
      'port',
      primaryEnd > input.port ? `${input.port}-${primaryEnd}` : String(input.port),
    );
    params.append('protocol', normalizedTransport(settings.transport));

    const extra = Array.isArray(settings.additionalPortBindings) ? settings.additionalPortBindings : [];
    for (const raw of extra) {
      const binding = asRecord(raw);
      const port = asNumber(binding.port, 0);
      if (port < 1 || port > 65535) continue;
      const end = asNumber(binding.portRangeEnd, 0);
      params.append('port', end > port ? `${port}-${end}` : String(port));
      params.append('protocol', normalizedTransport(binding.transport));
    }
  }

  const authority = `${encodeUserinfo(email)}:${encodeUserinfo(password)}@${formatUrlHost(input.address)}`;
  return [{ link: buildLink(`mierus://${authority}`, params, input.remark), label: 'Mieru' }];
}

export function getSupplementalClients(inbound: Inbound): SupplementalClientShape[] | null {
  switch (inbound.protocol) {
    case 'tuic':
    case 'anytls':
    case 'shadowtls':
    case 'naive':
    case 'mieru': {
      const settings = asRecord(inbound.settings);
      return Array.isArray(settings.clients) ? (settings.clients as SupplementalClientShape[]) : [];
    }
    default:
      return null;
  }
}

export function genSupplementalLinks(input: GenSupplementalLinksInput): SupplementalLinkVariant[] {
  const { inbound, client, address, port, remark = '', externalProxy = null } = input;
  const settings = asRecord(inbound.settings);
  const host = formatUrlHost(address);
  if (!host || port < 1 || port > 65535) return [];

  switch (inbound.protocol) {
    case 'tuic': {
      if (!client.id || !client.password) return [];
      const params = new URLSearchParams();
      applyTlsParams(inbound, externalProxy, params);
      const congestion = asString(settings.congestionControl);
      if (congestion) params.set('congestion_control', congestion);
      if (asBoolean(settings.zeroRTTHandshake)) params.set('zero_rtt_handshake', '1');
      const authority = `${encodeUserinfo(client.id)}:${encodeUserinfo(client.password)}@${host}:${port}`;
      return [{ link: buildLink(`tuic://${authority}`, params, remark), label: 'TUIC' }];
    }
    case 'anytls': {
      if (!client.password) return [];
      const params = new URLSearchParams();
      applyTlsParams(inbound, externalProxy, params);
      return [
        {
          link: buildLink(`anytls://${encodeUserinfo(client.password)}@${host}:${port}`, params, remark),
          label: 'AnyTLS',
        },
      ];
    }
    case 'shadowtls':
      return buildShadowTlsLinks(input);
    case 'naive':
      return buildNaiveLinks(input);
    case 'mieru':
      return buildMieruLink(input);
    default:
      return [];
  }
}
