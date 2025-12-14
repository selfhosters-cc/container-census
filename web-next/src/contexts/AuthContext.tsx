'use client';

import { createContext, useContext, useState, useEffect, ReactNode } from 'react';
import { useRouter, usePathname } from 'next/navigation';

interface AuthContextType {
  isAuthenticated: boolean;
  isLoading: boolean;
  authEnabled: boolean;
  login: () => void;
  logout: () => Promise<void>;
  checkAuth: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [authEnabled, setAuthEnabled] = useState(true);
  const [isLoading, setIsLoading] = useState(true);
  const router = useRouter();
  const pathname = usePathname();

  // Check auth status by calling a protected endpoint
  const checkAuth = async () => {
    setIsLoading(true);
    try {
      // Try a protected endpoint to verify auth
      const containersRes = await fetch('/api/containers', {
        credentials: 'include'
      });

      if (containersRes.status === 200) {
        // Either auth is disabled OR we have valid session
        setIsAuthenticated(true);
        setAuthEnabled(false); // If we got 200 without logging in, auth is disabled
        localStorage.setItem('census-auth', 'true');
      } else if (containersRes.status === 401) {
        // Auth is enabled and we're not authenticated
        setIsAuthenticated(false);
        setAuthEnabled(true);
        localStorage.removeItem('census-auth');

        // Redirect to login if not already there
        if (pathname !== '/login') {
          router.push('/login');
        }
      }
    } catch (error) {
      console.error('Auth check failed:', error);
      setIsAuthenticated(false);
      localStorage.removeItem('census-auth');
    } finally {
      setIsLoading(false);
    }
  };

  // Check auth on mount
  useEffect(() => {
    checkAuth();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const login = () => {
    setIsAuthenticated(true);
    localStorage.setItem('census-auth', 'true');
  };

  const logout = async () => {
    try {
      await fetch('/api/logout', {
        method: 'POST',
        credentials: 'include'
      });
    } catch (error) {
      console.error('Logout failed:', error);
    }
    setIsAuthenticated(false);
    localStorage.removeItem('census-auth');
    router.push('/login');
  };

  return (
    <AuthContext.Provider value={{
      isAuthenticated,
      isLoading,
      authEnabled,
      login,
      logout,
      checkAuth
    }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return context;
}
