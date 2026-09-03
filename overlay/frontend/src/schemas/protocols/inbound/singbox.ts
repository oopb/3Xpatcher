import { z } from 'zod';

const IntegratedClientSchema = z
  .object({
    email: z.string().optional(),
    id: z.string().optional(),
    password: z.string().optional(),
    auth: z.string().optional(),
    subId: z.string().optional(),
    enable: z.boolean().optional(),
  })
  .loose();

const listenTuning = {
  bindInterface: z.string().default(''),
  routingMark: z.number().int().min(0).default(0),
  reuseAddr: z.boolean().default(false),
  netns: z.string().default(''),
  tcpFastOpen: z.boolean().default(false),
  tcpMultiPath: z.boolean().default(false),
  disableTCPKeepAlive: z.boolean().default(false),
  tcpKeepAlive: z.string().default(''),
  tcpKeepAliveInterval: z.string().default(''),
  udpFragment: z.boolean().optional(),
  udpTimeout: z.string().default(''),
};

const quicTuning = {
  idleTimeout: z.string().default(''),
  keepAlivePeriod: z.string().default(''),
  streamReceiveWindow: z.union([z.number().positive(), z.string()]).optional(),
  connectionReceiveWindow: z.union([z.number().positive(), z.string()]).optional(),
  maxConcurrentStreams: z.number().int().min(0).default(0),
  initialPacketSize: z.number().int().min(0).default(0),
  disablePathMTUDiscovery: z.boolean().default(false),
};

export const TuicInboundSettingsSchema = z
  .object({
    clients: z.array(IntegratedClientSchema).default([]),
    congestionControl: z.enum(['cubic', 'new_reno', 'bbr']).default('cubic'),
    authTimeout: z.string().default('3s'),
    zeroRTTHandshake: z.boolean().default(false),
    heartbeat: z.string().default('10s'),
    ...listenTuning,
    ...quicTuning,
  })
  .loose();

export const AnyTlsInboundSettingsSchema = z
  .object({
    clients: z.array(IntegratedClientSchema).default([]),
    paddingScheme: z.array(z.string()).default([]),
    ...listenTuning,
  })
  .loose();

export const ShadowTlsInboundSettingsSchema = z
  .object({
    clients: z.array(IntegratedClientSchema).default([]),
    version: z.literal(3).default(3),
    handshakeServer: z.string().default('www.cloudflare.com'),
    handshakePort: z.number().int().min(1).max(65535).default(443),
    handshakeForServerNameJson: z.string().default(''),
    strictMode: z.boolean().default(false),
    wildcardSNI: z.enum(['off', 'authed', 'all']).default('off'),
    innerMethod: z
      .enum([
        '2022-blake3-aes-128-gcm',
        '2022-blake3-aes-256-gcm',
        '2022-blake3-chacha20-poly1305',
      ])
      .default('2022-blake3-aes-128-gcm'),
    innerPassword: z.string().default(''),
    ...listenTuning,
  })
  .loose();

export const NaiveInboundSettingsSchema = z
  .object({
    clients: z.array(IntegratedClientSchema).default([]),
    network: z.enum(['tcp', 'udp', '']).default(''),
    quicCongestionControl: z.enum(['bbr', 'cubic', 'reno', '']).default('bbr'),
    ...listenTuning,
  })
  .loose();

export type TuicInboundSettings = z.infer<typeof TuicInboundSettingsSchema>;
export type AnyTlsInboundSettings = z.infer<typeof AnyTlsInboundSettingsSchema>;
export type ShadowTlsInboundSettings = z.infer<typeof ShadowTlsInboundSettingsSchema>;
export type NaiveInboundSettings = z.infer<typeof NaiveInboundSettingsSchema>;
