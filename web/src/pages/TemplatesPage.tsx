import { useCallback, useState } from 'react';
import { App, Button, Card, Form, Input, InputNumber, Modal, Switch, Table, Tag, Typography } from 'antd';
import { labopsApi } from '@/api/labops';
import { useLoadable } from '@/hooks/useLoadable';

export default function TemplatesPage() {
  const { message } = App.useApp();
  const fetcher = useCallback(() => labopsApi.commandTemplates(), []);
  const { data = [], loading, reload } = useLoadable(fetcher, { onError: () => message.error('加载模板失败') });
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm();
  const create = async () => {
    const values = await form.validateFields();
    await labopsApi.createCommandTemplate({ ...values, os: 'linux', args: values.args ? values.args.split('\n').filter(Boolean) : [], parameters: [], requiresPrivilege: false, enabled: true });
    setOpen(false); form.resetFields(); await reload();
  };
  return <div className="page">
    <div className="page-head"><div><Typography.Title level={2}>命令模板</Typography.Title><Typography.Text className="muted">模板直接执行绝对路径与参数数组，不经过 Shell。</Typography.Text></div><Button type="primary" onClick={() => setOpen(true)}>新建模板</Button></div>
    <Card><Table rowKey="id" loading={loading} dataSource={data ?? []} columns={[
      { title: '名称', dataIndex: 'name' }, { title: '可执行文件', dataIndex: 'executable' },
      { title: '参数', dataIndex: 'args', render: (v: string[]) => v.join(' ') || '-' },
      { title: '超时', dataIndex: 'timeoutSeconds', render: v => `${v}s` },
      { title: '权限', render: (_, row) => row.requiresPrivilege ? <Tag color="red">特权（不可执行）</Tag> : <Tag color="green">低权限</Tag> },
      { title: '启用', render: (_, row) => <Switch checked={row.enabled} onChange={async enabled => { await labopsApi.updateCommandTemplate(row.id, { ...row, enabled }); await reload(); }} /> },
    ]} /></Card>
    <Modal title="新建低权限模板" open={open} onCancel={() => setOpen(false)} onOk={create}><Form form={form} layout="vertical" initialValues={{ timeoutSeconds: 30 }}>
      <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input /></Form.Item>
      <Form.Item name="description" label="说明"><Input /></Form.Item>
      <Form.Item name="executable" label="绝对可执行文件" rules={[{ required: true, pattern: /^\//, message: '必须是 Linux 绝对路径' }]}><Input placeholder="/usr/bin/uptime" /></Form.Item>
      <Form.Item name="args" label="参数（每行一个）"><Input.TextArea rows={4} /></Form.Item>
      <Form.Item name="timeoutSeconds" label="超时秒数" rules={[{ required: true }]}><InputNumber min={1} max={300} /></Form.Item>
    </Form></Modal>
  </div>;
}
