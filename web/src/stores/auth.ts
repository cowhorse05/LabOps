import { create } from 'zustand';
import type { User } from '@/types';

interface AuthState {
  token: string | null;
  user: User | null;
  mustChangePassword: boolean;
  setAuth: (token: string, user: User, mustChangePassword?: boolean) => void;
  clear: () => void;
  setUser: (user: User) => void;
}

const tokenKey = 'labops.token';
const userKey = 'labops.user';
const mustChangePwdKey = 'labops.mustChangePassword';

function readUser() {
  const raw = localStorage.getItem(userKey);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as User;
  } catch {
    return null;
  }
}

function readMustChangePwd(): boolean {
  return localStorage.getItem(mustChangePwdKey) === 'true';
}

export const useAuthStore = create<AuthState>((set) => ({
  token: localStorage.getItem(tokenKey),
  user: readUser(),
  mustChangePassword: readMustChangePwd(),
  setAuth: (token, user, mustChangePassword = false) => {
    localStorage.setItem(tokenKey, token);
    localStorage.setItem(userKey, JSON.stringify(user));
    localStorage.setItem(mustChangePwdKey, String(mustChangePassword));
    set({ token, user, mustChangePassword });
  },
  setUser: (user) => {
    localStorage.setItem(userKey, JSON.stringify(user));
    set({ user });
  },
  clear: () => {
    localStorage.removeItem(tokenKey);
    localStorage.removeItem(userKey);
    localStorage.removeItem(mustChangePwdKey);
    set({ token: null, user: null, mustChangePassword: false });
  },
}));
