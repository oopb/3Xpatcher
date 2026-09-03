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

export const MieruInboundSettingsSchema = z
  .object({
    clients: z.array(IntegratedClientSchema).default([]),
    transport: z.enum(['TCP', 'UDP']).default('TCP'),
    portRangeEnd: z.number().int().min(0).max(65535).default(0),
    mtu: z.number().int().min(1280).max(65535).default(1400),
    loggingLevel: z.enum(['FATAL', 'ERROR', 'WARN', 'INFO', 'DEBUG', 'TRACE']).default('INFO'),
    allowPrivateIP: z.boolean().default(false),
    allowLoopbackIP: z.boolean().default(false),
    quotaDays: z.number().int().min(0).default(0),
    quotaMegabytes: z.number().int().min(0).default(0),
    metricsLoggingInterval: z.string().default(''),
    userHintIsMandatory: z.boolean().default(false),
    trafficPatternEnabled: z.boolean().default(false),
    trafficSeed: z.number().int().min(0).default(0),
    trafficUnlockAll: z.boolean().default(false),
    tcpFragmentEnable: z.boolean().default(false),
    tcpFragmentMaxSleepMs: z.number().int().min(0).max(100).default(0),
    nonceType: z
      .enum(['', 'NONCE_TYPE_RANDOM', 'NONCE_TYPE_PRINTABLE', 'NONCE_TYPE_PRINTABLE_SUBSET', 'NONCE_TYPE_FIXED'])
      .default(''),
    nonceApplyToAllUDP: z.boolean().default(false),
    nonceMinLen: z.number().int().min(0).max(12).default(0),
    nonceMaxLen: z.number().int().min(0).max(12).default(0),
    nonceCustomHexStrings: z.array(z.string()).default([]),
    paddingMaxMiddleLen: z.number().int().min(0).max(255).optional(),
    paddingMaxEndLen: z.number().int().min(0).max(255).optional(),
    lowEntropyMode: z
      .enum(['LOW_ENTROPY_MODE_OFF', 'LOW_ENTROPY_MODE_32', 'LOW_ENTROPY_MODE_40', 'LOW_ENTROPY_MODE_48', 'LOW_ENTROPY_MODE_56'])
      .default('LOW_ENTROPY_MODE_OFF'),
    lowEntropyMaskRotation: z.string().default('LOW_ENTROPY_MASK_NO_ROTATION'),
    clientMultiplexing: z
      .enum(['MULTIPLEXING_OFF', 'MULTIPLEXING_LOW', 'MULTIPLEXING_MIDDLE', 'MULTIPLEXING_HIGH'])
      .default('MULTIPLEXING_LOW'),
    clientHandshakeMode: z.enum(['HANDSHAKE_STANDARD', 'HANDSHAKE_NO_WAIT']).default('HANDSHAKE_STANDARD'),
    clientTrafficPattern: z.string().default(''),
  })
  .loose();

export type TuicInboundSettings = z.infer<typeof TuicInboundSettingsSchema>;
export type AnyTlsInboundSettings = z.infer<typeof AnyTlsInboundSettingsSchema>;
export type ShadowTlsInboundSettings = z.infer<typeof ShadowTlsInboundSettingsSchema>;
export type NaiveInboundSettings = z.infer<typeof NaiveInboundSettingsSchema>;
export type MieruInboundSettings = z.infer<typeof MieruInboundSettingsSchema>;
