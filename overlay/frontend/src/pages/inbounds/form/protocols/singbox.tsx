import { Alert, Button, Divider, Input, InputNumber, Select, Space, Switch } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { RandomUtil } from '@/utils';

function ListenTuningFields() {
  return (
    <>
      <Divider orientation="left" plain>Listen / Socket</Divider>
      <FormField label="Bind Interface" name={['settings', 'bindInterface']}><Input placeholder="eth0 (optional)" /></FormField>
      <FormField label="Routing Mark" name={['settings', 'routingMark']}><InputNumber min={0} style={{ width: '100%' }} /></FormField>
      <FormField label="Network Namespace" name={['settings', 'netns']}><Input placeholder="optional" /></FormField>
      <FormField label="Reuse Address" name={['settings', 'reuseAddr']} valueProp="checked"><Switch /></FormField>
      <FormField label="TCP Fast Open" name={['settings', 'tcpFastOpen']} valueProp="checked"><Switch /></FormField>
      <FormField label="TCP Multi Path" name={['settings', 'tcpMultiPath']} valueProp="checked"><Switch /></FormField>
      <FormField label="Disable TCP Keep Alive" name={['settings', 'disableTCPKeepAlive']} valueProp="checked"><Switch /></FormField>
      <FormField label="TCP Keep Alive" name={['settings', 'tcpKeepAlive']}><Input placeholder="5m" /></FormField>
      <FormField label="TCP Keep Alive Interval" name={['settings', 'tcpKeepAliveInterval']}><Input placeholder="75s" /></FormField>
      <FormField label="UDP Fragment" name={['settings', 'udpFragment']}>
        <Select allowClear placeholder="Protocol default" options={[{ value: true, label: 'Enabled' }, { value: false, label: 'Disabled' }]} />
      </FormField>
      <FormField label="UDP Timeout" name={['settings', 'udpTimeout']}><Input placeholder="5m" /></FormField>
    </>
  );
}

function QuicTuningFields() {
  return (
    <>
      <Divider orientation="left" plain>QUIC Advanced</Divider>
      <FormField label="Idle Timeout" name={['settings', 'idleTimeout']}><Input placeholder="30s" /></FormField>
      <FormField label="Keep Alive Period" name={['settings', 'keepAlivePeriod']}><Input placeholder="10s" /></FormField>
      <FormField label="Stream Receive Window" name={['settings', 'streamReceiveWindow']}><Input placeholder="8mb or bytes" /></FormField>
      <FormField label="Connection Receive Window" name={['settings', 'connectionReceiveWindow']}><Input placeholder="32mb or bytes" /></FormField>
      <FormField label="Max Concurrent Streams" name={['settings', 'maxConcurrentStreams']}><InputNumber min={0} style={{ width: '100%' }} /></FormField>
      <FormField label="Initial Packet Size" name={['settings', 'initialPacketSize']}><InputNumber min={0} style={{ width: '100%' }} /></FormField>
      <FormField label="Disable Path MTU Discovery" name={['settings', 'disablePathMTUDiscovery']} valueProp="checked"><Switch /></FormField>
    </>
  );
}

export function TuicFields() {
  return (
    <>
      <FormField label="Congestion Control" name={['settings', 'congestionControl']}><Select options={['cubic', 'new_reno', 'bbr'].map((value) => ({ value, label: value }))} /></FormField>
      <FormField label="Auth Timeout" name={['settings', 'authTimeout']}><Input placeholder="3s" /></FormField>
      <FormField label="Heartbeat" name={['settings', 'heartbeat']}><Input placeholder="10s" /></FormField>
      <FormField label="0-RTT Handshake" name={['settings', 'zeroRTTHandshake']} valueProp="checked"><Switch /></FormField>
      <QuicTuningFields />
      <ListenTuningFields />
    </>
  );
}

export function AnyTlsFields() {
  return (
    <>
      <FormField label="Padding Scheme" name={['settings', 'paddingScheme']}>
        <Select mode="tags" tokenSeparators={[',']} placeholder="Leave empty for sing-box defaults" style={{ width: '100%' }} />
      </FormField>
      <ListenTuningFields />
    </>
  );
}

