import { Button, Input, InputNumber, Select, Space, Switch } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { RandomUtil } from '@/utils';

export function TuicFields() {
  return (
    <>
      <FormField label="Congestion Control" name={['settings', 'congestionControl']}>
        <Select options={['cubic', 'new_reno', 'bbr'].map((value) => ({ value, label: value }))} />
      </FormField>
      <FormField label="Auth Timeout" name={['settings', 'authTimeout']}>
        <Input placeholder="3s" />
      </FormField>
      <FormField label="Heartbeat" name={['settings', 'heartbeat']}>
        <Input placeholder="10s" />
      </FormField>
      <FormField label="0-RTT Handshake" name={['settings', 'zeroRTTHandshake']} valueProp="checked">
        <Switch />
      </FormField>
    </>
  );
}

export function AnyTlsFields() {
  return (
    <FormField label="Padding Scheme" name={['settings', 'paddingScheme']}>
      <Select
        mode="tags"
        tokenSeparators={[',']}
        placeholder="Leave empty for sing-box defaults"
        style={{ width: '100%' }}
      />
    </FormField>
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
      <FormField label="Version" name={['settings', 'version']}>
        <InputNumber value={3} disabled style={{ width: '100%' }} />
      </FormField>
      <FormField label="Handshake Server" name={['settings', 'handshakeServer']}>
        <Input placeholder="www.cloudflare.com" />
      </FormField>
      <FormField label="Handshake Port" name={['settings', 'handshakePort']}>
        <InputNumber min={1} max={65535} style={{ width: '100%' }} />
      </FormField>
      <FormField label="Strict Mode" name={['settings', 'strictMode']} valueProp="checked">
        <Switch />
      </FormField>
      <FormField label="Wildcard SNI" name={['settings', 'wildcardSNI']}>
        <Select options={['off', 'authed', 'all'].map((value) => ({ value, label: value }))} />
      </FormField>
      <FormField label="Inner Shadowsocks Method" name={['settings', 'innerMethod']}>
        <Select
          options={[
            '2022-blake3-aes-128-gcm',
            '2022-blake3-aes-256-gcm',
            '2022-blake3-chacha20-poly1305',
          ].map((value) => ({ value, label: value }))}
        />
      </FormField>
      <FormField label="Inner Shadowsocks Password" name={['settings', 'innerPassword']}>
        <Space.Compact block>
          <Input.Password />
          <Button onClick={regenerateInnerPassword}>Generate</Button>
        </Space.Compact>
      </FormField>
    </>
  );
}

export function NaiveFields() {
  return (
    <>
      <FormField label="Network" name={['settings', 'network']}>
        <Select
          options={[
            { value: '', label: 'TCP + UDP' },
            { value: 'tcp', label: 'TCP' },
            { value: 'udp', label: 'UDP / QUIC' },
          ]}
        />
      </FormField>
      <FormField label="QUIC Congestion Control" name={['settings', 'quicCongestionControl']}>
        <Select options={['bbr', 'cubic', 'reno'].map((value) => ({ value, label: value }))} />
      </FormField>
    </>
  );
}
