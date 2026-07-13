import { describe, expect, it } from 'vitest';
import { setupStatusMessage, shouldRedirectSetupToLogin, shouldShowSetupEntry } from './setup';
import type { SystemStatus } from '@/types';

function status(overrides: Partial<SystemStatus>): SystemStatus {
  return {
    initialized: false,
    adminExists: false,
    activeAdminExists: false,
    registrationAllowed: true,
    recoveryRequired: false,
    totalUsers: 0,
    ...overrides,
  };
}

describe('setup status UI decisions', () => {
  it('shows setup entry when registration is allowed', () => {
    expect(shouldShowSetupEntry(status({ registrationAllowed: true }))).toBe(true);
  });

  it('hides setup entry and redirects setup page when an active admin exists', () => {
    const initialized = status({ initialized: true, adminExists: true, activeAdminExists: true, registrationAllowed: false });
    expect(shouldShowSetupEntry(initialized)).toBe(false);
    expect(shouldRedirectSetupToLogin(initialized)).toBe(true);
  });

  it('uses a recovery message when users exist but no active admin exists', () => {
    const recovery = status({ totalUsers: 1, recoveryRequired: true, registrationAllowed: true });
    expect(setupStatusMessage(recovery)).toContain('没有可用管理员');
    expect(shouldRedirectSetupToLogin(recovery)).toBe(false);
  });
});
