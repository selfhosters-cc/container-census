'use client';

import { useEffect, useState, useRef, useMemo, useCallback } from 'react';

// Declare Chart.js as a global variable (loaded from CDN in layout)
declare const Chart: any;
import {
  fetchPluginApi,
  getHosts,
  getContainers,
  getContainerLifecycleSummaries,
  startContainer,
  stopContainer,
  restartContainer,
  removeContainer,
  getContainerLogs,
  getContainerStats,
  getContainerLifecycleEvents,
  bulkCheckUpdates,
  bulkUpdate,
  updateContainer
} from '@/lib/api';
import type { Host, Container, ContainerLifecycleSummary, ContainerStatsPoint, ContainerLifecycleEvent } from '@/types';

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
  onSubmit: (data: { name: string; url: string; email: string; password: string; enabled: boolean }) => Promise<void>;
  editInstance?: NpmInstance | null;
}

function AddInstanceModal({ isOpen, onClose, onSubmit, editInstance }: AddInstanceModalProps) {
  const [name, setName] = useState('');
  const [url, setUrl] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [enabled, setEnabled] = useState(true);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (editInstance) {
      setName(editInstance.name);
      setUrl(editInstance.url);
      setEmail(editInstance.email);
      setPassword('');
      setEnabled(editInstance.enabled);
    } else {
      setName('');
      setUrl('');
      setEmail('');
      setPassword('');
      setEnabled(true);
    }
  }, [editInstance, isOpen]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await onSubmit({ name, url, email, password, enabled });
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

          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="enabled"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              className="w-4 h-4"
            />
            <label htmlFor="enabled" className="text-sm text-[var(--text-tertiary)]">
              Enabled (sync proxy hosts)
            </label>
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

// ========== Container Card Helper Functions and Modals ==========

function formatUptime(startedAt: string | undefined, endAt?: string): string {
  if (!startedAt) return '';

  const end = endAt ? new Date(endAt) : new Date();
  const started = new Date(startedAt);

  if (isNaN(started.getTime()) || started.getFullYear() < 2000) {
    return '';
  }

  const diff = end.getTime() - started.getTime();
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

function extractImageTag(image: string, imageTags?: string[]): string {
  if (imageTags && imageTags.length > 0) {
    const tag = imageTags[0];
    const parts = tag.split(':');
    return parts[parts.length - 1] || 'latest';
  }
  const parts = image.split(':');
  return parts[parts.length - 1] || 'latest';
}

// Inline sparkline chart component for container cards
function InlineChart({ hostId, containerId }: { hostId: number; containerId: string }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const chartRef = useRef<any>(null);
  const [hasData, setHasData] = useState<boolean | null>(null); // null = loading
  const [chartLoaded, setChartLoaded] = useState(false);

  // Wait for Chart.js to load
  useEffect(() => {
    let attempts = 0;
    const maxAttempts = 50; // 5 seconds total

    const checkChartJs = () => {
      if (typeof Chart !== 'undefined') {
        console.log('[InlineChart] Chart.js loaded successfully');
        setChartLoaded(true);
      } else if (attempts < maxAttempts) {
        attempts++;
        setTimeout(checkChartJs, 100);
      } else {
        console.error('[InlineChart] Chart.js failed to load after 5 seconds');
        setHasData(false);
      }
    };
    checkChartJs();
  }, []);

  useEffect(() => {
    if (!chartLoaded) return;

    let mounted = true;

    const loadAndRender = async () => {
      try {
        const stats = await getContainerStats(hostId, containerId, '1h');
        console.log(`[InlineChart] Stats for ${containerId}:`, stats?.length || 0, 'points');

        if (!mounted) return;

        if (!stats || stats.length === 0) {
          console.log(`[InlineChart] No stats data for ${containerId}`);
          setHasData(false);
          return;
        }

        console.log(`[InlineChart] Rendering chart for ${containerId}`);

        const canvas = canvasRef.current;
        if (!canvas) {
          console.log(`[InlineChart] Canvas ref is null for ${containerId}`);
          return;
        }

        // Destroy existing chart
        if (chartRef.current) {
          chartRef.current.destroy();
          chartRef.current = null;
        }

        const ctx = canvas.getContext('2d');
        if (!ctx) {
          console.log(`[InlineChart] Could not get 2d context for ${containerId}`);
          return;
        }

        // Take last 20 points for sparkline
        const recentStats = stats.slice(-20);
        const cpuData = recentStats.map((s: ContainerStatsPoint) => s.cpu_percent || 0);
        const memoryData = recentStats.map((s: ContainerStatsPoint) => (s.memory_usage || 0) / 1024 / 1024);

        // Set canvas dimensions explicitly
        const parentWidth = canvas.parentElement?.offsetWidth || 500;
        canvas.width = parentWidth;
        canvas.height = 128;
        console.log(`[InlineChart] Canvas dimensions for ${containerId}: ${canvas.width}x${canvas.height}`);

        chartRef.current = new Chart(ctx, {
          type: 'line',
          data: {
            labels: recentStats.map(() => ''),
            datasets: [
              {
                label: 'CPU %',
                data: cpuData,
                borderColor: 'rgb(75, 192, 192)',
                backgroundColor: 'rgba(75, 192, 192, 0.1)',
                borderWidth: 2,
                pointRadius: 0,
                tension: 0.4,
                yAxisID: 'y',
                fill: true,
              },
              {
                label: 'Memory MB',
                data: memoryData,
                borderColor: 'rgb(255, 99, 132)',
                backgroundColor: 'rgba(255, 99, 132, 0.1)',
                borderWidth: 2,
                pointRadius: 0,
                tension: 0.4,
                yAxisID: 'y1',
                fill: true,
              },
            ],
          },
          options: {
            responsive: true,
            maintainAspectRatio: false,
            interaction: {
              mode: 'index',
              intersect: false,
            },
            plugins: {
              legend: {
                display: true,
                position: 'top',
                labels: {
                  boxWidth: 10,
                  padding: 6,
                  font: { size: 10 },
                  color: '#94a3b8',
                },
              },
              tooltip: {
                enabled: true,
                mode: 'index',
                intersect: false,
                callbacks: {
                  label: function(context: { dataset: { label?: string; yAxisID?: string }; parsed: { y: number | null } }) {
                    let label = context.dataset.label || '';
                    if (label) label += ': ';
                    if (context.parsed.y !== null) {
                      label += context.parsed.y.toFixed(2);
                      if (context.dataset.yAxisID === 'y') {
                        label += '%';
                      } else {
                        label += ' MB';
                      }
                    }
                    return label;
                  },
                },
              },
            },
            scales: {
              x: { display: false },
              y: {
                display: true,
                beginAtZero: true,
                position: 'left',
                title: { display: true, text: 'CPU %', font: { size: 9 }, color: '#94a3b8' },
                ticks: { font: { size: 8 }, color: '#94a3b8' },
                grid: { color: 'rgba(148, 163, 184, 0.1)' },
              },
              y1: {
                display: true,
                beginAtZero: true,
                position: 'right',
                title: { display: true, text: 'Memory MB', font: { size: 9 }, color: '#94a3b8' },
                ticks: { font: { size: 8 }, color: '#94a3b8' },
                grid: { drawOnChartArea: false },
              },
            },
          },
        });

        console.log(`[InlineChart] Chart created successfully for ${containerId}`);
        setHasData(true);
      } catch (error) {
        console.error('Error loading inline chart:', error);
        if (mounted) setHasData(false);
      }
    };

    loadAndRender();

    return () => {
      mounted = false;
      if (chartRef.current) {
        chartRef.current.destroy();
        chartRef.current = null;
      }
    };
  }, [chartLoaded, hostId, containerId]);

  return (
    <div className="h-32 relative">
      <canvas ref={canvasRef} className="w-full h-full"></canvas>
      {!chartLoaded && (
        <div className="absolute inset-0 flex items-center justify-center text-xs text-[var(--text-secondary)] bg-[var(--bg-tertiary)]">
          Loading Chart.js...
        </div>
      )}
      {chartLoaded && hasData === null && (
        <div className="absolute inset-0 flex items-center justify-center text-xs text-[var(--text-secondary)] bg-[var(--bg-tertiary)]">
          Loading chart data...
        </div>
      )}
      {chartLoaded && hasData === false && (
        <div className="absolute inset-0 flex items-center justify-center text-xs text-[var(--text-secondary)] bg-[var(--bg-tertiary)]">
          No stats data available
        </div>
      )}
    </div>
  );
}
function LogsModal({ container, onClose }: { container: Container | null; onClose: () => void }) {
  const [logs, setLogs] = useState('');
  const [loading, setLoading] = useState(true);
  const [tail, setTail] = useState(100);

  useEffect(() => {
    if (container) {
      setLoading(true);
      getContainerLogs(container.host_id, container.id, tail)
        .then(data => setLogs(data.logs || 'No logs available'))
        .catch(err => setLogs(`Error loading logs: ${err.message}`))
        .finally(() => setLoading(false));
    }
  }, [container, tail]);

  if (!container) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg w-full max-w-4xl max-h-[80vh] flex flex-col">
        <div className="p-4 border-b border-[var(--border)] flex justify-between items-center">
          <div>
            <h2 className="text-xl font-bold">Container Logs</h2>
            <div className="text-sm text-[var(--text-tertiary)]">{container.name}</div>
          </div>
          <div className="flex items-center gap-4">
            <select
              value={tail}
              onChange={e => setTail(Number(e.target.value))}
              className="bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-2 py-1 text-sm"
            >
              <option value={50}>Last 50 lines</option>
              <option value={100}>Last 100 lines</option>
              <option value={500}>Last 500 lines</option>
              <option value={1000}>Last 1000 lines</option>
            </select>
            <button onClick={onClose} className="text-2xl hover:opacity-70">×</button>
          </div>
        </div>
        <div className="flex-1 overflow-auto p-4">
          {loading ? (
            <div className="text-[var(--text-tertiary)]">Loading logs...</div>
          ) : (
            <pre className="text-xs font-mono whitespace-pre-wrap break-all bg-[var(--bg-tertiary)] p-4 rounded">
              {logs}
            </pre>
          )}
        </div>
      </div>
    </div>
  );
}

