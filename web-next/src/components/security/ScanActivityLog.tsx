'use client';

import { useEffect, useState } from 'react';
import { getScanProgress, getScanQueue } from '@/lib/api';
import type { ScanProgress, ScanQueueStatus } from '@/types';

export default function ScanActivityLog() {
  const [isExpanded, setIsExpanded] = useState(true);
  const [progress, setProgress] = useState<ScanProgress | null>(null);
  const [queueDetails, setQueueDetails] = useState<ScanQueueStatus | null>(null);
  const [loading, setLoading] = useState(true);

  // Fetch data function
  const loadActivity = async () => {
    try {
      const [progressData, queueData] = await Promise.all([
        getScanProgress().catch(() => null),
        getScanQueue().catch(() => null),
      ]);
      setProgress(progressData);
      setQueueDetails(queueData);

      // Auto-expand if activity detected
      if (progressData && (progressData.in_progress > 0 || progressData.pending > 0)) {
        setIsExpanded(true);
      }
    } catch (error) {
      console.error('Failed to load scan activity:', error);
    } finally {
      setLoading(false);
    }
  };

  // 5-second polling
  useEffect(() => {
    loadActivity();
    const interval = setInterval(loadActivity, 5000);
    return () => clearInterval(interval);
  }, []);

  // Calculate display data
  const hasActivity = progress && (progress.in_progress > 0 || progress.pending > 0);
  const currentScans = progress?.current_scans || [];
  const queuedItems = queueDetails?.queue_items || [];
  const completedToday = queueDetails?.completed_today || 0;
  const failedToday = queueDetails?.failed_today || 0;

  // Format elapsed time
  const formatElapsed = (startedAt: string) => {
    const elapsed = Math.floor((Date.now() - new Date(startedAt).getTime()) / 1000);
    if (elapsed < 60) return `${elapsed}s ago`;
    return `${Math.floor(elapsed / 60)}m ago`;
  };

  if (loading) {
    return (
      <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4">
        <div className="text-sm text-[var(--text-tertiary)]">Loading scan activity...</div>
      </div>
    );
  }

  // Don't show if no data
  if (!progress && !queueDetails) return null;

  return (
    <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg overflow-hidden">
      {/* Header */}
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        className="w-full flex items-center justify-between p-4 hover:bg-[var(--bg-tertiary)] transition-colors"
      >
        <div className="flex items-center gap-3">
          <span className="text-lg">{isExpanded ? '▼' : '▶'}</span>
          <span className="font-medium">Scan Activity</span>
          {hasActivity && (
            <span className="text-sm text-[var(--text-tertiary)]">
              ({progress.in_progress} scanning, {progress.pending} queued)
            </span>
          )}
          {hasActivity && (
            <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-[var(--accent)]"></div>
          )}
        </div>
        {!hasActivity && (
          <span className="text-sm text-[var(--text-tertiary)]">Idle</span>
        )}
      </button>

      {/* Collapsible content */}
      {isExpanded && (
        <div className="border-t border-[var(--border)] p-4 space-y-4">
          {/* Currently Scanning */}
          {currentScans.length > 0 && (
            <div>
              <h4 className="text-sm font-semibold mb-2 text-[var(--text-secondary)]">
                Currently Scanning
              </h4>
              <div className="space-y-2">
                {currentScans.map((scan, idx) => (
                  <div
                    key={`${scan.image_id}-${idx}`}
                    className="flex items-center gap-3 p-3 bg-[var(--bg-tertiary)] rounded"
                  >
                    <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-[var(--accent)]"></div>
                    <div className="flex-1">
                      <div className="text-sm font-medium">{scan.image_name}</div>
                      <div className="text-xs text-[var(--text-tertiary)]">
                        on {scan.host_name} (started {formatElapsed(scan.started_at)})
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Queued */}
          {queuedItems.length > 0 && (
            <div>
              <h4 className="text-sm font-semibold mb-2 text-[var(--text-secondary)]">
                Queued ({queuedItems.length})
              </h4>
              <div className="space-y-1">
                {queuedItems.slice(0, 5).map((item, idx) => (
                  <div key={idx} className="text-sm text-[var(--text-tertiary)] pl-3">
                    • {item.image_name} <span className="text-xs">(on {item.host_name})</span>
                  </div>
                ))}
                {queuedItems.length > 5 && (
                  <div className="text-sm text-[var(--text-tertiary)] pl-3">
                    [+ {queuedItems.length - 5} more...]
                  </div>
                )}
              </div>
            </div>
          )}

          {/* Daily Stats */}
          {(completedToday > 0 || failedToday > 0) && (
            <div className="pt-3 border-t border-[var(--border)]">
              <div className="text-sm text-[var(--text-tertiary)]">
                <span className="text-[var(--success)]">✓ {completedToday} completed</span>
                {failedToday > 0 && (
                  <>
                    {' | '}
                    <span className="text-[var(--danger)]">⚠ {failedToday} failed</span>
                  </>
                )}
              </div>
            </div>
          )}

          {/* No Activity */}
          {!hasActivity && currentScans.length === 0 && queuedItems.length === 0 && (
            <div className="text-center py-4 text-[var(--text-tertiary)] text-sm">
              No scans in progress
            </div>
          )}
        </div>
      )}
    </div>
  );
}
