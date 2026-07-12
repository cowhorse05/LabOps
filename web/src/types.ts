export interface User {
  id: string;
  username: string;
  displayName: string;
  roles: string[];
  role: 'admin' | 'operator' | 'viewer';
  permissions: string[];
  status: 'active' | 'disabled';
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
  credentialStatus: 'pending_reenrollment' | 'active' | 'revoked';
  revokedAt?: string;
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
  kind: 'template' | 'ad_hoc';
  templateId?: string;
  executable?: string;
  args?: string[];
  timeoutSeconds: number;
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
  actorId?: string;
  actorRole?: string;
  remoteAddr?: string;
  requestId?: string;
  action: string;
  deviceId?: string;
  device?: string;
  taskId?: string;
  status: string;
  message: string;
  createdAt: string;
}

export interface EnrollmentCode {
  id: string;
  code?: string;
  expiresAt: string;
  maxUses: number;
  usedCount: number;
  createdBy: string;
  createdAt: string;
  revokedAt?: string;
}

export interface TemplateParameter {
  name: string;
  type: 'string' | 'integer';
  pattern?: string;
  enum?: string[];
  min?: number;
  max?: number;
}

export interface CommandTemplate {
  id: string;
  name: string;
  description: string;
  os: string;
  executable: string;
  args: string[];
  parameters: TemplateParameter[];
  requiresPrivilege: boolean;
  enabled: boolean;
  timeoutSeconds: number;
  createdAt: string;
  updatedAt: string;
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
