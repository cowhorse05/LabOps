import { createBrowserRouter, Navigate } from 'react-router-dom';
import { useEffect, useState, type ReactNode } from 'react';
import AppLayout from '@/layouts/AppLayout';
import LoginPage from '@/pages/LoginPage';
import SetupPage from '@/pages/SetupPage';
import DashboardPage from '@/pages/DashboardPage';
import DevicesPage from '@/pages/DevicesPage';
import DeviceDetailPage from '@/pages/DeviceDetailPage';
import TasksPage from '@/pages/TasksPage';
import AuditPage from '@/pages/AuditPage';
import AiOpsPage from '@/pages/AiOpsPage';
import AiOpsSettingsPage from '@/pages/AiOpsSettingsPage';
import GroupsPage from '@/pages/GroupsPage';
import UsersPage from '@/pages/UsersPage';
import EnrollmentPage from '@/pages/EnrollmentPage';
import TemplatesPage from '@/pages/TemplatesPage';
import LogsPage from '@/pages/LogsPage';
import { useAuthStore } from '@/stores/auth';
import { authApi } from '@/api/labops';

function RequireAuth({ children }: { children: ReactNode }) {
  const user = useAuthStore((s) => s.user);
  const setUser = useAuthStore((s) => s.setUser);
  const clear = useAuthStore((s) => s.clear);
  const [checking, setChecking] = useState(!user);
  useEffect(() => {
    if (user) return;
    authApi.me().then(setUser).catch(clear).finally(() => setChecking(false));
  }, [user, setUser, clear]);
  if (checking) return <div style={{ padding: 48 }}>正在验证登录状态…</div>;
  if (!user) {
    return <Navigate to="/login" replace />;
  }
  return <>{children}</>;
}

export const router = createBrowserRouter([
  {
    path: '/login',
    element: <LoginPage />,
  },
  {
    path: '/setup',
    element: <SetupPage />,
  },
  {
    path: '/',
    element: (
      <RequireAuth>
        <AppLayout />
      </RequireAuth>
    ),
    children: [
      { index: true, element: <Navigate to="/dashboard" replace /> },
      { path: 'dashboard', element: <DashboardPage /> },
      { path: 'devices', element: <DevicesPage /> },
      { path: 'devices/:id', element: <DeviceDetailPage /> },
      { path: 'groups', element: <GroupsPage /> },
      { path: 'tasks', element: <TasksPage /> },
      { path: 'audit', element: <AuditPage /> },
      { path: 'aiops', element: <AiOpsPage /> },
      { path: 'aiops/settings', element: <AiOpsSettingsPage /> },
      { path: 'enrollment', element: <EnrollmentPage /> },
      { path: 'logs', element: <LogsPage /> },
      { path: 'templates', element: <TemplatesPage /> },
      { path: 'users', element: <UsersPage /> },
    ],
  },
]);
