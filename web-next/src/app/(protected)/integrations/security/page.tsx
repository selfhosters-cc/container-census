'use client';

import { useState, useEffect } from 'react';
import { scanAllImages } from '@/lib/api';
import ScanHostSelectionModal from '@/components/ScanHostSelectionModal';
import ScanProgressModal from '@/components/ScanProgressModal';
import TrivyDatabaseModal from '@/components/TrivyDatabaseModal';
import AgentCapabilityBanner from '@/components/AgentCapabilityBanner';
import SecurityContent from '@/app/security.old/SecurityContent';

export default function SecurityIntegrationPage() {
  const [showScanHostSelection, setShowScanHostSelection] = useState(false);
  const [showScanProgress, setShowScanProgress] = useState(false);
  const [showDbModal, setShowDbModal] = useState(false);
  const [scannedCount, setScannedCount] = useState(0);
  const [pluginEnabled, setPluginEnabled] = useState<boolean | null>(null);

  const handleScanAll = () => {
    setShowScanHostSelection(true);
  };

  const handleStartScan = async (hostIds: number[]) => {
    // Open progress modal FIRST, before starting the scan
    // This ensures the modal is open when scans complete (even if they're instant from cache)
    setShowScanProgress(true);

    // Then trigger the scan and capture how many images were queued
    const result = await scanAllImages(hostIds);
    setScannedCount(result?.total_queued || 0);
  };

  const handleUpdateDb = () => {
    setShowDbModal(true);
  };

  // Check if security plugin is enabled
  useEffect(() => {
    const checkPluginStatus = async () => {
      try {
        const response = await fetch('/api/plugins');
        const plugins = await response.json();
        const securityPlugin = plugins.find((p: any) => p.id === 'security');
        setPluginEnabled(securityPlugin?.enabled ?? false);
      } catch (error) {
        console.error('Failed to check plugin status:', error);
        setPluginEnabled(false);
      }
    };
    checkPluginStatus();
  }, []);

  // Show loading state while checking
  if (pluginEnabled === null) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-400">Loading...</div>
      </div>
    );
  }

  // Show disabled message if plugin is not enabled
  if (!pluginEnabled) {
    return (
      <div className="max-w-2xl mx-auto mt-12">
        <div className="bg-yellow-500/10 border border-yellow-500/20 rounded-lg p-8 text-center">
          <div className="text-4xl mb-4">🛡️</div>
          <h2 className="text-2xl font-semibold text-yellow-500 mb-2">
            Security Plugin Disabled
          </h2>
          <p className="text-gray-400 mb-6">
            The Security plugin is currently disabled. Enable it in the Integrations page to access vulnerability scanning features.
          </p>
          <a
            href="/integrations"
            className="inline-block px-6 py-2 bg-yellow-500 hover:bg-yellow-600 text-black font-medium rounded-lg transition-colors"
          >
            Go to Integrations
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Agent Capability Banner */}
      <AgentCapabilityBanner />

      {/* Security Content */}
      <SecurityContent
        onScanAll={handleScanAll}
        onUpdateDb={handleUpdateDb}
      />

      {/* Scan Host Selection Modal */}
      <ScanHostSelectionModal
        isOpen={showScanHostSelection}
        onClose={() => setShowScanHostSelection(false)}
        onStartScan={handleStartScan}
      />

      {/* Scan Progress Modal */}
      <ScanProgressModal
        isOpen={showScanProgress}
        onClose={() => setShowScanProgress(false)}
        totalQueued={scannedCount}
      />

      {/* Trivy Database Update Modal */}
      <TrivyDatabaseModal
        isOpen={showDbModal}
        onClose={() => setShowDbModal(false)}
      />
    </div>
  );
}
