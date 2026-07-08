import { Button, Card, Col, Progress, Row, Skeleton, Space, Statistic, Table, Tag, Typography } from 'antd';
import { ArrowRightOutlined, DesktopOutlined, PlayCircleOutlined, ProfileOutlined, ReloadOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import dayjs from 'dayjs';
import { labopsApi } from '@/api/labops';
import { statusColor, statusText } from '@/utils/status';
import { useLoadableAll } from '@/hooks/useLoadable';

export default function DashboardPage() {
  const navigate = useNavigate();

  const { data, loading, reload } = useLoadableAll(
    [labopsApi.stats, labopsApi.devices, labopsApi.tasks, labopsApi.groups, labopsApi.auditLogs],
    { intervalMs: 10000 },
  );

  const stats = data?.[0] ?? { total: 0, online: 0, offline: 0 };
  const devices = data?.[1] ?? [];
  const tasks = data?.[2] ?? [];
  const groups = data?.[3] ?? [];
  const audits = data?.[4] ?? [];

  const onlineRate = stats.total ? Math.round((stats.online / stats.total) * 100) : 0;

  return (
    <div className="page">
      <div className="page-head">
        <div>
          <Typography.Title level={2}>仪表盘</Typography.Title>
          <Typography.Text className="muted">今天是 {dayjs().format('YYYY-MM-DD')}，当前展示 Docker 模拟设备的真实连接状态。</Typography.Text>
        </div>
        <Button size="large" icon={<ReloadOutlined />} onClick={reload}>
          刷新
        </Button>
      </div>

      <Row gutter={[16, 16]}>
        <Col xs={24} md={12} xl={6}>
          <Card>
            <Statistic title="设备总数" value={stats.total} prefix={<DesktopOutlined />} />
          </Card>
        </Col>
        <Col xs={24} md={12} xl={6}>
          <Card>
            <Statistic title="在线设备" value={stats.online} valueStyle={{ color: '#16a34a' }} />
          </Card>
        </Col>
        <Col xs={24} md={12} xl={6}>
          <Card>
            <Statistic title="离线设备" value={stats.offline} valueStyle={{ color: '#64748b' }} />
          </Card>
        </Col>
        <Col xs={24} md={12} xl={6}>
          <Card>
            <Statistic title="任务数量" value={tasks.length} prefix={<ProfileOutlined />} />
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} xl={15}>
          <Card
            title="设备概览"
            extra={
              <Button type="link" onClick={() => navigate('/devices')}>
                查看设备 <ArrowRightOutlined />
              </Button>
            }
          >
            {loading && devices.length === 0 ? (
              <Skeleton active />
            ) : (
              <Table
                rowKey="id"
                size="middle"
                pagination={false}
                dataSource={devices.slice(0, 6)}
                columns={[
                  { title: '设备', dataIndex: 'name' },
                  { title: '分组', dataIndex: 'groupName' },
                  { title: '系统', dataIndex: 'os' },
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
                ]}
              />
            )}
          </Card>
        </Col>
        <Col xs={24} xl={9}>
          <Card title="在线率">
            <Progress type="dashboard" percent={onlineRate} />
            <div className="group-list">
              {groups.map((group) => (
                <div key={group.name} className="group-row">
                  <span>{group.name}</span>
                  <Tag color="blue">
                    {group.online}/{group.total}
                  </Tag>
                </div>
              ))}
            </div>
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} xl={12}>
          <Card title="最近任务" extra={<Button type="link" onClick={() => navigate('/tasks')}>全部任务</Button>}>
            <Table
              rowKey="id"
              size="small"
              pagination={false}
              dataSource={tasks.slice(0, 5)}
              columns={[
                { title: '设备', dataIndex: 'deviceName' },
                { title: '命令', dataIndex: 'command', ellipsis: true },
                {
                  title: '状态',
                  dataIndex: 'status',
                  render: (status) => <Tag color={statusColor(status)}>{statusText(status)}</Tag>,
                },
              ]}
            />
          </Card>
        </Col>
        <Col xs={24} xl={12}>
          <Card title="最近审计" extra={<Button type="link" onClick={() => navigate('/audit')}>全部审计</Button>}>
            <Space direction="vertical" size={10} style={{ width: '100%' }}>
              {audits.slice(0, 5).map((audit) => (
                <div key={audit.id} className="audit-line">
                  <PlayCircleOutlined />
                  <span>{audit.action}</span>
                  <span className="muted">{audit.device || audit.deviceId}</span>
                  <Tag color={statusColor(audit.status)}>{statusText(audit.status)}</Tag>
                </div>
              ))}
            </Space>
          </Card>
        </Col>
      </Row>
    </div>
  );
}
