import { useState } from 'react';
import { Form } from 'antd';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { FormProvider, useForm } from 'react-hook-form';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { HttpUtil } from '@/utils';

import XhttpForm from '@/pages/inbounds/form/transport/xhttp';
import { rawInboundToFormValues } from '@/lib/xray/inbound-form-adapter';
import { normalizeXhttpForWire } from '@/lib/xray/stream-wire-normalize';
import { XHttpStreamSettingsSchema } from '@/schemas/protocols/stream/xhttp';


vi.mock('@/pages/inbounds/form/security/RealityTargetScannerModal', () => ({
  default: ({ open, onPick, onClose }: {
    open: boolean;
    onPick: (value: unknown) => void;
    onClose: () => void;
  }) => open ? (
    <button type="button" onClick={() => {
      onPick({
        target: 'www.amd.com:443',
        host: 'www.amd.com',
        serverNames: ['www.amd.com', 'amd.com'],
        feasible: true,
      });
      onClose();
    }}>Use scanned REALITY target</button>
  ) : null,
}));

const validDownload = {
  address: 'cdn.example.com',
  port: 443,
  security: 'tls',
  network: 'xhttp',
  xhttpSettings: { host: 'cdn.example.com' },
  tlsSettings: {
    serverName: 'cdn.example.com',
    fingerprint: 'chrome',
    alpn: ['h2'],
  },
  realitySettings: {
    serverName: '', publicKey: '', shortId: '', fingerprint: 'chrome',
  },
};

function editValues() {
  return rawInboundToFormValues({
    protocol: 'vless',
    port: 443,
    listen: '0.0.0.0',
    settings: { clients: [], decryption: 'none', encryption: 'none' },
    streamSettings: {
      network: 'xhttp',
      security: 'none',
      xhttpSettings: {
        path: '/shared',
        mode: 'auto',
        downloadSettings: validDownload,
      },
    },
  });
}

function Harness({ values }: { values: Record<string, unknown> }) {
  const methods = useForm<Record<string, unknown>>({ defaultValues: values });
  const [wire, setWire] = useState<Record<string, unknown> | null>(null);

  return (
    <FormProvider {...methods}>
      <Form layout="vertical">
        <XhttpForm />
        <button type="button" onClick={() => {
          const xhttp = methods.getValues('streamSettings.xhttpSettings');
          const parsed = XHttpStreamSettingsSchema.parse(xhttp);
          setWire(normalizeXhttpForWire(parsed, 'inbound'));
        }}>Serialize inbound</button>
        <button type="button" onClick={() => {
          setWire(methods.getValues('streamSettings.xhttpSettings') as unknown as Record<string, unknown>);
        }}>Inspect XHTTP form</button>
        <pre data-testid="wire">{JSON.stringify(wire)}</pre>
      </Form>
    </FormProvider>
  );
}

function output(): Record<string, unknown> {
  return JSON.parse(screen.getByTestId('wire').textContent || '{}') as Record<string, unknown>;
}

