'use client';

import { useEffect, useState } from 'react';
import { getHosts, createHost, updateHost, deleteHost, scanHost } from '@/lib/api';
import type { Host } from '@/types';

function formatDate(dateStr: string | null | undefined): string {
  if (!dateStr) return '-';
  const date = new Date(dateStr);
  return date.toLocaleString();
}

function HostTypeIcon({ type }: { type: string }) {
  const icons: Record<string, string> = {
    agent: '🤖',
    unix: '🐳',
    tcp: '🌐',
    ssh: '🔐',
  };
  return <span title={type}>{icons[type] || '❓'}</span>;
}

function StatusBadge({ host }: { host: Host }) {
  if (!host.enabled) {
    return <span className="px-2 py-1 text-xs rounded bg-[var(--bg-tertiary)] text-[var(--text-tertiary)]">Disabled</span>;
  }
  if (host.host_type === 'agent') {
    if (host.agent_status === 'online') {
      return <span className="px-2 py-1 text-xs rounded bg-[var(--success)] text-white">Online</span>;
    }
    if (host.agent_status === 'auth_failed') {
      return <span className="px-2 py-1 text-xs rounded bg-[var(--danger)] text-white" title="API token mismatch">Auth Failed</span>;
    }
    return <span className="px-2 py-1 text-xs rounded bg-[var(--warning)] text-white">Offline</span>;
  }
  return <span className="px-2 py-1 text-xs rounded bg-[var(--success)] text-white">Enabled</span>;
}

interface AddHostModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (host: Partial<Host>) => Promise<void>;
  editHost?: Host | null;
}

