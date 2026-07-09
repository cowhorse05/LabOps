import { useState } from 'react';
import { Navigate } from 'react-router-dom';
import { App, Button, Form, Input, Typography } from 'antd';
import { UserOutlined, LockOutlined } from '@ant-design/icons';
import { authApi } from '@/api/labops';
import { useAuthStore } from '@/stores/auth';

export default function LoginPage() {
  const { message } = App.useApp();
  const token = useAuthStore((s) => s.token);
  const mustChangePwd = useAuthStore((s) => s.mustChangePassword);
  const setAuth = useAuthStore((s) => s.setAuth);
  const [loading, setLoading] = useState(false);
  const [changing, setChanging] = useState(false);

  // Already logged in and no forced change — go to dashboard
  if (token && !mustChangePwd) {
    return <Navigate to="/" replace />;
  }

  async function handleLogin(values: { username: string; password: string }) {
    setLoading(true);
    try {
      const result = await authApi.login(values.username, values.password);
      setAuth(result.token, result.user, result.mustChangePassword ?? false);
      if (result.mustChangePassword) {
        message.warning('Please change your password before continuing');
      }
    } catch {
      message.error('Invalid username or password');
    } finally {
      setLoading(false);
    }
  }

  async function handleChangePassword(values: { oldPassword: string; newPassword: string }) {
    setChanging(true);
    try {
      const result = await authApi.changePassword(values.oldPassword, values.newPassword);
      // Clear the mustChangePassword flag
      setAuth(result.token, useAuthStore.getState().user!, false);
      message.success('Password changed successfully');
    } catch (err: any) {
      const msg = err?.response?.data?.error || 'Failed to change password';
      message.error(msg);
    } finally {
      setChanging(false);
    }
  }

  // Show change password form if forced
  if (token && mustChangePwd) {
    return (
      <div className="login-page">
        <div className="login-panel" style={{ maxWidth: 400 }}>
          <div className="login-mark">P</div>
          <Typography.Title level={3} style={{ textAlign: 'center' }}>
            Change Password
          </Typography.Title>
          <Typography.Text type="secondary" style={{ display: 'block', textAlign: 'center', marginBottom: 24 }}>
            You must change your password on first login.
          </Typography.Text>
          <Form onFinish={handleChangePassword} className="login-form" size="large">
            <Form.Item name="oldPassword" rules={[{ required: true, message: 'Enter current password' }]}>
              <Input.Password prefix={<LockOutlined />} placeholder="Current password" />
            </Form.Item>
            <Form.Item name="newPassword" rules={[
              { required: true, message: 'Enter new password' },
              { min: 4, message: 'At least 4 characters' },
            ]}>
              <Input.Password prefix={<LockOutlined />} placeholder="New password (min 4 chars)" />
            </Form.Item>
            <Form.Item>
              <Button type="primary" htmlType="submit" block loading={changing}>
                Change Password
              </Button>
            </Form.Item>
          </Form>
        </div>
      </div>
    );
  }

  // Normal login
  return (
    <div className="login-page">
      <div className="login-panel">
        <div className="login-mark">L</div>
        <Typography.Title level={3} style={{ textAlign: 'center' }}>
          LabOps
        </Typography.Title>
        <Typography.Text type="secondary" style={{ display: 'block', textAlign: 'center', marginBottom: 24 }}>
          Lightweight Operations Platform
        </Typography.Text>
        <Form onFinish={handleLogin} className="login-form" size="large">
          <Form.Item name="username" rules={[{ required: true, message: 'Enter username' }]}>
            <Input prefix={<UserOutlined />} placeholder="Username" autoFocus />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: 'Enter password' }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="Password" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" block loading={loading}>
              Log in
            </Button>
          </Form.Item>
        </Form>
      </div>
    </div>
  );
}
