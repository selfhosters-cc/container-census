'use client';

import { useEffect, useState, useMemo, useRef, useCallback } from 'react';
import { getContainers, getHosts, startContainer, stopContainer, restartContainer, removeContainer, getContainerLogs, getContainerStats, updateContainer, bulkCheckUpdates, bulkUpdate, getContainerLifecycleEvents, getContainerLifecycleSummaries } from '@/lib/api';
import type { Container, Host, ContainerStatsPoint, ContainerLifecycleEvent, ContainerLifecycleSummary } from '@/types';

// Chart.js imports (will be loaded from CDN in production, this is for types)
declare const Chart: {
  new (ctx: CanvasRenderingContext2D, config: unknown): {
    destroy: () => void;
    update: () => void;
  };
  getChart: (id: string | HTMLCanvasElement) => { destroy: () => void } | undefined;
};

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
  const chartRef = useRef<ReturnType<typeof Chart.prototype.constructor> | null>(null);
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

// Logs Modal Component
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

interface ContainerCardProps {
  container: Container;
  lifecycleSummary?: ContainerLifecycleSummary;
  onAction: () => void;
  onViewLogs: (container: Container) => void;
  onViewStats: (container: Container) => void;
  onViewHistory: (container: Container) => void;
  onViewUpdate: (container: Container) => void;
}

