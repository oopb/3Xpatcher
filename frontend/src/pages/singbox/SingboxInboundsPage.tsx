import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Col,
  Divider,
  Flex,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import type { FormInstance, TableProps } from 'antd';
import {
  CheckCircleOutlined,
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  ReloadOutlined,
  SyncOutlined,
} from '@ant-design/icons';

import { HttpUtil } from '@/utils';

type Protocol = 'tuic' | 'anytls' | 'shadowtls' | 'naive';

type SingboxInbound = {
  id: number;
  remark: string;
  enable: boolean;
  listen: string;
  port: number;
  protocol: Protocol;
  settings: string;
  tag: string;
  createdAt: number;
  updatedAt: number;
};

type StatusPayload = {
  status: string;
  version: string;
};

type TLSSettings = {
  enabled: boolean;
  serverName?: string;
  alpn?: string[];
  certificatePath: string;
  keyPath: string;
};

type FormValues = {
  remark: string;
  enable: boolean;
  listen: string;
  port: number;
  protocol: Protocol;
  tag?: string;
  settings: Record<string, unknown>;
};

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };
const protocolOptions = [
  { value: 'tuic', label: 'TUIC' },
  { value: 'anytls', label: 'AnyTLS' },
  { value: 'shadowtls', label: 'ShadowTLS v3' },
  { value: 'naive', label: 'Naive' },
] satisfies { value: Protocol; label: string }[];

function randomPassword(bytes = 18) {
  const raw = new Uint8Array(bytes);
  crypto.getRandomValues(raw);
  return btoa(String.fromCharCode(...raw)).replace(/=+$/g, '');
}

function randomBase64(bytes: number) {
  const raw = new Uint8Array(bytes);
  crypto.getRandomValues(raw);
  return btoa(String.fromCharCode(...raw));
}

function defaultTLS(): TLSSettings {
  return {
    enabled: true,
    certificatePath: '',
    keyPath: '',
  };
}

function defaultSettings(protocol: Protocol): Record<string, unknown> {
  switch (protocol) {
    case 'tuic':
      return {
        users: [{ name: 'user', uuid: crypto.randomUUID(), password: randomPassword() }],
        congestionControl: 'cubic',
        heartbeat: '10s',
        zeroRTTHandshake: false,
        tls: defaultTLS(),
      };
    case 'anytls':
      return {
        users: [{ name: 'user', password: randomPassword() }],
        paddingScheme: [],
        tls: defaultTLS(),
      };
    case 'shadowtls':
      return {
        users: [{ name: 'user', password: randomPassword() }],
        handshakeServer: 'www.microsoft.com',
        handshakePort: 443,
        strictMode: false,
        wildcardSNI: 'off',
        innerMethod: '2022-blake3-aes-128-gcm',
        innerPassword: randomBase64(16),
      };
    case 'naive':
      return {
        network: '',
        users: [{ username: 'user', password: randomPassword() }],
        quicCongestionControl: 'bbr',
        tls: defaultTLS(),
      };
  }
}

function safeParseSettings(row: SingboxInbound): Record<string, unknown> {
  try {
    const parsed = JSON.parse(row.settings);
    return parsed && typeof parsed === 'object' ? parsed : defaultSettings(row.protocol);
  } catch {
    return defaultSettings(row.protocol);
  }
}

function protocolLabel(protocol: Protocol) {
  return protocolOptions.find((item) => item.value === protocol)?.label || protocol;
}

function TLSFields() {
  return (
    <>
      <Divider orientation="left">TLS</Divider>
      <Form.Item name={['settings', 'tls', 'enabled']} valuePropName="checked" initialValue>
        <Switch checkedChildren="TLS on" unCheckedChildren="TLS off" disabled />
      </Form.Item>
      <Row gutter={12}>
        <Col xs={24} md={12}>
          <Form.Item
            label="Certificate path"
            name={['settings', 'tls', 'certificatePath']}
            rules={[{ required: true }]}
          >
            <Input placeholder="/root/cert/fullchain.pem" />
          </Form.Item>
        </Col>
        <Col xs={24} md={12}>
          <Form.Item
            label="Private key path"
            name={['settings', 'tls', 'keyPath']}
            rules={[{ required: true }]}
          >
            <Input placeholder="/root/cert/privkey.pem" />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={12}>
        <Col xs={24} md={12}>
          <Form.Item label="Server name" name={['settings', 'tls', 'serverName']}>
            <Input placeholder="Optional" />
          </Form.Item>
        </Col>
        <Col xs={24} md={12}>
          <Form.Item label="ALPN" name={['settings', 'tls', 'alpn']}>
            <Select mode="tags" tokenSeparators={[',']} placeholder="e.g. h3" />
          </Form.Item>
        </Col>
      </Row>
    </>
  );
}

