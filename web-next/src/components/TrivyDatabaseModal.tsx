'use client';

import { useEffect, useState } from 'react';
import { getTrivyStatus, updateVulnerabilityDb } from '@/lib/api';
import type { TrivyHostStatus, UpdateDBResponse } from '@/types';

interface TrivyDatabaseModalProps {
  isOpen: boolean;
  onClose: () => void;
  onComplete?: () => void;
}

export default function TrivyDatabaseModal({ isOpen, onClose, onComplete }: TrivyDatabaseModalProps) {
  const [hosts, setHosts] = useState<TrivyHostStatus[]>([]);
  const [selectedHostIds, setSelectedHostIds] = useState<number[]>([]);
  const [loading, setLoading] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [results, setResults] = useState<UpdateDBResponse | null>(null);

  useEffect(() => {
    if (isOpen) {
      loadTrivyStatus();
    }
  }, [isOpen]);

  const loadTrivyStatus = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await getTrivyStatus();
      setHosts(data.hosts);
      // Pre-select hosts that have Trivy
      setSelectedHostIds(data.hosts.filter(h => h.has_trivy).map(h => h.host_id));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load Trivy status');
    } finally {
      setLoading(false);
    }
  };

  const handleToggleHost = (hostId: number) => {
    setSelectedHostIds(prev =>
      prev.includes(hostId)
        ? prev.filter(id => id !== hostId)
        : [...prev, hostId]
    );
  };

  const handleSelectAll = () => {
    setSelectedHostIds(hosts.filter(h => h.has_trivy).map(h => h.host_id));
  };

  const handleDeselectAll = () => {
    setSelectedHostIds([]);
  };

  const handleUpdate = async () => {
    if (selectedHostIds.length === 0) {
      setError('Please select at least one host');
      return;
    }

    setUpdating(true);
    setError(null);
    setResults(null);

    try {
      const data = await updateVulnerabilityDb(selectedHostIds);
      setResults(data);

      // Refresh status after update
      await loadTrivyStatus();

      if (onComplete) {
        onComplete();
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update database');
    } finally {
      setUpdating(false);
    }
  };

  const formatDate = (dateStr: string) => {
    if (!dateStr) return 'Never';
    const date = new Date(dateStr);
    if (isNaN(date.getTime())) return 'Never';

    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
    const diffDays = Math.floor(diffHours / 24);

    if (diffHours < 1) return 'Less than 1 hour ago';
    if (diffHours < 24) return `${diffHours} hour${diffHours > 1 ? 's' : ''} ago`;
    if (diffDays < 7) return `${diffDays} day${diffDays > 1 ? 's' : ''} ago`;

    return date.toLocaleDateString();
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl p-6 max-w-3xl w-full mx-4 max-h-[90vh] overflow-y-auto">
        <div className="flex justify-between items-center mb-4">
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
            Update Trivy Database
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

        {results && (
          <div className="mb-4 p-4 bg-blue-50 dark:bg-blue-900 border border-blue-200 dark:border-blue-700 rounded">
            <h3 className="font-semibold text-blue-900 dark:text-blue-100 mb-2">Update Results</h3>
            <div className="space-y-1 text-sm">
              {results.results.map((result, idx) => (
                <div key={idx} className="flex items-center">
                  {result.success ? (
                    <svg className="w-4 h-4 text-green-600 dark:text-green-400 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                    </svg>
                  ) : (
                    <svg className="w-4 h-4 text-red-600 dark:text-red-400 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  )}
                  <span className="text-blue-800 dark:text-blue-200">
                    {result.host_name}: {result.success ? 'Updated' : result.error || 'Failed'}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}

        {loading ? (
          <div className="text-center py-8">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto"></div>
            <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">Loading hosts...</p>
          </div>
        ) : (
          <>
            <div className="mb-4 flex justify-between items-center">
              <p className="text-sm text-gray-600 dark:text-gray-400">
                Select hosts to update their Trivy vulnerability databases
              </p>
              <div className="space-x-2">
                <button
                  onClick={handleSelectAll}
                  className="text-xs text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                >
                  Select All
                </button>
                <button
                  onClick={handleDeselectAll}
                  className="text-xs text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                >
                  Deselect All
                </button>
              </div>
            </div>

            <div className="space-y-2 mb-6 max-h-80 overflow-y-auto">
              {hosts.map(host => (
                <div
                  key={host.host_id}
                  className={`p-4 border rounded-lg ${
                    host.has_trivy
                      ? 'bg-white dark:bg-gray-700 border-gray-200 dark:border-gray-600'
                      : 'bg-gray-50 dark:bg-gray-800 border-gray-200 dark:border-gray-700 opacity-60'
                  }`}
                >
                  <div className="flex items-start">
                    <input
                      type="checkbox"
                      checked={selectedHostIds.includes(host.host_id)}
                      onChange={() => handleToggleHost(host.host_id)}
                      disabled={!host.has_trivy}
                      className="mt-1 mr-3 h-4 w-4 text-blue-600 rounded focus:ring-blue-500"
                    />
                    <div className="flex-1">
                      <div className="flex items-center justify-between">
                        <h3 className="font-medium text-gray-900 dark:text-white">
                          {host.host_name}
                        </h3>
                        {!host.has_trivy && (
                          <span className="text-xs px-2 py-1 bg-yellow-100 dark:bg-yellow-900 text-yellow-800 dark:text-yellow-200 rounded">
                            No Trivy
                          </span>
                        )}
                      </div>
                      {host.has_trivy ? (
                        <div className="mt-1 text-sm text-gray-600 dark:text-gray-400">
                          <div className="flex items-center space-x-4">
                            <span>Version: {host.trivy_version || 'Unknown'}</span>
                            <span>DB: {host.db_version ? formatDate(host.db_version) : 'Unknown'}</span>
                          </div>
                          <div className="text-xs mt-1">
                            Last updated: {formatDate(host.last_updated)}
                          </div>
                        </div>
                      ) : (
                        <div className="mt-1 text-xs text-gray-500 dark:text-gray-500">
                          Trivy not available on this host
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>

            <div className="flex justify-end space-x-3">
              <button
                onClick={onClose}
                disabled={updating}
                className="px-4 py-2 text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded hover:bg-gray-200 dark:hover:bg-gray-600 disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                onClick={handleUpdate}
                disabled={updating || selectedHostIds.length === 0}
                className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50 flex items-center"
              >
                {updating && (
                  <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
                )}
                {updating ? 'Updating...' : `Update ${selectedHostIds.length} Host${selectedHostIds.length !== 1 ? 's' : ''}`}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
