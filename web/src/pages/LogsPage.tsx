import { useMemo, useState } from 'react';
import { Button, Card, Input, Select, Space, Table, Tag, Typography } from 'antd';
import { ClearOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import { useLogStore, type LogLevel, type LogEntry } from '@/stores/logStore';
import dayjs from 'dayjs';

const levelColors: Record<LogLevel, string> = {
  debug: 'default',
  info: 'blue',
  warn: 'orange',
  error: 'red',
};

const levelLabels: Record<LogLevel, string> = {
  debug: 'DEBUG',
  info: 'INFO',
  warn: 'WARN',
  error: 'ERROR',
};

export default function LogsPage() {
  const logs = useLogStore((s) => s.logs);
  const clear = useLogStore((s) => s.clear);
  const [levelFilter, setLevelFilter] = useState<LogLevel | 'all'>('all');
  const [keyword, setKeyword] = useState('');
  const [selectedLog, setSelectedLog] = useState<LogEntry | null>(null);

  const filtered = useMemo(() => {
    let list = [...logs].reverse(); // newest first
    if (levelFilter !== 'all') {
      list = list.filter((l) => l.level === levelFilter);
    }
    const k = keyword.trim().toLowerCase();
    if (k) {
      list = list.filter(
        (l) =>
          l.message.toLowerCase().includes(k) ||
          l.source.toLowerCase().includes(k) ||
          (l.detail ?? '').toLowerCase().includes(k),
      );
    }
    return list;
  }, [logs, levelFilter, keyword]);

  return (
    <div className="page">
      <div className="page-head">
        <div>
          <Typography.Title level={2}>系统日志</Typography.Title>
          <Typography.Text className="muted">
            记录前端运行时错误和关键事件，仅保存在当前会话中。
          </Typography.Text>
        </div>
        <Space wrap>
          <Select
            value={levelFilter}
            onChange={(v) => setLevelFilter(v)}
            style={{ width: 100 }}
            options={[
              { label: '全部', value: 'all' },
              { label: 'ERROR', value: 'error' },
              { label: 'WARN', value: 'warn' },
              { label: 'INFO', value: 'info' },
              { label: 'DEBUG', value: 'debug' },
            ]}
          />
          <Input
            allowClear
            prefix={<SearchOutlined />}
            placeholder="搜索日志内容"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            style={{ width: 240 }}
          />
          <Button icon={<ClearOutlined />} danger onClick={clear} disabled={logs.length === 0}>
            清空
          </Button>
          <Button icon={<ReloadOutlined />} onClick={() => { setKeyword(''); setLevelFilter('all'); }}>
            重置
          </Button>
        </Space>
      </div>

      {selectedLog ? (
        <Card
          title={
            <Space>
              <Tag color={levelColors[selectedLog.level]}>{levelLabels[selectedLog.level]}</Tag>
              <span>{selectedLog.source}</span>
              <Typography.Text type="secondary">{dayjs(selectedLog.timestamp).format('YYYY-MM-DD HH:mm:ss')}</Typography.Text>
            </Space>
          }
          extra={<Button type="link" onClick={() => setSelectedLog(null)}>返回列表</Button>}
        >
          <Typography.Paragraph strong>{selectedLog.message}</Typography.Paragraph>
          {selectedLog.detail && (
            <pre style={{ background: '#f5f5f5', padding: 12, borderRadius: 6, overflow: 'auto', maxHeight: 400, fontSize: 13 }}>
              {selectedLog.detail}
            </pre>
          )}
        </Card>
      ) : (
        <Card>
          <Table<LogEntry>
            scroll={{ x: 'max-content' }}
            rowKey="id"
            dataSource={filtered}
            locale={{ emptyText: '暂无日志记录' }}
            size="small"
            columns={[
              {
                title: '时间',
                dataIndex: 'timestamp',
                width: 180,
                render: (v: string) => dayjs(v).format('HH:mm:ss'),
              },
              {
                title: '级别',
                dataIndex: 'level',
                width: 80,
                render: (level: LogLevel) => <Tag color={levelColors[level]}>{levelLabels[level]}</Tag>,
              },
              {
                title: '来源',
                dataIndex: 'source',
                width: 140,
                render: (v: string) => <Typography.Text code>{v}</Typography.Text>,
              },
              {
                title: '消息',
                dataIndex: 'message',
                ellipsis: true,
              },
              {
                title: '操作',
                width: 60,
                render: (_, record) => (
                  <Button type="link" size="small" onClick={() => setSelectedLog(record)}>
                    详情
                  </Button>
                ),
              },
            ]}
            onRow={(record) => ({
              style: { cursor: 'pointer' },
              onClick: () => setSelectedLog(record),
            })}
          />
        </Card>
      )}

      <Typography.Text type="secondary" style={{ display: 'block', marginTop: 12, textAlign: 'center' }}>
        共 {filtered.length} 条记录（最多保留 {logs.length >= 500 ? '500' : logs.length} 条，刷新页面后清空）
      </Typography.Text>
    </div>
  );
}