function PasswordUsers({ kind }: { kind: 'password' | 'tuic' | 'naive' }) {
  return (
    <Form.List name={['settings', 'users']}>
      {(fields, { add, remove }) => (
        <Space direction="vertical" style={{ width: '100%' }}>
          {fields.map((field) => (
            <Card key={field.key} size="small">
              <Row gutter={12} align="middle">
                <Col xs={24} md={kind === 'tuic' ? 6 : 8}>
                  <Form.Item
                    {...field}
                    label={kind === 'naive' ? 'Username' : 'Name'}
                    name={[field.name, kind === 'naive' ? 'username' : 'name']}
                    rules={kind === 'naive' ? [{ required: true }] : []}
                  >
                    <Input />
                  </Form.Item>
                </Col>
                {kind === 'tuic' && (
                  <Col xs={24} md={8}>
                    <Form.Item
                      {...field}
                      label="UUID"
                      name={[field.name, 'uuid']}
                      rules={[{ required: true }]}
                    >
                      <Input />
                    </Form.Item>
                  </Col>
                )}
                <Col xs={24} md={kind === 'tuic' ? 8 : 14}>
                  <Form.Item
                    {...field}
                    label="Password"
                    name={[field.name, 'password']}
                    rules={[{ required: true }]}
                  >
                    <Input.Password />
                  </Form.Item>
                </Col>
                <Col xs={24} md={2}>
                  <Button danger type="text" icon={<DeleteOutlined />} onClick={() => remove(field.name)} />
                </Col>
              </Row>
            </Card>
          ))}
          <Button
            type="dashed"
            block
            onClick={() =>
              add(
                kind === 'tuic'
                  ? { name: `user${fields.length + 1}`, uuid: crypto.randomUUID(), password: randomPassword() }
                  : kind === 'naive'
                    ? { username: `user${fields.length + 1}`, password: randomPassword() }
                    : { name: `user${fields.length + 1}`, password: randomPassword() },
              )
            }
          >
            Add user
          </Button>
        </Space>
      )}
    </Form.List>
  );
}

