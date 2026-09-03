import { Button, Divider, Input, InputNumber, Select, Space, Switch } from 'antd';
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
        <Select
          allowClear
          placeholder="Protocol default"
          options={[
            { value: true, label: 'Enabled' },
            { value: false, label: 'Disabled' },
          ]}
        />
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
      <FormField label="Congestion Control" name={['settings', 'congestionControl']}>
        <Select options={['cubic', 'new_reno', 'bbr'].map((value) => ({ value, label: value }))} />
      </FormField>
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
        <Input.TextArea
          autoSize={{ minRows: 3, maxRows: 8 }}
          placeholder={'{"example.com":{"server":"example.com","server_port":443}}'}
        />
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
      <FormField label="Network" name={['settings', 'network']}>
        <Select options={[{ value: '', label: 'TCP + UDP' }, { value: 'tcp', label: 'TCP' }, { value: 'udp', label: 'UDP / QUIC' }]} />
      </FormField>
      <FormField label="QUIC Congestion Control" name={['settings', 'quicCongestionControl']}>
        <Select options={['bbr', 'cubic', 'reno'].map((value) => ({ value, label: value }))} />
      </FormField>
      <ListenTuningFields />
    </>
  );
}
