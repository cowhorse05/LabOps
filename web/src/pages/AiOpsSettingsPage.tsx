import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Alert, App, Button, Card, Collapse, Form, Input, Select, Space, Switch, Tag, Typography } from 'antd';
import { RobotOutlined, SaveOutlined, ApiOutlined, CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons';
import { labopsApi } from '@/api/labops';
import type { AiOpsLLMConfig, LLMTestResult } from '@/types';

export default function AiOpsSettingsPage() {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [config, setConfig] = useState<AiOpsLLMConfig | null>(null);
  const [testResult, setTestResult] = useState<LLMTestResult | null>(null);
  const [form] = Form.useForm();

  useEffect(() => {
    setLoading(true);
    labopsApi
      .llmConfig()
      .then((cfg) => {
        setConfig(cfg);
        form.setFieldsValue({
          providerUrl: cfg.providerUrl,
          apiKey: '',
          model: cfg.model || '',
          providerType: cfg.providerType || 'openai',
          enabled: cfg.enabled,
          autoExecuteReadOnly: cfg.autoExecuteReadOnly || false,
        });
      })
      .catch(() => message.error('加载 LLM 配置失败'))
      .finally(() => setLoading(false));
  }, [form, message]);

  async function handleSave(values: {
    providerUrl: string;
    apiKey: string;
    model: string;
    providerType: string;
    enabled: boolean;
    autoExecuteReadOnly: boolean;
  }) {
    setSaving(true);
    try {
      const apiKey = values.apiKey || config?.apiKey || '';
      if (values.enabled && !values.providerUrl) {
        message.error('启用 LLM 分析需要提供 Provider URL');
        return;
      }
      await labopsApi.saveLLMConfig({
        providerUrl: values.providerUrl,
        apiKey,
        model: values.model || '',
        providerType: values.providerType || 'openai',
        enabled: values.enabled,
        autoExecuteReadOnly: values.autoExecuteReadOnly || false,
      });
      message.success('LLM 配置已保存，分析将在下次运行中生效');
      navigate('/aiops');
    } catch {
      message.error('保存 LLM 配置失败');
    } finally {
      setSaving(false);
    }
  }

  async function handleTest() {
    setTesting(true);
    setTestResult(null);
    try {
      const result = await labopsApi.testLLM();
      setTestResult(result);
      if (result.ok) {
        message.success('LLM 连接测试通过！');
      } else {
        message.error(result.error || 'LLM 连接测试失败');
      }
    } catch {
      message.error('测试请求失败，请检查网络');
    } finally {
      setTesting(false);
    }
  }

  return (
    <div className="page">
      <div className="page-head">
        <div>
          <Typography.Title level={2}>
            <RobotOutlined style={{ marginRight: 10 }} />
            LLM 分析设置
          </Typography.Title>
          <Typography.Text className="muted">
            配置 OpenAI 或 Anthropic 兼容的 LLM API 以启用智能分析
          </Typography.Text>
        </div>
        <Button onClick={() => navigate('/aiops')}>返回分析</Button>
      </div>

      <Card loading={loading} style={{ maxWidth: 640 }}>
        <Alert
          type="info"
          showIcon
          message="支持的 LLM 服务"
          description="支持 OpenAI 兼容（DeepSeek、Ollama、vLLM、LM Studio 等）和 Anthropic 兼容（Claude 等）的 API。请选择对应的 API 类型。"
          style={{ marginBottom: 24 }}
        />

        <Form
          form={form}
          layout="vertical"
          onFinish={handleSave}
          initialValues={{ enabled: false, model: '', providerType: 'openai', autoExecuteReadOnly: false }}
        >
          <Form.Item
            name="providerType"
            label="API 类型"
            extra="OpenAI: 使用 /v1/chat/completions, Bearer Token 认证 · Anthropic: 使用 /v1/messages, x-api-key 认证"
          >
            <Select
              options={[
                { label: 'OpenAI 兼容（DeepSeek / Ollama / vLLM / ...）', value: 'openai' },
                { label: 'Anthropic 兼容（Claude / DeepSeek Anthropic / ...）', value: 'anthropic' },
              ]}
            />
          </Form.Item>

          <Form.Item
            name="providerUrl"
            label="Provider URL"
            rules={[{ type: 'url', message: '请输入有效的 URL' }]}
            extra="仅需填写 Base URL（不含具体路径）。例如: https://api.deepseek.com 或 https://api.deepseek.com/anthropic"
          >
            <Input placeholder="https://api.deepseek.com" />
          </Form.Item>

          <Form.Item
            name="model"
            label="模型名称"
            extra="OpenAI: gpt-4o, deepseek-chat ｜ Anthropic: claude-sonnet-4-6, deepseek-v4-pro[1M]"
          >
            <Input placeholder="deepseek-chat" />
          </Form.Item>

          <Form.Item
            name="apiKey"
            label="API Key"
            extra={config?.apiKey && config.apiKey.includes('****') ? `已保存的 Key: ${config.apiKey}` : '留空则不修改已有的 Key'}
          >
            <Input.Password placeholder={config?.apiKey?.includes('****') ? '(已设置，留空不修改)' : 'sk-...'} />
          </Form.Item>

          <Form.Item name="enabled" label="启用 LLM 分析" valuePropName="checked">
            <Switch />
          </Form.Item>

          <Form.Item name="autoExecuteReadOnly" label="自动执行非破坏性建议" valuePropName="checked"
            extra="启用后，LLM 生成的只读诊断命令（如 df、top、free）将自动创建并下发任务">
            <Switch />
          </Form.Item>

          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={saving}>
                保存配置
              </Button>
              <Button icon={<ApiOutlined />} loading={testing} onClick={handleTest}>
                测试连接
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Card>

      {testResult && (
        <Card
          title={
            <Space>
              {testResult.ok ? (
                <Tag icon={<CheckCircleOutlined />} color="success">连接成功</Tag>
              ) : (
                <Tag icon={<CloseCircleOutlined />} color="error">连接失败</Tag>
              )}
              <Typography.Text type="secondary">模型: {testResult.modelUsed}</Typography.Text>
            </Space>
          }
          style={{ maxWidth: 640, marginTop: 16 }}
        >
          <Collapse
            size="small"
            items={[
              {
                key: 'request',
                label: '请求详情',
                children: (
                  <div>
                    <Typography.Text strong>URL: </Typography.Text>
                    <Typography.Text code style={{ wordBreak: 'break-all' }}>{testResult.requestUrl}</Typography.Text>
                    <br /><br />
                    <Typography.Text strong>Headers:</Typography.Text>
                    <pre style={{ background: '#f5f5f5', padding: 8, borderRadius: 4, fontSize: 12, whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                      {testResult.reqHeaders}
                    </pre>
                    <Typography.Text strong>Request Body:</Typography.Text>
                    <pre style={{ background: '#f5f5f5', padding: 8, borderRadius: 4, fontSize: 12, whiteSpace: 'pre-wrap', wordBreak: 'break-all', maxHeight: 300, overflow: 'auto' }}>
                      {testResult.requestBody}
                    </pre>
                  </div>
                ),
              },
              {
                key: 'response',
                label: `响应 (HTTP ${testResult.respStatus})`,
                children: (
                  <div>
                    {testResult.error && (
                      <Alert type="error" message={testResult.error} style={{ marginBottom: 12 }} />
                    )}
                    <Typography.Text strong>Response Body:</Typography.Text>
                    <pre style={{ background: '#f5f5f5', padding: 8, borderRadius: 4, fontSize: 12, whiteSpace: 'pre-wrap', wordBreak: 'break-all', maxHeight: 400, overflow: 'auto' }}>
                      {formatJSON(testResult.respBody)}
                    </pre>
                  </div>
                ),
              },
            ]}
          />
        </Card>
      )}
    </div>
  );
}

function formatJSON(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}
