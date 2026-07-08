import { useState } from 'react';
import { Button, Form, Input, Typography, App } from 'antd';
import { LockOutlined, UserOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { authApi } from '@/api/labops';
import { useAuthStore } from '@/stores/auth';

export default function LoginPage() {
  const navigate = useNavigate();
  const { message } = App.useApp();
  const setAuth = useAuthStore((s) => s.setAuth);
  const [loading, setLoading] = useState(false);

  async function submit(values: { username: string; password: string }) {
    setLoading(true);
    try {
      const result = await authApi.login(values.username, values.password);
      setAuth(result.token, result.user);
      navigate('/dashboard', { replace: true });
    } catch (err: unknown) {
      if (err && typeof err === 'object' && 'response' in err) {
        const axiosErr = err as { response?: { status?: number } };
        if (axiosErr.response?.status === 401) {
          message.error('用户名或密码不正确');
        } else if (axiosErr.response?.status && axiosErr.response.status >= 500) {
          message.error('服务器内部错误，请稍后重试');
        } else {
          message.error('登录失败，请检查网络连接');
        }
      } else {
        message.error('登录失败：无法连接到服务器');
      }
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="login-page">
      <div className="login-panel">
        <div className="login-mark">L</div>
        <Typography.Title level={2} style={{ margin: 0 }}>
          LabOps
        </Typography.Title>
        <Typography.Text className="muted">轻量开源运维控制台</Typography.Text>
        <Form
          layout="vertical"
          className="login-form"
          initialValues={{ username: '', password: '' }}
          onFinish={submit}
        >
          <Form.Item name="username" label="用户名" rules={[{ required: true }]}>
            <Input size="large" prefix={<UserOutlined />} />
          </Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true }]}>
            <Input.Password size="large" prefix={<LockOutlined />} />
          </Form.Item>
          <Button type="primary" size="large" htmlType="submit" loading={loading} block>
            登录
          </Button>
        </Form>
      </div>
    </div>
  );
}
