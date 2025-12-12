'use client';

import { useEffect, useState } from 'react';
import { getScanProgress } from '@/lib/api';
import type { ScanProgress } from '@/types';

interface ScanProgressModalProps {
  isOpen: boolean;
  onClose: () => void;
  onComplete?: () => void;
  totalQueued?: number; // Number of images that were queued for scanning
}

export default function ScanProgressModal({ isOpen, onClose, onComplete, totalQueued }: ScanProgressModalProps) {
  const [progress, setProgress] = useState<ScanProgress | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [startTime] = useState(Date.now());
  const [hasSeenActivity, setHasSeenActivity] = useState(false);
  const [completedAt, setCompletedAt] = useState<number | null>(null);

  useEffect(() => {
    if (!isOpen) {
      // Reset state when modal closes
      setProgress(null);
      setError(null);
      setHasSeenActivity(false);
      setCompletedAt(null);
      return;
    }

    let pollInterval: NodeJS.Timeout;

    const poll = async () => {
      try {
        const data = await getScanProgress();
        setProgress(data);
        setError(null);

        // Track if we've seen any activity
        // Consider it activity if there's anything in the queue OR if images were queued OR if this is the first poll
        // (scans might complete so fast they're done before we poll)
        if (data.total > 0 || data.in_progress > 0 || data.pending > 0) {
          setHasSeenActivity(true);
        } else if (!hasSeenActivity && (progress === null || (totalQueued && totalQueued > 0))) {
          // First poll came back empty but we know images were queued
          // Scans completed very quickly (likely from cache)
          setHasSeenActivity(true);
        }

        // Check if complete
        const isComplete = data.total === 0 && data.in_progress === 0 && data.pending === 0;

        if (isComplete && hasSeenActivity && !completedAt) {
          // Mark completion time
          setCompletedAt(Date.now());
          if (onComplete) onComplete();
        }

        // Auto-close 3 seconds after completion (only if we saw activity)
        if (completedAt && Date.now() - completedAt > 3000) {
          onClose();
        }

        // Auto-close after 5 minutes
        if (Date.now() - startTime > 5 * 60 * 1000) {
          onClose();
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to fetch progress');
      }
    };

    // Initial poll
    poll();

    // Poll every 2 seconds
    pollInterval = setInterval(poll, 2000);

    return () => {
      if (pollInterval) clearInterval(pollInterval);
    };
  }, [isOpen, onClose, onComplete, startTime, hasSeenActivity, completedAt]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl p-6 max-w-2xl w-full mx-4">
        <div className="flex justify-between items-center mb-4">
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
            Scan Progress
          </h2>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          >
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {error && (
          <div className="mb-4 p-3 bg-red-100 dark:bg-red-900 border border-red-400 dark:border-red-700 rounded text-red-700 dark:text-red-200">
            {error}
          </div>
        )}

        {progress && (
          <>
            {/* Overall Progress */}
            <div className="mb-6">
              <div className="flex justify-between text-sm text-gray-600 dark:text-gray-400 mb-2">
                <span>
                  {progress.in_progress} scanning, {progress.pending} queued
                </span>
                <span>
                  {progress.total > 0 ? `${progress.in_progress + progress.pending} remaining` : 'Complete'}
                </span>
              </div>
              <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2.5">
                <div
                  className="bg-blue-600 h-2.5 rounded-full transition-all duration-300"
                  style={{
                    width: progress.total > 0 ? `${((progress.in_progress / progress.total) * 100)}%` : '100%'
                  }}
                ></div>
              </div>
            </div>

            {/* Current Scans */}
            {progress.current_scans && progress.current_scans.length > 0 ? (
              <div>
                <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">
                  Currently Scanning
                </h3>
                <div className="space-y-2 max-h-60 overflow-y-auto">
                  {progress.current_scans.map((scan, idx) => (
                    <div
                      key={`${scan.image_id}-${idx}`}
                      className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg"
                    >
                      <div className="flex-1 min-w-0">
                        <div className="text-sm font-medium text-gray-900 dark:text-white truncate">
                          {scan.image_name}
                        </div>
                        <div className="text-xs text-gray-500 dark:text-gray-400">
                          on {scan.host_name}
                        </div>
                      </div>
                      <div className="ml-4">
                        <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-blue-600"></div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ) : progress.total === 0 ? (
              <div className="text-center py-8">
                {hasSeenActivity ? (
                  <>
                    <svg className="mx-auto h-12 w-12 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
                      All scans complete!
                    </p>
                    {totalQueued && totalQueued > 0 && (
                      <p className="mt-1 text-xs text-gray-500 dark:text-gray-500">
                        {totalQueued} image{totalQueued !== 1 ? 's' : ''} scanned
                      </p>
                    )}
                  </>
                ) : (
                  <>
                    <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
                      No images to scan
                    </p>
                    <p className="mt-1 text-xs text-gray-500 dark:text-gray-500">
                      Make sure you have containers running with images to scan
                    </p>
                  </>
                )}
              </div>
            ) : (
              <div className="text-center py-8 text-gray-500 dark:text-gray-400">
                <p className="text-sm">Waiting for scans to start...</p>
              </div>
            )}
          </>
        )}

        {!progress && !error && (
          <div className="text-center py-8">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto"></div>
            <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">Loading...</p>
          </div>
        )}
      </div>
    </div>
  );
}
