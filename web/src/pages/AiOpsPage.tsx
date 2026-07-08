import { useEffect, useState } from 'react';
import { Button, Card, Col, Empty, Progress, Row, Statistic, Table, Tag, Typography, App } from 'antd';
import {
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  InfoCircleOutlined,
  ReloadOutlined,
  RobotOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import { api } from '@/api/client';

interface DeviceInsight {
  type: 'warning' | 'info' | 'success';
  deviceId: string;
  device: string;
  groupName: string;
  title: string;
  detail: string;
  score: number;
}

interface GroupSummary {
  groupName: string;
  total: number;
  online: number;
  offline: number;
  avgScore: number;
  warningCount: number;
}

interface AiOpsReport {
  generatedAt: string;
  summary: string;
  deviceCount: number;
  onlineCount: number;
  offlineCnt: number;
  avgHealth: number;
  insights: DeviceInsight[];
  groups: GroupSummary[];
}

function insightIcon(type: string) {
  switch (type) {
    case 'warning': return <ExclamationCircleOutlined style={{ color: '#dc2626' }} />;
    case 'info':    return <InfoCircleOutlined style={{ color: '#2563eb' }} />;
    default:        return <CheckCircleOutlined style={{ color: '#16a34a' }} />;
  }
}

function scoreColor(score: number) {
  if (score >= 90) return '#16a34a';
  if (score >= 70) return '#d97706';
  return '#dc2626';
}

export default function AiOpsPage() {
  const { message } = App.useApp();
  const [report, setReport] = useState<AiOpsReport | null>(null);
  const [loading, setLoading] = useState(true);

  async function load() {
    setLoading(true);
    try {
      const { data } = await api.get<AiOpsReport | { message: string }>('/aiops/report');
      if ('insights' in data) {
        setReport(data);
      } else {
        message.info(data.message || '分析进行中');
      }
    } catch {
      if (report) {
        // Already have data, keep showing last report on refresh failure
      } else {
        message.warning('AI Ops 分析数据加载失败，请稍后刷新');
      }
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
    const timer = window.setInterval(load, 30000);
    return () => window.clearInterval(timer);
  }, []);

  return (
    <div className="page">
      <div className="page-head">
        <div>
          <Typography.Title level={2}>
            <RobotOutlined style={{ marginRight: 10 }} />AI Ops 智能分析
          </Typography.Title>
          <Typography.Text className="muted">
            {report
              ? `分析时间: ${dayjs(report.generatedAt).format('YYYY-MM-DD HH:mm:ss')}（每 30 分钟自动刷新）`
              : '首次分析进行中，请稍候...'}
          </Typography.Text>
        </div>
        <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
          刷新
        </Button>
      </div>

      {report ? (
        <>
          <Card style={{ marginBottom: 16, borderLeft: '4px solid #2563eb' }}>
            <Typography.Title level={4} style={{ margin: 0 }}>
              {report.summary}
            </Typography.Title>
          </Card>

          <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
            <Col xs={12} md={6}>
              <Card><Statistic title="设备总数" value={report.deviceCount} prefix={<RobotOutlined />} /></Card>
            </Col>
            <Col xs={12} md={6}>
              <Card><Statistic title="在线设备" value={report.onlineCount} valueStyle={{ color: '#16a34a' }} /></Card>
            </Col>
            <Col xs={12} md={6}>
              <Card><Statistic title="离线设备" value={report.offlineCnt} valueStyle={{ color: report.offlineCnt > 0 ? '#dc2626' : '#64748b' }} /></Card>
            </Col>
            <Col xs={12} md={6}>
              <Card><Statistic title="健康评分" value={report.avgHealth} suffix="/100" valueStyle={{ color: scoreColor(report.avgHealth) }} /></Card>
            </Col>
          </Row>

          <Row gutter={[16, 16]}>
            <Col xs={24} xl={14}>
              <Card title="设备洞察" extra={<Tag>{report.insights.length} 台设备</Tag>}>
                {report.insights.length === 0 ? (
                  <Empty description="暂无设备数据" />
                ) : (
                  <div style={{ maxHeight: 520, overflow: 'auto' }}>
                    {report.insights.map((di) => (
                      <Card
                        key={di.deviceId}
                        size="small"
                        style={{ marginBottom: 10 }}
                        title={
                          <span>
                            {insightIcon(di.type)}{' '}
                            <strong>{di.device}</strong>
                            <Tag style={{ marginLeft: 8 }}>{di.groupName}</Tag>
                          </span>
                        }
                        extra={
                          <Progress
                            type="circle"
                            percent={di.score}
                            size={36}
                            strokeColor={scoreColor(di.score)}
                            format={(p) => `${p}`}
                          />
                        }
                      >
                        <div><WarningOutlined style={{ color: di.type === 'warning' ? '#dc2626' : '#64748b' }} /> {di.title}</div>
                        <div className="muted small" style={{ marginTop: 4 }}>{di.detail}</div>
                      </Card>
                    ))}
                  </div>
                )}
              </Card>
            </Col>
            <Col xs={24} xl={10}>
              <Card title="分组概览">
                <Table
                  rowKey="groupName"
                  size="small"
                  pagination={false}
                  dataSource={report.groups}
                  columns={[
                    { title: '分组', dataIndex: 'groupName' },
                    { title: '设备', render: (_, r: GroupSummary) => `${r.online}/${r.total}` },
                    {
                      title: '健康',
                      dataIndex: 'avgScore',
                      render: (v: number) => <Progress percent={v} size="small" strokeColor={scoreColor(v)} />,
                    },
                    {
                      title: '告警',
                      dataIndex: 'warningCount',
                      render: (v: number) => v > 0 ? <Tag color="red">{v}</Tag> : <Tag color="green">0</Tag>,
                    },
                  ]}
                />
              </Card>
            </Col>
          </Row>
        </>
      ) : (
        <Card>
          <Empty
            description={loading ? '加载中...' : '等待首次 AI Ops 分析完成'}
            image={<RobotOutlined style={{ fontSize: 64, color: '#94a3b8' }} />}
          />
        </Card>
      )}
    </div>
  );
}
