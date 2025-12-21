'use client';

import { useEffect, useState } from 'react';
import { getHealth, checkVersion, clearDismissedVersion, getTelemetryStatus, updateTelemetryEndpoint, createTelemetryEndpoint, deleteTelemetryEndpoint, testTelemetryEndpoint } from '@/lib/api';
import type { HealthStatus, VersionCheckResponse, TelemetryEndpoint, TelemetryEndpointCreate } from '@/types';

interface Settings {
  scanner: {
    interval_seconds: number;
    timeout_seconds: number;
  };
  telemetry: {
    interval_hours: number;
  };
  notification: {
    rate_limit_max: number;
    rate_limit_batch_interval: number;
    threshold_duration: number;
    cooldown_period: number;
  };
}

interface Plugin {
  id: string;
  name: string;
  version: string;
  description: string;
  author: string;
  repository: string;
  enabled: boolean;
  status: string;
  installed_at: string;
}

async function getSettings(): Promise<Settings> {
  const response = await fetch('/api/settings', { credentials: 'include' });
  if (!response.ok) throw new Error('Failed to load settings');
  return response.json();
}

async function updateSettings(settings: Settings): Promise<void> {
  const response = await fetch('/api/settings', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(settings),
  });
  if (!response.ok) throw new Error('Failed to save settings');
}

function SettingRow({ label, description, children }: { label: string; description?: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between py-4 border-b border-[var(--border)]">
      <div className="flex-1">
        <div className="font-medium">{label}</div>
        {description && <div className="text-sm text-[var(--text-tertiary)]">{description}</div>}
      </div>
      <div className="flex-shrink-0 ml-4">{children}</div>
    </div>
  );
}

