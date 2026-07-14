import { api } from './client';
import type { AiOpsLLMConfig, AuditLog, CommandTemplate, Device, DeviceGroup, DeviceStats, EnrollmentCode, LLMRecommendation, LLMTestResult, SystemStatus, Task, User } from '@/types';

export const setupApi = {
  async systemStatus() {
    const { data } = await api.get<SystemStatus>('/v1/system/status');
    return data;
  },
  async status() {
    const { data } = await api.get<{ setupRequired: boolean }>('/setup/status');
    return data;
  },
  async bootstrap(input: { username: string; password: string; confirmPassword: string; displayName?: string }) {
    const { data } = await api.post<{ user: User; mustChangePassword: boolean }>('/v1/system/bootstrap', input);
    return data;
  },
  async createAdmin(input: { username: string; password: string; confirmPassword: string; displayName?: string }) {
    const { data } = await api.post<{ user: User; mustChangePassword: boolean }>('/setup/admin', input);
    return data;
  },
};

export const authApi = {
  async login(username: string, password: string) {
    const { data } = await api.post<{ user: User; mustChangePassword?: boolean }>('/auth/login', { username, password });
    return data;
  },
  async register(input: { username: string; displayName?: string; password: string; confirmPassword: string }) {
    const { data } = await api.post<{ user: User; mustChangePassword: boolean }>('/auth/register', input);
    return data;
  },
  async me() {
    const { data } = await api.get<User>('/auth/me');
    return data;
  },
  async changePassword(oldPassword: string, newPassword: string) {
    const { data } = await api.post<{ status: string }>('/auth/change-password', { oldPassword, newPassword });
    return data;
  },
  async logout() {
    await api.post('/auth/logout');
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
  async createTask(input: { deviceId?: string; groupName?: string; kind: 'template' | 'ad_hoc'; command?: string; templateId?: string; arguments?: Record<string, unknown>; confirmation?: string }) {
    const { data } = await api.post<{ tasks: Task[] }>('/tasks', input);
    return data;
  },
  async auditLogs() {
    const { data } = await api.get<AuditLog[]>('/audit-logs');
    return data;
  },
  async users() { const { data } = await api.get<User[]>('/users'); return data; },
  async createUser(input: { username: string; displayName: string; password: string; role: string }) { const { data } = await api.post<User>('/users', input); return data; },
  async updateUser(id: string, input: { role: string; status: string }) { await api.put(`/users/${id}`, input); },
  async enrollmentCodes() { const { data } = await api.get<EnrollmentCode[]>('/enrollment-codes'); return data; },
  async createEnrollmentCode(input: { expiresInSeconds?: number; maxUses?: number } = {}) { const { data } = await api.post<EnrollmentCode>('/enrollment-codes', input); return data; },
  async revokeEnrollmentCode(id: string) { await api.delete(`/enrollment-codes/${id}`); },
  async revokeDevice(id: string) { await api.post(`/devices/${id}/revoke`); },
  async commandTemplates() { const { data } = await api.get<CommandTemplate[]>('/command-templates'); return data; },
  async createCommandTemplate(input: Omit<CommandTemplate, 'id' | 'createdAt' | 'updatedAt'>) { const { data } = await api.post<CommandTemplate>('/command-templates', input); return data; },
  async updateCommandTemplate(id: string, input: CommandTemplate) { const { data } = await api.put<CommandTemplate>(`/command-templates/${id}`, input); return data; },
  async llmConfig() {
    const { data } = await api.get<AiOpsLLMConfig>('/aiops/llm-config');
    return data;
  },
  async saveLLMConfig(input: { providerUrl: string; apiKey: string; model: string; providerType: string; enabled: boolean; autoExecuteReadOnly?: boolean }) {
    const { data } = await api.put<{ status: string }>('/aiops/llm-config', input);
    return data;
  },
  async testLLM() {
    const { data } = await api.post<LLMTestResult>('/aiops/llm-test');
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
