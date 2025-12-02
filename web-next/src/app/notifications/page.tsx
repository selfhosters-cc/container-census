'use client';

import { useEffect, useState, useMemo } from 'react';
import {
  getNotificationLog,
  getNotificationChannels,
  getNotificationRules,
  getNotificationSilences,
  getNotificationStatus,
  markNotificationRead,
  markAllNotificationsRead,
  clearOldNotifications,
  createNotificationChannel,
  updateNotificationChannel,
  deleteNotificationChannel,
  testNotificationChannel,
  createNotificationRule,
  updateNotificationRule,
  deleteNotificationRule,
  createNotificationSilence,
  deleteNotificationSilence,
} from '@/lib/api';
import type {
  NotificationLog,
  NotificationChannel,
  NotificationRule,
  NotificationSilence,
} from '@/types';

type Tab = 'log' | 'rules' | 'channels' | 'silences';

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString();
}

function EventTypeBadge({ type }: { type: string }) {
  const colors: Record<string, string> = {
    container_started: 'bg-[var(--success)]',
    container_stopped: 'bg-[var(--danger)]',
    new_image: 'bg-[var(--accent)]',
    high_cpu: 'bg-[var(--warning)]',
    high_memory: 'bg-[var(--warning)]',
    anomalous_behavior: 'bg-purple-500',
    vulnerability_critical: 'bg-[#ff1744]',
    vulnerability_high: 'bg-[#ff9800]',
  };

  return (
    <span className={`px-2 py-0.5 text-xs rounded text-white ${colors[type] || 'bg-gray-500'}`}>
      {type.replace(/_/g, ' ')}
    </span>
  );
}

interface NotificationLogListProps {
  logs: NotificationLog[];
  onMarkRead: (id: number) => void;
}

