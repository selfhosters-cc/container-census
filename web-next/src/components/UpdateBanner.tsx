'use client';

import { useState, useEffect } from 'react';
import { checkVersion, getDismissedVersion, dismissVersion } from '@/lib/api';
import type { VersionCheckResponse } from '@/types';

export default function UpdateBanner() {
  const [updateInfo, setUpdateInfo] = useState<VersionCheckResponse | null>(null);
  const [showBanner, setShowBanner] = useState(false);
  const [showModal, setShowModal] = useState(false);

  useEffect(() => {
    checkForUpdates();

    // Check daily (24 hours)
    const interval = setInterval(checkForUpdates, 24 * 60 * 60 * 1000);
    return () => clearInterval(interval);
  }, []);

  const checkForUpdates = async () => {
    try {
      const [versionInfo, dismissedPref] = await Promise.all([
        checkVersion(),
        getDismissedVersion()
      ]);

      setUpdateInfo(versionInfo);

      // Show banner if update available and not dismissed
      if (versionInfo.update_available) {
        const shouldShow = shouldShowUpdate(
          versionInfo.latest_version,
          dismissedPref.dismissed_version,
          dismissedPref.dismiss_until_major
        );
        setShowBanner(shouldShow);
      }
    } catch (error) {
      console.error('Version check failed:', error);
      // Silently fail - don't show banner if collector unreachable
    }
  };

  const shouldShowUpdate = (
    latestVersion: string,
    dismissedVersion: string | null,
    dismissUntilMajor: boolean
  ): boolean => {
    if (!dismissedVersion) return true;

    if (dismissUntilMajor) {
      // Only show if major version changed
      const latestMajor = parseInt(latestVersion.split('.')[0]);
      const dismissedMajor = parseInt(dismissedVersion.split('.')[0]);
      return latestMajor > dismissedMajor;
    } else {
      // Show if any newer version available
      return latestVersion !== dismissedVersion;
    }
  };

  const handleDismiss = async (dismissUntilMajor: boolean) => {
    if (!updateInfo) return;

    try {
      await dismissVersion(updateInfo.latest_version, dismissUntilMajor);
      setShowBanner(false);
      setShowModal(false);
    } catch (error) {
      console.error('Failed to dismiss version:', error);
    }
  };

  if (!showBanner || !updateInfo) return null;

  return (
    <>
      {/* Banner */}
      <div className="bg-[var(--accent)] text-white px-4 py-3 flex items-center justify-between sticky top-0 z-40">
        <div className="flex items-center gap-3">
          <span className="text-2xl">🎉</span>
          <div>
            <span className="font-medium">
              Container Census v{updateInfo.latest_version} is available!
            </span>
            <span className="text-white/80 ml-2">
              You&apos;re on v{updateInfo.current_version}
            </span>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <a
            href={updateInfo.release_url}
            target="_blank"
            rel="noopener noreferrer"
            className="px-4 py-1.5 bg-white text-[var(--accent)] rounded hover:bg-white/90 transition-colors text-sm font-medium"
          >
            View Release
          </a>
          <button
            onClick={() => setShowModal(true)}
            className="px-4 py-1.5 bg-white/20 hover:bg-white/30 rounded transition-colors text-sm"
          >
            Dismiss
          </button>
        </div>
      </div>

      {/* Dismissal Modal */}
      {showModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg w-full max-w-md p-6">
            <h2 className="text-xl font-bold mb-4">Dismiss Update Notification</h2>
            <p className="text-[var(--text-secondary)] mb-6">
              How would you like to dismiss this update notification?
            </p>
            <div className="space-y-3">
              <button
                onClick={() => handleDismiss(false)}
                className="w-full px-4 py-3 bg-[var(--bg-tertiary)] border border-[var(--border)] rounded hover:bg-[var(--bg-primary)] transition-colors text-left"
              >
                <div className="font-medium">Dismiss v{updateInfo.latest_version}</div>
                <div className="text-sm text-[var(--text-secondary)]">
                  Hide this version, show me the next one
                </div>
              </button>
              <button
                onClick={() => handleDismiss(true)}
                className="w-full px-4 py-3 bg-[var(--bg-tertiary)] border border-[var(--border)] rounded hover:bg-[var(--bg-primary)] transition-colors text-left"
              >
                <div className="font-medium">Dismiss until next major release</div>
                <div className="text-sm text-[var(--text-secondary)]">
                  Only notify me for major version updates (e.g., v1.0.0 → v2.0.0)
                </div>
              </button>
            </div>
            <button
              onClick={() => setShowModal(false)}
              className="w-full mt-4 px-4 py-2 text-[var(--text-secondary)] hover:text-[var(--text-primary)] transition-colors"
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </>
  );
}
