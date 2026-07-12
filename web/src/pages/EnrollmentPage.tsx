import { useCallback, useState } from 'react';
import { App, Button, Card, Input, Modal, Popconfirm, Space, Table, Tag, Typography } from 'antd';
import dayjs from 'dayjs';
import { labopsApi } from '@/api/labops';
import { useLoadable } from '@/hooks/useLoadable';

export default function EnrollmentPage() {
  const { message } = App.useApp();
  const fetcher = useCallback(() => labopsApi.enrollmentCodes(), []);
  const { data = [], loading, reload } = useLoadable(fetcher, { onError: () => message.error('加载注册码失败') });
  const [createdCode, setCreatedCode] = useState('');
  const create = async () => {
    const item = await labopsApi.createEnrollmentCode({ expiresInSeconds: 600, maxUses: 1 });
    setCreatedCode(item.code || '');
    await reload();
  };
  return <div className="page">
    <div className="page-head"><div><Typography.Title level={2}>设备安全接入</Typography.Title><Typography.Text className="muted">一次性注册码只显示一次，默认 10 分钟内有效。</Typography.Text></div><Button type="primary" onClick={create}>生成注册码</Button></div>
    <Card><Table rowKey="id" loading={loading} dataSource={data ?? []} columns={[
      { title: 'ID', dataIndex: 'id' },
      { title: '使用次数', render: (_, row) => `${row.usedCount}/${row.maxUses}` },
      { title: '过期时间', dataIndex: 'expiresAt', render: (v) => dayjs(v).format('YYYY-MM-DD HH:mm:ss') },
      { title: '状态', render: (_, row) => <Tag color={row.revokedAt ? 'red' : dayjs(row.expiresAt).isBefore(dayjs()) ? 'default' : 'green'}>{row.revokedAt ? '已吊销' : dayjs(row.expiresAt).isBefore(dayjs()) ? '已过期' : '有效'}</Tag> },
      { title: '操作', render: (_, row) => <Popconfirm title="吊销该注册码？" onConfirm={async () => { await labopsApi.revokeEnrollmentCode(row.id); await reload(); }}><Button danger type="link" disabled={Boolean(row.revokedAt)}>吊销</Button></Popconfirm> },
    ]} /></Card>
    <Modal open={Boolean(createdCode)} title="一次性注册码" onCancel={() => setCreatedCode('')} footer={<Button type="primary" onClick={() => setCreatedCode('')}>我已保存</Button>}>
      <Typography.Paragraph type="warning">关闭后无法再次查看，请通过安全渠道复制到目标主机。</Typography.Paragraph>
      <Space.Compact block>
        <Input.Password value={createdCode} readOnly visibilityToggle />
        <Button onClick={() => { void navigator.clipboard.writeText(createdCode); message.success('已复制'); }}>复制</Button>
      </Space.Compact>
      <Typography.Paragraph copyable style={{ marginTop: 16 }}><code>sudo labops-agent --server=https://${'{SERVER_HOST}'} --enroll-code={createdCode} --real</code></Typography.Paragraph>
    </Modal>
  </div>;
}
