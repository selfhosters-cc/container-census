'use client';

import { useEffect, useState } from 'react';
import { getPlugins, getPluginTabs, enablePlugin, disablePlugin } from '@/lib/api';
import type { PluginInfo, PluginTab } from '@/types';

export default function IntegrationsPage() {
  const [plugins, setPlugins] = useState<PluginInfo[]>([]);
  const [tabs, setTabs] = useState<PluginTab[]>([]);
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [pluginRepoUrl, setPluginRepoUrl] = useState('');
  const [installingPlugin, setInstallingPlugin] = useState(false);
  const [pluginError, setPluginError] = useState<string | null>(null);

  const loadData = async () => {
    try {
      const [pluginsData, tabsData] = await Promise.all([
        getPlugins().catch(() => []),
        getPluginTabs().catch(() => []),
      ]);
      setPlugins(pluginsData);
      setTabs(tabsData);
    } catch (error) {
      console.error('Failed to load plugins:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const handleTogglePlugin = async (pluginId: string, enable: boolean) => {
    setActionLoading(pluginId);
    try {
      if (enable) {
        await enablePlugin(pluginId);
      } else {
        await disablePlugin(pluginId);
      }
      await loadData();
    } catch (error) {
      console.error('Failed to toggle plugin:', error);
    } finally {
      setActionLoading(null);
    }
  };

  const installPlugin = async () => {
    if (!pluginRepoUrl.trim()) {
      setPluginError('Please enter a GitHub repository URL');
      return;
    }

    setInstallingPlugin(true);
    setPluginError(null);

    try {
      const response = await fetch('/api/plugins/install', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ repository_url: pluginRepoUrl }),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Installation failed');
      }

      setPluginRepoUrl('');
      await loadData();
    } catch (error) {
      setPluginError(error instanceof Error ? error.message : 'Installation failed');
    } finally {
      setInstallingPlugin(false);
    }
  };

  const uninstallPlugin = async (pluginId: string, pluginName: string) => {
    if (!confirm(`Are you sure you want to uninstall ${pluginName}?`)) {
      return;
    }

    setActionLoading(pluginId);
    try {
      const response = await fetch(`/api/plugins/${pluginId}`, {
        method: 'DELETE',
        credentials: 'include',
      });

      if (response.ok) {
        await loadData();
      }
    } catch (error) {
      console.error('Failed to uninstall plugin:', error);
    } finally {
      setActionLoading(null);
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
        <h1 className="text-2xl font-bold">Integrations</h1>
      </div>

      {/* Install Plugin Section */}
      <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4">
        <h2 className="text-lg font-semibold mb-3">🔌 Install External Plugin</h2>
        <p className="text-sm text-[var(--text-secondary)] mb-3">
          Install plugins from GitHub to extend Container Census functionality. Plugins can add custom tabs, badges, API routes, and more.
        </p>
        <div className="flex gap-2 mb-2">
          <input
            type="text"
            value={pluginRepoUrl}
            onChange={(e) => setPluginRepoUrl(e.target.value)}
            placeholder="https://github.com/selfhosters-cc/cc-graph"
            className="flex-1 px-3 py-2 text-sm bg-[var(--bg-tertiary)] border border-[var(--border)] rounded focus:outline-none focus:ring-2 focus:ring-[var(--accent)]"
            disabled={installingPlugin}
          />
          <button
            onClick={installPlugin}
            disabled={installingPlugin}
            className="px-4 py-2 text-sm bg-[var(--accent)] text-white rounded hover:bg-[var(--accent-hover)] transition-colors disabled:opacity-50"
          >
            {installingPlugin ? 'Installing...' : 'Install Plugin'}
          </button>
        </div>
        {pluginError && (
          <div className="text-sm text-[var(--danger)] mt-2">
            {pluginError}
          </div>
        )}
        <div className="text-xs text-[var(--text-secondary)] mt-2">
          Example: <code className="px-1 py-0.5 bg-[var(--bg-tertiary)] rounded">https://github.com/selfhosters-cc/cc-graph</code>
        </div>
      </div>

      {plugins.length === 0 ? (
        <div className="text-center py-12 text-[var(--text-tertiary)]">
          <div className="text-4xl mb-4">🔌</div>
          <div>No plugins installed</div>
          <div className="text-sm mt-2">Plugins extend Container Census with additional features</div>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {plugins.map(plugin => (
            <div
              key={plugin.id}
              className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4"
            >
              <div className="flex items-start justify-between">
                <div className="flex-1">
                  <div className="flex items-center gap-2 mb-2">
                    <span className="text-2xl">{tabs.find(t => t.id === plugin.id)?.icon || '🔌'}</span>
                    <h3 className="font-semibold">{plugin.name}</h3>
                    <span className="px-2 py-0.5 text-xs bg-[var(--bg-tertiary)] rounded">v{plugin.version}</span>
                    {plugin.built_in && (
                      <span className="px-2 py-0.5 text-xs bg-[var(--accent)] text-white rounded">Built-in</span>
                    )}
                  </div>
                  <p className="text-sm text-[var(--text-tertiary)] mb-3">{plugin.description}</p>
                  {plugin.capabilities?.length ? (
                    <div className="flex flex-wrap gap-1 mb-3">
                      {plugin.capabilities.map(cap => (
                        <span key={cap} className="px-2 py-0.5 text-xs bg-[var(--bg-tertiary)] rounded">
                          {cap.replace(/_/g, ' ')}
                        </span>
                      ))}
                    </div>
                  ) : null}
                  {plugin.author && (
                    <div className="text-xs text-[var(--text-tertiary)]">
                      By {plugin.author}
                      {plugin.homepage && (
                        <>
                          {' • '}
                          <a href={plugin.homepage} target="_blank" rel="noopener noreferrer" className="text-[var(--accent)]">
                            Homepage
                          </a>
                        </>
                      )}
                    </div>
                  )}
                </div>
                <div className="flex flex-col gap-2">
                  <button
                    onClick={() => handleTogglePlugin(plugin.id, !plugin.enabled)}
                    disabled={actionLoading === plugin.id}
                    className={`px-3 py-1.5 text-sm rounded transition-colors disabled:opacity-50 ${
                      plugin.enabled
                        ? 'bg-[var(--success)] text-white hover:opacity-80'
                        : 'bg-[var(--bg-tertiary)] text-[var(--text-tertiary)] hover:bg-[var(--border)]'
                    }`}
                  >
                    {actionLoading === plugin.id ? '...' : plugin.enabled ? 'Enabled' : 'Disabled'}
                  </button>
                  {!plugin.built_in && (
                    <button
                      onClick={() => uninstallPlugin(plugin.id, plugin.name)}
                      disabled={actionLoading === plugin.id}
                      className="px-3 py-1.5 text-sm border border-[var(--danger)] text-[var(--danger)] rounded hover:bg-[var(--danger)] hover:text-white transition-colors disabled:opacity-50"
                    >
                      Uninstall
                    </button>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Plugin Tabs */}
      {tabs.length > 0 && (
        <div className="mt-8">
          <h2 className="text-lg font-semibold mb-4">Available Plugin Tabs</h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {tabs.map(tab => (
              <a
                key={tab.id}
                href={`/integrations/${tab.id}`}
                className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4 hover:border-[var(--accent)] transition-colors"
              >
                <div className="flex items-center gap-2">
                  <span className="text-2xl">{tab.icon || '🔌'}</span>
                  <span className="font-medium">{tab.label}</span>
                </div>
              </a>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
