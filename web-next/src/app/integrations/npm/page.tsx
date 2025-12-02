'use client';

import { useEffect, useState } from 'react';
import { fetchPluginApi } from '@/lib/api';

interface NpmInstance {
  id: number;
  name: string;
  url: string;
  email: string;
  enabled: boolean;
  last_sync: string | null;
  last_error: string | null;
  proxy_host_count: number;
}

interface ProxyHost {
  id: number;
  npm_instance_id: number;
  npm_host_id: number;
  domain_names: string[];
  forward_host: string;
  forward_port: number;
  ssl_enabled: boolean;
  enabled: boolean;
  container_name?: string;
  host_name?: string;
}

interface AddInstanceModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (data: { name: string; url: string; email: string; password: string }) => Promise<void>;
  editInstance?: NpmInstance | null;
}

function AddInstanceModal({ isOpen, onClose, onSubmit, editInstance }: AddInstanceModalProps) {
  const [name, setName] = useState('');
  const [url, setUrl] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (editInstance) {
      setName(editInstance.name);
      setUrl(editInstance.url);
      setEmail(editInstance.email);
      setPassword('');
    } else {
      setName('');
      setUrl('');
      setEmail('');
      setPassword('');
    }
  }, [editInstance, isOpen]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await onSubmit({ name, url, email, password });
      onClose();
    } catch (error) {
      console.error('Failed to save instance:', error);
    } finally {
      setLoading(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg w-full max-w-md p-6">
        <h2 className="text-xl font-bold mb-4">{editInstance ? 'Edit Instance' : 'Add NPM Instance'}</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm text-[var(--text-tertiary)] mb-1">Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              placeholder="My NPM Server"
              className="w-full bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2"
            />
          </div>

          <div>
            <label className="block text-sm text-[var(--text-tertiary)] mb-1">URL</label>
            <input
              type="url"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              required
              placeholder="http://npm.example.com:81"
              className="w-full bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2"
            />
          </div>

          <div>
            <label className="block text-sm text-[var(--text-tertiary)] mb-1">Email</label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              placeholder="admin@example.com"
              className="w-full bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2"
            />
          </div>

          <div>
            <label className="block text-sm text-[var(--text-tertiary)] mb-1">
              Password {editInstance && '(leave blank to keep current)'}
            </label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required={!editInstance}
              placeholder="••••••••"
              className="w-full bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2"
            />
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

export default function NpmPluginPage() {
  const [instances, setInstances] = useState<NpmInstance[]>([]);
  const [proxyHosts, setProxyHosts] = useState<ProxyHost[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingInstance, setEditingInstance] = useState<NpmInstance | null>(null);
  const [actionLoading, setActionLoading] = useState<number | null>(null);
  const [searchTerm, setSearchTerm] = useState('');

  const loadData = async () => {
    try {
      const [instancesData, hostsData] = await Promise.all([
        fetchPluginApi<NpmInstance[]>('npm', '/instances').catch(() => []),
        fetchPluginApi<ProxyHost[]>('npm', '/proxy-hosts').catch(() => []),
      ]);
      setInstances(instancesData);
      setProxyHosts(hostsData);
    } catch (error) {
      console.error('Failed to load NPM data:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
    const interval = setInterval(loadData, 60000);
    return () => clearInterval(interval);
  }, []);

  const handleAddInstance = async (data: { name: string; url: string; email: string; password: string }) => {
    await fetchPluginApi('npm', '/instances', {
      method: 'POST',
      body: JSON.stringify(data),
    });
    await loadData();
  };

  const handleUpdateInstance = async (data: { name: string; url: string; email: string; password: string }) => {
    if (!editingInstance) return;
    await fetchPluginApi('npm', `/instances/${editingInstance.id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
    await loadData();
  };

  const handleDeleteInstance = async (id: number) => {
    if (!confirm('Are you sure you want to delete this NPM instance?')) return;
    setActionLoading(id);
    try {
      await fetchPluginApi('npm', `/instances/${id}`, { method: 'DELETE' });
      await loadData();
    } finally {
      setActionLoading(null);
    }
  };

  const handleSyncInstance = async (id: number) => {
    setActionLoading(id);
    try {
      await fetchPluginApi('npm', `/instances/${id}/sync`, { method: 'POST' });
      await loadData();
    } finally {
      setActionLoading(null);
    }
  };

  const filteredProxyHosts = proxyHosts.filter(host => {
    if (searchTerm === '') return true;
    const search = searchTerm.toLowerCase();
    return (
      host.domain_names.some(d => d.toLowerCase().includes(search)) ||
      host.forward_host.toLowerCase().includes(search) ||
      (host.container_name || '').toLowerCase().includes(search)
    );
  });

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
        <div>
          <h1 className="text-2xl font-bold">🌐 Nginx Proxy Manager</h1>
          <p className="text-sm text-[var(--text-tertiary)]">
            Manage NPM instances and view exposed services
          </p>
        </div>
        <button
          onClick={() => { setEditingInstance(null); setModalOpen(true); }}
          className="px-4 py-2 bg-[var(--accent)] text-white rounded hover:opacity-80"
        >
          + Add Instance
        </button>
      </div>

      {/* Instances */}
      <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4">
        <h2 className="text-lg font-semibold mb-4">NPM Instances</h2>
        {instances.length === 0 ? (
          <div className="text-center py-8 text-[var(--text-tertiary)]">
            No NPM instances configured. Add one to get started.
          </div>
        ) : (
          <div className="space-y-2">
            {instances.map(instance => (
              <div
                key={instance.id}
                className="bg-[var(--bg-tertiary)] rounded-lg p-4 flex items-center justify-between"
              >
                <div className="flex-1">
                  <div className="flex items-center gap-2 mb-1">
                    <span className="font-medium">{instance.name}</span>
                    <span className={`px-2 py-0.5 text-xs rounded ${
                      instance.enabled ? 'bg-[var(--success)] text-white' : 'bg-[var(--bg-secondary)]'
                    }`}>
                      {instance.enabled ? 'Active' : 'Disabled'}
                    </span>
                    {instance.last_error && (
                      <span className="px-2 py-0.5 text-xs bg-[var(--danger)] text-white rounded" title={instance.last_error}>
                        Error
                      </span>
                    )}
                  </div>
                  <div className="text-sm text-[var(--text-tertiary)]">
                    <code>{instance.url}</code>
                    {' • '}
                    {instance.proxy_host_count} proxy hosts
                    {instance.last_sync && (
                      <>
                        {' • '}
                        Last sync: {new Date(instance.last_sync).toLocaleString()}
                      </>
                    )}
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => handleSyncInstance(instance.id)}
                    disabled={actionLoading === instance.id}
                    className="px-3 py-1.5 text-sm border border-[var(--border)] rounded hover:bg-[var(--bg-secondary)] disabled:opacity-50"
                  >
                    {actionLoading === instance.id ? '...' : '🔄 Sync'}
                  </button>
                  <button
                    onClick={() => { setEditingInstance(instance); setModalOpen(true); }}
                    className="p-1.5 hover:bg-[var(--bg-secondary)] rounded"
                  >
                    ✏️
                  </button>
                  <button
                    onClick={() => handleDeleteInstance(instance.id)}
                    disabled={actionLoading === instance.id}
                    className="p-1.5 hover:bg-[var(--danger)] hover:text-white rounded disabled:opacity-50"
                  >
                    🗑️
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Proxy Hosts */}
      {proxyHosts.length > 0 && (
        <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-semibold">Exposed Services ({proxyHosts.length})</h2>
            <input
              type="text"
              placeholder="Search domains, hosts..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-3 py-2 text-sm w-64"
            />
          </div>
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="text-left text-sm text-[var(--text-tertiary)]">
                  <th className="pb-2">Domain</th>
                  <th className="pb-2">Forward To</th>
                  <th className="pb-2">SSL</th>
                  <th className="pb-2">Container</th>
                  <th className="pb-2">Status</th>
                </tr>
              </thead>
              <tbody>
                {filteredProxyHosts.map(host => (
                  <tr key={`${host.npm_instance_id}-${host.npm_host_id}`} className="border-t border-[var(--border)]">
                    <td className="py-3">
                      <div className="space-y-1">
                        {host.domain_names.map(domain => (
                          <a
                            key={domain}
                            href={`https://${domain}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="block text-[var(--accent)] hover:underline"
                          >
                            {domain}
                          </a>
                        ))}
                      </div>
                    </td>
                    <td className="py-3">
                      <code className="text-sm">{host.forward_host}:{host.forward_port}</code>
                    </td>
                    <td className="py-3">
                      {host.ssl_enabled ? (
                        <span className="text-[var(--success)]">🔒</span>
                      ) : (
                        <span className="text-[var(--text-tertiary)]">—</span>
                      )}
                    </td>
                    <td className="py-3">
                      {host.container_name ? (
                        <span className="text-sm">{host.container_name}</span>
                      ) : (
                        <span className="text-sm text-[var(--text-tertiary)]">Not matched</span>
                      )}
                    </td>
                    <td className="py-3">
                      <span className={`px-2 py-0.5 text-xs rounded ${
                        host.enabled ? 'bg-[var(--success)] text-white' : 'bg-[var(--bg-tertiary)]'
                      }`}>
                        {host.enabled ? 'Active' : 'Disabled'}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Add/Edit Modal */}
      <AddInstanceModal
        isOpen={modalOpen}
        onClose={() => { setModalOpen(false); setEditingInstance(null); }}
        onSubmit={editingInstance ? handleUpdateInstance : handleAddInstance}
        editInstance={editingInstance}
      />
    </div>
  );
}
