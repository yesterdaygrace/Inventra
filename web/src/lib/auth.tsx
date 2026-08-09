import * as React from "react";
import { Navigate, useLocation } from "react-router-dom";
import type { Role, User } from "@/types/api";
import { authApi, clearStoredAuth, emitUnauthorized, loadStoredAuth, storeAuth } from "@/lib/api";
import { queryClient } from "@/lib/query";

interface AuthContextValue {
  user: User | null;
  isAuthenticated: boolean;
  isInitializing: boolean;
  login: (email: string, password: string) => Promise<void>;
  loginDemo: () => Promise<void>;
  register: (name: string, email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  setUser: (user: User) => void;
}

const AuthContext = React.createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = React.useState<User | null>(null);
  const [isInitializing, setIsInitializing] = React.useState(true);

  const applySession = React.useCallback(
    (session: { access_token: string; refresh_token: string; expires_in: number; user: User }) => {
      storeAuth(session);
      setUser(session.user);
    },
    [],
  );

  React.useEffect(() => {
    const stored = loadStoredAuth();
    let cancelled = false;

    const init = async () => {
      if (!stored?.accessToken) {
        setIsInitializing(false);
        return;
      }
      try {
        const me = await authApi.me();
        if (!cancelled) {
          setUser(me);
        }
      } catch {
        clearStoredAuth();
        setUser(null);
      } finally {
        if (!cancelled) setIsInitializing(false);
      }
    };

    init();

    const onUnauthorized = () => {
      clearStoredAuth();
      setUser(null);
    };
    window.addEventListener("inventra:unauthorized", onUnauthorized);

    return () => {
      cancelled = true;
      window.removeEventListener("inventra:unauthorized", onUnauthorized);
    };
  }, []);

  const login = React.useCallback(
    async (email: string, password: string) => {
      const session = await authApi.login({ email, password });
      applySession(session);
    },
    [applySession],
  );

  const loginDemo = React.useCallback(async () => {
    const session = await authApi.demo();
    applySession(session);
  }, [applySession]);

  const register = React.useCallback(
    async (name: string, email: string, password: string) => {
      await authApi.register({ name, email, password });
      const session = await authApi.login({ email, password });
      applySession(session);
    },
    [applySession],
  );

  const logout = React.useCallback(async () => {
    const stored = loadStoredAuth();
    try {
      if (stored?.refreshToken) {
        await authApi.logout(stored.refreshToken);
      }
    } catch {
      // Ignore — we clear local state regardless.
    }
    clearStoredAuth();
    setUser(null);
    queryClient.clear();
    emitUnauthorized();
  }, []);

  const value = React.useMemo<AuthContextValue>(
    () => ({
      user,
      isAuthenticated: !!user,
      isInitializing,
      login,
      loginDemo,
      register,
      logout,
      setUser,
    }),
    [user, isInitializing, login, loginDemo, register, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = React.useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}

export function RequireAuth({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isInitializing } = useAuth();
  const location = useLocation();

  if (isInitializing) {
    return null;
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  return <>{children}</>;
}

export function RequireRole({ roles, children }: { roles: Role[]; children: React.ReactNode }) {
  const { user, isAuthenticated, isInitializing } = useAuth();
  const location = useLocation();

  if (isInitializing) return null;

  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  if (!user || !roles.includes(user.role)) {
    return <Navigate to="/" replace />;
  }

  return <>{children}</>;
}

export function RequireGuest({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isInitializing } = useAuth();

  if (isInitializing) return null;

  if (isAuthenticated) {
    return <Navigate to="/" replace />;
  }

  return <>{children}</>;
}