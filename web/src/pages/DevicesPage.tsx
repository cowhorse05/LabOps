import { useMemo, useState } from 'react';
import { App, Button, Card, Form, Input, InputNumber, Modal, Popconfirm, Progress, Select, Space, Table, Tag, Typography } from 'antd';
import { DeleteOutlined, EyeOutlined, PlusOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import dayjs from 'dayjs';
import { labopsApi } from '@/api/labops';
import { useLoadable } from '@/hooks/useLoadable';
import { statusColor, statusText } from '@/utils/status';
import { useAuthStore } from '@/stores/auth';

export default function DevicesPage() {
  const navigate = useNavigate();
  const [keyword, setKeyword] = useState('');
  const { message } = App.useApp();
  const { data: devices, loading, reload } = useLoadable(() => labopsApi.devices(), { intervalMs: 10000, onError: () => message.error('加载设备列表失败') });
  const { data: groups } = useLoadable(() => labopsApi.groups(), { intervalMs: 60000 });

  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [createForm] = Form.useForm();
  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState<string | null>(null);
  const canManage = useAuthStore((s) => s.user?.permissions.includes('devices:revoke') ?? false);

  const filtered = useMemo(() => {
    const k = keyword.trim().toLowerCase();
    if (!k) return devices ?? [];
    return (devices ?? []).filter((d) => [d.name, d.groupName, d.os, d.ip, d.hostname].some((v) => v.toLowerCase().includes(k)));
  }, [devices, keyword]);

  const handleCreate = async () => {
    try {
      const values = await createForm.validateFields();
      setCreating(true);
      await labopsApi.createDevice(values);
      message.success(`设备 ${values.name} 创建成功`);
      setCreateModalVisible(false);
      createForm.resetFields();
      reload();
    } catch (err: any) {
      if (err?.errorFields) return;
      message.error(`创建设备失败: ${err?.response?.data?.error || err?.message}`);
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      setDeleting(id);
      await labopsApi.revokeDevice(id);
      message.success('设备凭据已吊销');
      reload();
    } catch (err: any) {
      message.error(`删除失败: ${err?.response?.data?.error || err?.message}`);
    } finally {
      setDeleting(null);
    }
  };

  return (
    <div className="page">
      <div className="page-head">
        <div>
          <Typography.Title level={2}>设备</Typography.Title>
          <Typography.Text className="muted">Agent 连接后会自动出现在这里，也可以手动创建。</Typography.Text>
        </div>
        <Space>
          {canManage && <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateModalVisible(true)}>
            新建设备
          </Button>}
          <Input
            allowClear
            prefix={<SearchOutlined />}
            placeholder="搜索设备、分组、IP"
            aria-label="搜索设备"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            style={{ width: 280 }}
          />
          <Button icon={<ReloadOutlined />} onClick={reload}>
            刷新
          </Button>
        </Space>
      </div>
      <Card>
        <Table
          rowKey="id"
          loading={loading}
          dataSource={filtered}
          columns={[
            {
              title: '设备',
              dataIndex: 'name',
              render: (name, record) => (
                <div>
                  <strong>{name}</strong>
                  <div className="muted small">{record.hostname}</div>
                </div>
              ),
            },
            { title: '分组', dataIndex: 'groupName' },
            { title: '系统', dataIndex: 'os' },
            { title: 'IP', dataIndex: 'ip' },
            {
              title: '状态',
              dataIndex: 'status',
              render: (status) => <Tag color={statusColor(status)}>{statusText(status)}</Tag>,
            },
            { title: '凭据', dataIndex: 'credentialStatus', render: (value) => <Tag color={value === 'active' ? 'green' : value === 'revoked' ? 'red' : 'orange'}>{value || '待登记'}</Tag> },
            {
              title: 'CPU',
              dataIndex: 'cpuUsage',
              render: (value) => <Progress percent={Math.round(value)} size="small" />,
            },
            {
              title: '最后心跳',
              dataIndex: 'lastSeen',
              render: (value) => dayjs(value).format('HH:mm:ss'),
            },
            {
              title: '操作',
              render: (_, record) => (
                <Space>
                  <Button type="link" icon={<EyeOutlined />} onClick={() => navigate(`/devices/${record.id}`)}>
                    详情
                  </Button>
                  {canManage && <Popconfirm
                    title="确定删除该设备吗？"
                    description="吊销后 Agent 会立即断开，必须重新登记才能接入。"
                    onConfirm={() => handleDelete(record.id)}
                    okText="确定"
                    cancelText="取消"
                  >
                    <Button type="link" danger icon={<DeleteOutlined />} loading={deleting === record.id}>
                      吊销
                    </Button>
                  </Popconfirm>}
                </Space>
              ),
            },
          ]}
        />
      </Card>
      <Modal
        title="新建设备"
        open={createModalVisible}
        onOk={handleCreate}
        onCancel={() => { setCreateModalVisible(false); createForm.resetFields(); }}
        confirmLoading={creating}
        destroyOnHidden
      >
        <Form form={createForm} layout="vertical">
          <Form.Item name="name" label="设备名称" rules={[{ required: true, message: '请输入设备名称' }]}>
            <Input placeholder="例如: 我的笔记本" />
          </Form.Item>
          <Form.Item name="groupName" label="分组" rules={[{ required: true, message: '请输入或选择分组' }]}>
            <Select
              mode="tags"
              maxCount={1}
              placeholder="输入分组名，例如: lab、prod"
              options={(groups ?? []).map((g) => ({ label: g.name, value: g.name }))}
            />
          </Form.Item>
          <Form.Item name="hostname" label="主机名">
            <Input placeholder="自动使用设备名称" />
          </Form.Item>
          <Form.Item name="os" label="操作系统">
            <Input placeholder="例如: Ubuntu 24.04" />
          </Form.Item>
          <Form.Item name="ip" label="IP 地址">
            <Input placeholder="例如: 192.168.1.100" />
          </Form.Item>
          <Form.Item name="cpuCores" label="CPU 核心数">
            <InputNumber min={1} max={256} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="memoryMb" label="内存 (MB)">
            <InputNumber min={128} max={1048576} step={1024} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="diskTotalGb" label="磁盘 (GB)">
            <InputNumber min={1} max={65536} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