export function ShadowTlsFields() {
  const { control, setValue } = useFormContext();
  const innerMethod = useWatch({ control, name: 'settings.innerMethod' }) as string | undefined;
  const regenerateInnerPassword = () =>
    setValue(
      'settings.innerPassword',
      RandomUtil.randomShadowsocksPassword(
        innerMethod === '2022-blake3-aes-256-gcm' || innerMethod === '2022-blake3-chacha20-poly1305'
          ? innerMethod
          : '2022-blake3-aes-128-gcm',
      ),
      { shouldDirty: true },
    );
  return (
    <>
      <FormField label="Version" name={['settings', 'version']}><InputNumber value={3} disabled style={{ width: '100%' }} /></FormField>
      <FormField label="Handshake Server" name={['settings', 'handshakeServer']}><Input placeholder="www.cloudflare.com" /></FormField>
      <FormField label="Handshake Port" name={['settings', 'handshakePort']}><InputNumber min={1} max={65535} style={{ width: '100%' }} /></FormField>
      <FormField label="Handshake by SNI (JSON)" name={['settings', 'handshakeForServerNameJson']}>
        <Input.TextArea autoSize={{ minRows: 3, maxRows: 8 }} placeholder={'{"example.com":{"server":"example.com","server_port":443}}'} />
      </FormField>
      <FormField label="Strict Mode" name={['settings', 'strictMode']} valueProp="checked"><Switch /></FormField>
      <FormField label="Wildcard SNI" name={['settings', 'wildcardSNI']}><Select options={['off', 'authed', 'all'].map((value) => ({ value, label: value }))} /></FormField>
      <FormField label="Inner Shadowsocks Method" name={['settings', 'innerMethod']}>
        <Select options={['2022-blake3-aes-128-gcm','2022-blake3-aes-256-gcm','2022-blake3-chacha20-poly1305'].map((value) => ({ value, label: value }))} />
      </FormField>
      <FormField label="Inner Shadowsocks Password" name={['settings', 'innerPassword']}>
        <Space.Compact block><Input.Password /><Button onClick={regenerateInnerPassword}>Generate</Button></Space.Compact>
      </FormField>
      <ListenTuningFields />
    </>
  );
}

export function NaiveFields() {
  return (
    <>
      <FormField label="Network" name={['settings', 'network']}><Select options={[{ value: '', label: 'TCP + UDP' }, { value: 'tcp', label: 'TCP' }, { value: 'udp', label: 'UDP / QUIC' }]} /></FormField>
      <FormField label="QUIC Congestion Control" name={['settings', 'quicCongestionControl']}><Select options={['bbr', 'cubic', 'reno'].map((value) => ({ value, label: value }))} /></FormField>
      <ListenTuningFields />
    </>
  );
}

