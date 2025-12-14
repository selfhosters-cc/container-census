'use client';

import { useEffect, useState, useMemo } from 'react';
import { getContainers, getHosts, getContainerStats } from '@/lib/api';
import type { Container, Host, ContainerStatsPoint } from '@/types';

function formatMemory(bytes: number): string {
  if (bytes === 0) return '0 MB';
  const mb = bytes / 1024 / 1024;
  if (mb >= 1024) {
    return `${(mb / 1024).toFixed(1)} GB`;
  }
  return `${mb.toFixed(0)} MB`;
}

interface StatsModalProps {
  isOpen: boolean;
  onClose: () => void;
  container: Container;
}

function StatsModal({ isOpen, onClose, container }: StatsModalProps) {
  const [stats, setStats] = useState<ContainerStatsPoint[]>([]);
  const [range, setRange] = useState('1h');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (isOpen) {
      setLoading(true);
      getContainerStats(container.host_id, container.id, range)
        .then(setStats)
        .catch(console.error)
        .finally(() => setLoading(false));
    }
  }, [isOpen, container.host_id, container.id, range]);

  if (!isOpen) return null;

  const maxCpu = Math.max(...stats.map(s => s.cpu_percent || 0), 1);
  const maxMem = Math.max(...stats.map(s => s.memory_usage || 0), 1);

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg w-full max-w-4xl max-h-[80vh] flex flex-col">
        <div className="p-4 border-b border-[var(--border)] flex justify-between items-center">
          <div>
            <h2 className="text-xl font-bold">{container.name}</h2>
            <div className="text-sm text-[var(--text-tertiary)]">Resource Usage History</div>
          </div>
          <button onClick={onClose} className="text-2xl hover:opacity-70">×</button>
        </div>

        {/* Time Range Selector */}
        <div className="p-4 border-b border-[var(--border)] flex gap-2">
          {['1h', '24h', '7d', 'all'].map(r => (
            <button
              key={r}
              onClick={() => setRange(r)}
              className={`px-3 py-1.5 text-sm rounded ${
                range === r
                  ? 'bg-[var(--accent)] text-white'
                  : 'bg-[var(--bg-tertiary)] hover:bg-[var(--border)]'
              }`}
            >
              {r === '1h' ? '1 Hour' : r === '24h' ? '24 Hours' : r === '7d' ? '7 Days' : 'All'}
            </button>
          ))}
        </div>

        {/* Charts */}
        <div className="flex-1 overflow-auto p-4 space-y-6">
          {loading ? (
            <div className="flex items-center justify-center h-64">
              <div className="text-[var(--text-tertiary)]">Loading...</div>
            </div>
          ) : stats.length === 0 ? (
            <div className="flex items-center justify-center h-64">
              <div className="text-[var(--text-tertiary)]">No data available for this time range</div>
            </div>
          ) : (
            <>
              {/* CPU Chart */}
              <div>
                <h3 className="text-sm font-medium mb-2">CPU Usage</h3>
                <div className="bg-[var(--bg-tertiary)] rounded-lg p-4 h-32 flex items-end gap-px">
                  {stats.map((s, idx) => (
                    <div
                      key={idx}
                      className="flex-1 bg-[var(--accent)] rounded-t transition-all"
                      style={{ height: `${((s.cpu_percent || 0) / maxCpu) * 100}%` }}
                      title={`${(s.cpu_percent || 0).toFixed(1)}% at ${new Date(s.timestamp).toLocaleString()}`}
                    />
                  ))}
                </div>
                <div className="flex justify-between text-xs text-[var(--text-tertiary)] mt-1">
                  <span>{stats.length > 0 ? new Date(stats[0].timestamp).toLocaleString() : ''}</span>
                  <span>{stats.length > 0 ? new Date(stats[stats.length - 1].timestamp).toLocaleString() : ''}</span>
                </div>
              </div>

              {/* Memory Chart */}
              <div>
                <h3 className="text-sm font-medium mb-2">Memory Usage</h3>
                <div className="bg-[var(--bg-tertiary)] rounded-lg p-4 h-32 flex items-end gap-px">
                  {stats.map((s, idx) => (
                    <div
                      key={idx}
                      className="flex-1 bg-[var(--warning)] rounded-t transition-all"
                      style={{ height: `${((s.memory_usage || 0) / maxMem) * 100}%` }}
                      title={`${formatMemory(s.memory_usage || 0)} at ${new Date(s.timestamp).toLocaleString()}`}
                    />
                  ))}
                </div>
                <div className="flex justify-between text-xs text-[var(--text-tertiary)] mt-1">
                  <span>{stats.length > 0 ? new Date(stats[0].timestamp).toLocaleString() : ''}</span>
                  <span>{stats.length > 0 ? new Date(stats[stats.length - 1].timestamp).toLocaleString() : ''}</span>
                </div>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

interface MonitoringCardProps {
  container: Container;
  onViewStats: () => void;
}

function MonitoringCard({ container, onViewStats }: MonitoringCardProps) {
  const hasStats = (container.memory_limit ?? 0) > 0;
  const cpuDisplay = hasStats ? `${(container.cpu_percent ?? 0).toFixed(1)}%` : '-';
  const memoryMB = hasStats ? formatMemory(container.memory_usage ?? 0) : '-';
  const limitMB = hasStats ? formatMemory(container.memory_limit ?? 0) : '?';
  const memoryPercent = hasStats ? `${(container.memory_percent ?? 0).toFixed(1)}%` : '-';

  return (
    <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg overflow-hidden">
      {/* Header */}
      <div className="p-4 border-b border-[var(--border)]">
        <div className="font-semibold">{container.name}</div>
        <div className="text-sm text-[var(--text-tertiary)]">📍 {container.host_name}</div>
        <div className="text-sm text-[var(--text-tertiary)] truncate" title={container.image}>🖼️ {container.image}</div>
      </div>

      {/* Stats */}
      <div className="p-4 grid grid-cols-2 gap-4">
        <div>
          <div className="text-xs text-[var(--text-tertiary)]">CPU Usage</div>
          <div className="text-xl font-bold">{cpuDisplay}</div>
          {hasStats && (
            <div className="h-1.5 bg-[var(--bg-tertiary)] rounded-full mt-2 overflow-hidden">
              <div
                className="h-full bg-[var(--accent)] rounded-full transition-all"
                style={{ width: `${Math.min(container.cpu_percent ?? 0, 100)}%` }}
              />
            </div>
          )}
        </div>
        <div>
          <div className="text-xs text-[var(--text-tertiary)]">Memory</div>
          <div className="text-xl font-bold">{memoryMB}</div>
          <div className="text-xs text-[var(--text-tertiary)]">of {limitMB} ({memoryPercent})</div>
          {hasStats && (
            <div className="h-1.5 bg-[var(--bg-tertiary)] rounded-full mt-2 overflow-hidden">
              <div
                className="h-full bg-[var(--warning)] rounded-full transition-all"
                style={{ width: `${Math.min(container.memory_percent ?? 0, 100)}%` }}
              />
            </div>
          )}
        </div>
      </div>

      {/* Actions */}
      <div className="p-4 border-t border-[var(--border)]">
        {hasStats ? (
          <button
            onClick={onViewStats}
            className="w-full px-4 py-2 text-sm bg-[var(--accent)] text-white rounded hover:opacity-80 transition-opacity"
          >
            📊 View Detailed Stats
          </button>
        ) : (
          <button
            disabled
            className="w-full px-4 py-2 text-sm bg-[var(--bg-tertiary)] text-[var(--text-tertiary)] rounded cursor-not-allowed"
            title="Stats collection not enabled or no data yet"
          >
            📊 No Stats Available
          </button>
        )}
      </div>
    </div>
  );
}

export default function MonitoringPage() {
  const [containers, setContainers] = useState<Container[]>([]);
  const [hosts, setHosts] = useState<Host[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState('');
  const [hostFilter, setHostFilter] = useState('');
  const [selectedContainer, setSelectedContainer] = useState<Container | null>(null);

  const loadData = async () => {
    try {
      const [containersData, hostsData] = await Promise.all([
        getContainers(),
        getHosts(),
      ]);
      setContainers(containersData);
      setHosts(hostsData);
    } catch (error) {
      console.error('Failed to load data:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
    const interval = setInterval(loadData, 30000);
    return () => clearInterval(interval);
  }, []);

  const runningContainers = useMemo(() => {
    return containers
      .filter(c => c.state === 'running')
      .filter(c => {
        const matchesSearch = searchTerm === '' ||
          c.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
          c.image.toLowerCase().includes(searchTerm.toLowerCase()) ||
          c.host_name.toLowerCase().includes(searchTerm.toLowerCase());

        const matchesHost = hostFilter === '' || c.host_id.toString() === hostFilter;

        return matchesSearch && matchesHost;
      });
  }, [containers, searchTerm, hostFilter]);

  const stats = useMemo(() => ({
    total: runningContainers.length,
    withStats: runningContainers.filter(c => (c.memory_limit ?? 0) > 0).length,
    avgCpu: runningContainers.length > 0
      ? runningContainers.reduce((sum, c) => sum + (c.cpu_percent ?? 0), 0) / runningContainers.length
      : 0,
    avgMemory: runningContainers.length > 0
      ? runningContainers.reduce((sum, c) => sum + (c.memory_percent ?? 0), 0) / runningContainers.length
      : 0,
  }), [runningContainers]);

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
        <h1 className="text-2xl font-bold">Monitoring</h1>
        <div className="flex items-center gap-4 text-sm text-[var(--text-tertiary)]">
          <span>Running: {stats.total}</span>
          <span>With Stats: {stats.withStats}</span>
          <span>Avg CPU: {stats.avgCpu.toFixed(1)}%</span>
          <span>Avg Memory: {stats.avgMemory.toFixed(1)}%</span>
        </div>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-4">
        <input
          type="text"
          placeholder="Search containers..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          className="flex-1 min-w-[200px] bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg px-4 py-2 text-[var(--text-primary)] placeholder-[var(--text-tertiary)] focus:outline-none focus:border-[var(--accent)]"
        />
        <select
          value={hostFilter}
          onChange={(e) => setHostFilter(e.target.value)}
          className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg px-4 py-2 text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
        >
          <option value="">All Hosts</option>
          {hosts.map(host => (
            <option key={host.id} value={host.id.toString()}>{host.name}</option>
          ))}
        </select>
      </div>

      {/* Monitoring Grid */}
      {runningContainers.length === 0 ? (
        <div className="text-center py-12 text-[var(--text-tertiary)]">
          No running containers found
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {runningContainers.map(container => (
            <MonitoringCard
              key={`${container.host_id}-${container.id}`}
              container={container}
              onViewStats={() => setSelectedContainer(container)}
            />
          ))}
        </div>
      )}

      {/* Stats Modal */}
      {selectedContainer && (
        <StatsModal
          isOpen={!!selectedContainer}
          onClose={() => setSelectedContainer(null)}
          container={selectedContainer}
        />
      )}
    </div>
  );
}