// Stats Modal Component
function StatsModal({ container, onClose }: { container: Container | null; onClose: () => void }) {
  const [stats, setStats] = useState<ContainerStatsPoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [range, setRange] = useState('1h');
  const cpuChartRef = useRef<HTMLCanvasElement>(null);
  const memChartRef = useRef<HTMLCanvasElement>(null);
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const cpuChartInstance = useRef<any>(null);
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const memChartInstance = useRef<any>(null);

  useEffect(() => {
    if (container) {
      setLoading(true);
      getContainerStats(container.host_id, container.id, range)
        .then(setStats)
        .catch(console.error)
        .finally(() => setLoading(false));
    }
  }, [container, range]);

  useEffect(() => {
    if (!stats.length || loading) return;

    const labels = stats.map(s => {
      const d = new Date(s.timestamp);
      return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    });

    // CPU Chart
    if (cpuChartRef.current && typeof Chart !== 'undefined') {
      if (cpuChartInstance.current) cpuChartInstance.current.destroy();
      cpuChartInstance.current = new Chart(cpuChartRef.current.getContext('2d')!, {
        type: 'line',
        data: {
          labels,
          datasets: [{
            label: 'CPU %',
            data: stats.map(s => s.cpu_percent),
            borderColor: '#3b82f6',
            backgroundColor: 'rgba(59, 130, 246, 0.1)',
            fill: true,
            tension: 0.3,
          }],
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          plugins: { legend: { display: false } },
          scales: {
            y: { beginAtZero: true, max: 100 },
            x: { ticks: { maxTicksLimit: 10 } },
          },
        },
      });
    }

    // Memory Chart
    if (memChartRef.current && typeof Chart !== 'undefined') {
      if (memChartInstance.current) memChartInstance.current.destroy();
      memChartInstance.current = new Chart(memChartRef.current.getContext('2d')!, {
        type: 'line',
        data: {
          labels,
          datasets: [{
            label: 'Memory (MB)',
            data: stats.map(s => s.memory_usage / 1024 / 1024),
            borderColor: '#f59e0b',
            backgroundColor: 'rgba(245, 158, 11, 0.1)',
            fill: true,
            tension: 0.3,
          }],
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          plugins: { legend: { display: false } },
          scales: {
            y: { beginAtZero: true },
            x: { ticks: { maxTicksLimit: 10 } },
          },
        },
      });
    }

    return () => {
      if (cpuChartInstance.current) cpuChartInstance.current.destroy();
      if (memChartInstance.current) memChartInstance.current.destroy();
    };
  }, [stats, loading]);

  if (!container) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg w-full max-w-4xl max-h-[80vh] flex flex-col">
        <div className="p-4 border-b border-[var(--border)] flex justify-between items-center">
          <div>
            <h2 className="text-xl font-bold">Container Stats</h2>
            <div className="text-sm text-[var(--text-tertiary)]">{container.name}</div>
          </div>
          <div className="flex items-center gap-4">
            <select
              value={range}
              onChange={e => setRange(e.target.value)}
              className="bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-2 py-1 text-sm"
            >
              <option value="1h">Last Hour</option>
              <option value="24h">Last 24 Hours</option>
              <option value="7d">Last 7 Days</option>
              <option value="all">All Time</option>
            </select>
            <button onClick={onClose} className="text-2xl hover:opacity-70">×</button>
          </div>
        </div>
        <div className="flex-1 overflow-auto p-4 space-y-6">
          {loading ? (
            <div className="text-[var(--text-tertiary)]">Loading stats...</div>
          ) : stats.length === 0 ? (
            <div className="text-[var(--text-tertiary)]">No stats available for this container</div>
          ) : (
            <>
              <div className="bg-[var(--bg-tertiary)] rounded-lg p-4">
                <h3 className="text-sm font-medium mb-2 text-[var(--text-secondary)]">CPU Usage</h3>
                <div className="h-48">
                  <canvas ref={cpuChartRef}></canvas>
                </div>
              </div>
              <div className="bg-[var(--bg-tertiary)] rounded-lg p-4">
                <h3 className="text-sm font-medium mb-2 text-[var(--text-secondary)]">Memory Usage</h3>
                <div className="h-48">
                  <canvas ref={memChartRef}></canvas>
                </div>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

// History Modal Component
function HistoryModal({ container, onClose }: { container: Container | null; onClose: () => void }) {
  const [events, setEvents] = useState<ContainerLifecycleEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Fetch lifecycle events when container changes
  useEffect(() => {
    if (container) {
      setLoading(true);
      setError('');
      getContainerLifecycleEvents(container.host_id, container.name)
        .then(data => {
          setEvents(data || []);
        })
        .catch(err => {
          console.error('[HistoryModal] Error fetching events:', err);
          setError(err.message || 'Failed to load history');
        })
        .finally(() => setLoading(false));
    }
  }, [container]);

  // Helper function to get event icon
  const getEventIcon = (eventType: string) => {
    switch (eventType) {
      case 'first_seen': return '🎉';
      case 'started': case 'resumed': return '▶️';
      case 'stopped': return '⏹️';
      case 'paused': return '⏸️';
      case 'restarted': return '⟳';
      case 'image_updated': return '📦';
      case 'disappeared': return '👻';
      case 'reappeared': return '✨';
      case 'state_change': return '🔄';
      case 'last_seen': return '📍';
      default: return '•';
    }
  };

  // Helper function to get event color
  const getEventColor = (eventType: string) => {
    if (['started', 'resumed', 'reappeared'].includes(eventType)) return 'text-green-500';
    if (['stopped', 'disappeared'].includes(eventType)) return 'text-red-500';
    if (['paused', 'restarted'].includes(eventType)) return 'text-yellow-500';
    if (['image_updated', 'first_seen'].includes(eventType)) return 'text-blue-500';
    return 'text-gray-500';
  };

  // Calculate summary stats
  const summaryStats = useMemo(() => {
    const stateChanges = events.filter(e =>
      ['started', 'stopped', 'paused', 'resumed', 'restarted'].includes(e.event_type)
    ).length;

    const imageUpdates = events.filter(e => e.event_type === 'image_updated').length;

    // Calculate uptime from first to last event
    const firstEvent = events[events.length - 1]; // Oldest (events are reverse chronological)
    const lastEvent = events[0]; // Newest
    const uptimeDuration = firstEvent && lastEvent
      ? new Date(lastEvent.timestamp).getTime() - new Date(firstEvent.timestamp).getTime()
      : 0;

    return {
      totalEvents: events.length,
      stateChanges,
      imageUpdates,
      uptimeDuration
    };
  }, [events]);

  // Format uptime duration
  const formatDuration = (ms: number) => {
    const seconds = Math.floor(ms / 1000);
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
  };

  // Format timestamp
  const formatTimestamp = (timestamp: string) => {
    const date = new Date(timestamp);
    const now = new Date();
    const diff = now.getTime() - date.getTime();
    const seconds = Math.floor(diff / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);

    let relative = '';
    if (days > 0) relative = `${days}d ago`;
    else if (hours > 0) relative = `${hours}h ago`;
    else if (minutes > 0) relative = `${minutes}m ago`;
    else relative = `${seconds}s ago`;

    const absolute = date.toLocaleString();

    return { relative, absolute };
  };

  if (!container) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4" onClick={onClose}>
      <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg w-full max-w-4xl max-h-[80vh] flex flex-col" onClick={e => e.stopPropagation()}>
        {/* Header */}
        <div className="p-4 border-b border-[var(--border)] flex justify-between items-center">
          <div>
            <h2 className="text-xl font-bold">Container History</h2>
            <div className="text-sm text-[var(--text-tertiary)]">{container.name}</div>
          </div>
          <button onClick={onClose} className="text-2xl hover:opacity-70">×</button>
        </div>

        {/* Body - Summary Stats + Timeline */}
        <div className="flex-1 overflow-y-auto p-4 space-y-6">
          {loading && (
            <div className="text-[var(--text-tertiary)]">Loading history...</div>
          )}
          {error && (
            <div className="text-red-500">Error: {error}</div>
          )}
          {!loading && !error && (
            <>
              {/* Summary Stats Grid */}
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <div className="bg-[var(--bg-tertiary)] rounded-lg p-3">
                  <div className="text-2xl font-bold">{summaryStats.totalEvents}</div>
                  <div className="text-sm text-[var(--text-secondary)]">Total Events</div>
                </div>
                <div className="bg-[var(--bg-tertiary)] rounded-lg p-3">
                  <div className="text-2xl font-bold">{summaryStats.stateChanges}</div>
                  <div className="text-sm text-[var(--text-secondary)]">State Changes</div>
                </div>
                <div className="bg-[var(--bg-tertiary)] rounded-lg p-3">
                  <div className="text-2xl font-bold">{summaryStats.uptimeDuration > 0 ? formatDuration(summaryStats.uptimeDuration) : 'N/A'}</div>
                  <div className="text-sm text-[var(--text-secondary)]">Lifetime</div>
                </div>
                <div className="bg-[var(--bg-tertiary)] rounded-lg p-3">
                  <div className="text-2xl font-bold">{summaryStats.imageUpdates}</div>
                  <div className="text-sm text-[var(--text-secondary)]">Image Updates</div>
                </div>
              </div>

              {/* Timeline */}
              <div>
                <h3 className="text-lg font-semibold mb-3">Event Timeline ({events.length} events)</h3>
                {events.length === 0 ? (
                  <div className="text-[var(--text-tertiary)] bg-[var(--bg-tertiary)] rounded-lg p-4 text-center">
                    No events recorded for this container
                  </div>
                ) : (
                  <div className="space-y-2">
                    {events.map((event, idx) => {
                      const time = formatTimestamp(event.timestamp);
                      const icon = getEventIcon(event.event_type);
                      const color = getEventColor(event.event_type);

                      return (
                        <div key={idx} className="bg-[var(--bg-tertiary)] rounded-lg p-3 hover:bg-[var(--bg-hover)] transition-colors">
                          <div className="flex items-start gap-3">
                            <div className={`text-2xl ${color}`}>{icon}</div>
                            <div className="flex-1 min-w-0">
                              <div className="flex items-center gap-2 flex-wrap">
                                <span className={`font-medium ${color}`}>{event.event_type.replace(/_/g, ' ').toUpperCase()}</span>
                                <span className="text-xs text-[var(--text-tertiary)]" title={time.absolute}>
                                  {time.relative}
                                </span>
                              </div>
                              {event.description && (
                                <div className="text-sm text-[var(--text-secondary)] mt-1">
                                  {event.description}
                                </div>
                              )}
                              {event.old_state && event.new_state && (
                                <div className="text-xs text-[var(--text-tertiary)] mt-1">
                                  State: {event.old_state} → {event.new_state}
                                </div>
                              )}
                              {event.old_image_tag && event.new_image_tag && (
                                <div className="text-xs text-[var(--text-tertiary)] mt-1">
                                  Image: {event.old_image_tag} → {event.new_image_tag}
                                </div>
                              )}
                              {event.restart_count !== undefined && event.restart_count > 0 && (
                                <div className="text-xs text-[var(--text-tertiary)] mt-1">
                                  Restart count: {event.restart_count}
                                </div>
                              )}
                            </div>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

// Bulk Update Modal Component
function BulkUpdateModal({ isOpen, onClose, containers, onUpdate }: { isOpen: boolean; onClose: () => void; containers: Container[]; onUpdate: () => void }) {
  const [checking, setChecking] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [updatingKeys, setUpdatingKeys] = useState<Set<string>>(new Set());
  const [updateResults, setUpdateResults] = useState<Record<string, { success: boolean; error?: string; new_container_id?: string }> | null>(null);
  const [results, setResults] = useState<Record<string, { available: boolean; message?: string }> | null>(null);
  const [selectedContainers, setSelectedContainers] = useState<Set<string>>(new Set());

  const handleCheckUpdates = useCallback(async () => {
    setChecking(true);
    setResults(null);

    // Filter to only :latest containers
    const latestContainers = containers.filter(c =>
      c.image.endsWith(':latest') || (!c.image.includes(':') && c.state === 'running')
    );

    if (latestContainers.length === 0) {
      setResults({});
      setChecking(false);
      return;
    }

    try {
      const containerList = latestContainers.map(c => ({
        host_id: c.host_id,
        container_id: c.id
      }));

      const updateResults = await bulkCheckUpdates(containerList);
      setResults(updateResults);
    } catch (error) {
      console.error('Failed to check for updates:', error);
      alert('Failed to check for updates. See console for details.');
    } finally {
      setChecking(false);
    }
  }, [containers]);

  useEffect(() => {
    if (isOpen) {
      // Auto-check on open
      handleCheckUpdates();
      // Reset update results when opening
      setUpdateResults(null);
    } else {
      // Reset on close
      setResults(null);
      setSelectedContainers(new Set());
      setUpdateResults(null);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen]); // Only depend on isOpen, not handleCheckUpdates

  const containersWithUpdates = useMemo(() => {
    if (!results) return [];

    return containers.filter(c => {
      const key = `${c.host_id}-${c.id}`;
      return results[key]?.available;
    });
  }, [results, containers]);

  const toggleContainer = (key: string) => {
    const newSelected = new Set(selectedContainers);
    if (newSelected.has(key)) {
      newSelected.delete(key);
    } else {
      newSelected.add(key);
    }
    setSelectedContainers(newSelected);
  };

  const selectAll = () => {
    const newSelected = new Set(containersWithUpdates.map(c => `${c.host_id}-${c.id}`));
    setSelectedContainers(newSelected);
  };

  const deselectAll = () => {
    setSelectedContainers(new Set());
  };

  const handleBulkUpdate = async () => {
    if (selectedContainers.size === 0) return;

    if (!confirm(`Update ${selectedContainers.size} container(s)?\n\nThis will pull new images, stop containers, and recreate them with the same configuration.`)) {
      return;
    }

    setUpdating(true);
    setUpdateResults({});
    setUpdatingKeys(selectedContainers);

    try {
      // Convert selected container keys to the format the API expects
      const toUpdate = Array.from(selectedContainers).map(key => {
        const [hostId, containerId] = key.split('-');
        return {
          host_id: parseInt(hostId),
          container_id: containerId
        };
      });

      const results = await bulkUpdate(toUpdate);
      setUpdateResults(results);
      setUpdatingKeys(new Set());

      // Refresh data after updates (silently in background)
      onUpdate();

      // Show success message
      const successCount = Object.values(results).filter(r => r.success).length;
      const failCount = Object.values(results).filter(r => !r.success).length;

      if (failCount === 0) {
        // Don't close automatically - let user see results
        // They can click Close when ready
      }
    } catch (error) {
      console.error('Bulk update failed:', error);
      alert('Bulk update failed. See console for details.');
      setUpdatingKeys(new Set());
    } finally {
      setUpdating(false);
    }
  };

  if (!isOpen) return null;

  const latestCount = containers.filter(c =>
    c.image.endsWith(':latest') || (!c.image.includes(':') && c.state === 'running')
  ).length;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg w-full max-w-4xl max-h-[90vh] flex flex-col">
        <div className="p-6 border-b border-[var(--border)]">
          <div className="flex items-center justify-between">
            <h2 className="text-xl font-bold">Container Updates Available</h2>
            <button onClick={onClose} className="text-2xl text-[var(--text-tertiary)] hover:text-[var(--text-primary)]">&times;</button>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto p-6">
          {checking && (
            <div className="flex flex-col items-center justify-center py-12">
              <div className="text-6xl mb-4">🔍</div>
              <div className="text-lg font-semibold mb-2">Checking for updates...</div>
              <div className="text-sm text-[var(--text-secondary)]">Checking {latestCount} container(s) against their registries</div>
            </div>
          )}

          {!checking && results !== null && (
            <>
              {/* Info banner */}
              <div className="bg-[var(--info)]/10 border border-[var(--info)] rounded-lg p-4 mb-6">
                <p className="text-sm">
                  <strong>ℹ️ Update Check:</strong> Checked <strong>{latestCount}</strong> of <strong>{containers.length}</strong> total containers.
                  Only containers using <code className="bg-[var(--bg-tertiary)] px-1">:latest</code> tags (or no tag) are checked for updates.
                  Containers with specific version tags (e.g., <code className="bg-[var(--bg-tertiary)] px-1">:1.2.3</code>) are skipped.
                </p>
              </div>

              {containersWithUpdates.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-12">
                  <div className="text-6xl mb-4">✅</div>
                  <h3 className="text-lg font-semibold mb-2">All containers are up to date!</h3>
                  <p className="text-sm text-[var(--text-secondary)]">All {latestCount} checked container(s) with <code className="bg-[var(--bg-tertiary)] px-1">:latest</code> tags are up to date.</p>
                </div>
              ) : (
                <>
                  <div className="mb-4">
                    <p className="mb-3"><strong>{containersWithUpdates.length} container(s)</strong> have updates available</p>
                    <div className="flex gap-2">
                      <button
                        onClick={selectAll}
                        disabled={updating}
                        className="px-3 py-1.5 text-sm border border-[var(--border)] rounded hover:bg-[var(--bg-tertiary)] transition-colors disabled:opacity-50"
                      >
                        Select All
                      </button>
                      <button
                        onClick={deselectAll}
                        disabled={updating}
                        className="px-3 py-1.5 text-sm border border-[var(--border)] rounded hover:bg-[var(--bg-tertiary)] transition-colors disabled:opacity-50"
                      >
                        Deselect All
                      </button>
                    </div>
                  </div>

                  <div className="bg-[var(--bg-tertiary)] border border-[var(--border)] rounded-lg overflow-hidden">
                    <table className="w-full">
                      <thead className="bg-[var(--bg-secondary)]">
                        <tr>
                          <th className="text-left px-4 py-3 text-sm font-medium w-12">
                            <input
                              type="checkbox"
                              checked={selectedContainers.size === containersWithUpdates.length && containersWithUpdates.length > 0}
                              onChange={(e) => e.target.checked ? selectAll() : deselectAll()}
                              disabled={updating}
                            />
                          </th>
                          <th className="text-left px-4 py-3 text-sm font-medium">Update Details</th>
                        </tr>
                      </thead>
                      <tbody>
                        {containersWithUpdates.map(container => {
                          const key = `${container.host_id}-${container.id}`;
                          const message = results[key]?.message;

                          return (
                            <tr key={key} className="border-t border-[var(--border)]">
                              <td className="px-4 py-3">
                                <input
                                  type="checkbox"
                                  checked={selectedContainers.has(key)}
                                  onChange={() => toggleContainer(key)}
                                  disabled={updating}
                                />
                              </td>
                              <td className="px-4 py-3">
                                <div className="space-y-1">
                                  <div className="font-medium">{container.name}</div>
                                  <div className="text-xs text-[var(--text-secondary)]">
                                    <span className="font-medium">Host:</span> {container.host_name} | <span className="font-medium">Image:</span> {container.image}
                                  </div>
                                  {message && !updateResults && !updatingKeys.has(key) && (
                                    <div className="text-xs bg-[var(--info)]/10 border border-[var(--info)] rounded px-2 py-1 mt-1">
                                      {message}
                                    </div>
                                  )}
                                  {updatingKeys.has(key) && (
                                    <div className="text-xs bg-[var(--warning)]/10 border border-[var(--warning)] rounded px-2 py-1 mt-1 flex items-center gap-2">
                                      <span className="inline-block animate-spin">⏳</span>
                                      <span>Updating container...</span>
                                    </div>
                                  )}
                                  {updateResults && updateResults[key] && !updatingKeys.has(key) && (
                                    <div className={`text-xs rounded px-2 py-1 mt-1 ${
                                      updateResults[key].success
                                        ? 'bg-[var(--success)]/10 border border-[var(--success)] text-[var(--success)]'
                                        : 'bg-[var(--danger)]/10 border border-[var(--danger)] text-[var(--danger)]'
                                    }`}>
                                      {updateResults[key].success ? (
                                        `✓ Successfully updated${updateResults[key].new_container_id ? ` (new ID: ${updateResults[key].new_container_id?.substring(0, 12)}...)` : ''}`
                                      ) : (
                                        <div>
                                          <div className="font-medium">✗ Update failed</div>
                                          <div className="mt-1 text-[10px] leading-tight opacity-90">
                                            {updateResults[key].error?.includes('agent returned status')
                                              ? updateResults[key].error.split('agent returned status')[1]?.replace(/^\s*\d+:\s*/, '').trim() || updateResults[key].error
                                              : updateResults[key].error || 'Unknown error'
                                            }
                                          </div>
                                        </div>
                                      )}
                                    </div>
                                  )}
                                </div>
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>

                  <div className="bg-[var(--info)]/10 border border-[var(--info)] rounded-lg p-4 mt-6">
                    <p className="text-sm">
                      <strong>💡 Tip:</strong> You can also check and update individual containers from the container cards. Each card has a dedicated update button.
                    </p>
                  </div>
                </>
              )}
            </>
          )}
        </div>

        <div className="p-6 border-t border-[var(--border)] flex justify-end gap-3">
          <button onClick={onClose} className="px-4 py-2 text-sm border border-[var(--border)] rounded hover:bg-[var(--bg-tertiary)] transition-colors">
            Close
          </button>
          {containersWithUpdates.length > 0 && (
            <button
              onClick={handleBulkUpdate}
              disabled={selectedContainers.size === 0 || updating}
              className="px-4 py-2 text-sm bg-[var(--accent)] text-white rounded hover:bg-[var(--accent-hover)] transition-colors disabled:opacity-50"
            >
              {updating ? `Updating ${selectedContainers.size}...` : `Update Selected (${selectedContainers.size})`}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

// Update Modal Component
function UpdateModal({ container, onClose, onUpdate }: { container: Container | null; onClose: () => void; onUpdate: () => void }) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  if (!container) return null;

  const handleUpdate = async () => {
    setLoading(true);
    setError(null);
    setSuccess(false);
    console.log('Updating container:', { host_id: container.host_id, name: container.name, id: container.id });
    try {
      await updateContainer(container.host_id, container.name);
      setSuccess(true);
      onUpdate(); // Refresh data in background
      // Auto-close after 2 seconds on success
      setTimeout(() => {
        onClose();
      }, 2000);
    } catch (error: unknown) {
      console.error('Failed to update container:', error);
      if (error instanceof Error) {
        setError(error.message);
      } else {
        setError('Unknown error occurred');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg w-full max-w-md p-6">
        <h2 className="text-xl font-bold mb-4">Update Container</h2>

        {!loading && !success && !error && (
          <>
            <div className="space-y-4">
              <div>
                <p className="text-sm"><strong>Container:</strong> {container.name}</p>
                <p className="text-sm"><strong>Image:</strong> {container.image}</p>
              </div>
              <div className="text-sm">
                <p className="mb-2">This will:</p>
                <ul className="list-disc ml-5 space-y-1">
                  <li>Pull the latest <code className="bg-[var(--bg-tertiary)] px-1">{container.image}</code> image</li>
                  <li>Stop and remove the current container</li>
                  <li>Create a new container with the same configuration</li>
                  <li>Start the new container</li>
                </ul>
              </div>
              <div className="bg-[var(--warning)]/10 border border-[var(--warning)] rounded p-3 text-sm">
                <p className="font-medium">⚠️ Note:</p>
                <p className="mt-1">The old image will be kept for rollback. Container configuration (env vars, volumes, ports, networks) will be preserved.</p>
                <p className="mt-1 text-[var(--danger)]">Non-volume data will be lost. Ensure important data is in volumes!</p>
              </div>
            </div>
            <div className="flex gap-3 mt-6">
              <button
                onClick={onClose}
                className="flex-1 px-4 py-2 text-sm border border-[var(--border)] rounded hover:bg-[var(--bg-tertiary)] transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleUpdate}
                className="flex-1 px-4 py-2 text-sm bg-[var(--accent)] text-white rounded hover:bg-[var(--accent-hover)] transition-colors"
              >
                Update Container
              </button>
            </div>
          </>
        )}

        {loading && (
          <div className="flex flex-col items-center justify-center py-8">
            <div className="text-6xl mb-4 animate-spin">⏳</div>
            <div className="text-lg font-semibold mb-2">Updating container...</div>
            <div className="text-sm text-[var(--text-secondary)] text-center">
              Pulling image, stopping container, and recreating with new version
            </div>
          </div>
        )}

        {success && (
          <div className="flex flex-col items-center justify-center py-8">
            <div className="text-6xl mb-4">✅</div>
            <div className="text-lg font-semibold mb-2 text-[var(--success)]">Update successful!</div>
            <div className="text-sm text-[var(--text-secondary)]">
              Container has been updated and restarted
            </div>
          </div>
        )}

        {error && (
          <>
            <div className="flex flex-col items-center justify-center py-6">
              <div className="text-6xl mb-4">❌</div>
              <div className="text-lg font-semibold mb-2 text-[var(--danger)]">Update failed</div>
              <div className="bg-[var(--danger)]/10 border border-[var(--danger)] rounded p-3 mt-4 w-full">
                <p className="text-sm text-[var(--danger)] break-words">
                  {error.includes('agent returned status')
                    ? error.split('agent returned status')[1]?.replace(/^\s*\d+:\s*/, '').trim() || error
                    : error
                  }
                </p>
              </div>
            </div>
            <div className="flex gap-3 mt-6">
              <button
                onClick={onClose}
                className="flex-1 px-4 py-2 text-sm border border-[var(--border)] rounded hover:bg-[var(--bg-tertiary)] transition-colors"
              >
                Close
              </button>
              <button
                onClick={() => {
                  setError(null);
                  setSuccess(false);
                }}
                className="flex-1 px-4 py-2 text-sm bg-[var(--accent)] text-white rounded hover:bg-[var(--accent-hover)] transition-colors"
              >
                Try Again
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

// ========== End of Container Card Components ==========

function formatRelativeTime(dateString: string | null): string {
  if (!dateString) return 'Never';
  const date = new Date(dateString);
  const now = new Date();
  const diff = now.getTime() - date.getTime();
  const minutes = Math.floor(diff / 60000);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (days > 0) return `${days}d ago`;
  if (hours > 0) return `${hours}h ago`;
  if (minutes > 0) return `${minutes}m ago`;
  return 'Just now';
}

export default function NpmPluginPage() {
  const [instances, setInstances] = useState<NpmInstance[]>([]);
  const [proxyHosts, setProxyHosts] = useState<ProxyHost[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingInstance, setEditingInstance] = useState<NpmInstance | null>(null);
  const [actionLoading, setActionLoading] = useState<number | null>(null);
  const [searchTerm, setSearchTerm] = useState('');

  // Container details modal state
  const [hosts, setHosts] = useState<Host[]>([]);
  const [selectedContainer, setSelectedContainer] = useState<{
    hostId: number;
    containerName: string;
  } | null>(null);
  const [containerModalOpen, setContainerModalOpen] = useState(false);

  const loadData = async () => {
    try {
      const [instancesData, hostsDataRaw] = await Promise.all([
        fetchPluginApi<NpmInstance[]>('npm', '/instances').catch(() => []),
        fetchPluginApi<any[]>('npm', '/proxy-hosts').catch(() => []),
      ]);

      // Transform the nested API response to flat structure
      const hostsData = hostsDataRaw.map((item: any) => ({
        id: item.host?.id || 0,
        npm_instance_id: item.instance_id,
        npm_host_id: item.host?.id || 0,
        domain_names: item.host?.domain_names || [],
        forward_host: item.host?.forward_host || '',
        forward_port: item.host?.forward_port || 0,
        ssl_enabled: (item.host?.certificate_id || 0) > 0,
        enabled: item.host?.enabled || false,
        container_name: item.container_name,
        host_name: item.host_name,
      }));

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

  // Load hosts data for container matching
  useEffect(() => {
    async function loadHostsData() {
      try {
        const hostsData = await getHosts();
        setHosts(hostsData);
      } catch (error) {
        console.error('Failed to load hosts:', error);
      }
    }
    loadHostsData();
  }, []);

  const handleAddInstance = async (data: { name: string; url: string; email: string; password: string; enabled: boolean }) => {
    await fetchPluginApi('npm', '/instances', {
      method: 'POST',
      body: JSON.stringify(data),
    });
    await loadData();
  };

  const handleUpdateInstance = async (data: { name: string; url: string; email: string; password: string; enabled: boolean }) => {
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
      (host.domain_names || []).some(d => d.toLowerCase().includes(search)) ||
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
                    {' • '}
                    <span title={instance.last_sync ? new Date(instance.last_sync).toLocaleString() : 'Never synced'}>
                      Last sync: {formatRelativeTime(instance.last_sync)}
                    </span>
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
                        {(host.domain_names || []).map(domain => (
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
                        <div className="flex items-center gap-2">
                          <button
                            onClick={() => {
                              // Find host_id by resolving host_name
                              const matchedHost = hosts.find(h => h.name === host.host_name);
                              if (matchedHost) {
                                setSelectedContainer({
                                  hostId: matchedHost.id,
                                  containerName: host.container_name!
                                });
                                setContainerModalOpen(true);
                              }
                            }}
                            className="px-2 py-1 text-xs bg-[var(--accent)] text-white rounded hover:opacity-80"
                          >
                            🔍
                          </button>
                          <span className="text-sm">{host.container_name}</span>
                        </div>
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

      {/* Container Details Modal */}
      {containerModalOpen && selectedContainer && (
        <ContainerDetailsModal
          hostId={selectedContainer.hostId}
          containerName={selectedContainer.containerName}
          onClose={() => {
            setContainerModalOpen(false);
            setSelectedContainer(null);
          }}
        />
      )}
    </div>
  );
}

// Container Details Modal - Full featured modal with embedded ContainerCard
function ContainerDetailsModal({
  hostId,
  containerName,
  onClose
}: {
  hostId: number;
  containerName: string;
  onClose: () => void;
}) {
  const [loading, setLoading] = useState(true);
  const [container, setContainer] = useState<Container | null>(null);
  const [lifecycle, setLifecycle] = useState<ContainerLifecycleSummary | null>(null);
  const [error, setError] = useState('');
  const [logsContainer, setLogsContainer] = useState<Container | null>(null);
  const [statsContainer, setStatsContainer] = useState<Container | null>(null);
  const [historyContainer, setHistoryContainer] = useState<Container | null>(null);
  const [updateContainer, setUpdateContainer] = useState<Container | null>(null);
  const [actionLoading, setActionLoading] = useState<string | null>(null);

  useEffect(() => {
    loadContainerData();
  }, [hostId, containerName]);

  async function loadContainerData() {
    setLoading(true);
    setError('');
    try {
      const containers = await getContainers();
      const match = containers.find(c => c.host_id === hostId && c.name === containerName);

      if (!match) {
        setError('Container not found or no longer running');
        setLoading(false);
        return;
      }

      setContainer(match);

      const summaries = await getContainerLifecycleSummaries(200, hostId);
      const lifecycleMatch = summaries.find(s => s.container_id === match.id && s.host_id === hostId);
      setLifecycle(lifecycleMatch || null);
    } catch (err: any) {
      setError(err.message || 'Failed to load container');
    } finally {
      setLoading(false);
    }
  }

  const handleAction = () => {
    loadContainerData();
  };

  if (!container && !loading && !error) return null;

  const isRunning = container?.state === 'running';
  const isStopped = container?.state === 'exited';
  const isPaused = container?.state === 'paused';
  const hasStats = (container?.cpu_percent ?? 0) > 0 || (container?.memory_usage ?? 0) > 0;

  const cpuDisplay = container && (container.cpu_percent ?? 0) > 0 ? (container.cpu_percent ?? 0).toFixed(1) : '0';
  const memoryMB = container && (container.memory_usage ?? 0) > 0 ? ((container.memory_usage ?? 0) / 1024 / 1024).toFixed(0) : '0';
  const memoryGB = container && (container.memory_limit ?? 0) > 0 ? ((container.memory_limit ?? 0) / 1024 / 1024 / 1024).toFixed(1) : '?';
  const memoryPercent = container && (container.memory_percent ?? 0) > 0 ? (container.memory_percent ?? 0).toFixed(1) : '0';

  const stateIcon = isRunning ? '✅' : isStopped ? '⏹️' : isPaused ? '⏸️' : '❓';

  let uptime = '';
  if (isRunning && lifecycle?.last_started) {
    uptime = formatUptime(lifecycle.last_started);
  }

  const lifetime = lifecycle && lifecycle.first_seen ? formatUptime(lifecycle.first_seen) : null;

  const handleStart = async () => {
    if (!container) return;
    setActionLoading('start');
    try {
      await startContainer(container.host_id, container.id);
      handleAction();
    } catch (error) {
      console.error('Failed to start container:', error);
    } finally {
      setActionLoading(null);
    }
  };

  const handleStop = async () => {
    if (!container) return;
    setActionLoading('stop');
    try {
      await stopContainer(container.host_id, container.id);
      handleAction();
    } catch (error) {
      console.error('Failed to stop container:', error);
    } finally {
      setActionLoading(null);
    }
  };

  const handleRestart = async () => {
    if (!container) return;
    setActionLoading('restart');
    try {
      await restartContainer(container.host_id, container.id);
      handleAction();
    } catch (error) {
      console.error('Failed to restart container:', error);
    } finally {
      setActionLoading(null);
    }
  };

  const handleRemove = async () => {
    if (!container) return;
    if (!confirm(`Are you sure you want to remove "${container.name}"?`)) {
      return;
    }
    setActionLoading('remove');
    try {
      await removeContainer(container.host_id, container.id);
      onClose();
    } catch (error) {
      console.error('Failed to remove container:', error);
      setActionLoading(null);
    }
  };

  const stateClass = isRunning ? 'border-l-[var(--success)]' : isStopped ? 'border-l-[var(--danger)]' : 'border-l-[var(--warning)]';

  return (
    <>
      <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4" onClick={onClose}>
        <div className="bg-[var(--bg-primary)] border border-[var(--border)] rounded-lg w-full max-w-5xl max-h-[90vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
          <div className="sticky top-0 bg-[var(--bg-secondary)] border-b border-[var(--border)] p-4 flex justify-between items-center z-10">
            <h2 className="text-xl font-bold">Container Details</h2>
            <button onClick={onClose} className="text-2xl hover:opacity-70 px-2">×</button>
          </div>

          <div className="p-6">
            {loading && (
              <div className="text-center py-12 text-[var(--text-tertiary)]">
                Loading container details...
              </div>
            )}

            {error && (
              <div className="text-center py-12 text-[var(--danger)]">
                {error}
              </div>
            )}

            {!loading && !error && container && (
              <div className={`bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg overflow-hidden border-l-4 ${stateClass}`}>
                {/* Header */}
                <div className="p-4 border-b border-[var(--border)]">
                  <div className="flex items-start justify-between">
                    <div className="flex items-center gap-3">
                      <span className="text-2xl">{stateIcon}</span>
                      <div>
                        <h3 className="font-semibold text-lg">{container.name}</h3>
                        <div className="flex items-center gap-2 text-sm text-[var(--text-tertiary)] flex-wrap">
                          <span>📍 {container.host_name}</span>
                          <span>•</span>
                          <span title={container.image}>🏷️ {extractImageTag(container.image, container.image_tags)}</span>
                          {uptime && (
                            <>
                              <span>•</span>
                              <span title="Current uptime">⏱️ {uptime}</span>
                            </>
                          )}
                          {lifetime && (
                            <>
                              <span>•</span>
                              <span title="Total lifetime">📅 {lifetime}</span>
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
                    <div className="flex items-center gap-2 flex-wrap">
                      <code className="text-sm bg-[var(--bg-tertiary)] px-2 py-1 rounded text-[var(--text-primary)]">{container.image}</code>
                      {container.update_available && (
                        <button
                          onClick={() => setUpdateContainer(container)}
                          className="text-xs bg-[var(--accent)] text-white px-2 py-1 rounded hover:opacity-80 transition-colors cursor-pointer"
                          title="Update container image"
                        >
                          ⬆️ Update
                        </button>
                      )}
                    </div>
                  </div>

                  {/* Ports */}
                  {container.ports && container.ports.length > 0 && container.ports.some(p => (p.public_port ?? 0) > 0) && (
                    <div>
                      <div className="text-xs text-[var(--text-tertiary)] mb-1">Ports</div>
                      <div className="flex flex-wrap gap-1">
                        {container.ports.filter(p => (p.public_port ?? 0) > 0).map((p, idx) => (
                          <span key={idx} className="text-xs bg-[var(--bg-tertiary)] px-2 py-1 rounded text-[var(--text-primary)]">
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
                      <div className="bg-[var(--bg-primary)] rounded-lg p-3 border border-[var(--border)]">
                        <div className="flex items-center gap-2 mb-1">
                          <span>💻</span>
                          <span className="text-xs text-[var(--text-secondary)]">CPU</span>
                        </div>
                        <div className="text-xl font-bold text-[var(--text-primary)]">{cpuDisplay}<span className="text-xs text-[var(--text-secondary)]">%</span></div>
                        <div className="h-1.5 bg-[var(--bg-tertiary)] rounded-full mt-2 overflow-hidden">
                          <div
                            className="h-full bg-[var(--accent)] rounded-full transition-all"
                            style={{ width: `${Math.min(parseFloat(cpuDisplay), 100)}%` }}
                          />
                        </div>
                      </div>

                      {/* Memory */}
                      <div className="bg-[var(--bg-primary)] rounded-lg p-3 border border-[var(--border)]">
                        <div className="flex items-center gap-2 mb-1">
                          <span>💾</span>
                          <span className="text-xs text-[var(--text-secondary)]">Memory</span>
                        </div>
                        <div className="text-xl font-bold text-[var(--text-primary)]">{memoryMB}<span className="text-xs text-[var(--text-secondary)]">MB</span></div>
                        <div className="text-xs text-[var(--text-secondary)]">of {memoryGB}GB ({memoryPercent}%)</div>
                        <div className="h-1.5 bg-[var(--bg-tertiary)] rounded-full mt-2 overflow-hidden">
                          <div
                            className="h-full bg-[var(--warning)] rounded-full transition-all"
                            style={{ width: `${Math.min(parseFloat(memoryPercent), 100)}%` }}
                          />
                        </div>
                      </div>
                    </div>
                  )}

                  {/* Inline Chart */}
                  {isRunning && (
                    <div className="mt-4 bg-[var(--bg-tertiary)] rounded-lg p-3 border border-[var(--border)]">
                      <div className="text-xs text-[var(--text-tertiary)] mb-2">Resource Usage (Last Hour)</div>
                      <InlineChart hostId={container.host_id} containerId={container.id} />
                    </div>
                  )}
                </div>

                {/* Footer Actions */}
                <div className="p-4 border-t border-[var(--border)] flex flex-wrap gap-2">
                  <button
                    onClick={() => setLogsContainer(container)}
                    className="px-3 py-1.5 text-sm border border-[var(--border)] text-[var(--text-secondary)] rounded hover:bg-[var(--bg-tertiary)] transition-colors"
                  >
                    📋 Logs
                  </button>

                  {isRunning && (
                    <button
                      onClick={() => setStatsContainer(container)}
                      className="px-3 py-1.5 text-sm border border-[var(--border)] text-[var(--text-secondary)] rounded hover:bg-[var(--bg-tertiary)] transition-colors"
                    >
                      📊 Stats
                    </button>
                  )}

                  <button
                    onClick={() => setHistoryContainer(container)}
                    className="px-3 py-1.5 text-sm border border-[var(--border)] text-[var(--text-secondary)] rounded hover:bg-[var(--bg-tertiary)] transition-colors"
                  >
                    📜 History
                  </button>

                  {isRunning && (
                    <>
                      <button
                        onClick={handleRestart}
                        disabled={actionLoading !== null}
                        className="px-3 py-1.5 text-sm border border-[var(--warning)] text-[var(--warning)] rounded hover:bg-[var(--warning)] hover:text-white transition-colors disabled:opacity-50"
                      >
                        {actionLoading === 'restart' ? '...' : '🔄 Restart'}
                      </button>
                      <button
                        onClick={handleStop}
                        disabled={actionLoading !== null}
                        className="px-3 py-1.5 text-sm border border-[var(--warning)] text-[var(--warning)] rounded hover:bg-[var(--warning)] hover:text-white transition-colors disabled:opacity-50"
                      >
                        {actionLoading === 'stop' ? '...' : '⏹ Stop'}
                      </button>
                    </>
                  )}
                  {isStopped && (
                    <>
                      <button
                        onClick={handleStart}
                        disabled={actionLoading !== null}
                        className="px-3 py-1.5 text-sm bg-[var(--success)] text-white rounded hover:opacity-80 transition-opacity disabled:opacity-50"
                      >
                        {actionLoading === 'start' ? '...' : '▶ Start'}
                      </button>
                      <button
                        onClick={handleRemove}
                        disabled={actionLoading !== null}
                        className="px-3 py-1.5 text-sm text-[var(--danger)] hover:bg-[var(--danger)] hover:text-white rounded transition-colors disabled:opacity-50"
                      >
                        {actionLoading === 'remove' ? '...' : '🗑 Remove'}
                      </button>
                    </>
                  )}
                  {isPaused && (
                    <>
                      <button
                        onClick={handleStart}
                        disabled={actionLoading !== null}
                        className="px-3 py-1.5 text-sm bg-[var(--success)] text-white rounded hover:opacity-80 transition-opacity disabled:opacity-50"
                      >
                        {actionLoading === 'start' ? '...' : '▶ Resume'}
                      </button>
                      <button
                        onClick={handleStop}
                        disabled={actionLoading !== null}
                        className="px-3 py-1.5 text-sm border border-[var(--warning)] text-[var(--warning)] rounded hover:bg-[var(--warning)] hover:text-white transition-colors disabled:opacity-50"
                      >
                        {actionLoading === 'stop' ? '...' : '⏹ Stop'}
                      </button>
                    </>
                  )}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Modals */}
      {logsContainer && <LogsModal container={logsContainer} onClose={() => setLogsContainer(null)} />}
      {statsContainer && <StatsModal container={statsContainer} onClose={() => setStatsContainer(null)} />}
      {historyContainer && <HistoryModal container={historyContainer} onClose={() => setHistoryContainer(null)} />}
      {updateContainer && <UpdateModal container={updateContainer} onClose={() => setUpdateContainer(null)} onUpdate={handleAction} />}
    </>
  );
}
