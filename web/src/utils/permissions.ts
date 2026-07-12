import type { User } from '@/types';

export function can(user: User | null | undefined, permission: string): boolean {
  return user?.status === 'active' && user.permissions.includes(permission);
}
