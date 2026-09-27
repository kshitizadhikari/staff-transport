import {
  createContext,
  use,
  useCallback,
  useMemo,
  useState,
  type PropsWithChildren,
} from 'react';

import { apiFetch, setAccessToken } from '@/lib/api';
import type { AuthUser } from '@/lib/types';

type TokenPair = {
  access_token: string;
  refresh_token: string;
  expires_in: number;
};

type AuthContextValue = {
  user: AuthUser | null;
  isAuthenticated: boolean;
  login: (email: string, password: string) => Promise<void>;
  signOut: () => void;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function useAuth() {
  const value = use(AuthContext);
  if (!value) {
    throw new Error('useAuth must be used within an <AuthProvider />');
  }
  return value;
}

export function AuthProvider({ children }: PropsWithChildren) {
  const [user, setUser] = useState<AuthUser | null>(null);

  const login = useCallback(async (email: string, password: string) => {
    const tokens = await apiFetch<TokenPair>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
    setAccessToken(tokens.access_token);
    try {
      const me = await apiFetch<AuthUser>('/me');
      if (me.role !== 'driver') {
        throw new Error('This app is for driver accounts only.');
      }
      setUser(me);
    } catch (error) {
      setAccessToken(null);
      throw error;
    }
  }, []);

  const signOut = useCallback(() => {
    setAccessToken(null);
    setUser(null);
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({ user, isAuthenticated: user !== null, login, signOut }),
    [user, login, signOut],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
