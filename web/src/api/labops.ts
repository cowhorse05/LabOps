import { api } from './client';
import type { AiOpsLLMConfig, AuditLog, Device, DeviceGroup, DeviceStats, LLMRecommendation, Task, User } from '@/types';

export const authApi = {
  async login(username: string, password: string) {
    const { data } = await api.post<{ token: string; user: User; mustChangePassword?: boolean }>('/auth/login', { username, password });
    return data;
  },
  async me() {
    const { data } = await api.get<User>('/auth/me');
    return data;
  },
  async changePassword(oldPassword: string, newPassword: string) {
    const { data } = await api.post<{ token: string }>('/auth/change-password', { oldPassword, newPassword });
    return data;
  },
};

export const labopsApi = {
  async stats() {
    const { data } = await api.get<DeviceStats>('/stats');
    return data;
  },
  async devices() {
    const { data } = await api.get<Device[]>('/devices');
    return data;
  },
  async device(id: string) {
    const { data } = await api.get<Device>(`/devices/${id}`);
    return data;
  },
  async createDevice(input: {
    name: string;
    groupName: string;
    hostname?: string;
    os?: string;
    ip?: string;
    cpuCores?: number;
    memoryMb?: number;
    diskTotalGb?: number;
  }) {
    const { data } = await api.post<Device>('/devices', input);
    return data;
  },
  async deleteDevice(id: string) {
    const { data } = await api.delete<{ status: string }>(`/devices/${id}`);
    return data;
  },
  async deviceTasks(id: string) {
    const { data } = await api.get<Task[]>(`/devices/${id}/tasks`);
    return data;
  },
  async groups() {
    const { data } = await api.get<DeviceGroup[]>('/groups');
    return data;
  },
  async tasks() {
    const { data } = await api.get<Task[]>('/tasks');
    return data;
  },
  async task(id: string) {
    const { data } = await api.get<Task>(`/tasks/${id}`);
    return data;
  },
  async createTask(input: { deviceId?: string; groupName?: string; command: string }) {
    const { data } = await api.post<{ tasks: Task[] }>('/tasks', input);
    return data;
  },
  async auditLogs() {
    const { data } = await api.get<AuditLog[]>('/audit-logs');
    return data;
  },
  async llmConfig() {
    const { data } = await api.get<AiOpsLLMConfig>('/aiops/llm-config');
    return data;
  },
  async saveLLMConfig(input: { providerUrl: string; apiKey: string; model: string; providerType: string; enabled: boolean; autoExecuteReadOnly?: boolean }) {
    const { data } = await api.put<{ status: string }>('/aiops/llm-config', input);
    return data;
  },
  async executeRecommendation(input: { recommendationId?: string; recommendationIds?: string[] }) {
    const { data } = await api.post<{ tasks: Task[]; errors?: string[] }>('/aiops/recommendations/execute', input);
    return data;
  },
  async autoModeConfig() {
    const { data } = await api.get<{ autoExecuteReadOnly: boolean }>('/aiops/auto-mode');
    return data;
  },
  async saveAutoMode(autoExecuteReadOnly: boolean) {
    const { data } = await api.put<{ status: string }>('/aiops/auto-mode', { autoExecuteReadOnly });
    return data;
  },
};
