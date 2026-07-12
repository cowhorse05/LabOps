import { useEffect, useState } from 'react';
import { Navigate } from 'react-router-dom';
import { App, Button, Form, Input, Typography, Spin } from 'antd';
import { UserOutlined, LockOutlined } from '@ant-design/icons';
import { authApi, setupApi } from '@/api/labops';
import { useAuthStore } from '@/stores/auth';

export default function LoginPage() {
  const { message } = App.useApp();
  const user = useAuthStore((s) => s.user);
  const mustChangePwd = useAuthStore((s) => s.mustChangePassword);
  const setAuth = useAuthStore((s) => s.setAuth);
  const [loading, setLoading] = useState(false);
  const [changing, setChanging] = useState(false);
  const [setupRequired, setSetupRequired] = useState<boolean | null>(null);
  const [setupSubmitting, setSetupSubmitting] = useState(false);

  useEffect(() => {
    setupApi
      .status()
      .then((res) => setSetupRequired(res.setupRequired))
      .catch(() => setSetupRequired(false));
  }, []);

  // Already logged in and no forced change — go to dashboard
  if (user && !mustChangePwd) {
    return <Navigate to="/" replace />;
  }

  async function handleLogin(values: { username: string; password: string }) {
    setLoading(true);
    try {
      const result = await authApi.login(values.username, values.password);
      setAuth(result.user, result.mustChangePassword ?? false);
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
      await authApi.changePassword(values.oldPassword, values.newPassword);
      setAuth(useAuthStore.getState().user!, false);
      message.success('Password changed successfully');
    } catch (err: any) {
      const msg = err?.response?.data?.error || 'Failed to change password';
      message.error(msg);
    } finally {
      setChanging(false);
    }
  }

  async function handleSetup(values: { username: string; password: string; confirmPassword: string }) {
    setSetupSubmitting(true);
    try {
      const result = await setupApi.createAdmin(values);
      setAuth(result.user, result.mustChangePassword ?? false);
      setSetupRequired(false);
      message.success('Administrator created — please change your password');
    } catch (err: any) {
      const msg = err?.response?.data?.error || 'Setup failed';
      message.error(msg);
    } finally {
      setSetupSubmitting(false);
    }
  }

  // Loading state while checking setup status
  if (setupRequired === null && !user) {
    return (
      <div className="login-page">
        <div className="login-panel" style={{ textAlign: 'center' }}>
          <Spin size="large" />
          <Typography.Text type="secondary" style={{ display: 'block', marginTop: 16 }}>
            Checking system status...
          </Typography.Text>
        </div>
      </div>
    );
  }

  // Show change password form if forced
  if (user && mustChangePwd) {
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
              { min: 12, message: 'At least 12 characters' },
            ]}>
              <Input.Password prefix={<LockOutlined />} placeholder="New password (min 12 chars)" />
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

  // Setup form — system is uninitialized, no admin exists
  if (setupRequired && !user) {
    return (
      <div className="login-page">
        <div className="login-panel">
          <div className="login-mark">L</div>
          <Typography.Title level={3} style={{ textAlign: 'center' }}>
            Create First Administrator
          </Typography.Title>
          <Typography.Text type="secondary" style={{ display: 'block', textAlign: 'center', marginBottom: 24 }}>
            No users exist yet. Create the first admin account to get started.
          </Typography.Text>
          <Form onFinish={handleSetup} className="login-form" size="large">
            <Form.Item name="username" rules={[
              { required: true, message: 'Enter username' },
              { min: 3, message: 'At least 3 characters' },
              { pattern: /^[a-z0-9]+$/, message: 'Lowercase letters and numbers only' },
            ]}>
              <Input prefix={<UserOutlined />} placeholder="Username (lowercase, 3+ chars)" autoFocus />
            </Form.Item>
            <Form.Item name="password" rules={[
              { required: true, message: 'Enter password' },
              { min: 12, message: 'At least 12 characters' },
            ]}>
              <Input.Password prefix={<LockOutlined />} placeholder="Password (min 12 chars)" />
            </Form.Item>
            <Form.Item name="confirmPassword" dependencies={['password']} rules={[
              { required: true, message: 'Confirm your password' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('password') === value) return Promise.resolve();
                  return Promise.reject(new Error('Passwords do not match'));
                },
              }),
            ]}>
              <Input.Password prefix={<LockOutlined />} placeholder="Confirm password" />
            </Form.Item>
            <Form.Item>
              <Button type="primary" htmlType="submit" block loading={setupSubmitting}>
                Create Administrator
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
