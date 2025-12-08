'use client';

import { useEffect, useState } from 'react';
import { getHealth, checkVersion, clearDismissedVersion } from '@/lib/api';
import type { HealthStatus, VersionCheckResponse } from '@/types';

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
  }, []);

  const handleCheckUpdates = async () => {
    setCheckingUpdates(true);
    try {
      // Clear any previous dismissals when manually checking
      await clearDismissedVersion();

      // Force fresh check via telemetry collector
      const versionData = await checkVersion();
      setVersionInfo(versionData);

      // Refresh health status to show in UI
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
        <h2 className="text-lg font-semibold mb-4">Telemetry Settings</h2>
        <SettingRow
          label="Report Frequency"
          description="How often to submit anonymous telemetry data"
        >
          <div className="flex items-center gap-2">
            <select
              value={settings?.telemetry.interval_hours || 168}
              onChange={(e) => handleTelemetryIntervalChange(e.target.value)}
              disabled={saving === 'telemetry_interval'}
              className="bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2 text-sm"
            >
              <option value="0">Disabled</option>
              <option value="24">Daily</option>
              <option value="168">Weekly</option>
              <option value="720">Monthly</option>
            </select>
            {saving === 'telemetry_interval' && <span className="text-sm text-[var(--text-secondary)]">Saving...</span>}
          </div>
        </SettingRow>
        <div className="text-xs text-[var(--text-secondary)] mt-2">
          Telemetry helps improve Container Census by sharing anonymous usage statistics.
          No container names, images, or sensitive data is collected.
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