function ProtocolFields({ protocol, form }: { protocol: Protocol; form: FormInstance<FormValues> }) {
  const method = Form.useWatch(['settings', 'innerMethod'], form) as string | undefined;

  if (protocol === 'tuic') {
    return (
      <>
        <Divider orientation="left">TUIC users</Divider>
        <PasswordUsers kind="tuic" />
        <Row gutter={12} style={{ marginTop: 16 }}>
          <Col xs={24} md={8}>
            <Form.Item label="Congestion control" name={['settings', 'congestionControl']}>
              <Select
                options={[
                  { value: 'bbr', label: 'BBR' },
                  { value: 'cubic', label: 'Cubic' },
                  { value: 'new_reno', label: 'New Reno' },
                ]}
              />
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item label="Auth timeout" name={['settings', 'authTimeout']}>
              <Input placeholder="Optional, e.g. 3s" />
            </Form.Item>
          </Col>
          <Col xs={24} md={8}>
            <Form.Item label="Heartbeat" name={['settings', 'heartbeat']}>
              <Input placeholder="10s" />
            </Form.Item>
          </Col>
        </Row>
        <Form.Item label="0-RTT handshake" name={['settings', 'zeroRTTHandshake']} valuePropName="checked">
          <Switch />
        </Form.Item>
        <TLSFields />
      </>
    );
  }

  if (protocol === 'anytls') {
    return (
      <>
        <Divider orientation="left">AnyTLS users</Divider>
        <PasswordUsers kind="password" />
        <Form.Item
          label="Padding scheme"
          name={['settings', 'paddingScheme']}
          tooltip="Optional. Each tag is passed as one sing-box padding scheme entry."
          style={{ marginTop: 16 }}
        >
          <Select mode="tags" placeholder="Leave empty for sing-box default; press Enter after each full line" />
        </Form.Item>
        <TLSFields />
      </>
    );
  }

  if (protocol === 'naive') {
    return (
      <>
        <Divider orientation="left">Naive users</Divider>
        <PasswordUsers kind="naive" />
        <Row gutter={12} style={{ marginTop: 16 }}>
          <Col xs={24} md={12}>
            <Form.Item label="Network" name={['settings', 'network']}>
              <Select
                options={[
                  { value: '', label: 'TCP + UDP (default)' },
                  { value: 'tcp', label: 'TCP' },
                  { value: 'udp', label: 'UDP / QUIC' },
                ]}
              />
            </Form.Item>
          </Col>
          <Col xs={24} md={12}>
            <Form.Item label="QUIC congestion control" name={['settings', 'quicCongestionControl']}>
              <Select
                options={[
                  { value: 'bbr', label: 'BBR' },
                  { value: 'cubic', label: 'Cubic' },
                  { value: 'reno', label: 'Reno' },
                ]}
              />
            </Form.Item>
          </Col>
        </Row>
        <TLSFields />
      </>
    );
  }

  const innerBytes = method === '2022-blake3-aes-128-gcm' ? 16 : 32;
  return (
    <>
      <Alert
        type="info"
        showIcon
        message="ShadowTLS v3 carrier"
        description="The patch automatically creates a hidden injectable Shadowsocks 2022 inbound. It is an implementation detail and is not exposed as a selectable protocol."
        style={{ marginBottom: 16 }}
      />
      <Divider orientation="left">ShadowTLS users</Divider>
      <PasswordUsers kind="password" />
      <Row gutter={12} style={{ marginTop: 16 }}>
        <Col xs={24} md={16}>
          <Form.Item
            label="Handshake server"
            name={['settings', 'handshakeServer']}
            rules={[{ required: true }]}
          >
            <Input placeholder="www.microsoft.com" />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item label="Handshake port" name={['settings', 'handshakePort']} rules={[{ required: true }]}>
            <InputNumber min={1} max={65535} style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={12}>
        <Col xs={24} md={12}>
          <Form.Item label="Wildcard SNI" name={['settings', 'wildcardSNI']}>
            <Select
              options={[
                { value: 'off', label: 'Off' },
                { value: 'authed', label: 'Authenticated' },
                { value: 'all', label: 'All' },
              ]}
            />
          </Form.Item>
        </Col>
        <Col xs={24} md={12}>
          <Form.Item label="Strict mode" name={['settings', 'strictMode']} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
      </Row>
      <Divider orientation="left">Hidden inner Shadowsocks</Divider>
      <Row gutter={12}>
        <Col xs={24} md={12}>
          <Form.Item label="Method" name={['settings', 'innerMethod']} rules={[{ required: true }]}>
            <Select
              options={[
                { value: '2022-blake3-aes-128-gcm', label: '2022-blake3-aes-128-gcm' },
                { value: '2022-blake3-aes-256-gcm', label: '2022-blake3-aes-256-gcm' },
                { value: '2022-blake3-chacha20-poly1305', label: '2022-blake3-chacha20-poly1305' },
              ]}
              onChange={(value) => {
                const bytes = value === '2022-blake3-aes-128-gcm' ? 16 : 32;
                form.setFieldValue(['settings', 'innerPassword'], randomBase64(bytes));
              }}
            />
          </Form.Item>
        </Col>
        <Col xs={24} md={12}>
          <Form.Item label="Inner key" required>
            <Space.Compact block>
              <Form.Item name={['settings', 'innerPassword']} noStyle rules={[{ required: true }]}>
                <Input.Password />
              </Form.Item>
              <Button onClick={() => form.setFieldValue(['settings', 'innerPassword'], randomBase64(innerBytes))}>
                Generate
              </Button>
            </Space.Compact>
          </Form.Item>
        </Col>
      </Row>
    </>
  );
}

