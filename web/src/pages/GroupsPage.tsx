import { App, Button, Card, Progress, Table, Typography } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { labopsApi } from '@/api/labops';
import { useLoadable } from '@/hooks/useLoadable';

export default function GroupsPage() {
  const { message } = App.useApp();
  const { data: groups, loading, reload } = useLoadable(() => labopsApi.groups(), { intervalMs: 10000, onError: () => message.error('加载分组数据失败') });

  return (
    <div className="page">
      <div className="page-head">
        <div>
          <Typography.Title level={2}>分组</Typography.Title>
          <Typography.Text className="muted">设备组来自 Agent 注册信息，可用于批量命令。</Typography.Text>
        </div>
        <Button icon={<ReloadOutlined />} onClick={reload}>
          刷新
        </Button>
      </div>
      <Card>
        <Table
          scroll={{ x: 'max-content' }}
          rowKey="name"
          loading={loading}
          dataSource={groups ?? []}
          columns={[
            { title: '分组', dataIndex: 'name' },
            { title: '设备数', dataIndex: 'total' },
            { title: '在线', dataIndex: 'online' },
            {
              title: '在线率',
              render: (_, record) => <Progress percent={record.total ? Math.round((record.online / record.total) * 100) : 0} />,
            },
            { title: '说明', dataIndex: 'description' },
          ]}
        />
      </Card>
    </div>
  );
}
