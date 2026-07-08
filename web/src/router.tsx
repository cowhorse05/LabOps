import { createBrowserRouter, Navigate } from 'react-router-dom';
import type { ReactNode } from 'react';
import AppLayout from '@/layouts/AppLayout';
import LoginPage from '@/pages/LoginPage';
import DashboardPage from '@/pages/DashboardPage';
import DevicesPage from '@/pages/DevicesPage';
import DeviceDetailPage from '@/pages/DeviceDetailPage';
import TasksPage from '@/pages/TasksPage';
import AuditPage from '@/pages/AuditPage';
import GroupsPage from '@/pages/GroupsPage';
import { useAuthStore } from '@/stores/auth';

function RequireAuth({ children }: { children: ReactNode }) {
  const token = useAuthStore((s) => s.token);
  if (!token) {
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
    ],
  },
]);
