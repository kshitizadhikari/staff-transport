"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";

import { apiFetch, setAccessToken } from "@/lib/api";

export type Role = "manager" | "driver" | "staff";

export type AuthUser = {
  id: string;
  name: string;
  role: Role;
};

type TokenPair = {
  access_token: string;
  refresh_token: string;
  expires_in: number;
};

type AuthContextValue = {
  user: AuthUser | null;
  isAuthenticated: boolean;
  isInitializing: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

const SESSION_HINT_COOKIE = "st_session";

function hasSessionHint(): boolean {
  return document.cookie
    .split("; ")
    .some((cookie) => cookie.startsWith(`${SESSION_HINT_COOKIE}=`));
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [isInitializing, setIsInitializing] = useState(true);

  useEffect(() => {
    let cancelled = false;

    async function restore() {
      try {
        // The refresh token lives in an httpOnly cookie. Only attempt an
        // exchange when the readable session hint says one may exist, so
        // anonymous visitors do not trigger a failed request.
        if (await Promise.resolve(hasSessionHint())) {
          const tokens = await apiFetch<TokenPair>("/auth/refresh", {
            method: "POST",
          });
          setAccessToken(tokens.access_token);
          const me = await apiFetch<AuthUser>("/me");
          if (!cancelled) {
            setUser(me);
          }
        }
      } catch {
        setAccessToken(null);
      } finally {
        if (!cancelled) {
          setIsInitializing(false);
        }
      }
    }

    void restore();
    return () => {
      cancelled = true;
    };
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    const tokens = await apiFetch<TokenPair>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    });
    setAccessToken(tokens.access_token);
    try {
      const me = await apiFetch<AuthUser>("/me");
      setUser(me);
    } catch (error) {
      setAccessToken(null);
      throw error;
    }
  }, []);

  const logout = useCallback(async () => {
    try {
      await apiFetch("/auth/logout", { method: "POST" });
    } finally {
      setAccessToken(null);
      setUser(null);
    }
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      isAuthenticated: user !== null,
      isInitializing,
      login,
      logout,
    }),
    [user, isInitializing, login, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return ctx;
}
