import { useEffect, useRef, useState } from 'react';
import { Navigate, useNavigate, useSearchParams } from 'react-router-dom';
import { Alert, App, Button, Form, Input, Spin, Typography } from 'antd';
import { LockOutlined, UserOutlined } from '@ant-design/icons';
import { authApi, setupApi } from '@/api/labops';
import { useAuthStore } from '@/stores/auth';
import type { SystemStatus } from '@/types';
import { setupStatusMessage, shouldShowSetupEntry } from '@/utils/setup';

export default function LoginPage() {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const user = useAuthStore((s) => s.user);
  const mustChangePwd = useAuthStore((s) => s.mustChangePassword);
  const setAuth = useAuthStore((s) => s.setAuth);
  const [loading, setLoading] = useState(false);
  const [changing, setChanging] = useState(false);
  const [status, setStatus] = useState<SystemStatus | null>(null);
  const [checkingStatus, setCheckingStatus] = useState(true);
  const successShown = useRef(false);

  useEffect(() => {
    let cancelled = false;
    setupApi.systemStatus()
      .then((result) => {
        if (!cancelled) setStatus(result);
      })
      .catch(() => {
        if (!cancelled) setStatus(null);
      })
      .finally(() => {
        if (!cancelled) setCheckingStatus(false);
      });
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    if (searchParams.get('setup') !== 'success' || successShown.current) return;
    const key = 'labops.setup.success.shown';
    if (sessionStorage.getItem(key) === 'true') return;
    successShown.current = true;
    sessionStorage.setItem(key, 'true');
    message.success('管理员创建成功，请使用新账号登录');
  }, [message, searchParams]);

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
      const msg = err?.response?.data?.message || err?.response?.data?.error || 'Failed to change password';
      message.error(msg);
    } finally {
      setChanging(false);
    }
  }

  if (checkingStatus && !user) {
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

  const needsSetup = shouldShowSetupEntry(status);

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
        {needsSetup ? (
          <>
            <Alert
              type={status?.recoveryRequired ? 'warning' : 'info'}
              showIcon
              style={{ marginBottom: 18 }}
              message={setupStatusMessage(status)}
            />
            <Button
              type="primary"
              block
              size="large"
              onClick={() => navigate('/setup')}
              data-testid="create-admin-entry"
            >
              首次使用？创建管理员账号
            </Button>
            <Typography.Text type="secondary" style={{ display: 'block', textAlign: 'center', marginTop: 12 }}>
              创建完成后即可用该账号登录 LabOps。
            </Typography.Text>
          </>
        ) : (
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
        )}
      </div>
    </div>
  );
}
