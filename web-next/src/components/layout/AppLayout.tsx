'use client';

import { ReactNode, useState } from 'react';
import Sidebar from './Sidebar';
import Header from './Header';
import { triggerScan, submitTelemetry } from '@/lib/api';

interface AppLayoutProps {
  children: ReactNode;
}

export default function AppLayout({ children }: AppLayoutProps) {
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null);

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    setToast({ message, type });
    setTimeout(() => setToast(null), 3000);
  };

  const handleScan = async () => {
    try {
      await triggerScan();
      showToast('Scan triggered successfully');
    } catch (error) {
      console.error('Failed to trigger scan:', error);
      showToast('Failed to trigger scan', 'error');
    }
  };

  const handleTelemetry = async () => {
    try {
      await submitTelemetry();
      showToast('Telemetry submitted successfully');
    } catch (error) {
      console.error('Failed to submit telemetry:', error);
      showToast('Failed to submit telemetry', 'error');
    }
  };

  return (
    <div className="flex h-screen">
      <Sidebar />
      <div className="flex-1 flex flex-col overflow-hidden">
        <Header onScan={handleScan} onTelemetry={handleTelemetry} />
        <main className="flex-1 overflow-auto p-6">
          {children}
        </main>
      </div>

      {/* Toast notification */}
      {toast && (
        <div
          className={`fixed bottom-4 right-4 px-4 py-2 rounded-lg text-white shadow-lg z-50 transition-opacity ${
            toast.type === 'success' ? 'bg-[var(--success)]' : 'bg-[var(--danger)]'
          }`}
        >
          {toast.message}
        </div>
      )}
    </div>
  );
}
