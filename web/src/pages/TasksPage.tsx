import { useCallback, useEffect, useState } from 'react';
import { App, Button, Card, Form, Input, Select, Space, Table, Tag, Typography } from 'antd';
import { useLoadable } from '@/hooks/useLoadable';
import { PlayCircleOutlined, ReloadOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { labopsApi } from '@/api/labops';
import type { DeviceGroup, Task } from '@/types';
import { statusColor, statusText } from '@/utils/status';

export default function TasksPage() {
  const { message } = App.useApp();
  const [form] = Form.useForm<{ groupName: string; command: string }>();
  const [submitting, setSubmitting] = useState(false);

  const fetcher = useCallback(async () => {
    const [nextGroups, nextTasks] = await Promise.all([labopsApi.groups(), labopsApi.tasks()]);
    return { groups: nextGroups, tasks: nextTasks };
  }, []);

  const { data, loading, reload } = useLoadable(fetcher, { intervalMs: 3000 });
  const groups = data?.groups ?? [];
  const tasks = data?.tasks ?? [];

  useEffect(() => {
    if (groups.length > 0 && !form.getFieldValue('groupName')) {
      form.setFieldsValue({ groupName: groups[0].name });
    }
  }, [groups, form]);

  async function submit(values: { groupName: string; command: string }) {
    setSubmitting(true);
    try {
      const result = await labopsApi.createTask({ groupName: values.groupName, command: values.command });
      message.success(`已创建 ${result.tasks.length} 个任务`);
      form.setFieldsValue({ command: values.command });
      await reload();
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
          initialValues={{ command: 'hostname && date', groupName: undefined }}
        >
          <Space align="end" wrap>
            <Form.Item name="groupName" label="目标分组" rules={[{ required: true }]}>
              <Select style={{ width: 240 }} placeholder="选择分组" options={groups.map((g) => ({ value: g.name, label: `${g.name} (${g.online}/${g.total})` }))} />
            </Form.Item>
            <Form.Item name="command" label="命令" rules={[{ required: true }]}>
              <Input style={{ width: 520 }} placeholder="hostname && date" />
            </Form.Item>
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
