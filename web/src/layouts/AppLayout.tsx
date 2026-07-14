import { useEffect, useState } from 'react';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import { Avatar, Button, Drawer, Dropdown, Layout, Menu, Space, Tag, Typography, App } from 'antd';
import {
  AuditOutlined,
  ClusterOutlined,
  FileTextOutlined,
  RobotOutlined,
  DashboardOutlined,
  DesktopOutlined,
  KeyOutlined,
  LogoutOutlined,
  MenuOutlined,
  ProfileOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
  TeamOutlined,
  CodeOutlined,
} from '@ant-design/icons';
import { authApi } from '@/api/labops';
import { useAuthStore } from '@/stores/auth';
import ChangePasswordModal from '@/components/ChangePasswordModal';

const { Header, Sider, Content } = Layout;

const baseNavItems = [
  { key: '/dashboard', icon: <DashboardOutlined />, label: '仪表盘' },
  { key: '/devices', icon: <DesktopOutlined />, label: '设备' },
  { key: '/groups', icon: <ClusterOutlined />, label: '分组' },
  { key: '/tasks', icon: <ProfileOutlined />, label: '任务' },
  { key: '/audit', icon: <AuditOutlined />, label: '审计' },
  { key: '/logs', icon: <FileTextOutlined />, label: '日志' },
  { key: '/aiops', icon: <RobotOutlined />, label: 'AI Ops' },
];

export default function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const { message } = App.useApp();
  const user = useAuthStore((s) => s.user);
  const setUser = useAuthStore((s) => s.setUser);
  const clear = useAuthStore((s) => s.clear);
  const hasPermission = (permission: string) => user?.permissions?.includes(permission) ?? false;
  const navItems = [
    ...baseNavItems,
    ...(hasPermission('enrollment:manage') ? [{ key: '/enrollment', icon: <SafetyCertificateOutlined />, label: '设备接入' }] : []),
    ...(hasPermission('templates:manage') ? [{ key: '/templates', icon: <CodeOutlined />, label: '命令模板' }] : []),
    ...(hasPermission('users:manage') ? [{ key: '/users', icon: <TeamOutlined />, label: '用户' }] : []),
  ];

  const [isMobile, setIsMobile] = useState(window.innerWidth < 992);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  useEffect(() => {
    const handleResize = () => setIsMobile(window.innerWidth < 992);
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  useEffect(() => {
    authApi
      .me()
      .then(setUser)
      .catch(() => {
        message.warning('登录状态已失效');
        clear();
        navigate('/login', { replace: true });
      });
  }, [clear, message, navigate, setUser]);

  const activeKey = navItems.find((item) => location.pathname.startsWith(item.key))?.key || '/dashboard';
  const [changePasswordOpen, setChangePasswordOpen] = useState(false);

  const userMenuItems = [
    { key: 'change-password', icon: <KeyOutlined />, label: '修改密码' },
    { key: 'logout', icon: <LogoutOutlined />, label: '退出登录' },
  ];

  const menuElement = (
    <>
      <div className="brand" onClick={() => { navigate('/dashboard'); setMobileMenuOpen(false); }} role="button" tabIndex={0} onKeyDown={(e) => { if (e.key === 'Enter') { navigate('/dashboard'); setMobileMenuOpen(false); } }}>
        <div className="brand-mark">L</div>
        <div>
          <div className="brand-name">LabOps</div>
          <div className="brand-sub">Open lab operations</div>
        </div>
      </div>
      <Menu
        mode="inline"
        selectedKeys={[activeKey]}
        items={navItems}
        onClick={(item) => { navigate(item.key); setMobileMenuOpen(false); }}
        className="shell-menu"
      />
    </>
  );

  return (
    <Layout className="shell">
      {!isMobile && (
        <Sider width={232} className="shell-sider">
          {menuElement}
        </Sider>
      )}
      <Layout>
        <Header className="shell-header">
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            {isMobile && (
              <Button
                type="text"
                icon={<MenuOutlined style={{ color: '#2563eb', fontSize: 20 }} />}
                onClick={() => setMobileMenuOpen(true)}
              />
            )}
            <Typography.Text className="muted">当前环境</Typography.Text>
            <Space size={8}>
              <Tag color="blue">Windows + Docker</Tag>
              <Tag color="green">MVP</Tag>
            </Space>
          </div>
          <Space size={14}>
            <Button icon={<ReloadOutlined />} onClick={() => window.location.reload()}>
              刷新
            </Button>
            <Dropdown
              menu={{
                items: userMenuItems,
                onClick: ({ key }) => {
                  if (key === 'change-password') {
                    setChangePasswordOpen(true);
                  } else if (key === 'logout') {
                    void authApi.logout();
                    clear();
                    navigate('/login', { replace: true });
                  }
                },
              }}
              trigger={['click']}
            >
              <Space size={10} className="user-chip clickable">
                <Avatar size={30}>{user?.displayName?.[0] || 'A'}</Avatar>
                <span>{user?.displayName || 'LabOps Admin'}</span>
                <Tag>{user?.role || 'viewer'}</Tag>
              </Space>
            </Dropdown>
          </Space>
        </Header>
        <Content className="shell-content">
          <Outlet />
        </Content>
      </Layout>
      <Drawer
        title={null}
        placement="left"
        closable={false}
        onClose={() => setMobileMenuOpen(false)}
        open={mobileMenuOpen}
        width={232}
        styles={{ body: { padding: 0, background: '#101827' } }}
      >
        {menuElement}
      </Drawer>
      <ChangePasswordModal
        open={changePasswordOpen}
        onClose={() => setChangePasswordOpen(false)}
      />
    </Layout>
  );
}
