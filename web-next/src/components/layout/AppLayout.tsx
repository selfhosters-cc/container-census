'use client';

import { ReactNode, useState } from 'react';
import Sidebar from './Sidebar';
import Header from './Header';
import { triggerScan, submitTelemetry } from '@/lib/api';

interface AppLayoutProps {
  children: ReactNode;
}

export default function AppLayout({ children }: AppLayoutProps) {
  const handleScan = async () => {
    // Scan progress is now handled by Header component's toast system
    await triggerScan();
  };

  const handleTelemetry = async () => {
    // Telemetry feedback could be added to Header's toast system if needed
    await submitTelemetry();
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
    </div>
  );
}
