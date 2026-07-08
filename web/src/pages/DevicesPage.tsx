import { useEffect, useMemo, useState } from 'react';
import { Button, Card, Input, Progress, Space, Table, Tag, Typography } from 'antd';
import { EyeOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import dayjs from 'dayjs';
import { labopsApi } from '@/api/labops';
import type { Device } from '@/types';
import { statusColor, statusText } from '@/utils/status';

export default function DevicesPage() {
  const navigate = useNavigate();
  const [devices, setDevices] = useState<Device[]>([]);
  const [loading, setLoading] = useState(false);
  const [keyword, setKeyword] = useState('');

  async function load() {
    setLoading(true);
    try {
      setDevices(await labopsApi.devices());
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
    const timer = window.setInterval(load, 10000);
    return () => window.clearInterval(timer);
  }, []);

  const filtered = useMemo(() => {
    const k = keyword.trim().toLowerCase();
    if (!k) return devices;
    return devices.filter((d) => [d.name, d.groupName, d.os, d.ip, d.hostname].some((v) => v.toLowerCase().includes(k)));
  }, [devices, keyword]);

  return (
    <div className="page">
      <div className="page-head">
        <div>
          <Typography.Title level={2}>设备</Typography.Title>
          <Typography.Text className="muted">Agent 连接后会自动出现在这里。</Typography.Text>
        </div>
        <Space>
          <Input
            allowClear
            prefix={<SearchOutlined />}
            placeholder="搜索设备、分组、IP"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            style={{ width: 280 }}
          />
          <Button icon={<ReloadOutlined />} onClick={load}>
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
                <Button type="link" icon={<EyeOutlined />} onClick={() => navigate(`/devices/${record.id}`)}>
                  详情
                </Button>
              ),
            },
          ]}
        />
      </Card>
    </div>
  );
}
