'use client';

import { useAuth } from '@/contexts/AuthContext';

interface ProtectedRouteProps {
  children: React.ReactNode;
}

export default function ProtectedRoute({ children }: ProtectedRouteProps) {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-center">
          <div className="animate-spin text-4xl mb-4">⏳</div>
          <p className="text-[var(--text-tertiary)]">Loading...</p>
        </div>
      </div>
    );
  }

  if (!isAuthenticated) {
    // Auth context will handle redirect
    return null;
  }

  return <>{children}</>;
}
