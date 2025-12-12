'use client';

import { useState } from 'react';
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
