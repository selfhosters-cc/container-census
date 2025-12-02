'use client';

import { useEffect, useState, useMemo } from 'react';
import { getContainers, getHosts, startContainer, stopContainer, restartContainer, removeContainer } from '@/lib/api';
import type { Container, Host } from '@/types';

function formatUptime(startedAt: string | undefined): string {
  if (!startedAt) return '';

  const now = new Date();
  const started = new Date(startedAt);

  if (isNaN(started.getTime()) || started.getFullYear() < 2000) {
    return '';
  }

  const diff = now.getTime() - started.getTime();
  if (diff < 0) return '';

  const seconds = Math.floor(diff / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (days > 0) {
    const remainingHours = hours % 24;
    return `${days}d ${remainingHours}h`;
  }
  if (hours > 0) {
    const remainingMins = minutes % 60;
    return `${hours}h ${remainingMins}m`;
  }
  if (minutes > 0) {
    return `${minutes}m`;
  }
  return `${seconds}s`;
}

function formatDate(dateStr: string): string {
  const date = new Date(dateStr);
  const now = new Date();
  const diff = now.getTime() - date.getTime();

  const seconds = Math.floor(diff / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (days > 7) {
    return date.toLocaleDateString();
  }
  if (days > 0) {
    return `${days}d ago`;
  }
  if (hours > 0) {
    return `${hours}h ago`;
  }
  if (minutes > 0) {
    return `${minutes}m ago`;
  }
  return 'Just now';
}

function extractImageTag(image: string, imageTags?: string[]): string {
  if (imageTags && imageTags.length > 0) {
    const tag = imageTags[0];
    const parts = tag.split(':');
    return parts[parts.length - 1] || 'latest';
  }
  const parts = image.split(':');
  return parts[parts.length - 1] || 'latest';
}

interface ContainerCardProps {
  container: Container;
  onAction: () => void;
}

function ContainerCard({ container, onAction }: ContainerCardProps) {
  const [loading, setLoading] = useState<string | null>(null);

  const isRunning = container.state === 'running';
  const isStopped = container.state === 'exited';
  const isPaused = container.state === 'paused';
  const hasStats = (container.cpu_percent ?? 0) > 0 || (container.memory_usage ?? 0) > 0;

  const cpuDisplay = (container.cpu_percent ?? 0) > 0 ? (container.cpu_percent ?? 0).toFixed(1) : '0';
  const memoryMB = (container.memory_usage ?? 0) > 0 ? ((container.memory_usage ?? 0) / 1024 / 1024).toFixed(0) : '0';
  const memoryGB = (container.memory_limit ?? 0) > 0 ? ((container.memory_limit ?? 0) / 1024 / 1024 / 1024).toFixed(1) : '?';
  const memoryPercent = (container.memory_percent ?? 0) > 0 ? (container.memory_percent ?? 0).toFixed(1) : '0';

  const stateIcon = isRunning ? '✅' : isStopped ? '⏹️' : isPaused ? '⏸️' : '❓';
  const uptime = isRunning && container.started_at ? formatUptime(container.started_at) : '';

  const handleStart = async () => {
    setLoading('start');
    try {
      await startContainer(container.host_id, container.id);
      onAction();
    } catch (error) {
      console.error('Failed to start container:', error);
    } finally {
      setLoading(null);
    }
  };

  const handleStop = async () => {
    setLoading('stop');
    try {
      await stopContainer(container.host_id, container.id);
      onAction();
    } catch (error) {
      console.error('Failed to stop container:', error);
    } finally {
      setLoading(null);
    }
  };

  const handleRestart = async () => {
    setLoading('restart');
    try {
      await restartContainer(container.host_id, container.id);
      onAction();
    } catch (error) {
      console.error('Failed to restart container:', error);
    } finally {
      setLoading(null);
    }
  };

  const handleRemove = async () => {
    if (!confirm(`Are you sure you want to remove "${container.name}"?`)) {
      return;
    }
    setLoading('remove');
    try {
      await removeContainer(container.host_id, container.id);
      onAction();
    } catch (error) {
      console.error('Failed to remove container:', error);
    } finally {
      setLoading(null);
    }
  };

  const stateClass = isRunning ? 'border-l-[var(--success)]' : isStopped ? 'border-l-[var(--danger)]' : 'border-l-[var(--warning)]';

  return (
    <div className={`bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg overflow-hidden border-l-4 ${stateClass}`}>
      {/* Header */}
      <div className="p-4 border-b border-[var(--border)]">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            <span className="text-2xl">{stateIcon}</span>
            <div>
              <h3 className="font-semibold text-lg">{container.name}</h3>
              <div className="flex items-center gap-2 text-sm text-[var(--text-tertiary)]">
                <span>📍 {container.host_name}</span>
                <span>•</span>
                <span title={container.image}>🏷️ {extractImageTag(container.image, container.image_tags)}</span>
                {uptime && (
                  <>
                    <span>•</span>
                    <span>⏱️ {uptime}</span>
                  </>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Body */}
      <div className="p-4 space-y-3">
        {/* Image */}
        <div>
          <div className="text-xs text-[var(--text-tertiary)] mb-1">Image</div>
          <div className="flex items-center gap-2">
            <code className="text-sm bg-[var(--bg-tertiary)] px-2 py-1 rounded">{container.image}</code>
            {container.update_available && (
              <span className="text-xs bg-[var(--accent)] text-white px-2 py-1 rounded">⬆️ Update</span>
            )}
          </div>
        </div>

        {/* Ports */}
        {container.ports && container.ports.length > 0 && container.ports.some(p => (p.public_port ?? 0) > 0) && (
          <div>
            <div className="text-xs text-[var(--text-tertiary)] mb-1">Ports</div>
            <div className="flex flex-wrap gap-1">
              {container.ports.filter(p => (p.public_port ?? 0) > 0).map((p, idx) => (
                <span key={idx} className="text-xs bg-[var(--bg-tertiary)] px-2 py-1 rounded">
                  {p.public_port}:{p.private_port}
                </span>
              ))}
            </div>
          </div>
        )}

        {/* Stats */}
        {hasStats && (
          <div className="grid grid-cols-2 gap-3 mt-4">
            {/* CPU */}
            <div className="bg-[var(--bg-tertiary)] rounded-lg p-3">
              <div className="flex items-center gap-2 mb-1">
                <span>💻</span>
                <span className="text-xs text-[var(--text-tertiary)]">CPU</span>
              </div>
              <div className="text-xl font-bold">{cpuDisplay}<span className="text-xs text-[var(--text-tertiary)]">%</span></div>
              <div className="h-1.5 bg-[var(--bg-secondary)] rounded-full mt-2 overflow-hidden">
                <div
                  className="h-full bg-[var(--accent)] rounded-full transition-all"
                  style={{ width: `${Math.min(parseFloat(cpuDisplay), 100)}%` }}
                />
              </div>
            </div>

            {/* Memory */}
            <div className="bg-[var(--bg-tertiary)] rounded-lg p-3">
              <div className="flex items-center gap-2 mb-1">
                <span>💾</span>
                <span className="text-xs text-[var(--text-tertiary)]">Memory</span>
              </div>
              <div className="text-xl font-bold">{memoryMB}<span className="text-xs text-[var(--text-tertiary)]">MB</span></div>
              <div className="text-xs text-[var(--text-tertiary)]">of {memoryGB}GB ({memoryPercent}%)</div>
              <div className="h-1.5 bg-[var(--bg-secondary)] rounded-full mt-2 overflow-hidden">
                <div
                  className="h-full bg-[var(--warning)] rounded-full transition-all"
                  style={{ width: `${Math.min(parseFloat(memoryPercent), 100)}%` }}
                />
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Footer Actions */}
      <div className="p-4 border-t border-[var(--border)] flex flex-wrap gap-2">
        {isRunning && (
          <>
            <button
              onClick={handleRestart}
              disabled={loading !== null}
              className="px-3 py-1.5 text-sm border border-[var(--warning)] text-[var(--warning)] rounded hover:bg-[var(--warning)] hover:text-white transition-colors disabled:opacity-50"
            >
              {loading === 'restart' ? '...' : '🔄 Restart'}
            </button>
            <button
              onClick={handleStop}
              disabled={loading !== null}
              className="px-3 py-1.5 text-sm border border-[var(--warning)] text-[var(--warning)] rounded hover:bg-[var(--warning)] hover:text-white transition-colors disabled:opacity-50"
            >
              {loading === 'stop' ? '...' : '⏹ Stop'}
            </button>
          </>
        )}
        {isStopped && (
          <>
            <button
              onClick={handleStart}
              disabled={loading !== null}
              className="px-3 py-1.5 text-sm bg-[var(--success)] text-white rounded hover:opacity-80 transition-opacity disabled:opacity-50"
            >
              {loading === 'start' ? '...' : '▶ Start'}
            </button>
            <button
              onClick={handleRemove}
              disabled={loading !== null}
              className="px-3 py-1.5 text-sm text-[var(--danger)] hover:bg-[var(--danger)] hover:text-white rounded transition-colors disabled:opacity-50"
            >
              {loading === 'remove' ? '...' : '🗑 Remove'}
            </button>
          </>
        )}
        {isPaused && (
          <>
            <button
              onClick={handleStart}
              disabled={loading !== null}
              className="px-3 py-1.5 text-sm bg-[var(--success)] text-white rounded hover:opacity-80 transition-opacity disabled:opacity-50"
            >
              {loading === 'start' ? '...' : '▶ Resume'}
            </button>
            <button
              onClick={handleStop}
              disabled={loading !== null}
              className="px-3 py-1.5 text-sm border border-[var(--warning)] text-[var(--warning)] rounded hover:bg-[var(--warning)] hover:text-white transition-colors disabled:opacity-50"
            >
              {loading === 'stop' ? '...' : '⏹ Stop'}
            </button>
          </>
        )}
      </div>
    </div>
  );
}

export default function ContainersPage() {
  const [containers, setContainers] = useState<Container[]>([]);
  const [hosts, setHosts] = useState<Host[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState('');
  const [hostFilter, setHostFilter] = useState('');
  const [stateFilter, setStateFilter] = useState('');

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

  const filteredContainers = useMemo(() => {
    return containers.filter(container => {
      const matchesSearch = searchTerm === '' ||
        container.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        container.image.toLowerCase().includes(searchTerm.toLowerCase()) ||
        container.host_name.toLowerCase().includes(searchTerm.toLowerCase());

      const matchesHost = hostFilter === '' || container.host_id.toString() === hostFilter;
      const matchesState = stateFilter === '' || container.state === stateFilter;

      return matchesSearch && matchesHost && matchesState;
    });
  }, [containers, searchTerm, hostFilter, stateFilter]);

  const stats = useMemo(() => ({
    total: containers.length,
    running: containers.filter(c => c.state === 'running').length,
    stopped: containers.filter(c => c.state === 'exited').length,
    paused: containers.filter(c => c.state === 'paused').length,
  }), [containers]);

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
        <h1 className="text-2xl font-bold">Containers</h1>
        <div className="flex items-center gap-4 text-sm text-[var(--text-tertiary)]">
          <span>Total: {stats.total}</span>
          <span className="text-[var(--success)]">Running: {stats.running}</span>
          <span className="text-[var(--danger)]">Stopped: {stats.stopped}</span>
          {stats.paused > 0 && <span className="text-[var(--warning)]">Paused: {stats.paused}</span>}
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
        <select
          value={stateFilter}
          onChange={(e) => setStateFilter(e.target.value)}
          className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg px-4 py-2 text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
        >
          <option value="">All States</option>
          <option value="running">Running</option>
          <option value="exited">Stopped</option>
          <option value="paused">Paused</option>
        </select>
      </div>

      {/* Container Grid */}
      {filteredContainers.length === 0 ? (
        <div className="text-center py-12 text-[var(--text-tertiary)]">
          No containers found
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-4">
          {filteredContainers.map(container => (
            <ContainerCard
              key={`${container.host_id}-${container.id}`}
              container={container}
              onAction={loadData}
            />
          ))}
        </div>
      )}
    </div>
  );
}
