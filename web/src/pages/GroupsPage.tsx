import { useEffect, useState } from 'react';
import { Button, Card, Progress, Table, Typography } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { labopsApi } from '@/api/labops';
import type { DeviceGroup } from '@/types';

export default function GroupsPage() {
  const [groups, setGroups] = useState<DeviceGroup[]>([]);
  const [loading, setLoading] = useState(false);

  async function load() {
    setLoading(true);
    try {
      setGroups(await labopsApi.groups());
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  return (
    <div className="page">
      <div className="page-head">
        <div>
          <Typography.Title level={2}>分组</Typography.Title>
          <Typography.Text className="muted">设备组来自 Agent 注册信息，可用于批量命令。</Typography.Text>
        </div>
        <Button icon={<ReloadOutlined />} onClick={load}>
          刷新
        </Button>
      </div>
      <Card>
        <Table
          rowKey="name"
          loading={loading}
          dataSource={groups}
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