function NotificationLogList({ logs, onMarkRead }: NotificationLogListProps) {
  if (logs.length === 0) {
    return (
      <div className="text-center py-8 text-[var(--text-tertiary)]">
        No notifications
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {logs.map(log => (
        <div
          key={log.id}
          className={`bg-[var(--bg-tertiary)] rounded-lg p-4 ${!log.read ? 'border-l-4 border-l-[var(--accent)]' : ''}`}
        >
          <div className="flex items-start justify-between gap-4">
            <div className="flex-1">
              <div className="flex items-center gap-2 mb-1">
                <EventTypeBadge type={log.event_type} />
                <span className="text-xs text-[var(--text-tertiary)]">{formatDate(log.created_at)}</span>
              </div>
              <div className="font-medium">{log.container_name || 'System'}</div>
              <div className="text-sm text-[var(--text-tertiary)]">{log.message}</div>
            </div>
            {!log.read && (
              <button
                onClick={() => onMarkRead(log.id)}
                className="px-2 py-1 text-xs bg-[var(--accent)] text-white rounded hover:opacity-80"
              >
                Mark Read
              </button>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}

interface ChannelFormModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (channel: Partial<NotificationChannel>) => Promise<void>;
  editChannel?: NotificationChannel | null;
}

function ChannelFormModal({ isOpen, onClose, onSubmit, editChannel }: ChannelFormModalProps) {
  const [name, setName] = useState('');
  const [type, setType] = useState<'webhook' | 'ntfy' | 'in_app'>('webhook');
  const [enabled, setEnabled] = useState(true);
  const [config, setConfig] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (editChannel) {
      setName(editChannel.name);
      setType(editChannel.type);
      setEnabled(editChannel.enabled);
      setConfig((editChannel.config || {}) as Record<string, string>);
    } else {
      setName('');
      setType('webhook');
      setEnabled(true);
      setConfig({});
    }
  }, [editChannel, isOpen]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await onSubmit({ name, type, enabled, config });
      onClose();
    } catch (error) {
      console.error('Failed to save channel:', error);
    } finally {
      setLoading(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg w-full max-w-md p-6">
        <h2 className="text-xl font-bold mb-4">{editChannel ? 'Edit Channel' : 'Add Channel'}</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm text-[var(--text-tertiary)] mb-1">Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              className="w-full bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2"
            />
          </div>

          <div>
            <label className="block text-sm text-[var(--text-tertiary)] mb-1">Type</label>
            <select
              value={type}
              onChange={(e) => setType(e.target.value as 'webhook' | 'ntfy' | 'in_app')}
              className="w-full bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2"
            >
              <option value="webhook">Webhook</option>
              <option value="ntfy">Ntfy</option>
              <option value="in_app">In-App</option>
            </select>
          </div>

          {type === 'webhook' && (
            <div>
              <label className="block text-sm text-[var(--text-tertiary)] mb-1">Webhook URL</label>
              <input
                type="url"
                value={config.url || ''}
                onChange={(e) => setConfig({ ...config, url: e.target.value })}
                required
                className="w-full bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2"
              />
            </div>
          )}

          {type === 'ntfy' && (
            <>
              <div>
                <label className="block text-sm text-[var(--text-tertiary)] mb-1">Server URL</label>
                <input
                  type="url"
                  value={config.server_url || ''}
                  onChange={(e) => setConfig({ ...config, server_url: e.target.value })}
                  placeholder="https://ntfy.sh"
                  className="w-full bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2"
                />
              </div>
              <div>
                <label className="block text-sm text-[var(--text-tertiary)] mb-1">Topic</label>
                <input
                  type="text"
                  value={config.topic || ''}
                  onChange={(e) => setConfig({ ...config, topic: e.target.value })}
                  required
                  className="w-full bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2"
                />
              </div>
              <div>
                <label className="block text-sm text-[var(--text-tertiary)] mb-1">Token (optional)</label>
                <input
                  type="password"
                  value={config.token || ''}
                  onChange={(e) => setConfig({ ...config, token: e.target.value })}
                  className="w-full bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2"
                />
              </div>
            </>
          )}

          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="enabled"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
            />
            <label htmlFor="enabled" className="text-sm">Enabled</label>
          </div>

          <div className="flex justify-end gap-2 pt-4">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm border border-[var(--border)] rounded hover:bg-[var(--bg-tertiary)]"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading}
              className="px-4 py-2 text-sm bg-[var(--accent)] text-white rounded hover:opacity-80 disabled:opacity-50"
            >
              {loading ? 'Saving...' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

export default function NotificationsPage() {
  const [activeTab, setActiveTab] = useState<Tab>('log');
  const [logs, setLogs] = useState<NotificationLog[]>([]);
  const [channels, setChannels] = useState<NotificationChannel[]>([]);
  const [rules, setRules] = useState<NotificationRule[]>([]);
  const [silences, setSilences] = useState<NotificationSilence[]>([]);
  const [status, setStatus] = useState<{ unread_count: number } | null>(null);
  const [loading, setLoading] = useState(true);
  const [channelModalOpen, setChannelModalOpen] = useState(false);
  const [editingChannel, setEditingChannel] = useState<NotificationChannel | null>(null);

  const loadData = async () => {
    try {
      const [logsData, channelsData, rulesData, silencesData, statusData] = await Promise.all([
        getNotificationLog(100),
        getNotificationChannels(),
        getNotificationRules(),
        getNotificationSilences(),
        getNotificationStatus(),
      ]);
      setLogs(logsData);
      setChannels(channelsData);
      setRules(rulesData);
      setSilences(silencesData);
      setStatus(statusData);
    } catch (error) {
      console.error('Failed to load notifications:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
    const interval = setInterval(loadData, 30000);
    return () => clearInterval(interval);
  }, []);

  const handleMarkRead = async (id: number) => {
    await markNotificationRead(id);
    await loadData();
  };

  const handleMarkAllRead = async () => {
    await markAllNotificationsRead();
    await loadData();
  };

  const handleClearOld = async () => {
    await clearOldNotifications();
    await loadData();
  };

  const handleSaveChannel = async (channelData: Partial<NotificationChannel>) => {
    if (editingChannel) {
      await updateNotificationChannel(editingChannel.id, channelData);
    } else {
      await createNotificationChannel(channelData);
    }
    await loadData();
  };

  const handleDeleteChannel = async (id: number) => {
    if (confirm('Are you sure you want to delete this channel?')) {
      await deleteNotificationChannel(id);
      await loadData();
    }
  };

  const handleTestChannel = async (id: number) => {
    try {
      const result = await testNotificationChannel(id);
      if (result.success) {
        alert('Test notification sent successfully!');
      } else {
        alert(`Test failed: ${result.error}`);
      }
    } catch (error) {
      alert(`Test failed: ${error}`);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-[var(--text-tertiary)]">Loading...</div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Notifications</h1>
        <div className="flex items-center gap-2 text-sm text-[var(--text-tertiary)]">
          {status && status.unread_count > 0 && (
            <span className="px-2 py-1 bg-[var(--accent)] text-white rounded">
              {status.unread_count} unread
            </span>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-2 border-b border-[var(--border)]">
        {(['log', 'rules', 'channels', 'silences'] as Tab[]).map(tab => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-4 py-2 text-sm capitalize ${
              activeTab === tab
                ? 'text-[var(--accent)] border-b-2 border-[var(--accent)]'
                : 'text-[var(--text-tertiary)] hover:text-[var(--text-primary)]'
            }`}
          >
            {tab === 'log' ? 'Activity Log' : tab}
            {tab === 'log' && status && status.unread_count > 0 && (
              <span className="ml-2 px-1.5 py-0.5 text-xs bg-[var(--accent)] text-white rounded-full">
                {status.unread_count}
              </span>
            )}
          </button>
        ))}
      </div>

      {/* Content */}
      {activeTab === 'log' && (
        <div className="space-y-4">
          <div className="flex gap-2">
            <button
              onClick={handleMarkAllRead}
              className="px-3 py-1.5 text-sm border border-[var(--border)] rounded hover:bg-[var(--bg-tertiary)]"
            >
              Mark All Read
            </button>
            <button
              onClick={handleClearOld}
              className="px-3 py-1.5 text-sm border border-[var(--border)] rounded hover:bg-[var(--bg-tertiary)]"
            >
              Clear Old
            </button>
          </div>
          <NotificationLogList logs={logs} onMarkRead={handleMarkRead} />
        </div>
      )}

      {activeTab === 'channels' && (
        <div className="space-y-4">
          <button
            onClick={() => { setEditingChannel(null); setChannelModalOpen(true); }}
            className="px-4 py-2 text-sm bg-[var(--accent)] text-white rounded hover:opacity-80"
          >
            + Add Channel
          </button>

          {channels.length === 0 ? (
            <div className="text-center py-8 text-[var(--text-tertiary)]">
              No notification channels configured
            </div>
          ) : (
            <div className="space-y-2">
              {channels.map(channel => (
                <div key={channel.id} className="bg-[var(--bg-tertiary)] rounded-lg p-4 flex items-center justify-between">
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="font-medium">{channel.name}</span>
                      <span className="px-2 py-0.5 text-xs bg-[var(--bg-secondary)] rounded">{channel.type}</span>
                      {!channel.enabled && (
                        <span className="px-2 py-0.5 text-xs bg-[var(--warning)] text-white rounded">Disabled</span>
                      )}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => handleTestChannel(channel.id)}
                      className="px-2 py-1 text-xs border border-[var(--border)] rounded hover:bg-[var(--bg-secondary)]"
                    >
                      Test
                    </button>
                    <button
                      onClick={() => { setEditingChannel(channel); setChannelModalOpen(true); }}
                      className="p-1 hover:bg-[var(--bg-secondary)] rounded"
                    >
                      ✏️
                    </button>
                    <button
                      onClick={() => handleDeleteChannel(channel.id)}
                      className="p-1 hover:bg-[var(--danger)] hover:text-white rounded"
                    >
                      🗑️
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {activeTab === 'rules' && (
        <div className="space-y-4">
          {rules.length === 0 ? (
            <div className="text-center py-8 text-[var(--text-tertiary)]">
              No notification rules configured
            </div>
          ) : (
            <div className="space-y-2">
              {rules.map(rule => (
                <div key={rule.id} className="bg-[var(--bg-tertiary)] rounded-lg p-4">
                  <div className="flex items-start justify-between">
                    <div>
                      <div className="flex items-center gap-2 mb-1">
                        <span className="font-medium">{rule.name}</span>
                        {!rule.enabled && (
                          <span className="px-2 py-0.5 text-xs bg-[var(--warning)] text-white rounded">Disabled</span>
                        )}
                      </div>
                      <div className="flex flex-wrap gap-1">
                        {rule.event_types.map(et => (
                          <EventTypeBadge key={et} type={et} />
                        ))}
                      </div>
                      {rule.container_pattern && (
                        <div className="text-xs text-[var(--text-tertiary)] mt-1">
                          Container: {rule.container_pattern}
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {activeTab === 'silences' && (
        <div className="space-y-4">
          {silences.length === 0 ? (
            <div className="text-center py-8 text-[var(--text-tertiary)]">
              No active silences
            </div>
          ) : (
            <div className="space-y-2">
              {silences.map(silence => (
                <div key={silence.id} className="bg-[var(--bg-tertiary)] rounded-lg p-4 flex items-center justify-between">
                  <div>
                    <div className="font-medium">{silence.container_pattern || silence.container_name || 'All'}</div>
                    <div className="text-xs text-[var(--text-tertiary)]">
                      Expires: {silence.expires_at ? formatDate(silence.expires_at) : 'Never'}
                    </div>
                  </div>
                  <button
                    onClick={() => deleteNotificationSilence(silence.id).then(loadData)}
                    className="p-1 hover:bg-[var(--danger)] hover:text-white rounded"
                  >
                    🗑️
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Channel Form Modal */}
      <ChannelFormModal
        isOpen={channelModalOpen}
        onClose={() => { setChannelModalOpen(false); setEditingChannel(null); }}
        onSubmit={handleSaveChannel}
        editChannel={editingChannel}
      />
    </div>
  );
}
