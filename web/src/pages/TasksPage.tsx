import { useCallback, useEffect, useState } from 'react';
import { App, Button, Card, Form, Input, Modal, Select, Space, Table, Tag, Typography } from 'antd';
import { useLoadable } from '@/hooks/useLoadable';
import { PlayCircleOutlined, ReloadOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { labopsApi } from '@/api/labops';
import type { DeviceGroup, Task } from '@/types';
import { statusColor, statusText } from '@/utils/status';
import { useAuthStore } from '@/stores/auth';

export default function TasksPage() {
  const { message } = App.useApp();
  const [form] = Form.useForm<{ groupName: string; kind: 'template' | 'ad_hoc'; templateId?: string; command?: string }>();
  const kind = Form.useWatch('kind', form);
  const user = useAuthStore((s) => s.user);
  const [submitting, setSubmitting] = useState(false);

  const fetcher = useCallback(async () => {
    const [nextGroups, nextTasks, templates] = await Promise.all([labopsApi.groups(), labopsApi.tasks(), labopsApi.commandTemplates()]);
    return { groups: nextGroups, tasks: nextTasks, templates };
  }, []);

  const { data, loading, reload } = useLoadable(fetcher, {
    intervalMs: 3000,
    onError: () => message.error('加载任务数据失败'),
  });
  const groups = data?.groups ?? [];
  const tasks = data?.tasks ?? [];
  const templates = (data?.templates ?? []).filter((item) => item.enabled && !item.requiresPrivilege);

  useEffect(() => {
    if (groups.length > 0 && !form.getFieldValue('groupName')) {
      form.setFieldsValue({ groupName: groups[0].name });
    }
  }, [groups, form]);

  async function submit(values: { groupName: string; kind: 'template' | 'ad_hoc'; templateId?: string; command?: string }) {
    setSubmitting(true);
    try {
      if (values.kind === 'ad_hoc') {
        await new Promise<void>((resolve, reject) => Modal.confirm({ title: '确认执行临时命令', content: '临时命令将通过 Shell 执行并完整审计。', okText: '确认执行', okType: 'danger', onOk: () => resolve(), onCancel: () => reject(new Error('cancelled')) }));
      }
      const result = await labopsApi.createTask({ groupName: values.groupName, kind: values.kind, templateId: values.templateId, command: values.command, confirmation: values.kind === 'ad_hoc' ? 'EXECUTE' : undefined, arguments: {} });
      message.success(`已创建 ${result.tasks.length} 个任务`);
      await reload();
    } catch (error) {
      if ((error as Error).message !== 'cancelled') message.error('创建任务失败');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="page">
      <div className="page-head">
        <div>
          <Typography.Title level={2}>任务</Typography.Title>
          <Typography.Text className="muted">创建批量命令任务，并查看每台设备的独立结果。</Typography.Text>
        </div>
        <Button icon={<ReloadOutlined />} onClick={reload}>
          刷新
        </Button>
      </div>

      <Card title="批量命令">
        <Form
          form={form}
          layout="vertical"
          onFinish={submit}
          initialValues={{ kind: 'template', groupName: undefined }}
        >
          <Space align="end" wrap>
            <Form.Item name="groupName" label="目标分组" rules={[{ required: true }]}>
              <Select style={{ width: '100%', maxWidth: 240, minWidth: 160 }} placeholder="选择分组" options={groups.map((g) => ({ value: g.name, label: `${g.name} (${g.online}/${g.total})` }))} />
            </Form.Item>
            <Form.Item name="kind" label="执行方式" rules={[{ required: true }]}><Select style={{ width: '100%', maxWidth: 160, minWidth: 120 }} options={[{ value: 'template', label: '安全模板' }, ...(user?.permissions.includes('commands:adhoc') ? [{ value: 'ad_hoc', label: '临时命令' }] : [])]} /></Form.Item>
            {kind === 'ad_hoc' ? <Form.Item name="command" label="临时命令" rules={[{ required: true }]}><Input style={{ width: '100%', maxWidth: 420 }} placeholder="uname -a" /></Form.Item> : <Form.Item name="templateId" label="命令模板" rules={[{ required: true }]}><Select style={{ width: '100%', maxWidth: 320 }} options={templates.map(item => ({ value: item.id, label: item.name }))} /></Form.Item>}
            <Form.Item>
              <Button type="primary" htmlType="submit" icon={<PlayCircleOutlined />} loading={submitting}>
                下发
              </Button>
            </Form.Item>
          </Space>
        </Form>
      </Card>

      <Card title="任务记录" style={{ marginTop: 16 }}>
        <Table
          scroll={{ x: 'max-content' }}
          rowKey="id"
          loading={loading}
          dataSource={tasks}
          expandable={{
            expandedRowRender: (record) => (
              <div className="result-grid">
                <pre>{record.result?.stdout || '-'}</pre>
                <pre>{record.result?.stderr || '-'}</pre>
              </div>
            ),
          }}
          columns={[
            { title: '设备', dataIndex: 'deviceName' },
            { title: '分组', dataIndex: 'groupName' },
            { title: '命令', dataIndex: 'command', ellipsis: true },
            { title: '类型', dataIndex: 'kind', render: (value) => <Tag>{value === 'template' ? '模板' : '临时命令'}</Tag> },
            {
              title: '状态',
              dataIndex: 'status',
              render: (status) => <Tag color={statusColor(status)}>{statusText(status)}</Tag>,
            },
            {
              title: '退出码',
              render: (_, record) => record.result?.exitCode ?? '-',
            },
            {
              title: '耗时',
              render: (_, record) => (record.result ? `${record.result.durationMs}ms` : '-'),
            },
            {
              title: '创建时间',
              dataIndex: 'createdAt',
              render: (value) => dayjs(value).format('MM-DD HH:mm:ss'),
            },
          ]}
        />
      </Card>
    </div>
  );
}