export function MieruFields() {
  const { control } = useFormContext();
  const trafficEnabled = !!useWatch({ control, name: 'settings.trafficPatternEnabled' });
  const nonceType = useWatch({ control, name: 'settings.nonceType' }) as string | undefined;
  const lowEntropyMode = useWatch({ control, name: 'settings.lowEntropyMode' }) as string | undefined;
  return (
    <>
      <Alert
        type="info"
        showIcon
        title="Official Mieru / mita runtime"
        description="Each 3x-ui Mieru inbound runs in an isolated mita instance so attached clients cannot authenticate on other Mieru inbound ports. The main Port field is the single port or start of the range."
        style={{ marginBottom: 12 }}
      />
      <FormField label="Transport" name={['settings', 'transport']}>
        <Select options={[{ value: 'TCP', label: 'TCP (recommended)' }, { value: 'UDP', label: 'UDP' }]} />
      </FormField>
      <FormField label="Port Range End" name={['settings', 'portRangeEnd']}>
        <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="0 = single port" />
      </FormField>
      <FormField label="MTU" name={['settings', 'mtu']}><InputNumber min={1280} max={65535} style={{ width: '100%' }} /></FormField>
      <FormField label="Logging Level" name={['settings', 'loggingLevel']}>
        <Select options={['FATAL', 'ERROR', 'WARN', 'INFO', 'DEBUG', 'TRACE'].map((value) => ({ value, label: value }))} />
      </FormField>

      <Divider orientation="left" plain>User Policy</Divider>
      <FormField label="Allow Private IP" name={['settings', 'allowPrivateIP']} valueProp="checked"><Switch /></FormField>
      <FormField label="Allow Loopback IP" name={['settings', 'allowLoopbackIP']} valueProp="checked"><Switch /></FormField>
      <FormField label="Quota Period (days)" name={['settings', 'quotaDays']}><InputNumber min={0} style={{ width: '100%' }} placeholder="0 = use 3x-ui limits only" /></FormField>
      <FormField label="Mieru Quota (MB)" name={['settings', 'quotaMegabytes']}><InputNumber min={0} style={{ width: '100%' }} placeholder="set together with days" /></FormField>
      <FormField label="Metrics Logging Interval" name={['settings', 'metricsLoggingInterval']}><Input placeholder="30s / 5m / 2h (optional)" /></FormField>
      <FormField label="Require User Hint" name={['settings', 'userHintIsMandatory']} valueProp="checked"><Switch /></FormField>

      <Divider orientation="left" plain>Traffic Pattern</Divider>
      <FormField label="Custom Traffic Pattern" name={['settings', 'trafficPatternEnabled']} valueProp="checked"><Switch /></FormField>
      {!trafficEnabled && (
        <Alert type="success" showIcon title="Using official Mieru implicit/default traffic pattern" style={{ marginBottom: 12 }} />
      )}
      {trafficEnabled && (
        <>
          <FormField label="Seed" name={['settings', 'trafficSeed']}><InputNumber min={0} style={{ width: '100%' }} /></FormField>
          <FormField label="Unlock All Implicit Patterns" name={['settings', 'trafficUnlockAll']} valueProp="checked"><Switch /></FormField>
          <FormField label="TCP Fragment" name={['settings', 'tcpFragmentEnable']} valueProp="checked"><Switch /></FormField>
          <FormField label="TCP Fragment Max Sleep (ms)" name={['settings', 'tcpFragmentMaxSleepMs']}><InputNumber min={0} max={100} style={{ width: '100%' }} /></FormField>
          <FormField label="Nonce Type" name={['settings', 'nonceType']}>
            <Select options={[
              { value: '', label: 'Implicit / not explicitly set' },
              { value: 'NONCE_TYPE_RANDOM', label: 'Random' },
              { value: 'NONCE_TYPE_PRINTABLE', label: 'Printable ASCII' },
              { value: 'NONCE_TYPE_PRINTABLE_SUBSET', label: 'Printable subset' },
              { value: 'NONCE_TYPE_FIXED', label: 'Fixed custom prefix' },
            ]} />
          </FormField>
          {!!nonceType && (
            <>
              <FormField label="Nonce Apply To All UDP Packets" name={['settings', 'nonceApplyToAllUDP']} valueProp="checked"><Switch /></FormField>
              <FormField label="Nonce Min Length" name={['settings', 'nonceMinLen']}><InputNumber min={0} max={12} style={{ width: '100%' }} /></FormField>
              <FormField label="Nonce Max Length" name={['settings', 'nonceMaxLen']}><InputNumber min={0} max={12} style={{ width: '100%' }} /></FormField>
              {nonceType === 'NONCE_TYPE_FIXED' && (
                <FormField label="Fixed Nonce Hex Prefixes" name={['settings', 'nonceCustomHexStrings']}>
                  <Select mode="tags" tokenSeparators={[',']} placeholder="00010203,04050607" />
                </FormField>
              )}
            </>
          )}
          <FormField label="Max Middle Padding Length" name={['settings', 'paddingMaxMiddleLen']}><InputNumber min={0} max={255} style={{ width: '100%' }} placeholder="empty = internal value; 0 = disabled" /></FormField>
          <FormField label="Max End Padding Length" name={['settings', 'paddingMaxEndLen']}><InputNumber min={0} max={255} style={{ width: '100%' }} placeholder="empty = internal value; 0 = disabled" /></FormField>
          <FormField label="Low Entropy Mode" name={['settings', 'lowEntropyMode']}>
            <Select options={[
              { value: 'LOW_ENTROPY_MODE_OFF', label: 'Off' },
              { value: 'LOW_ENTROPY_MODE_32', label: '32-bit data / 64-bit chunk' },
              { value: 'LOW_ENTROPY_MODE_40', label: '40-bit data / 64-bit chunk' },
              { value: 'LOW_ENTROPY_MODE_48', label: '48-bit data / 64-bit chunk' },
              { value: 'LOW_ENTROPY_MODE_56', label: '56-bit data / 64-bit chunk' },
            ]} />
          </FormField>
          {lowEntropyMode && lowEntropyMode !== 'LOW_ENTROPY_MODE_OFF' && (
            <FormField label="Low Entropy Mask Rotation" name={['settings', 'lowEntropyMaskRotation']}>
              <Select showSearch options={[
                { value: 'LOW_ENTROPY_MASK_NO_ROTATION', label: 'No rotation' },
                ...Array.from({ length: 15 }, (_, i) => ({ value: `LOW_ENTROPY_MASK_ROTATE_RIGHT_${i + 1}`, label: `Rotate right ${i + 1}` })),
                ...Array.from({ length: 15 }, (_, i) => ({ value: `LOW_ENTROPY_MASK_ROTATE_LEFT_${i + 1}`, label: `Rotate left ${i + 1}` })),
              ]} />
            </FormField>
          )}
        </>
      )}

      <Divider orientation="left" plain>Client Share Defaults</Divider>
      <Alert type="info" showIcon title="These fields affect generated mierus:// / Mihomo client profiles, not the mita server traffic-pattern object above." style={{ marginBottom: 12 }} />
      <FormField label="Multiplexing" name={['settings', 'clientMultiplexing']}>
        <Select options={['MULTIPLEXING_OFF', 'MULTIPLEXING_LOW', 'MULTIPLEXING_MIDDLE', 'MULTIPLEXING_HIGH'].map((value) => ({ value, label: value }))} />
      </FormField>
      <FormField label="Handshake Mode" name={['settings', 'clientHandshakeMode']}>
        <Select options={[{ value: 'HANDSHAKE_STANDARD', label: 'STANDARD' }, { value: 'HANDSHAKE_NO_WAIT', label: 'NO_WAIT (0-RTT)' }]} />
      </FormField>
      <FormField label="Client Traffic Pattern" name={['settings', 'clientTrafficPattern']}>
        <Input placeholder="optional official encoded traffic-pattern value" />
      </FormField>
    </>
  );
}