export default function SingboxInboundsPage() {
  const [rows, setRows] = useState<SingboxInbound[]>([]);
  const [status, setStatus] = useState<StatusPayload>({ status: 'unknown', version: '' });
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<SingboxInbound | null>(null);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm<FormValues>();
  const protocol = (Form.useWatch('protocol', form) || 'tuic') as Protocol;
  const [messageApi, contextHolder] = message.useMessage();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      // Keep database management available even when the supplemental binary or
      // systemd service is not installed yet. A status failure must not hide the
      // configured rows or prevent the user from fixing them.
      const [listResult, statusResult] = await Promise.allSettled([
        HttpUtil.get('/panel/api/singbox/list', undefined, { silent: true }),
        HttpUtil.get('/panel/api/singbox/status', undefined, { silent: true }),
      ]);

      if (listResult.status === 'rejected') throw listResult.reason;
      const listMsg = listResult.value;
      if (!listMsg?.success) throw new Error(listMsg?.msg || 'Failed to load sing-box inbounds');
      setRows(Array.isArray(listMsg.obj) ? (listMsg.obj as SingboxInbound[]) : []);

      if (statusResult.status === 'fulfilled' && statusResult.value?.success) {
        setStatus((statusResult.value.obj || { status: 'unknown', version: '' }) as StatusPayload);
      } else {
        setStatus({ status: 'unavailable', version: '' });
      }
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : String(error));
    } finally {
      setLoading(false);
    }
  }, [messageApi]);

  useEffect(() => {
    void load();
  }, [load]);

  const openAdd = () => {
    setEditing(null);
    form.setFieldsValue({
      remark: '',
      enable: true,
      listen: '::',
      port: 443,
      protocol: 'tuic',
      tag: '',
      settings: defaultSettings('tuic'),
    });
    setModalOpen(true);
  };

  const openEdit = (row: SingboxInbound) => {
    setEditing(row);
    form.setFieldsValue({
      remark: row.remark,
      enable: row.enable,
      listen: row.listen,
      port: row.port,
      protocol: row.protocol,
      tag: row.tag,
      settings: safeParseSettings(row),
    });
    setModalOpen(true);
  };

  const save = async () => {
    setSaving(true);
    try {
      const values = await form.validateFields();
      const payload = { ...values, tag: values.tag?.trim() || '', settings: values.settings };
      const url = editing ? `/panel/api/singbox/update/${editing.id}` : '/panel/api/singbox/add';
      const msg = await HttpUtil.post(url, payload, JSON_HEADERS);
      if (!msg?.success) throw new Error(msg?.msg || 'Failed to save sing-box inbound');
      messageApi.success(editing ? 'Sing-box inbound updated' : 'Sing-box inbound created');
      setModalOpen(false);
      await load();
    } catch (error) {
      if (error && typeof error === 'object' && 'errorFields' in error) return;
      messageApi.error(error instanceof Error ? error.message : String(error));
    } finally {
      setSaving(false);
    }
  };

  const setEnable = async (row: SingboxInbound, enable: boolean) => {
    const msg = await HttpUtil.post(`/panel/api/singbox/setEnable/${row.id}`, { enable }, JSON_HEADERS);
    if (!msg?.success) {
      messageApi.error(msg?.msg || 'Failed to update inbound state');
      return;
    }
    await load();
  };

  const remove = async (row: SingboxInbound) => {
    const msg = await HttpUtil.post(`/panel/api/singbox/del/${row.id}`);
    if (!msg?.success) {
      messageApi.error(msg?.msg || 'Failed to delete inbound');
      return;
    }
    messageApi.success('Sing-box inbound deleted');
    await load();
  };

  const check = async () => {
    const msg = await HttpUtil.post('/panel/api/singbox/check');
    if (msg?.success) messageApi.success('Current database config passes sing-box check');
    else messageApi.error(msg?.msg || 'sing-box config check failed');
  };

  const restart = async () => {
    const msg = await HttpUtil.post('/panel/api/singbox/restart');
    if (!msg?.success) {
      messageApi.error(msg?.msg || 'Failed to restart sing-box');
      return;
    }
    messageApi.success('sing-box restarted');
    await load();
  };

  const columns = useMemo<TableProps<SingboxInbound>['columns']>(
    () => [
      { title: 'ID', dataIndex: 'id', width: 70 },
      {
        title: 'Remark',
        dataIndex: 'remark',
        render: (value: string) => value || <Typography.Text type="secondary">—</Typography.Text>,
      },
      {
        title: 'Protocol',
        dataIndex: 'protocol',
        render: (value: Protocol) => <Tag>{protocolLabel(value)}</Tag>,
      },
      {
        title: 'Listen',
        render: (_, row) => `${row.listen}:${row.port}`,
      },
      { title: 'Tag', dataIndex: 'tag', responsive: ['lg'] },
      {
        title: 'Enabled',
        dataIndex: 'enable',
        width: 100,
        render: (value: boolean, row) => (
          <Switch checked={value} onChange={(next) => void setEnable(row, next)} />
        ),
      },
      {
        title: 'Actions',
        width: 130,
        render: (_, row) => (
          <Space>
            <Button type="text" icon={<EditOutlined />} onClick={() => openEdit(row)} />
            <Popconfirm title="Delete this sing-box inbound?" onConfirm={() => void remove(row)}>
              <Button danger type="text" icon={<DeleteOutlined />} />
            </Popconfirm>
          </Space>
        ),
      },
    ],
    [],
  );

  const active = status.status === 'active';

  return (
    <div style={{ padding: 24 }}>
      {contextHolder}
      <Flex justify="space-between" align="center" wrap gap={12} style={{ marginBottom: 16 }}>
        <div>
          <Typography.Title level={2} style={{ margin: 0 }}>
            Sing-box Supplemental Core
          </Typography.Title>
          <Typography.Text type="secondary">
            Only protocols intentionally not managed by the Xray side of this patch.
          </Typography.Text>
        </div>
        <Space wrap>
          <Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>
            Refresh
          </Button>
          <Button icon={<CheckCircleOutlined />} onClick={() => void check()}>
            Check config
          </Button>
          <Button icon={<SyncOutlined />} onClick={() => void restart()}>
            Restart core
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={openAdd}>
            Add inbound
          </Button>
        </Space>
      </Flex>

      <Card size="small" style={{ marginBottom: 16 }}>
        <Space wrap size="large">
          <span>
            Core status: <Tag color={active ? 'success' : 'error'}>{status.status || 'unknown'}</Tag>
          </span>
          <span>
            Version: <Typography.Text code>{status.version || 'unknown'}</Typography.Text>
          </span>
          <span>
            V1: <Tag>TUIC</Tag><Tag>AnyTLS</Tag><Tag>ShadowTLS v3</Tag><Tag>Naive</Tag>
          </span>
          <span>
            <Tag color="default">Snell deferred</Tag>
          </span>
        </Space>
      </Card>

      <Alert
        type="warning"
        showIcon
        message="Independent from Xray"
        description="Saving, enabling, deleting, checking or restarting entries on this page only rebuilds/restarts x-ui-singbox.service. It does not replace or restart Xray."
        style={{ marginBottom: 16 }}
      />

      <Table<SingboxInbound>
        rowKey="id"
        columns={columns}
        dataSource={rows}
        loading={loading}
        pagination={false}
        scroll={{ x: 760 }}
      />

      <Modal
        title={editing ? `Edit sing-box inbound #${editing.id}` : 'Add sing-box inbound'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => void save()}
        okText={editing ? 'Save' : 'Create'}
        confirmLoading={saving}
        width={900}
        destroyOnHidden
      >
        <Form<FormValues> form={form} layout="vertical" preserve={false}>
          <Row gutter={12}>
            <Col xs={24} md={10}>
              <Form.Item label="Remark" name="remark">
                <Input placeholder="Tokyo TUIC" />
              </Form.Item>
            </Col>
            <Col xs={24} md={6}>
              <Form.Item label="Protocol" name="protocol" rules={[{ required: true }]}>
                <Select
                  options={protocolOptions}
                  onChange={(next: Protocol) => {
                    form.setFieldValue('settings', defaultSettings(next));
                  }}
                />
              </Form.Item>
            </Col>
            <Col xs={24} md={4}>
              <Form.Item label="Port" name="port" rules={[{ required: true }]}>
                <InputNumber min={1} max={65535} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col xs={24} md={4}>
              <Form.Item label="Enabled" name="enable" valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={12}>
            <Col xs={24} md={12}>
              <Form.Item label="Listen address" name="listen" rules={[{ required: true }]}>
                <Input placeholder="::" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item
                label="Tag"
                name="tag"
                tooltip={editing ? 'Leave unchanged unless you know why you need a custom tag.' : 'Leave empty to auto-generate a stable tag.'}
              >
                <Input placeholder="Auto-generated when empty" />
              </Form.Item>
            </Col>
          </Row>

          <ProtocolFields protocol={protocol} form={form} />
        </Form>
      </Modal>
    </div>
  );
}
