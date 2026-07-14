import { useMemo, useState } from 'react';
import { Alert, App, Button, Card, Form, Input, InputNumber, Modal, Popconfirm, Progress, Segmented, Select, Space, Table, Tabs, Tag, Typography } from 'antd';
import { CopyOutlined, DeleteOutlined, EyeOutlined, PlusOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import dayjs from 'dayjs';
import { labopsApi } from '@/api/labops';
import { useLoadable } from '@/hooks/useLoadable';
import { statusColor, statusText } from '@/utils/status';
import { useAuthStore } from '@/stores/auth';
import { logger } from '@/utils/logger';

export default function DevicesPage() {
  const navigate = useNavigate();
  const [keyword, setKeyword] = useState('');
  const { message } = App.useApp();
  const { data: devices, loading, reload } = useLoadable(() => labopsApi.devices(), { intervalMs: 10000, onError: () => message.error('加载设备列表失败') });
  const { data: groups } = useLoadable(() => labopsApi.groups(), { intervalMs: 60000 });

  const [addDeviceOpen, setAddDeviceOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<string>('agent');
  const [osType, setOsType] = useState<'linux' | 'windows'>('linux');
  const [createForm] = Form.useForm();
  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [enrollmentCreating, setEnrollmentCreating] = useState(false);
  const [createdCode, setCreatedCode] = useState('');
  const [serverUrl, setServerUrl] = useState(() => window.location.origin);
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
      setAddDeviceOpen(false);
      setActiveTab('agent');
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

  const handleCreateEnrollmentCode = async () => {
    try {
      setEnrollmentCreating(true);
      const item = await labopsApi.createEnrollmentCode({ expiresInSeconds: 600, maxUses: 1 });
      setCreatedCode(item.code || '');
      message.success('一次性注册码已生成');
      logger.info('DevicesPage', '生成注册码成功', { osType, serverUrl });
    } catch (err: any) {
      const msg = err?.response?.data?.error || err?.message;
      message.error(`生成注册码失败: ${msg}`);
      logger.error('DevicesPage', '生成注册码失败', err);
    } finally {
      setEnrollmentCreating(false);
    }
  };

  const closeAddDevice = () => {
    setAddDeviceOpen(false);
    setCreatedCode('');
    setActiveTab('agent');
    setOsType('linux');
    createForm.resetFields();
  };

  const linuxCommand = createdCode
    ? `sudo labops-agent --server=${serverUrl || '<LABOPS_SERVER_URL>'} --enroll-code=${createdCode} --name "$(hostname)" --group lab --real`
    : '';

  const windowsCommand = createdCode
    ? `.\\labops-agent.exe --server=${serverUrl || '<LABOPS_SERVER_URL>'} --enroll-code=${createdCode} --name $env:COMPUTERNAME --group lab --real`
    : '';

  const enrollCommand = osType === 'windows' ? windowsCommand : linuxCommand;

  const copyText = async (value: string, success: string) => {
    await navigator.clipboard.writeText(value);
    message.success(success);
  };

  return (
    <div className="page">
      <div className="page-head">
        <div>
          <Typography.Title level={2}>设备</Typography.Title>
          <Typography.Text className="muted">Agent 连接后会自动出现在这里，也可以手动创建。</Typography.Text>
        </div>
        <Space wrap>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setAddDeviceOpen(true)}>
            添加设备
          </Button>
          <Input
            allowClear
            prefix={<SearchOutlined />}
            placeholder="搜索设备、分组、IP"
            aria-label="搜索设备"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            style={{ width: '100%', maxWidth: 280 }}
          />
          <Button icon={<ReloadOutlined />} onClick={reload}>
            刷新
          </Button>
        </Space>
      </div>
      <Card>
        <Table
          scroll={{ x: 'max-content' }}
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
        title="添加设备"
        open={addDeviceOpen}
        onCancel={closeAddDevice}
        footer={activeTab === 'manual' ? [
          <Button key="cancel" onClick={closeAddDevice}>取消</Button>,
          <Button key="submit" type="primary" loading={creating} onClick={handleCreate}>确认创建</Button>,
        ] : [
          <Button key="close" onClick={closeAddDevice}>关闭</Button>,
          <Button key="generate" type="primary" loading={enrollmentCreating} onClick={handleCreateEnrollmentCode}>生成一次性注册码</Button>,
        ]}
        width={720}
        destroyOnHidden
      >
        <Tabs
          activeKey={activeTab}
          onChange={(key) => { setActiveTab(key); setCreatedCode(''); }}
          items={[
            {
              key: 'agent',
              label: 'Agent 接入',
              children: (
                <>
                  <Typography.Paragraph className="muted">
                    在目标主机上安装 LabOps Agent 后，使用注册码一键接入。Agent 会自动上报主机名、系统、IP、CPU、内存、磁盘等信息，无需手动填写。
                  </Typography.Paragraph>
                  <Alert
                    type="info"
                    showIcon
                    style={{ marginBottom: 16 }}
                    message="前置条件：安装 LabOps Agent"
                    description={
                      osType === 'linux' ? (
                        <Typography.Text>
                          在目标 Linux 主机上，先构建 Agent 二进制：<br />
                          <code>cd agent &amp;&amp; go build -o labops-agent ./cmd/agent/</code><br />
                          也可使用一键安装脚本：<code>sudo bash scripts/install-agent.sh --server={serverUrl} --enroll-code=&lt;注册码&gt;</code>
                        </Typography.Text>
                      ) : (
                        <Typography.Text>
                          在目标 Windows 主机上（需安装 Go），构建 Agent：<br />
                          <code>cd agent; go build -o labops-agent.exe ./cmd/agent/</code><br />
                          将 <code>labops-agent.exe</code> 放到目标机器上，然后用下方生成的命令运行。
                        </Typography.Text>
                      )
                    }
                  />
                  <Form layout="vertical">
                    <Form.Item label="目标系统">
                      <Segmented
                        options={[
                          { label: 'Linux', value: 'linux' },
                          { label: 'Windows', value: 'windows' },
                        ]}
                        value={osType}
                        onChange={(value) => setOsType(value as 'linux' | 'windows')}
                      />
                    </Form.Item>
                    <Form.Item label="Server URL" extra="确保目标主机能访问此地址">
                      <Input
                        value={serverUrl}
                        onChange={(event) => setServerUrl(event.target.value)}
                        placeholder="例如: https://cowhorse.xyz 或 http://你的本机IP:8080"
                      />
                    </Form.Item>
                  </Form>
                  {createdCode ? (
                    <Space direction="vertical" size={14} style={{ width: '100%' }}>
                      <div>
                        <Typography.Text strong>一次性注册码</Typography.Text>
                        <Typography.Text type="secondary" style={{ marginLeft: 8 }}>仅显示一次，10 分钟内有效</Typography.Text>
                        <Space.Compact block style={{ marginTop: 8 }}>
                          <Input.Password value={createdCode} readOnly visibilityToggle />
                          <Button icon={<CopyOutlined />} onClick={() => copyText(createdCode, '注册码已复制')}>复制</Button>
                        </Space.Compact>
                      </div>
                      <div>
                        <Typography.Text strong>{osType === 'windows' ? 'Windows PowerShell' : 'Ubuntu/Linux'} 接入命令</Typography.Text>
                        <Typography.Paragraph className="task-output" copyable={{ text: enrollCommand }} style={{ marginTop: 8 }}>
                          <code>{enrollCommand}</code>
                        </Typography.Paragraph>
                      </div>
                    </Space>
                  ) : (
                    <Card size="small">
                      <Typography.Text>点击"生成一次性注册码"后，将生成的命令复制到目标主机执行即可完成接入。</Typography.Text>
                    </Card>
                  )}
                </>
              ),
            },
            {
              key: 'manual',
              label: '手动创建',
              children: (
                <>
                  <Typography.Paragraph className="muted" style={{ marginBottom: 16 }}>
                    适用于无法运行 Agent 的设备（网络设备、打印机等），手动录入设备信息进行跟踪管理。
                  </Typography.Paragraph>
                  <Form form={createForm} layout="vertical">
                    <Form.Item name="name" label="设备名称" rules={[{ required: true, message: '请输入设备名称' }]}>
                      <Input placeholder="例如: 核心交换机" />
                    </Form.Item>
                    <Form.Item name="groupName" label="分组" rules={[{ required: true, message: '请输入或选择分组' }]}>
                      <Select
                        mode="tags"
                        maxCount={1}
                        placeholder="输入分组名，例如: network、lab"
                        options={(groups ?? []).map((g) => ({ label: g.name, value: g.name }))}
                      />
                    </Form.Item>
                    <Form.Item name="hostname" label="主机名 / IP">
                      <Input placeholder="例如: 192.168.1.1" />
                    </Form.Item>
                    <Form.Item name="os" label="操作系统 / 固件">
                      <Input placeholder="例如: Cisco IOS 17.3" />
                    </Form.Item>
                    <Form.Item name="ip" label="管理 IP">
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
                </>
              ),
            },
          ]}
        />
      </Modal>
    </div>
  );
}
