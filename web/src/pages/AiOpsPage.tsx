import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Alert, Button, Card, Col, Empty, Modal, Progress, Row, Space, Statistic, Switch, Table, Tag, Tooltip, Typography, App } from 'antd';
import {
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  InfoCircleOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  RobotOutlined,
  SettingOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import { api } from '@/api/client';
import { labopsApi } from '@/api/labops';
import type { LLMRecommendation } from '@/types';

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
  llmAnalysis?: string;
  deviceCount: number;
  onlineCount: number;
  offlineCnt: number;
  avgHealth: number;
  insights: DeviceInsight[];
  groups: GroupSummary[];
  recommendations?: LLMRecommendation[];
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
  const navigate = useNavigate();
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

  const [autoMode, setAutoMode] = useState(false);
  const [executing, setExecuting] = useState<Set<string>>(new Set());

  useEffect(() => {
    load();
    labopsApi.autoModeConfig().then((cfg) => setAutoMode(cfg.autoExecuteReadOnly)).catch(() => {});
    const timer = window.setInterval(load, 30000);
    return () => window.clearInterval(timer);
  }, []);

  const handleAutoModeToggle = useCallback(async (checked: boolean) => {
    setAutoMode(checked);
    try {
      await labopsApi.saveAutoMode(checked);
      message.success(checked ? '已开启自动执行模式' : '已关闭自动执行模式');
    } catch {
      setAutoMode(!checked);
      message.error('切换自动执行模式失败');
    }
  }, [message]);

  const handleExecute = useCallback(async (rec: LLMRecommendation) => {
    if (rec.isMutation) {
      Modal.confirm({
        title: '确认执行变更操作',
        icon: <ExclamationCircleOutlined />,
        content: (
          <div>
            <p>该操作将修改系统状态：</p>
            <pre style={{ background: '#f5f5f5', padding: 8, borderRadius: 4 }}>{rec.command}</pre>
            <p>设备：<strong>{rec.deviceName}</strong></p>
            <p>原因：{rec.reason}</p>
          </div>
        ),
        okText: '确认执行',
        cancelText: '取消',
        onOk: () => doExecute(rec.id),
      });
    } else {
      doExecute(rec.id);
    }
  }, [message]);

  const doExecute = useCallback(async (recId: string) => {
    setExecuting((prev) => new Set(prev).add(recId));
    try {
      const result = await labopsApi.executeRecommendation({ recommendationId: recId });
      if (result.errors && result.errors.length > 0) {
        message.warning(`部分失败: ${result.errors.join('; ')}`);
      } else {
        message.success(`任务已创建并下发 (${result.tasks.length} 个)`);
      }
      // Update local state
      setReport((prev) => {
        if (!prev) return prev;
        const recs = prev.recommendations?.map((r) =>
          r.id === recId ? { ...r, status: 'executed' as const } : r
        );
        return { ...prev, recommendations: recs };
      });
    } catch {
      message.error('执行推荐操作失败');
    } finally {
      setExecuting((prev) => {
        const next = new Set(prev);
        next.delete(recId);
        return next;
      });
    }
  }, [message]);

  const priorityColor = (p: string) => p === 'high' ? 'red' : p === 'medium' ? 'orange' : 'green';
  const priorityText = (p: string) => p === 'high' ? '高' : p === 'medium' ? '中' : '低';

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
        <Space>
          <Button icon={<SettingOutlined />} onClick={() => navigate('/aiops/settings')}>
            LLM 设置
          </Button>
          <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
            刷新
          </Button>
        </Space>
      </div>

      {report ? (
        <>
          <Card style={{ marginBottom: 16, borderLeft: '4px solid #2563eb' }}>
            <Typography.Title level={4} style={{ margin: 0 }}>
              {report.summary}
            </Typography.Title>
          </Card>

          {!report.llmAnalysis && (
            <Alert
              type="info"
              showIcon
              message="LLM 智能分析未启用"
              description={<span>配置 LLM API 以获取 AI 驱动的设备分析。<Button type="link" size="small" onClick={() => navigate('/aiops/settings')}>前往设置</Button></span>}
              style={{ marginBottom: 16 }}
            />
          )}

          {report.llmAnalysis && (
            <Card
              title={<span><RobotOutlined style={{ marginRight: 8 }} />LLM 智能分析</span>}
              style={{ marginBottom: 16, borderLeft: '4px solid #7c3aed' }}
            >
              <div style={{ whiteSpace: 'pre-wrap', lineHeight: 1.8 }}>
                {report.llmAnalysis}
              </div>
            </Card>
          )}

          {report.recommendations && report.recommendations.length > 0 && (
            <Card
              title={<span><RobotOutlined style={{ marginRight: 8 }} />LLM 建议操作</span>}
              extra={
                <Tooltip title="自动执行只读诊断命令（如 df、top、free）">
                  <Space>
                    <Typography.Text type="secondary" style={{ fontSize: 13 }}>自动执行</Typography.Text>
                    <Switch size="small" checked={autoMode} onChange={handleAutoModeToggle} />
                  </Space>
                </Tooltip>
              }
              style={{ marginBottom: 16, borderLeft: '4px solid #f59e0b' }}
            >
              {report.recommendations.map((rec) => (
                <Card
                  key={rec.id}
                  size="small"
                  style={{ marginBottom: 8 }}
                  type={rec.status === 'executed' ? 'inner' : 'default'}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                    <div style={{ flex: 1 }}>
                      <Space size={4} wrap>
                        <Tag color={priorityColor(rec.priority)}>{priorityText(rec.priority)}</Tag>
                        <strong>{rec.deviceName}</strong>
                        <Tag>{rec.groupName}</Tag>
                        {rec.isMutation && <Tag color="red">需要确认</Tag>}
                        {rec.status === 'executed' && <Tag color="blue">已执行</Tag>}
                      </Space>
                      <div style={{ marginTop: 4 }}>
                        <pre style={{
                          background: '#1e293b', color: '#e2e8f0', padding: '6px 10px',
                          borderRadius: 4, fontSize: 13, margin: '4px 0',
                        }}>
                          {rec.command}
                        </pre>
                      </div>
                      <Typography.Text type="secondary" style={{ fontSize: 13 }}>
                        {rec.reason}
                      </Typography.Text>
                    </div>
                    <div style={{ marginLeft: 16, whiteSpace: 'nowrap' }}>
                      {rec.status === 'executed' ? (
                        <Button size="small" disabled>已执行</Button>
                      ) : (
                        <Button
                          type={rec.isMutation ? 'primary' : 'default'}
                          size="small"
                          icon={<PlayCircleOutlined />}
                          loading={executing.has(rec.id)}
                          danger={rec.isMutation}
                          onClick={() => handleExecute(rec)}
                        >
                          {rec.isMutation ? '审批并执行' : '执行'}
                        </Button>
                      )}
                    </div>
                  </div>
                </Card>
              ))}
            </Card>
          )}

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
