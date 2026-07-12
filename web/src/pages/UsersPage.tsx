import { useCallback, useState } from 'react';
import { App, Button, Card, Form, Input, Modal, Select, Space, Table, Tag, Typography } from 'antd';
import { labopsApi } from '@/api/labops';
import { useLoadable } from '@/hooks/useLoadable';

export default function UsersPage() {
  const { message } = App.useApp();
  const fetcher = useCallback(() => labopsApi.users(), []);
  const { data = [], loading, reload } = useLoadable(fetcher, { onError: () => message.error('加载用户失败') });
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm();
  const create = async () => {
    const values = await form.validateFields();
    await labopsApi.createUser(values);
    message.success('用户已创建，首次登录必须修改密码');
    setOpen(false); form.resetFields(); await reload();
  };
  return <div className="page">
    <div className="page-head"><div><Typography.Title level={2}>用户与角色</Typography.Title><Typography.Text className="muted">固定角色：admin、operator、viewer。</Typography.Text></div><Button type="primary" onClick={() => setOpen(true)}>新建用户</Button></div>
    <Card><Table scroll={{ x: 'max-content' }} rowKey="id" loading={loading} dataSource={data ?? []} columns={[
      { title: '用户名', dataIndex: 'username' }, { title: '显示名', dataIndex: 'displayName' },
      { title: '角色', dataIndex: 'role', render: (v) => <Tag>{v}</Tag> },
      { title: '状态', dataIndex: 'status', render: (v) => <Tag color={v === 'active' ? 'green' : 'red'}>{v}</Tag> },
      { title: '操作', render: (_, row) => <Space><Select value={row.role} style={{ width: 120 }} options={['admin','operator','viewer'].map(value => ({ value }))} onChange={async role => { await labopsApi.updateUser(row.id, { role, status: row.status }); await reload(); }} /><Button danger={row.status === 'active'} onClick={async () => { await labopsApi.updateUser(row.id, { role: row.role, status: row.status === 'active' ? 'disabled' : 'active' }); await reload(); }}>{row.status === 'active' ? '禁用' : '启用'}</Button></Space> },
    ]} /></Card>
    <Modal title="新建用户" open={open} onCancel={() => setOpen(false)} onOk={create}><Form form={form} layout="vertical">
      <Form.Item name="username" label="用户名" rules={[{ required: true }, { min: 3 }]}><Input /></Form.Item>
      <Form.Item name="displayName" label="显示名" rules={[{ required: true }]}><Input /></Form.Item>
      <Form.Item name="password" label="初始密码" rules={[{ required: true }, { min: 12 }]}><Input.Password /></Form.Item>
      <Form.Item name="role" label="角色" initialValue="viewer" rules={[{ required: true }]}><Select options={['admin','operator','viewer'].map(value => ({ value }))} /></Form.Item>
    </Form></Modal>
  </div>;
}
