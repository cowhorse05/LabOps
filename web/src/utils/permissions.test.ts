import { describe, expect, it } from 'vitest';
import { can } from './permissions';

describe('can', () => {
  it('requires an active user with the exact permission', () => {
    const user = { id: '1', username: 'viewer', displayName: 'Viewer', roles: ['viewer'], role: 'viewer' as const, permissions: ['system:read'], status: 'active' as const };
    expect(can(user, 'system:read')).toBe(true);
    expect(can(user, 'commands:adhoc')).toBe(false);
    expect(can({ ...user, status: 'disabled' }, 'system:read')).toBe(false);
  });
});
