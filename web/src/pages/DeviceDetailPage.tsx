import { useCallback, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useLoadable } from '@/hooks/useLoadable';
import { App, Button, Card, Col, Descriptions, Input, Progress, Row, Space, Table, Tag, Typography } from 'antd';
import { PlayCircleOutlined, ReloadOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { labopsApi } from '@/api/labops';
import type { Device, Task } from '@/types';
import { statusColor, statusText } from '@/utils/status';

const defaultCommand = 'uname -a && echo hello-from-labops';

export default function DeviceDetailPage() {
  const { id } = useParams();
  const { message } = App.useApp();
  const [command, setCommand] = useState(defaultCommand);
  const [running, setRunning] = useState(false);

  const fetchData = useCallback(async () => {
    if (!id) return null;
    const [nextDevice, allTasks] = await Promise.all([labopsApi.device(id), labopsApi.tasks()]);
    return { device: nextDevice, tasks: allTasks.filter((task) => task.deviceId === id) };
  }, [id]);

  const { data, loading, reload } = useLoadable(fetchData, {
    intervalMs: 3000,
    onError: () => message.error('加载设备详情失败'),
  });
  const device = data?.device ?? null;
  const tasks = data?.tasks ?? [];

  async function runCommand() {
    if (!id || !command.trim()) return;
    setRunning(true);
    try {
      await labopsApi.createTask({ deviceId: id, command });
      message.success('命令已下发');
      await reload();
    } finally {
      setRunning(false);
    }
  }

  return (
    <div className="page">
      <div className="page-head">
        <div>
          <Typography.Title level={2}>{device?.name || '设备详情'}</Typography.Title>
          <Typography.Text className="muted">查看资产、实时指标，并向 Agent 下发命令。</Typography.Text>
        </div>
        <Button icon={<ReloadOutlined />} onClick={reload} loading={loading}>
          刷新
        </Button>
      </div>

      {device && (
        <>
          <Row gutter={[16, 16]}>
            <Col xs={24} xl={15}>
              <Card title="资产信息">
                <Descriptions column={2}>
                  <Descriptions.Item label="状态">
                    <Tag color={statusColor(device.status)}>{statusText(device.status)}</Tag>
                  </Descriptions.Item>
                  <Descriptions.Item label="分组">{device.groupName}</Descriptions.Item>
                  <Descriptions.Item label="系统">{device.os}</Descriptions.Item>
                  <Descriptions.Item label="主机名">{device.hostname}</Descriptions.Item>
                  <Descriptions.Item label="IP">{device.ip}</Descriptions.Item>
                  <Descriptions.Item label="Agent">{device.version}</Descriptions.Item>
                  <Descriptions.Item label="CPU">{device.cpuCores} cores</Descriptions.Item>
                  <Descriptions.Item label="内存">{device.memoryMb} MB</Descriptions.Item>
                  <Descriptions.Item label="磁盘">{device.diskTotalGb} GB</Descriptions.Item>
                  <Descriptions.Item label="最后心跳">{dayjs(device.lastSeen).format('YYYY-MM-DD HH:mm:ss')}</Descriptions.Item>
                </Descriptions>
              </Card>
            </Col>
            <Col xs={24} xl={9}>
              <Card title="实时指标">
                <div className="metric">
                  <span>CPU</span>
                  <Progress percent={Math.round(device.cpuUsage)} />
                </div>
                <div className="metric">
                  <span>内存</span>
                  <Progress percent={Math.round(device.memoryUsage)} />
                </div>
                <div className="metric">
                  <span>磁盘</span>
                  <Progress percent={Math.round(device.diskUsage)} />
                </div>
              </Card>
            </Col>
          </Row>

          <Card title="命令执行" style={{ marginTop: 16 }}>
            <Space.Compact style={{ width: '100%' }}>
              <Input.TextArea
                value={command}
                onChange={(e) => setCommand(e.target.value)}
                autoSize={{ minRows: 2, maxRows: 5 }}
              />
              <Button type="primary" icon={<PlayCircleOutlined />} onClick={runCommand} loading={running}>
                执行
              </Button>
            </Space.Compact>
          </Card>

          <Card title="最近任务" style={{ marginTop: 16 }}>
            <Table
              rowKey="id"
              dataSource={tasks}
              columns={[
                { title: '命令', dataIndex: 'command', ellipsis: true },
                {
                  title: '状态',
                  dataIndex: 'status',
                  render: (status) => <Tag color={statusColor(status)}>{statusText(status)}</Tag>,
                },
                {
                  title: '退出码',
                  render: (_, record) => record.result?.exitCode ?? '-',
                },
                {
                  title: '输出',
                  render: (_, record) => (
                    <pre className="task-output">{record.result?.stdout || record.result?.stderr || '-'}</pre>
                  ),
                },
              ]}
            />
          </Card>
        </>
      )}
    </div>
  );
}