function AddHostModal({ isOpen, onClose, onSubmit, editHost }: AddHostModalProps) {
  const [name, setName] = useState('');
  const [address, setAddress] = useState('');
  const [hostType, setHostType] = useState('agent');
  const [description, setDescription] = useState('');
  const [apiToken, setApiToken] = useState('');
  const [collectStats, setCollectStats] = useState(true);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (editHost) {
      setName(editHost.name);
      setAddress(editHost.address);
      setHostType(editHost.host_type || 'agent');
      setDescription(editHost.description || '');
      setApiToken(editHost.api_token || '');
      setCollectStats(editHost.collect_stats ?? true);
    } else {
      setName('');
      setAddress('');
      setHostType('agent');
      setDescription('');
      setApiToken('');
      setCollectStats(true);
    }
  }, [editHost, isOpen]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await onSubmit({
        name,
        address,
        host_type: hostType,
        description,
        api_token: apiToken || undefined,
        collect_stats: collectStats,
        enabled: true,
      });
      onClose();
    } catch (error) {
      console.error('Failed to save host:', error);
    } finally {
      setLoading(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg w-full max-w-md p-6">
        <h2 className="text-xl font-bold mb-4">{editHost ? 'Edit Host' : 'Add Host'}</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm text-[var(--text-tertiary)] mb-1">Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              className="w-full bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2 text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
            />
          </div>

          <div>
            <label className="block text-sm text-[var(--text-tertiary)] mb-1">Type</label>
            <select
              value={hostType}
              onChange={(e) => setHostType(e.target.value)}
              className="w-full bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2 text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
            >
              <option value="agent">Agent (Recommended)</option>
              <option value="unix">Unix Socket</option>
              <option value="tcp">TCP</option>
              <option value="ssh">SSH</option>
            </select>
          </div>

          <div>
            <label className="block text-sm text-[var(--text-tertiary)] mb-1">Address</label>
            <input
              type="text"
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              required
              placeholder={hostType === 'agent' ? 'http://hostname:9876' : hostType === 'unix' ? '/var/run/docker.sock' : 'tcp://hostname:2375'}
              className="w-full bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2 text-[var(--text-primary)] placeholder-[var(--text-tertiary)] focus:outline-none focus:border-[var(--accent)]"
            />
          </div>

          {hostType === 'agent' && (
            <div>
              <label className="block text-sm text-[var(--text-tertiary)] mb-1">API Token</label>
              <input
                type="password"
                value={apiToken}
                onChange={(e) => setApiToken(e.target.value)}
                placeholder="Agent API token"
                className="w-full bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2 text-[var(--text-primary)] placeholder-[var(--text-tertiary)] focus:outline-none focus:border-[var(--accent)]"
              />
            </div>
          )}

          <div>
            <label className="block text-sm text-[var(--text-tertiary)] mb-1">Description</label>
            <input
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional description"
              className="w-full bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2 text-[var(--text-primary)] placeholder-[var(--text-tertiary)] focus:outline-none focus:border-[var(--accent)]"
            />
          </div>

          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="collectStats"
              checked={collectStats}
              onChange={(e) => setCollectStats(e.target.checked)}
              className="rounded"
            />
            <label htmlFor="collectStats" className="text-sm">Collect CPU/Memory stats</label>
          </div>

          <div className="flex justify-end gap-2 pt-4">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm border border-[var(--border)] rounded hover:bg-[var(--bg-tertiary)] transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading}
              className="px-4 py-2 text-sm bg-[var(--accent)] text-white rounded hover:opacity-80 transition-opacity disabled:opacity-50"
            >
              {loading ? 'Saving...' : editHost ? 'Update' : 'Add Host'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

export default function HostsPage() {
  const [hosts, setHosts] = useState<Host[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingHost, setEditingHost] = useState<Host | null>(null);
  const [actionLoading, setActionLoading] = useState<number | null>(null);

  const loadData = async () => {
    try {
      const hostsData = await getHosts();
      setHosts(hostsData);
    } catch (error) {
      console.error('Failed to load hosts:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
    const interval = setInterval(loadData, 30000);
    return () => clearInterval(interval);
  }, []);

  const handleAddHost = async (hostData: Partial<Host>) => {
    await createHost(hostData);
    await loadData();
  };

  const handleUpdateHost = async (hostData: Partial<Host>) => {
    if (editingHost) {
      await updateHost(editingHost.id, { ...editingHost, ...hostData });
      await loadData();
    }
  };

  const handleToggleHost = async (host: Host) => {
    setActionLoading(host.id);
    try {
      await updateHost(host.id, { ...host, enabled: !host.enabled });
      await loadData();
    } finally {
      setActionLoading(null);
    }
  };

  const handleToggleStats = async (host: Host) => {
    setActionLoading(host.id);
    try {
      await updateHost(host.id, { ...host, collect_stats: !host.collect_stats });
      await loadData();
    } finally {
      setActionLoading(null);
    }
  };

  const handleDeleteHost = async (host: Host) => {
    if (!confirm(`Are you sure you want to delete "${host.name}"?\n\nThis will remove all associated container history.`)) {
      return;
    }
    setActionLoading(host.id);
    try {
      await deleteHost(host.id);
      await loadData();
    } finally {
      setActionLoading(null);
    }
  };

  const handleScanHost = async (host: Host) => {
    setActionLoading(host.id);
    try {
      await scanHost(host.id);
      await loadData();
    } finally {
      setActionLoading(null);
    }
  };

  const openEditModal = (host: Host) => {
    setEditingHost(host);
    setModalOpen(true);
  };

  const closeModal = () => {
    setModalOpen(false);
    setEditingHost(null);
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
        <h1 className="text-2xl font-bold">Hosts</h1>
        <button
          onClick={() => { setEditingHost(null); setModalOpen(true); }}
          className="px-4 py-2 bg-[var(--accent)] text-white rounded hover:opacity-80 transition-opacity"
        >
          + Add Host
        </button>
      </div>

      {hosts.length === 0 ? (
        <div className="text-center py-12 text-[var(--text-tertiary)]">
          No hosts configured. Add a host to start monitoring containers.
        </div>
      ) : (
        <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg overflow-hidden">
          <table className="w-full">
            <thead>
              <tr className="bg-[var(--bg-tertiary)]">
                <th className="text-left px-4 py-3 text-sm font-medium">Name</th>
                <th className="text-left px-4 py-3 text-sm font-medium">Type</th>
                <th className="text-left px-4 py-3 text-sm font-medium">Address</th>
                <th className="text-left px-4 py-3 text-sm font-medium">Status</th>
                <th className="text-left px-4 py-3 text-sm font-medium">Stats</th>
                <th className="text-left px-4 py-3 text-sm font-medium">Containers</th>
                <th className="text-left px-4 py-3 text-sm font-medium">Last Seen</th>
                <th className="text-left px-4 py-3 text-sm font-medium">Actions</th>
              </tr>
            </thead>
            <tbody>
              {hosts.map(host => (
                <tr key={host.id} className="border-t border-[var(--border)] hover:bg-[var(--bg-tertiary)]">
                  <td className="px-4 py-3">
                    <div>
                      <span className="font-medium">{host.name}</span>
                      {host.description && (
                        <div className="text-xs text-[var(--text-tertiary)]">{host.description}</div>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <HostTypeIcon type={host.host_type || 'unknown'} />
                      <span className="text-sm">{host.host_type}</span>
                      {host.host_type === 'agent' && host.agent_version && (
                        <span className="text-xs bg-[var(--accent)] text-white px-1.5 py-0.5 rounded">
                          v{host.agent_version}
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <code className="text-sm">{host.address}</code>
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge host={host} />
                  </td>
                  <td className="px-4 py-3">
                    <button
                      onClick={() => handleToggleStats(host)}
                      disabled={actionLoading === host.id}
                      className={`px-2 py-1 text-xs rounded ${
                        host.collect_stats
                          ? 'bg-[var(--success)] text-white'
                          : 'bg-[var(--bg-tertiary)] text-[var(--text-tertiary)]'
                      }`}
                      title={host.collect_stats ? 'Click to disable' : 'Click to enable'}
                    >
                      {host.collect_stats ? '✓ Enabled' : 'Disabled'}
                    </button>
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-sm">
                      {host.running_count ?? 0}/{host.container_count ?? 0}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-sm text-[var(--text-tertiary)]">
                    {formatDate(host.last_seen)}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1">
                      <button
                        onClick={() => handleScanHost(host)}
                        disabled={actionLoading === host.id || !host.enabled}
                        className="p-1.5 rounded hover:bg-[var(--bg-tertiary)] transition-colors disabled:opacity-50"
                        title="Scan now"
                      >
                        🔍
                      </button>
                      <button
                        onClick={() => openEditModal(host)}
                        disabled={actionLoading === host.id}
                        className="p-1.5 rounded hover:bg-[var(--bg-tertiary)] transition-colors disabled:opacity-50"
                        title="Edit"
                      >
                        ✏️
                      </button>
                      <button
                        onClick={() => handleToggleHost(host)}
                        disabled={actionLoading === host.id}
                        className="p-1.5 rounded hover:bg-[var(--bg-tertiary)] transition-colors disabled:opacity-50"
                        title={host.enabled ? 'Disable' : 'Enable'}
                      >
                        {host.enabled ? '⏸️' : '▶️'}
                      </button>
                      <button
                        onClick={() => handleDeleteHost(host)}
                        disabled={actionLoading === host.id}
                        className="p-1.5 rounded hover:bg-[var(--danger)] hover:text-white transition-colors disabled:opacity-50"
                        title="Delete"
                      >
                        🗑️
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <AddHostModal
        isOpen={modalOpen}
        onClose={closeModal}
        onSubmit={editingHost ? handleUpdateHost : handleAddHost}
        editHost={editingHost}
      />
    </div>
  );
}
