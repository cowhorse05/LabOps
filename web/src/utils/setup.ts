import type { SystemStatus } from '@/types';

export function shouldShowSetupEntry(status: SystemStatus | null): boolean {
  return status?.registrationAllowed === true;
}

export function setupStatusMessage(status: SystemStatus | null): string {
  if (status?.recoveryRequired) {
    return '检测到系统存在用户数据但没有可用管理员，请创建首个可登录管理员账号。';
  }
  return '系统尚未创建可用管理员，请先创建首个管理员账号。';
}

export function shouldRedirectSetupToLogin(status: SystemStatus | null): boolean {
  return status?.activeAdminExists === true;
}