export default function SettingsPage() {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [health, setHealth] = useState<HealthStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState<string | null>(null);
  const [showUpdateModal, setShowUpdateModal] = useState(false);
  const [checkingUpdates, setCheckingUpdates] = useState(false);
  const [versionInfo, setVersionInfo] = useState<VersionCheckResponse | null>(null);

  // Telemetry state
  const [collectors, setCollectors] = useState<TelemetryEndpoint[]>([]);
  const [togglingCollector, setTogglingCollector] = useState<string | null>(null);
  const [deletingCollector, setDeletingCollector] = useState<string | null>(null);

  // Add collector form state
  const [newCollectorName, setNewCollectorName] = useState('');
  const [newCollectorUrl, setNewCollectorUrl] = useState('');
  const [newCollectorApiKey, setNewCollectorApiKey] = useState('');
  const [addingCollector, setAddingCollector] = useState(false);
  const [testingConnection, setTestingConnection] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);

  const loadCollectors = async () => {
    try {
      const data = await getTelemetryStatus();
      setCollectors(data);
    } catch (error) {
      console.error('Failed to load collectors:', error);
    }
  };

  const loadData = async () => {
    try {
      const [settingsData, healthData] = await Promise.all([
        getSettings(),
        getHealth(),
      ]);
      setSettings(settingsData);
      setHealth(healthData);
    } catch (error) {
      console.error('Failed to load settings:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
    loadCollectors();
  }, []);

  const handleCheckUpdates = async () => {
    setCheckingUpdates(true);
    try {
      // Clear any previous dismissals when manually checking
      await clearDismissedVersion();

      // Trigger fresh version check via collector
      const versionData = await checkVersion();
      setVersionInfo(versionData);

      // Also refresh health status to update header
      const healthData = await getHealth();
      setHealth(healthData);

      // Show modal with results
      setShowUpdateModal(true);
    } catch (error) {
      console.error('Failed to check for updates:', error);
      alert('Failed to check for updates. Please try again later.');
    } finally {
      setCheckingUpdates(false);
    }
  };

  const handleSave = async (key: string, updatedSettings: Settings) => {
    setSaving(key);
    try {
      await updateSettings(updatedSettings);
      setSettings(updatedSettings);
    } catch (error) {
      console.error('Failed to save settings:', error);
    } finally {
      setSaving(null);
    }
  };

  const handleScanIntervalChange = (value: string) => {
    if (!settings) return;
    const updated = {
      ...settings,
      scanner: { ...settings.scanner, interval_seconds: parseInt(value) },
    };
    handleSave('scan_interval', updated);
  };

  const handleTelemetryIntervalChange = (value: string) => {
    if (!settings) return;
    const updated = {
      ...settings,
      telemetry: { ...settings.telemetry, interval_hours: parseInt(value) },
    };
    handleSave('telemetry_interval', updated);
  };

  const handleToggleCollector = async (name: string, enabled: boolean) => {
    setTogglingCollector(name);
    try {
      await updateTelemetryEndpoint(name, { enabled });
      await loadCollectors();
    } catch (error) {
      console.error('Failed to toggle collector:', error);
    } finally {
      setTogglingCollector(null);
    }
  };

  const handleDeleteCollector = async (name: string) => {
    if (!confirm(`Are you sure you want to delete the collector "${name}"?`)) return;

    setDeletingCollector(name);
    try {
      await deleteTelemetryEndpoint(name);
      await loadCollectors();
    } catch (error) {
      console.error('Failed to delete collector:', error);
    } finally {
      setDeletingCollector(null);
    }
  };

  const handleTestConnection = async () => {
    if (!newCollectorUrl) return;

    setTestingConnection(true);
    setTestResult(null);
    try {
      await testTelemetryEndpoint(newCollectorUrl, newCollectorApiKey || undefined);
      setTestResult({ success: true, message: 'Connection successful!' });
    } catch (error) {
      setTestResult({ success: false, message: error instanceof Error ? error.message : 'Connection failed' });
    } finally {
      setTestingConnection(false);
    }
  };

  const handleAddCollector = async () => {
    if (!newCollectorName || !newCollectorUrl) return;

    setAddingCollector(true);
    try {
      const endpoint: TelemetryEndpointCreate = {
        name: newCollectorName,
        url: newCollectorUrl,
        enabled: true,
        api_key: newCollectorApiKey || undefined,
      };
      await createTelemetryEndpoint(endpoint);
      await loadCollectors();
      // Reset form
      setNewCollectorName('');
      setNewCollectorUrl('');
      setNewCollectorApiKey('');
      setTestResult(null);
    } catch (error) {
      console.error('Failed to add collector:', error);
      alert(error instanceof Error ? error.message : 'Failed to add collector');
    } finally {
      setAddingCollector(false);
    }
  };

  const formatStatus = (collector: TelemetryEndpoint) => {
    if (collector.last_success) {
      const date = new Date(collector.last_success);
      return { text: `Last success: ${date.toLocaleString()}`, type: 'success' as const };
    }
    if (collector.last_failure) {
      const date = new Date(collector.last_failure);
      return { text: `Last failure: ${date.toLocaleString()}`, type: 'error' as const };
    }
    return { text: 'No submissions yet', type: 'neutral' as const };
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-[var(--text-tertiary)]">Loading...</div>
      </div>
    );
  }

  return (
    <div className="space-y-6 max-w-3xl">
      <h1 className="text-2xl font-bold">Settings</h1>

      {/* System Info */}
      {health && (
        <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4">
          <h2 className="text-lg font-semibold mb-4">System Information</h2>
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4 text-sm">
              <div>
                <span className="text-[var(--text-secondary)]">Version:</span>{' '}
                <span className="font-medium">v{health.version}</span>
                {health.update_available && (
                  <a
                    href={health.release_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="ml-2 text-[var(--accent)]"
                  >
                    ⬆️ v{health.latest_version} available
                  </a>
                )}
              </div>
              <div>
                <span className="text-[var(--text-secondary)]">Status:</span>{' '}
                <span className={`font-medium ${health.status === 'healthy' ? 'text-[var(--success)]' : 'text-[var(--danger)]'}`}>
                  {health.status}
                </span>
              </div>
            </div>
            <div>
              <button
                onClick={handleCheckUpdates}
                disabled={checkingUpdates}
                className="px-4 py-2 text-sm bg-[var(--accent)] text-white rounded hover:bg-[var(--accent-hover)] transition-colors disabled:opacity-50"
              >
                {checkingUpdates ? 'Checking...' : 'Check for Updates'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Scanner Settings */}
      <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4">
        <h2 className="text-lg font-semibold mb-4">Scanner Settings</h2>
        <SettingRow
          label="Scan Interval"
          description="How often to scan Docker hosts for container changes"
        >
          <div className="flex items-center gap-2">
            <select
              value={settings?.scanner.interval_seconds || 300}
              onChange={(e) => handleScanIntervalChange(e.target.value)}
              disabled={saving === 'scan_interval'}
              className="bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2 text-sm"
            >
              <option value="30">30 seconds</option>
              <option value="60">1 minute</option>
              <option value="120">2 minutes</option>
              <option value="300">5 minutes</option>
              <option value="600">10 minutes</option>
              <option value="900">15 minutes</option>
              <option value="1800">30 minutes</option>
            </select>
            {saving === 'scan_interval' && <span className="text-sm text-[var(--text-tertiary)]">Saving...</span>}
          </div>
        </SettingRow>
      </div>

      {/* Telemetry Settings */}
      <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4">
        <h2 className="text-lg font-semibold mb-4">Telemetry Collectors</h2>
        <p className="text-sm text-[var(--text-tertiary)] mb-4">
          Configure telemetry endpoints to track anonymous container usage statistics.
        </p>

        {/* Submission Frequency */}
        <SettingRow
          label="Submission Frequency"
          description="How often to submit telemetry data to all enabled collectors"
        >
          <div className="flex items-center gap-2">
            <select
              value={settings?.telemetry.interval_hours || 168}
              onChange={(e) => handleTelemetryIntervalChange(e.target.value)}
              disabled={saving === 'telemetry_interval'}
              className="bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2 text-sm"
            >
              <option value="24">Daily</option>
              <option value="168">Weekly (recommended)</option>
              <option value="336">Every 2 weeks</option>
              <option value="720">Monthly</option>
            </select>
            {saving === 'telemetry_interval' && <span className="text-sm text-[var(--text-secondary)]">Saving...</span>}
          </div>
        </SettingRow>

        {/* Community Collector */}
        {(() => {
          const communityCollector = collectors.find(c => c.name === 'community');
          if (!communityCollector) return null;

          const status = formatStatus(communityCollector);
          return (
            <div className="mt-6 p-4 bg-[var(--bg-tertiary)] rounded-lg border-2 border-[var(--accent)]">
              <div className="flex items-start justify-between mb-4">
                <div>
                  <h3 className="font-semibold flex items-center gap-2">
                    Community Collector
                    <span className={`px-2 py-0.5 text-xs rounded-full ${
                      communityCollector.enabled
                        ? 'bg-[var(--success)] text-white'
                        : 'bg-[var(--text-tertiary)] text-white'
                    }`}>
                      {communityCollector.enabled ? 'Enabled' : 'Disabled'}
                    </span>
                  </h3>
                  <p className="text-sm text-[var(--text-tertiary)] mt-1">
                    Help improve Container Census by sharing anonymous usage statistics.
                  </p>
                  <p className="text-xs font-mono text-[var(--text-tertiary)] mt-1">
                    {communityCollector.url}
                  </p>
                </div>
                <button
                  onClick={() => handleToggleCollector(communityCollector.name, !communityCollector.enabled)}
                  disabled={togglingCollector === communityCollector.name}
                  className={`px-4 py-2 text-sm rounded transition-colors ${
                    communityCollector.enabled
                      ? 'bg-[var(--warning)] text-white hover:opacity-90'
                      : 'bg-[var(--accent)] text-white hover:opacity-90'
                  } disabled:opacity-50`}
                >
                  {togglingCollector === communityCollector.name ? 'Updating...' : communityCollector.enabled ? 'Disable' : 'Enable'}
                </button>
              </div>

              {/* Privacy Info */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 p-3 bg-[var(--bg-secondary)] rounded-lg mb-3">
                <div>
                  <h4 className="text-xs font-semibold text-[var(--success)] mb-2">What gets shared:</h4>
                  <ul className="text-xs text-[var(--text-tertiary)] space-y-1">
                    <li>Container Census version</li>
                    <li>Number of containers and hosts</li>
                    <li>Popular container images (names only)</li>
                    <li>Container registry distribution</li>
                    <li>Geographic region (timezone-based)</li>
                  </ul>
                </div>
                <div>
                  <h4 className="text-xs font-semibold text-[var(--danger)] mb-2">What is NOT shared:</h4>
                  <ul className="text-xs text-[var(--text-tertiary)] space-y-1">
                    <li>Host names or IP addresses</li>
                    <li>Container names or env variables</li>
                    <li>Any credentials or secrets</li>
                    <li>Personal information</li>
                  </ul>
                </div>
              </div>

              {/* Status */}
              <div className={`text-xs ${
                status.type === 'success' ? 'text-[var(--success)]' :
                status.type === 'error' ? 'text-[var(--danger)]' :
                'text-[var(--text-tertiary)]'
              }`}>
                {status.text}
              </div>
              {communityCollector.last_failure_reason && (
                <div className="text-xs text-[var(--danger)] mt-1" title={communityCollector.last_failure_reason}>
                  {communityCollector.last_failure_reason.substring(0, 80)}
                  {communityCollector.last_failure_reason.length > 80 ? '...' : ''}
                </div>
              )}

              {/* View Dashboard Link */}
              <div className="mt-3 pt-3 border-t border-[var(--border)]">
                <a
                  href="https://selfhosters.cc/stats"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm text-[var(--accent)] hover:underline"
                >
                  View Community Dashboard →
                </a>
              </div>
            </div>
          );
        })()}

        {/* Custom Collectors */}
        {(() => {
          const customCollectors = collectors.filter(c => c.name !== 'community');
          return (
            <div className="mt-6">
              <h3 className="font-semibold mb-3">Custom Collectors</h3>
              {customCollectors.length === 0 ? (
                <p className="text-sm text-[var(--text-tertiary)] italic">No custom collectors configured.</p>
              ) : (
                <div className="space-y-3">
                  {customCollectors.map(collector => {
                    const status = formatStatus(collector);
                    return (
                      <div key={collector.name} className="p-3 bg-[var(--bg-tertiary)] rounded-lg">
                        <div className="flex items-start justify-between">
                          <div>
                            <div className="font-medium flex items-center gap-2">
                              {collector.name}
                              <span className={`px-2 py-0.5 text-xs rounded-full ${
                                collector.enabled
                                  ? 'bg-[var(--success)] text-white'
                                  : 'bg-[var(--text-tertiary)] text-white'
                              }`}>
                                {collector.enabled ? 'Enabled' : 'Disabled'}
                              </span>
                            </div>
                            <p className="text-xs font-mono text-[var(--text-tertiary)] mt-1">{collector.url}</p>
                            {collector.api_key && (
                              <p className="text-xs text-[var(--text-tertiary)] mt-1">API Key configured</p>
                            )}
                            <div className={`text-xs mt-1 ${
                              status.type === 'success' ? 'text-[var(--success)]' :
                              status.type === 'error' ? 'text-[var(--danger)]' :
                              'text-[var(--text-tertiary)]'
                            }`}>
                              {status.text}
                            </div>
                            {collector.last_failure_reason && (
                              <div className="text-xs text-[var(--danger)] mt-1">
                                {collector.last_failure_reason.substring(0, 60)}
                                {collector.last_failure_reason.length > 60 ? '...' : ''}
                              </div>
                            )}
                          </div>
                          <div className="flex gap-2">
                            <button
                              onClick={() => handleToggleCollector(collector.name, !collector.enabled)}
                              disabled={togglingCollector === collector.name}
                              className="px-3 py-1 text-xs bg-[var(--bg-secondary)] border border-[var(--border)] rounded hover:bg-[var(--bg-primary)] disabled:opacity-50"
                            >
                              {togglingCollector === collector.name ? '...' : collector.enabled ? 'Disable' : 'Enable'}
                            </button>
                            <button
                              onClick={() => handleDeleteCollector(collector.name)}
                              disabled={deletingCollector === collector.name}
                              className="px-3 py-1 text-xs text-[var(--danger)] border border-[var(--danger)] rounded hover:bg-[var(--danger)] hover:text-white disabled:opacity-50"
                            >
                              {deletingCollector === collector.name ? '...' : 'Delete'}
                            </button>
                          </div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          );
        })()}

        {/* Add New Collector Form */}
        <div className="mt-6 p-4 bg-[var(--bg-tertiary)] rounded-lg">
          <h3 className="font-semibold mb-3">Add New Collector</h3>
          <div className="space-y-3">
            <div>
              <label className="block text-xs text-[var(--text-tertiary)] mb-1">Name *</label>
              <input
                type="text"
                value={newCollectorName}
                onChange={(e) => setNewCollectorName(e.target.value)}
                placeholder="My Telemetry Server"
                className="w-full px-3 py-2 text-sm bg-[var(--bg-secondary)] border border-[var(--border)] rounded"
              />
            </div>
            <div>
              <label className="block text-xs text-[var(--text-tertiary)] mb-1">URL *</label>
              <input
                type="url"
                value={newCollectorUrl}
                onChange={(e) => setNewCollectorUrl(e.target.value)}
                placeholder="https://telemetry.example.com/api/ingest"
                className="w-full px-3 py-2 text-sm bg-[var(--bg-secondary)] border border-[var(--border)] rounded font-mono"
              />
            </div>
            <div>
              <label className="block text-xs text-[var(--text-tertiary)] mb-1">API Key (optional)</label>
              <input
                type="password"
                value={newCollectorApiKey}
                onChange={(e) => setNewCollectorApiKey(e.target.value)}
                placeholder="Optional API key for authentication"
                className="w-full px-3 py-2 text-sm bg-[var(--bg-secondary)] border border-[var(--border)] rounded"
              />
            </div>

            {testResult && (
              <div className={`p-2 rounded text-sm ${
                testResult.success
                  ? 'bg-[rgba(34,197,94,0.1)] text-[var(--success)] border border-[var(--success)]'
                  : 'bg-[rgba(239,68,68,0.1)] text-[var(--danger)] border border-[var(--danger)]'
              }`}>
                {testResult.message}
              </div>
            )}

            <div className="flex gap-2 pt-2">
              <button
                onClick={handleTestConnection}
                disabled={testingConnection || !newCollectorUrl}
                className="px-4 py-2 text-sm bg-[var(--bg-secondary)] border border-[var(--border)] rounded hover:bg-[var(--bg-primary)] disabled:opacity-50"
              >
                {testingConnection ? 'Testing...' : 'Test Connection'}
              </button>
              <button
                onClick={handleAddCollector}
                disabled={addingCollector || !newCollectorName || !newCollectorUrl}
                className="px-4 py-2 text-sm bg-[var(--accent)] text-white rounded hover:opacity-90 disabled:opacity-50"
              >
                {addingCollector ? 'Adding...' : 'Add Collector'}
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Notification Rate Limits */}
      <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4">
        <h2 className="text-lg font-semibold mb-4">Notification Settings</h2>
        <div className="grid grid-cols-2 gap-4 text-sm">
          <div>
            <span className="text-[var(--text-secondary)]">Rate Limit:</span>{' '}
            <span className="font-medium">{settings?.notification.rate_limit_max || 100}/hour</span>
          </div>
          <div>
            <span className="text-[var(--text-secondary)]">Batch Interval:</span>{' '}
            <span className="font-medium">{settings?.notification.rate_limit_batch_interval || 600}s</span>
          </div>
          <div>
            <span className="text-[var(--text-secondary)]">Threshold Duration:</span>{' '}
            <span className="font-medium">{settings?.notification.threshold_duration || 120}s</span>
          </div>
          <div>
            <span className="text-[var(--text-secondary)]">Cooldown Period:</span>{' '}
            <span className="font-medium">{settings?.notification.cooldown_period || 300}s</span>
          </div>
        </div>
        <div className="text-xs text-[var(--text-secondary)] mt-2">
          Notification rate limits are configured via environment variables.
        </div>
      </div>

      {/* Danger Zone */}
      <div className="bg-[var(--bg-secondary)] border border-[var(--danger)] rounded-lg p-4">
        <h2 className="text-lg font-semibold mb-4 text-[var(--danger)]">Danger Zone</h2>
        <div className="flex items-center justify-between">
          <div>
            <div className="font-medium">Reset All Settings</div>
            <div className="text-sm text-[var(--text-secondary)]">Reset all settings to their default values</div>
          </div>
          <button
            onClick={() => {
              if (confirm('Are you sure you want to reset all settings to defaults?')) {
                // TODO: Implement reset
              }
            }}
            className="px-4 py-2 text-sm border border-[var(--danger)] text-[var(--danger)] rounded hover:bg-[var(--danger)] hover:text-white transition-colors"
          >
            Reset Settings
          </button>
        </div>
      </div>

      {/* Update Modal */}
      {showUpdateModal && versionInfo && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg w-full max-w-md p-6">
            <h2 className="text-xl font-bold mb-4">Software Update</h2>
            {versionInfo.update_available ? (
              <>
                <p className="text-[var(--text-secondary)] mb-4">
                  A new version of Container Census is available!
                </p>
                <div className="space-y-2 mb-6">
                  <div className="flex justify-between">
                    <span className="text-[var(--text-secondary)]">Current Version:</span>
                    <span className="font-medium">v{versionInfo.current_version}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-[var(--text-secondary)]">Latest Version:</span>
                    <span className="font-medium text-[var(--success)]">v{versionInfo.latest_version}</span>
                  </div>
                </div>
                <div className="flex gap-3">
                  <a
                    href={versionInfo.release_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex-1 px-4 py-2 text-sm text-center bg-[var(--accent)] text-white rounded hover:bg-[var(--accent-hover)] transition-colors"
                  >
                    View Release Notes
                  </a>
                  <button
                    onClick={() => setShowUpdateModal(false)}
                    className="px-4 py-2 text-sm border border-[var(--border)] rounded hover:bg-[var(--bg-tertiary)] transition-colors"
                  >
                    Close
                  </button>
                </div>
              </>
            ) : (
              <>
                <p className="text-[var(--text-secondary)] mb-6">
                  You are running the latest version of Container Census (v{versionInfo.current_version}).
                </p>
                <button
                  onClick={() => setShowUpdateModal(false)}
                  className="w-full px-4 py-2 text-sm bg-[var(--accent)] text-white rounded hover:bg-[var(--accent-hover)] transition-colors"
                >
                  Close
                </button>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
