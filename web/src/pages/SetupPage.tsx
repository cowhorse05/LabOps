import { useEffect, useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { Alert, App, Button, Form, Input, Spin, Typography } from 'antd';
import { LockOutlined, UserOutlined } from '@ant-design/icons';
import { setupApi } from '@/api/labops';
import type { SystemStatus } from '@/types';
import { shouldRedirectSetupToLogin } from '@/utils/setup';

export default function SetupPage() {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const [status, setStatus] = useState<SystemStatus | null>(null);
  const [loadingStatus, setLoadingStatus] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setupApi.systemStatus()
      .then((result) => {
        if (!cancelled) setStatus(result);
      })
      .catch(() => {
        if (!cancelled) message.error('无法确认系统初始化状态，请检查后端服务是否可用');
      })
      .finally(() => {
        if (!cancelled) setLoadingStatus(false);
      });
    return () => { cancelled = true; };
  }, [message]);

  async function handleSubmit(values: { username: string; password: string; confirmPassword: string; displayName?: string }) {
    if (submitting) return;
    setSubmitting(true);
    try {
      await setupApi.bootstrap(values);
      form.resetFields();
      navigate('/login?setup=success', { replace: true });
    } catch (err: any) {
      const data = err?.response?.data;
      message.error(data?.message || data?.error || '创建管理员失败');
      if (err?.response?.status === 409) {
        const latest = await setupApi.systemStatus().catch(() => null);
        if (latest) setStatus(latest);
      }
    } finally {
      setSubmitting(false);
    }
  }

  if (loadingStatus) {
    return (
      <div className="login-page">
        <div className="login-panel" style={{ textAlign: 'center' }}>
          <Spin size="large" />
          <Typography.Text type="secondary" style={{ display: 'block', marginTop: 16 }}>
            正在检查系统初始化状态…
          </Typography.Text>
        </div>
      </div>
    );
  }

  if (shouldRedirectSetupToLogin(status)) {
    return <Navigate to="/login" replace />;
  }

  return (
    <div className="login-page">
      <div className="login-panel">
        <div className="login-mark">L</div>
        <Typography.Title level={3} style={{ textAlign: 'center' }}>
          注册管理员账号
        </Typography.Title>
        <Typography.Text type="secondary" style={{ display: 'block', textAlign: 'center', marginBottom: 18 }}>
          首次部署时创建第一个可登录管理员账号，用于管理和控制远程设备。
        </Typography.Text>
        {status?.recoveryRequired ? (
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 18 }}
            message="检测到系统存在用户数据但没有可用管理员，将进入安全恢复创建流程。"
          />
        ) : (
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 18 }}
            message="系统尚未创建可用管理员，请先创建首个管理员账号。"
          />
        )}
        <Form form={form} onFinish={handleSubmit} className="login-form" size="large" layout="vertical">
          <Form.Item name="username" label="用户名" rules={[
            { required: true, message: '请输入用户名' },
            { min: 3, message: '至少 3 个字符' },
            { max: 64, message: '最多 64 个字符' },
            { pattern: /^[a-z0-9][a-z0-9._-]{2,63}$/, message: '仅支持小写字母、数字、点、下划线和短横线，且必须以字母或数字开头' },
          ]}>
            <Input prefix={<UserOutlined />} placeholder="admin" autoFocus />
          </Form.Item>
          <Form.Item name="displayName" label="昵称（可选）">
            <Input prefix={<UserOutlined />} placeholder="Administrator" />
          </Form.Item>
          <Form.Item name="password" label="密码" extra="至少 12 个字符" rules={[
            { required: true, message: '请输入密码' },
            { min: 12, message: '至少 12 个字符' },
          ]}>
            <Input.Password prefix={<LockOutlined />} placeholder="Password (min 12 chars)" />
          </Form.Item>
          <Form.Item name="confirmPassword" label="确认密码" dependencies={['password']} rules={[
            { required: true, message: '请再次输入密码' },
            ({ getFieldValue }) => ({
              validator(_, value) {
                if (!value || getFieldValue('password') === value) return Promise.resolve();
                return Promise.reject(new Error('两次输入的密码不一致'));
              },
            }),
          ]}>
            <Input.Password prefix={<LockOutlined />} placeholder="Confirm password" />
          </Form.Item>
          <Form.Item>
            <Button
              type="primary"
              htmlType="submit"
              block
              loading={submitting}
              disabled={submitting}
              data-testid="submit-admin-registration"
            >
              创建管理员账号
            </Button>
          </Form.Item>
          <Form.Item style={{ marginBottom: 0 }}>
            <Button type="link" block onClick={() => navigate('/login')}>
              返回登录页
            </Button>
          </Form.Item>
        </Form>
      </div>
    </div>
  );
}