function ContainerCard({ container, lifecycleSummary, onAction, onViewLogs, onViewStats, onViewHistory, onViewUpdate }: ContainerCardProps) {
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

  // Calculate current uptime (time since container started running)
  let uptime = '';
  if (isRunning && lifecycleSummary?.last_started) {
    uptime = formatUptime(lifecycleSummary.last_started);
  }

  // Calculate lifetime from lifecycle data (total time tracked, from first_seen to now)
  const lifetime = lifecycleSummary && lifecycleSummary.first_seen
    ? formatUptime(lifecycleSummary.first_seen)
    : null;

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
                    <span title="Current uptime">⏱️ {uptime}</span>
                  </>
                )}
                {lifetime && (
                  <>
                    <span>•</span>
                    <span title="Total lifetime (first seen to last seen)">📅 {lifetime}</span>
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
            <code className="text-sm bg-[var(--bg-tertiary)] px-2 py-1 rounded text-[var(--text-primary)]">{container.image}</code>
            {container.update_available && (
              <button
                onClick={() => onViewUpdate(container)}
                className="text-xs bg-[var(--accent)] text-white px-2 py-1 rounded hover:bg-[var(--accent-hover)] transition-colors cursor-pointer"
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

        {/* Stats - improved readability */}
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

        {/* Inline Chart - for running containers */}
        {isRunning && (
          <div className="mt-4 bg-[var(--bg-tertiary)] rounded-lg p-3 border border-[var(--border)]">
            <div className="text-xs text-[var(--text-tertiary)] mb-2">Resource Usage (Last Hour)</div>
            <InlineChart hostId={container.host_id} containerId={container.id} />
          </div>
        )}
      </div>

      {/* Footer Actions */}
      <div className="p-4 border-t border-[var(--border)] flex flex-wrap gap-2">
        {/* View Logs button - always available */}
        <button
          onClick={() => onViewLogs(container)}
          className="px-3 py-1.5 text-sm border border-[var(--border)] text-[var(--text-secondary)] rounded hover:bg-[var(--bg-tertiary)] transition-colors"
        >
          📋 Logs
        </button>

        {/* Stats button - only for running containers with stats */}
        {isRunning && (
          <button
            onClick={() => onViewStats(container)}
            className="px-3 py-1.5 text-sm border border-[var(--border)] text-[var(--text-secondary)] rounded hover:bg-[var(--bg-tertiary)] transition-colors"
          >
            📊 Stats
          </button>
        )}

        {/* History button - always available */}
        <button
          onClick={() => onViewHistory(container)}
          className="px-3 py-1.5 text-sm border border-[var(--border)] text-[var(--text-secondary)] rounded hover:bg-[var(--bg-tertiary)] transition-colors"
        >
          📜 History
        </button>

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
  const [lifecycleSummaries, setLifecycleSummaries] = useState<ContainerLifecycleSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState('');
  const [hostFilter, setHostFilter] = useState('');
  const [stateFilter, setStateFilter] = useState('');
  const [logsContainer, setLogsContainer] = useState<Container | null>(null);
  const [statsContainer, setStatsContainer] = useState<Container | null>(null);
  const [historyContainer, setHistoryContainer] = useState<Container | null>(null);
  const [updateContainer, setUpdateContainer] = useState<Container | null>(null);
  const [showBulkUpdateModal, setShowBulkUpdateModal] = useState(false);

  const loadData = async () => {
    try {
      const [containersData, hostsData, lifecycleData] = await Promise.all([
        getContainers(),
        getHosts(),
        getContainerLifecycleSummaries().catch(() => []), // Don't fail if lifecycle data unavailable
      ]);
      setContainers(containersData);
      setHosts(hostsData);
      setLifecycleSummaries(lifecycleData);
    } catch (error) {
      console.error('Failed to load data:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();

    // Load Chart.js from CDN
    const script = document.createElement('script');
    script.src = 'https://cdn.jsdelivr.net/npm/chart.js@4.4.0';
    script.async = true;
    document.body.appendChild(script);

    const interval = setInterval(loadData, 30000);
    return () => {
      clearInterval(interval);
      if (script.parentNode) script.parentNode.removeChild(script);
    };
  }, []);

  const filteredContainers = useMemo(() => {
    const validHostIds = new Set(hosts.map(h => h.id));
    return containers.filter(container => {
      // Filter out containers from deleted hosts
      if (!validHostIds.has(container.host_id)) {
        return false;
      }

      const matchesSearch = searchTerm === '' ||
        container.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        container.image.toLowerCase().includes(searchTerm.toLowerCase()) ||
        container.host_name.toLowerCase().includes(searchTerm.toLowerCase());

      const matchesHost = hostFilter === '' || container.host_id.toString() === hostFilter;
      const matchesState = stateFilter === '' || container.state === stateFilter;

      return matchesSearch && matchesHost && matchesState;
    });
  }, [containers, hosts, searchTerm, hostFilter, stateFilter]);

  const stats = useMemo(() => ({
    total: containers.length,
    running: containers.filter(c => c.state === 'running').length,
    stopped: containers.filter(c => c.state === 'exited').length,
    paused: containers.filter(c => c.state === 'paused').length,
  }), [containers]);

  // Create a lookup map for lifecycle data by container name and host
  const lifecycleMap = useMemo(() => {
    const map = new Map<string, ContainerLifecycleSummary>();
    lifecycleSummaries.forEach(summary => {
      const key = `${summary.host_id}-${summary.container_name}`;
      map.set(key, summary);
    });
    return map;
  }, [lifecycleSummaries]);

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
        <div className="flex items-center gap-4">
          <button
            onClick={() => setShowBulkUpdateModal(true)}
            className="px-4 py-2 text-sm bg-[var(--accent)] text-white rounded hover:bg-[var(--accent-hover)] transition-colors"
          >
            Check Updates
          </button>
          <div className="flex items-center gap-4 text-sm text-[var(--text-tertiary)]">
            <span>Total: {stats.total}</span>
            <span className="text-[var(--success)]">Running: {stats.running}</span>
            <span className="text-[var(--danger)]">Stopped: {stats.stopped}</span>
            {stats.paused > 0 && <span className="text-[var(--warning)]">Paused: {stats.paused}</span>}
          </div>
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
          {filteredContainers.map(container => {
            const lifecycleKey = `${container.host_id}-${container.name}`;
            const lifecycleSummary = lifecycleMap.get(lifecycleKey);

            return (
              <ContainerCard
                key={`${container.host_id}-${container.id}`}
                container={container}
                lifecycleSummary={lifecycleSummary}
                onAction={loadData}
                onViewLogs={setLogsContainer}
                onViewStats={setStatsContainer}
                onViewHistory={setHistoryContainer}
                onViewUpdate={setUpdateContainer}
              />
            );
          })}
        </div>
      )}

      {/* Modals */}
      <LogsModal container={logsContainer} onClose={() => setLogsContainer(null)} />
      <StatsModal container={statsContainer} onClose={() => setStatsContainer(null)} />
      <HistoryModal container={historyContainer} onClose={() => setHistoryContainer(null)} />
      <UpdateModal container={updateContainer} onClose={() => setUpdateContainer(null)} onUpdate={loadData} />
      <BulkUpdateModal isOpen={showBulkUpdateModal} onClose={() => setShowBulkUpdateModal(false)} containers={containers} onUpdate={loadData} />
    </div>
  );
}
