'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { getTrivySummary } from '@/lib/api';
import type { TrivySummary } from '@/types';

const BANNER_DISMISSED_KEY = 'agent-capability-banner-dismissed';

export default function AgentCapabilityBanner() {
  const [summary, setSummary] = useState<TrivySummary | null>(null);
  const [isDismissed, setIsDismissed] = useState(false);
  const [loading, setLoading] = useState(true);
  const router = useRouter();

  useEffect(() => {
    // Check if banner was dismissed
    const dismissed = localStorage.getItem(BANNER_DISMISSED_KEY);
    if (dismissed === 'true') {
      setIsDismissed(true);
      setLoading(false);
      return;
    }

    loadSummary();
  }, []);

  const loadSummary = async () => {
    try {
      const data = await getTrivySummary();
      setSummary(data);
    } catch (err) {
      console.error('Failed to load Trivy summary:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleDismiss = () => {
    localStorage.setItem(BANNER_DISMISSED_KEY, 'true');
    setIsDismissed(true);
  };

  const handleNavigate = () => {
    router.push('/hosts');
  };

  // Don't show if loading, dismissed, or no issues
  if (loading || isDismissed || !summary) {
    return null;
  }

  // Only show if there are hosts without Trivy or disabled
  const hasIssues = summary.without_trivy > 0 || summary.disabled > 0;
  if (!hasIssues) {
    return null;
  }

  return (
    <div className="bg-yellow-50 dark:bg-yellow-900/20 border-l-4 border-yellow-400 dark:border-yellow-600 p-4 mb-6">
      <div className="flex items-start">
        <div className="flex-shrink-0">
          <svg className="h-5 w-5 text-yellow-400 dark:text-yellow-500" viewBox="0 0 20 20" fill="currentColor">
            <path fillRule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clipRule="evenodd" />
          </svg>
        </div>
        <div className="ml-3 flex-1">
          <div className="text-sm text-yellow-700 dark:text-yellow-300">
            <p className="font-medium">Agent Vulnerability Scanning Status</p>
            <p className="mt-1">
              {summary.with_trivy > 0 && (
                <span className="font-semibold text-green-700 dark:text-green-400">
                  {summary.with_trivy} host{summary.with_trivy !== 1 ? 's' : ''} with Trivy
                </span>
              )}
              {summary.with_trivy > 0 && (summary.without_trivy > 0 || summary.disabled > 0) && ', '}
              {summary.without_trivy > 0 && (
                <span className="font-semibold text-yellow-800 dark:text-yellow-400">
                  {summary.without_trivy} without Trivy
                </span>
              )}
              {summary.without_trivy > 0 && summary.disabled > 0 && ', '}
              {summary.disabled > 0 && (
                <span className="font-semibold text-gray-700 dark:text-gray-400">
                  {summary.disabled} disabled
                </span>
              )}
              .{' '}
              <button
                onClick={handleNavigate}
                className="underline hover:text-yellow-900 dark:hover:text-yellow-200"
              >
                Click to manage hosts
              </button>
            </p>
          </div>
        </div>
        <div className="ml-auto pl-3">
          <button
            onClick={handleDismiss}
            className="inline-flex rounded-md text-yellow-400 hover:text-yellow-500 focus:outline-none focus:ring-2 focus:ring-yellow-500 focus:ring-offset-2"
          >
            <span className="sr-only">Dismiss</span>
            <svg className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
              <path fillRule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clipRule="evenodd" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  );
}
