import { useEffect, useRef, useState } from 'react';
import { Navigate, useNavigate, useSearchParams } from 'react-router-dom';
import { Alert, App, Button, Form, Input, Modal, Spin, Typography } from 'antd';
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

  // Self-registration state
  const [registerOpen, setRegisterOpen] = useState(false);
  const [registerForm] = Form.useForm();
  const [registering, setRegistering] = useState(false);

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

  async function handleRegister(values: { username: string; displayName?: string; password: string; confirmPassword: string }) {
    setRegistering(true);
    try {
      const result = await authApi.register(values);
      setAuth(result.user, result.mustChangePassword ?? false);
      registerForm.resetFields();
      setRegisterOpen(false);
      message.success(`欢迎，${result.user.displayName || result.user.username}！`);
    } catch (err: any) {
      const msg = err?.response?.data?.message || err?.response?.data?.error || '注册失败';
      message.error(msg);
    } finally {
      setRegistering(false);
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

  // needsSetup: no active admin exists and registration is allowed (bootstrap flow)
  const needsSetup = shouldShowSetupEntry(status) && !status?.activeAdminExists;
  // canSelfRegister: an admin already exists but open registration keeps the door open
  const canSelfRegister = status?.registrationAllowed === true && status?.activeAdminExists === true;

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
          <>
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
            {canSelfRegister && (
              <div style={{ textAlign: 'center', marginTop: -8 }}>
                <Typography.Text type="secondary">没有账号？</Typography.Text>
                <Button type="link" onClick={() => { registerForm.resetFields(); setRegisterOpen(true); }}>
                  注册新账号
                </Button>
              </div>
            )}
          </>
        )}
      </div>

      <Modal
        title="注册新账号"
        open={registerOpen}
        onCancel={() => setRegisterOpen(false)}
        footer={null}
        destroyOnHidden
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message="注册后将获得浏览权限，可查看设备、任务和仪表盘。"
        />
        <Form
          form={registerForm}
          layout="vertical"
          onFinish={handleRegister}
          size="large"
        >
          <Form.Item
            name="username"
            label="用户名"
            rules={[
              { required: true, message: '请输入用户名' },
              { min: 3, message: '至少 3 个字符' },
              { max: 64, message: '最多 64 个字符' },
              { pattern: /^[a-z0-9][a-z0-9._-]{2,63}$/, message: '仅支持小写字母、数字、点、下划线和短横线' },
            ]}
          >
            <Input prefix={<UserOutlined />} placeholder="yourname" autoFocus />
          </Form.Item>
          <Form.Item name="displayName" label="昵称（可选）">
            <Input prefix={<UserOutlined />} placeholder="Your Name" />
          </Form.Item>
          <Form.Item
            name="password"
            label="密码"
            extra="至少 12 个字符"
            rules={[
              { required: true, message: '请输入密码' },
              { min: 12, message: '至少 12 个字符' },
            ]}
          >
            <Input.Password prefix={<LockOutlined />} placeholder="Password (min 12 chars)" />
          </Form.Item>
          <Form.Item
            name="confirmPassword"
            label="确认密码"
            dependencies={['password']}
            rules={[
              { required: true, message: '请再次输入密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('password') === value) return Promise.resolve();
                  return Promise.reject(new Error('两次输入的密码不一致'));
                },
              }),
            ]}
          >
            <Input.Password prefix={<LockOutlined />} placeholder="Confirm password" />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0 }}>
            <Button type="primary" htmlType="submit" block loading={registering}>
              注册
            </Button>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
