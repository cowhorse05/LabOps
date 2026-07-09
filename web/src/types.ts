export interface User {
  id: string;
  username: string;
  displayName: string;
  roles: string[];
}

export interface Device {
  id: string;
  name: string;
  groupName: string;
  profile: string;
  version: string;
  hostname: string;
  os: string;
  ip: string;
  cpuCores: number;
  memoryMb: number;
  diskTotalGb: number;
  cpuUsage: number;
  memoryUsage: number;
  diskUsage: number;
  status: 'online' | 'offline';
  lastSeen: string;
  createdAt: string;
  updatedAt: string;
}

export interface DeviceStats {
  total: number;
  online: number;
  offline: number;
}

export interface DeviceGroup {
  name: string;
  total: number;
  online: number;
  description: string;
}

export interface TaskResult {
  taskId: string;
  stdout: string;
  stderr: string;
  exitCode: number;
  durationMs: number;
  createdAt: string;
}

export interface Task {
  id: string;
  deviceId: string;
  deviceName: string;
  groupName: string;
  command: string;
  status: 'pending' | 'running' | 'success' | 'failed' | 'timeout';
  requestedBy: string;
  createdAt: string;
  startedAt?: string;
  finishedAt?: string;
  result?: TaskResult;
}

export interface AuditLog {
  id: string;
  actor: string;
  action: string;
  deviceId?: string;
  device?: string;
  taskId?: string;
  status: string;
  message: string;
  createdAt: string;
}

export interface ChangePasswordRequest {
  oldPassword: string;
  newPassword: string;
}

export interface LLMTestResult {
  ok: boolean;
  status: string;
  requestUrl: string;
  requestBody: string;
  reqHeaders: string;
  respStatus: number;
  respBody: string;
  error?: string;
  modelUsed: string;
}

export interface AiOpsLLMConfig {
  providerUrl: string;
  apiKey: string;
  model: string;
  providerType: string;
  enabled: boolean;
  autoExecuteReadOnly: boolean;
  updatedAt: string;
}

export interface LLMRecommendation {
  id: string;
  deviceId: string;
  deviceName: string;
  groupName: string;
  command: string;
  reason: string;
  priority: 'high' | 'medium' | 'low';
  isMutation: boolean;
  status: 'pending' | 'executed' | 'error';
  taskId?: string;
  createdAt: string;
}
