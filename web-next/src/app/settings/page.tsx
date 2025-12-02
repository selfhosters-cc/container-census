'use client';

import { useEffect, useState } from 'react';
import { getHealth } from '@/lib/api';
import type { HealthStatus } from '@/types';

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
  ui: {
    card_design: string;
  };
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

  const handleCardDesignChange = (value: string) => {
    if (!settings) return;
    const updated = {
      ...settings,
      ui: { ...settings.ui, card_design: value },
    };
    handleSave('card_design', updated);
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
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span className="text-[var(--text-tertiary)]">Version:</span>{' '}
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
              <span className="text-[var(--text-tertiary)]">Status:</span>{' '}
              <span className={`font-medium ${health.status === 'healthy' ? 'text-[var(--success)]' : 'text-[var(--danger)]'}`}>
                {health.status}
              </span>
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
            {saving === 'telemetry_interval' && <span className="text-sm text-[var(--text-tertiary)]">Saving...</span>}
          </div>
        </SettingRow>
        <div className="text-xs text-[var(--text-tertiary)] mt-2">
          Telemetry helps improve Container Census by sharing anonymous usage statistics.
          No container names, images, or sensitive data is collected.
        </div>
      </div>

      {/* UI Settings */}
      <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4">
        <h2 className="text-lg font-semibold mb-4">UI Settings</h2>
        <SettingRow
          label="Container Card Design"
          description="Visual style for container cards in the Containers tab"
        >
          <div className="flex items-center gap-2">
            <select
              value={settings?.ui.card_design || 'material'}
              onChange={(e) => handleCardDesignChange(e.target.value)}
              disabled={saving === 'card_design'}
              className="bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2 text-sm"
            >
              <option value="material">Material</option>
              <option value="compact">Compact</option>
              <option value="dashboard">Dashboard</option>
            </select>
            {saving === 'card_design' && <span className="text-sm text-[var(--text-tertiary)]">Saving...</span>}
          </div>
        </SettingRow>
      </div>

      {/* Notification Rate Limits */}
      <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4">
        <h2 className="text-lg font-semibold mb-4">Notification Settings</h2>
        <div className="grid grid-cols-2 gap-4 text-sm">
          <div>
            <span className="text-[var(--text-tertiary)]">Rate Limit:</span>{' '}
            <span className="font-medium">{settings?.notification.rate_limit_max || 100}/hour</span>
          </div>
          <div>
            <span className="text-[var(--text-tertiary)]">Batch Interval:</span>{' '}
            <span className="font-medium">{settings?.notification.rate_limit_batch_interval || 600}s</span>
          </div>
          <div>
            <span className="text-[var(--text-tertiary)]">Threshold Duration:</span>{' '}
            <span className="font-medium">{settings?.notification.threshold_duration || 120}s</span>
          </div>
          <div>
            <span className="text-[var(--text-tertiary)]">Cooldown Period:</span>{' '}
            <span className="font-medium">{settings?.notification.cooldown_period || 300}s</span>
          </div>
        </div>
        <div className="text-xs text-[var(--text-tertiary)] mt-2">
          Notification rate limits are configured via environment variables.
        </div>
      </div>

      {/* Danger Zone */}
      <div className="bg-[var(--bg-secondary)] border border-[var(--danger)] rounded-lg p-4">
        <h2 className="text-lg font-semibold mb-4 text-[var(--danger)]">Danger Zone</h2>
        <div className="flex items-center justify-between">
          <div>
            <div className="font-medium">Reset All Settings</div>
            <div className="text-sm text-[var(--text-tertiary)]">Reset all settings to their default values</div>
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
    </div>
  );
}
