import { create } from 'zustand';
import type { User } from '@/types';

interface AuthState {
  user: User | null;
  mustChangePassword: boolean;
  setAuth: (user: User, mustChangePassword?: boolean) => void;
  clear: () => void;
  setUser: (user: User) => void;
}

const userKey = 'labops.user';
const mustChangePwdKey = 'labops.mustChangePassword';

function readUser() {
  const raw = localStorage.getItem(userKey);
  if (!raw) return null;
  try { return JSON.parse(raw) as User; } catch { return null; }
}

export const useAuthStore = create<AuthState>((set) => ({
  user: readUser(),
  mustChangePassword: localStorage.getItem(mustChangePwdKey) === 'true',
  setAuth: (user, mustChangePassword = false) => {
    localStorage.setItem(userKey, JSON.stringify(user));
    localStorage.setItem(mustChangePwdKey, String(mustChangePassword));
    set({ user, mustChangePassword });
  },
  setUser: (user) => {
    localStorage.setItem(userKey, JSON.stringify(user));
    set({ user });
  },
  clear: () => {
    localStorage.removeItem(userKey);
    localStorage.removeItem(mustChangePwdKey);
    set({ user: null, mustChangePassword: false });
  },
}));
