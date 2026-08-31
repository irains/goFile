import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { CircularProgress, Stack, Typography } from '@mui/material';
import { ApiError, api, setCSRFToken, setUnauthenticatedHandler, type BrowserSession } from '../api/client';
import { baseUrl } from '../runtime';

interface SessionContextValue {
  session: BrowserSession | null;
  setSession: (session: BrowserSession | null) => void;
  logout: () => Promise<void>;
}

const SessionContext = createContext<SessionContextValue | null>(null);

function loggedOutLocation() {
  return baseUrl('login');
}

function LoginRedirect() {
  useEffect(() => { window.location.replace(loggedOutLocation()); }, []);
  return null;
}

function SessionLoading() {
  return <Stack component="main" alignItems="center" justifyContent="center" spacing={2} sx={{ minHeight: '100dvh' }}><CircularProgress /><Typography color="text.secondary">FileHarbor</Typography></Stack>;
}

export function SessionProvider({ children, loginPage }: { children: ReactNode; loginPage: boolean }) {
  const [session, setSession] = useState<BrowserSession | null>(null);
  const [loading, setLoading] = useState(!loginPage);

  useEffect(() => {
    setUnauthenticatedHandler(() => {
      setCSRFToken('');
      setSession(null);
      if (!loginPage) window.location.replace(loggedOutLocation());
    });
    return () => setUnauthenticatedHandler();
  }, [loginPage]);

  useEffect(() => {
    if (loginPage) return;
    let active = true;
    void api.getSession().then((loaded) => {
      if (!active) return;
      setCSRFToken(loaded.csrfToken);
      setSession(loaded);
      document.documentElement.lang = loaded.language;
    }).catch((error: unknown) => {
      if (active && !(error instanceof ApiError && error.status === 401)) setSession(null);
    }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [loginPage]);

  const value = useMemo<SessionContextValue>(() => ({
    session,
    setSession(next) {
      setCSRFToken(next?.csrfToken ?? '');
      setSession(next);
      if (next) document.documentElement.lang = next.language;
    },
    async logout() {
      await api.logout();
      setCSRFToken('');
      setSession(null);
    }
  }), [session]);

  if (!loginPage && loading) return <SessionLoading />;
  if (!loginPage && !session) return <LoginRedirect />;
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession() {
  const context = useContext(SessionContext);
  if (!context) throw new Error('useSession must be used within SessionProvider');
  return context;
}

export function useRuntimeBasePath() {
  return baseUrl().replace(/\/$/, '');
}