describe('VLESS XHTTP download switch', () => {
  afterEach(() => vi.restoreAllMocks());
  it('can switch off a saved configuration without nested fields turning it back on', async () => {
    const values = editValues() as unknown as Record<string, unknown>;
    render(<Harness values={values} />);
    const toggle = screen.getByRole('switch', { name: '启用独立下行连接' });
    expect(toggle.getAttribute('aria-checked')).toBe('true');

    fireEvent.click(toggle);
    await waitFor(() => expect(toggle.getAttribute('aria-checked')).toBe('false'));
    await waitFor(() => expect(screen.queryByText('下行连接地址 (IP / 域名)')).toBeNull());

    fireEvent.click(screen.getByRole('button', { name: 'Serialize inbound' }));
    expect(output()).not.toHaveProperty('downloadSettings');
    expect(output()).not.toHaveProperty('enableDownloadSettings');

    fireEvent.click(toggle);
    await waitFor(() => expect(toggle.getAttribute('aria-checked')).toBe('true'));
    fireEvent.click(screen.getByRole('button', { name: 'Serialize inbound' }));
    expect(output()).toHaveProperty('downloadSettings');
  });

  it('leaves origin address and listen port blank instead of guessing CDN/443/9443', () => {
    const values = {
      protocol: 'vless',
      port: 443,
      shareAddr: 'cdn.example.com',
      listen: '0.0.0.0',
      streamSettings: {
        network: 'xhttp',
        security: 'tls',
        tlsSettings: { serverName: 'cdn.example.com' },
        xhttpSettings: XHttpStreamSettingsSchema.parse({ path: '/shared', host: 'cdn.example.com' }),
      },
    };
    render(<Harness values={values} />);
    fireEvent.click(screen.getByRole('switch', { name: '启用独立下行连接' }));
    fireEvent.click(screen.getByRole('button', { name: 'Inspect XHTTP form' }));
    const down = output().downloadSettings as Record<string, any>;
    expect(down.address).toBe('');
    expect(down.port).toBeUndefined();
    expect(down.xhttpSettings.host).toBe('');
    expect(down.realitySettings.serverName).toBe('');
  });

  it('preserves manually selected REALITY downlink port, Target and minClientVer', async () => {
    const values = editValues() as Record<string, any>;
    values.streamSettings.security = 'tls';
    values.streamSettings.xhttpSettings.downloadSettings = {
      address: '203.0.113.5',
      port: 10443,
      network: 'xhttp',
      security: 'reality',
      xhttpSettings: { host: '' },
      realitySettings: { serverName: 'amd.com', publicKey: 'pk', shortId: 'abcd', fingerprint: 'chrome' },
      managedListener: {
        enabled: false,
        listen: '0.0.0.0',
        realitySettings: {
          target: 'amd.com:8443',
          serverNames: ['amd.com'],
          shortIds: ['abcd'],
          privateKey: 'secret',
          publicKey: 'pk',
          minClientVer: '2.4.6',
        },
      },
    };
    render(<Harness values={values} />);
    const toggle = screen.getByRole('switch', { name: 'REALITY 服务端自动监听' });
    fireEvent.click(toggle);
    await waitFor(() => expect(toggle.getAttribute('aria-checked')).toBe('true'));
    fireEvent.click(screen.getByRole('button', { name: 'Inspect XHTTP form' }));
    const down = output().downloadSettings as Record<string, any>;
    expect(down.port).toBe(10443);
    expect(down.managedListener.realitySettings.target).toBe('amd.com:8443');
    expect(down.managedListener.realitySettings.minClientVer).toBe('2.4.6');
  });


  it('places the native REALITY picker beside outer SNI, without advanced settings', () => {
    const values = editValues() as Record<string, any>;
    values.streamSettings.security = 'tls';
    values.streamSettings.xhttpSettings.downloadSettings = {
      ...validDownload,
      security: 'reality',
      realitySettings: { serverName: '', publicKey: '', shortId: '', fingerprint: 'chrome' },
    };
    render(<Harness values={values} />);
    expect(screen.getByRole('button', { name: '选择 REALITY 伪装目标' })).toBeTruthy();
    expect(screen.getByRole('switch', { name: 'REALITY 服务端自动监听' })).toBeTruthy();
    expect(screen.queryByText('高级设置（通常无需修改）')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '选择 REALITY 伪装目标' }));
    fireEvent.click(screen.getByRole('button', { name: 'Use scanned REALITY target' }));
    fireEvent.click(screen.getByRole('button', { name: 'Inspect XHTTP form' }));
    const down = output().downloadSettings as Record<string, any>;
    expect(down.realitySettings.serverName).toBe('www.amd.com');
    expect(down.managedListener.realitySettings.target).toBe('www.amd.com:443');
    expect(down.managedListener.realitySettings.serverNames).toEqual(['www.amd.com', 'amd.com']);
    expect(down.managedListener.enabled).toBeFalsy();
  });

  it('shows editable managed fields and synchronizes manual SNI and key edits', async () => {
    const values = editValues() as Record<string, any>;
    values.streamSettings.security = 'tls';
    values.streamSettings.xhttpSettings.downloadSettings = {
      address: '203.0.113.5',
      port: 10443,
      network: 'xhttp',
      security: 'reality',
      xhttpSettings: { host: '' },
      realitySettings: { serverName: 'www.amd.com', publicKey: 'pk', shortId: 'abcd', fingerprint: 'chrome' },
      managedListener: {
        enabled: false,
        listen: '0.0.0.0',
        realitySettings: {
          target: 'www.amd.com:443', serverNames: ['www.amd.com'],
          shortIds: ['abcd'], privateKey: 'secret', publicKey: 'pk', minClientVer: '1.8.2',
        },
      },
    };
    render(<Harness values={values} />);
    expect(screen.getByText('REALITY Public Key')).toBeTruthy();
    const toggle = screen.getByRole('switch', { name: 'REALITY 服务端自动监听' });
    fireEvent.click(toggle);
    await waitFor(() => expect(toggle.getAttribute('aria-checked')).toBe('true'));
    expect(screen.getByText('REALITY Target（远端伪装目标）')).toBeTruthy();
    expect(screen.getByText('REALITY Public Key')).toBeTruthy();
    expect(screen.getByText('REALITY Short ID')).toBeTruthy();
    for (const name of [
      'REALITY 监听 IP',
      'REALITY Target（远端伪装目标）',
      'REALITY serverNames（允许的 SNI）',
      'REALITY shortIds（允许的 Short ID）',
      'REALITY Private Key（私钥，仅服务端）',
      'REALITY 服务端 Public Key',
      'REALITY 最低客户端版本 (minClientVer)',
    ]) expect(screen.getByText(name)).toBeTruthy();
    expect(screen.getByRole('button', { name: '重新生成 REALITY 密钥对（同步公钥）' })).toBeTruthy();
    expect(screen.getByRole('button', { name: '重新生成 Short ID（同步客户端）' })).toBeTruthy();
    fireEvent.change(screen.getByPlaceholderText('选择或输入 REALITY SNI'), {
      target: { value: 'new.example.com' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Inspect XHTTP form' }));
    const down = output().downloadSettings as Record<string, any>;
    expect(down.realitySettings.serverName).toBe('new.example.com');
    expect(down.managedListener.realitySettings.serverNames).toEqual(['www.amd.com', 'new.example.com']);
    fireEvent.input(screen.getByPlaceholderText('自动生成，也可手动输入对应公钥'), {
      target: { value: 'custom-pk' },
    });
    await waitFor(() => {
      expect((screen.getByPlaceholderText('自动生成，也可手动输入对应公钥') as HTMLInputElement).value)
        .toBe('custom-pk');
    });
    fireEvent.click(screen.getByRole('button', { name: 'Inspect XHTTP form' }));
    expect((output().downloadSettings as Record<string, any>).realitySettings.publicKey)
      .toBe('custom-pk');
    fireEvent.input(screen.getByPlaceholderText('自动生成，也可手动输入十六进制 Short ID'), {
      target: { value: 'aa12' },
    });
    fireEvent.input(screen.getByPlaceholderText('自动生成，也可输入与公钥匹配的私钥'), {
      target: { value: 'custom-private' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Inspect XHTTP form' }));
    const edited = output().downloadSettings as Record<string, any>;
    expect(edited.realitySettings.publicKey).toBe('custom-pk');
    expect(edited.managedListener.realitySettings.publicKey).toBe('custom-pk');
    expect(edited.realitySettings.shortId).toBe('aa12');
    expect(edited.managedListener.realitySettings.shortIds).toContain('aa12');
    expect(edited.managedListener.realitySettings.privateKey).toBe('custom-private');
    fireEvent.click(toggle);
    await waitFor(() => expect(toggle.getAttribute('aria-checked')).toBe('false'));
    expect(screen.getByText('REALITY Public Key')).toBeTruthy();
    expect(screen.queryByText('REALITY Target（远端伪装目标）')).toBeNull();
  });

  it('generates a managed REALITY keypair on enable and exposes both values', async () => {
    const values = editValues() as Record<string, any>;
    values.streamSettings.security = 'tls';
    values.streamSettings.xhttpSettings.downloadSettings = {
      address: '203.0.113.5',
      port: 10443,
      network: 'xhttp',
      security: 'reality',
      xhttpSettings: { host: '' },
      realitySettings: { serverName: 'www.amd.com', publicKey: '', shortId: '', fingerprint: 'chrome' },
      managedListener: {
        enabled: false,
        listen: '0.0.0.0',
        realitySettings: {
          target: 'www.amd.com:443', serverNames: ['www.amd.com'],
          shortIds: [], privateKey: '', publicKey: '', minClientVer: '1.8.2',
        },
      },
    };
    vi.spyOn(HttpUtil, 'get').mockResolvedValue({
      success: true,
      obj: { privateKey: 'generated-private', publicKey: 'generated-public' },
    } as any);
    render(<Harness values={values} />);
    const toggle = screen.getByRole('switch', { name: 'REALITY 服务端自动监听' });
    fireEvent.click(toggle);
    await waitFor(() => expect(toggle.getAttribute('aria-checked')).toBe('true'));
    fireEvent.click(screen.getByRole('button', { name: 'Inspect XHTTP form' }));
    const down = output().downloadSettings as Record<string, any>;
    expect(down.realitySettings.publicKey).toBe('generated-public');
    expect(down.managedListener.realitySettings.publicKey).toBe('generated-public');
    expect(down.managedListener.realitySettings.privateKey).toBe('generated-private');
    expect(down.realitySettings.shortId).toBeTruthy();
    expect(down.managedListener.realitySettings.shortIds).toContain(down.realitySettings.shortId);
    expect(screen.getByPlaceholderText('自动生成，也可输入与公钥匹配的私钥')).toBeTruthy();
    expect(screen.getByPlaceholderText('自动生成，与客户端公钥保持一致')).toBeTruthy();
  });

  it('defaults to disabled for a new inbound and discards invalid fields when disabled', async () => {
    const values = {
      protocol: 'vless',
      port: 443,
      listen: '0.0.0.0',
      streamSettings: {
        network: 'xhttp',
        security: 'none',
        xhttpSettings: XHttpStreamSettingsSchema.parse({ path: '/new' }),
      },
    };
    render(<Harness values={values} />);
    const toggle = screen.getByRole('switch', { name: '启用独立下行连接' });
    expect(toggle.getAttribute('aria-checked')).toBe('false');

    fireEvent.click(toggle);
    await waitFor(() => expect(toggle.getAttribute('aria-checked')).toBe('true'));
    fireEvent.click(toggle);
    await waitFor(() => expect(toggle.getAttribute('aria-checked')).toBe('false'));
    fireEvent.click(screen.getByRole('button', { name: 'Serialize inbound' }));
    expect(output()).not.toHaveProperty('downloadSettings');
  });
});
