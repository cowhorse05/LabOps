import { App, Button, Card, Table, Tag, Typography } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { labopsApi } from '@/api/labops';
import { useLoadable } from '@/hooks/useLoadable';
import { statusColor, statusText } from '@/utils/status';

export default function AuditPage() {
  const { message } = App.useApp();
  const { data: logs, loading, reload } = useLoadable(() => labopsApi.auditLogs(), { intervalMs: 15000, onError: () => message.error('加载审计日志失败') });

  return (
    <div className="page">
      <div className="page-head">
        <div>
          <Typography.Title level={2}>审计</Typography.Title>
          <Typography.Text className="muted">记录 Agent 连接、命令派发和任务完成结果。</Typography.Text>
        </div>
        <Button icon={<ReloadOutlined />} onClick={reload}>
          刷新
        </Button>
      </div>
      <Card>
        <Table
          scroll={{ x: 'max-content' }}
          rowKey="id"
          loading={loading}
          dataSource={logs ?? []}
          columns={[
            { title: '时间', dataIndex: 'createdAt', render: (value) => dayjs(value).format('MM-DD HH:mm:ss') },
            { title: '操作者', dataIndex: 'actor' },
            { title: '动作', dataIndex: 'action' },
            { title: '设备', render: (_, record) => record.device || record.deviceId || '-' },
            {
              title: '状态',
              dataIndex: 'status',
              render: (status) => <Tag color={statusColor(status)}>{statusText(status)}</Tag>,
            },
            { title: '说明', dataIndex: 'message', ellipsis: true },
          ]}
        />
      </Card>
    </div>
  );
}
