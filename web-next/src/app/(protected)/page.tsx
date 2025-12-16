'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { getContainers, getHosts, getNotificationStatus, getTelemetryEndpoints, getTelemetrySchedule, updateTelemetryEndpoint } from '@/lib/api';
import type { Container, Host, TelemetryEndpoint, TelemetrySchedule } from '@/types';

function StatCard({ label, value, icon, color = 'text-[var(--text-primary)]' }: {
  label: string;
  value: number | string;
  icon: string;
  color?: string;
}) {
  return (
    <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm text-[var(--text-tertiary)]">{label}</p>
          <p className={`text-2xl font-bold ${color}`}>{value}</p>
        </div>
        <span className="text-3xl">{icon}</span>
      </div>
    </div>
  );
}

function ContainersByState({ containers }: { containers: Container[] }) {
  const running = containers.filter(c => c.state === 'running').length;
  const stopped = containers.filter(c => c.state === 'exited').length;
  const paused = containers.filter(c => c.state === 'paused').length;
  const other = containers.length - running - stopped - paused;

  const total = containers.length;
  const runningPct = total ? (running / total) * 100 : 0;
  const stoppedPct = total ? (stopped / total) * 100 : 0;
  const pausedPct = total ? (paused / total) * 100 : 0;
  const otherPct = total ? (other / total) * 100 : 0;

  return (
    <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4">
      <h3 className="text-lg font-semibold mb-4">Containers by State</h3>
      <div className="h-4 bg-[var(--bg-tertiary)] rounded-full overflow-hidden flex">
        {runningPct > 0 && (
          <div
            className="bg-[var(--success)] h-full"
            style={{ width: `${runningPct}%` }}
            title={`Running: ${running}`}
          />
        )}
        {stoppedPct > 0 && (
          <div
            className="bg-[var(--danger)] h-full"
            style={{ width: `${stoppedPct}%` }}
            title={`Stopped: ${stopped}`}
          />
        )}
        {pausedPct > 0 && (
          <div
            className="bg-[var(--warning)] h-full"
            style={{ width: `${pausedPct}%` }}
            title={`Paused: ${paused}`}
          />
        )}
        {otherPct > 0 && (
          <div
            className="bg-[var(--text-tertiary)] h-full"
            style={{ width: `${otherPct}%` }}
            title={`Other: ${other}`}
          />
        )}
      </div>
      <div className="flex justify-between mt-2 text-sm">
        <span className="flex items-center gap-1">
          <span className="w-3 h-3 bg-[var(--success)] rounded-full"></span>
          Running ({running})
        </span>
        <span className="flex items-center gap-1">
          <span className="w-3 h-3 bg-[var(--danger)] rounded-full"></span>
          Stopped ({stopped})
        </span>
        <span className="flex items-center gap-1">
          <span className="w-3 h-3 bg-[var(--warning)] rounded-full"></span>
          Paused ({paused})
        </span>
      </div>
    </div>
  );
}

function HostsList({ hosts }: { hosts: Host[] }) {
  return (
    <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4">
      <h3 className="text-lg font-semibold mb-4">Hosts</h3>
      <div className="space-y-2">
        {hosts.map((host) => (
          <div
            key={host.id}
            className="flex items-center justify-between p-2 bg-[var(--bg-tertiary)] rounded"
          >
            <div className="flex items-center gap-2">
              <span className={`w-2 h-2 rounded-full ${host.enabled ? 'bg-[var(--success)]' : 'bg-[var(--text-tertiary)]'}`}></span>
              <span className="font-medium">{host.name}</span>
            </div>
            <span className="text-sm text-[var(--text-tertiary)]">
              {host.running_count ?? 0}/{host.container_count ?? 0} running
            </span>
          </div>
        ))}
        {hosts.length === 0 && (
          <p className="text-[var(--text-tertiary)] text-center py-4">
            No hosts configured
          </p>
        )}
      </div>
    </div>
  );
}

function RecentContainers({ containers }: { containers: Container[] }) {
  // Show the 5 most recently created containers
  const recent = [...containers]
    .sort((a, b) => new Date(b.created).getTime() - new Date(a.created).getTime())
    .slice(0, 5);

  return (
    <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4">
      <h3 className="text-lg font-semibold mb-4">Recent Containers</h3>
      <div className="space-y-2">
        {recent.map((container) => (
          <div
            key={`${container.host_id}-${container.id}`}
            className="flex items-center justify-between p-2 bg-[var(--bg-tertiary)] rounded"
          >
            <div className="flex items-center gap-2">
              <span className={`w-2 h-2 rounded-full ${
                container.state === 'running' ? 'bg-[var(--success)]' :
                container.state === 'exited' ? 'bg-[var(--danger)]' :
                'bg-[var(--warning)]'
              }`}></span>
              <span className="font-medium truncate max-w-[150px]" title={container.name}>
                {container.name.replace(/^\//, '')}
              </span>
            </div>
            <span className="text-xs text-[var(--text-tertiary)]">
              {container.host_name}
            </span>
          </div>
        ))}
        {recent.length === 0 && (
          <p className="text-[var(--text-tertiary)] text-center py-4">
            No containers found
          </p>
        )}
      </div>
    </div>
  );
}

function CommunityAnalyticsCard({
  endpoints,
  schedule,
  onToggle,
  isToggling
}: {
  endpoints: TelemetryEndpoint[];
  schedule: TelemetrySchedule | null;
  onToggle: (enabled: boolean) => void;
  isToggling: boolean;
}) {
  const communityEndpoint = endpoints.find(e => e.name === 'community');
  const isEnabled = communityEndpoint?.enabled ?? false;

  return (
    <div className="bg-[var(--bg-secondary)] border border-[var(--accent)] rounded-lg p-4">
      {/* Header */}
      <div className="flex items-start justify-between mb-4 pb-4 border-b border-[var(--border)]">
        <div>
          <h3 className="text-lg font-semibold">Community Analytics</h3>
          <p className="text-sm text-[var(--text-tertiary)]">
            Help improve Container Census by sharing anonymous usage statistics
          </p>
        </div>
        <div className="flex flex-col items-center gap-1 p-2 bg-[var(--bg-tertiary)] rounded-lg border border-[var(--border)]">
          <span className="text-xs text-[var(--text-tertiary)]">Share Data</span>
          <button
            onClick={() => onToggle(!isEnabled)}
            disabled={isToggling || !communityEndpoint}
            className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
              isEnabled ? 'bg-[var(--accent)]' : 'bg-[var(--bg-tertiary)]'
            } ${isToggling ? 'opacity-50 cursor-wait' : ''}`}
          >
            <span
              className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                isEnabled ? 'translate-x-6' : 'translate-x-1'
              }`}
            />
          </button>
        </div>
      </div>

      {/* Content */}
      <div className="mb-4">
        {!communityEndpoint ? (
          <div className="p-3 bg-[var(--bg-tertiary)] rounded-lg text-center">
            <p className="text-[var(--text-tertiary)]">Telemetry status unavailable</p>
          </div>
        ) : !isEnabled ? (
          <div className="p-3 bg-[rgba(245,158,11,0.1)] rounded-lg border border-[var(--warning)]">
            <p className="text-[var(--warning)] font-semibold mb-1">Telemetry Disabled</p>
            <p className="text-sm text-[var(--text-secondary)]">
              Enable telemetry to contribute anonymous usage statistics and help improve Container Census.
            </p>
          </div>
        ) : (
          <div className="space-y-2">
            <div className="flex justify-between items-center">
              <span className="text-[var(--text-secondary)]">Status</span>
              <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-[var(--success)] text-white">
                Active
              </span>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-[var(--text-secondary)]">Next Submission</span>
              <span className="font-semibold">
                {schedule?.next_submission
                  ? new Date(schedule.next_submission).toLocaleString()
                  : 'Unknown'}
              </span>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-[var(--text-secondary)]">Frequency</span>
              <span className="font-semibold">
                {schedule?.interval_hours ? `${schedule.interval_hours}h` : 'Unknown'}
              </span>
            </div>
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="flex gap-3 pt-4 border-t border-[var(--border)]">
        <a
          href="https://selfhosters.cc/stats"
          target="_blank"
          rel="noopener noreferrer"
          className="flex-1 px-4 py-2 text-sm text-center bg-[var(--bg-tertiary)] border border-[var(--border)] rounded hover:bg-[var(--bg-primary)] transition-colors"
        >
          View Community Dashboard
        </a>
        <Link
          href="/settings"
          className="flex-1 px-4 py-2 text-sm text-center bg-[var(--accent)] text-white rounded hover:opacity-90 transition-opacity"
        >
          Configure Telemetry
        </Link>
      </div>
    </div>
  );
}

export default function DashboardPage() {
  const [containers, setContainers] = useState<Container[]>([]);
  const [hosts, setHosts] = useState<Host[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [telemetryEndpoints, setTelemetryEndpoints] = useState<TelemetryEndpoint[]>([]);
  const [telemetrySchedule, setTelemetrySchedule] = useState<TelemetrySchedule | null>(null);
  const [isTogglingTelemetry, setIsTogglingTelemetry] = useState(false);
  const [loading, setLoading] = useState(true);

  const loadTelemetryData = async () => {
    try {
      const [endpoints, schedule] = await Promise.all([
        getTelemetryEndpoints().catch(() => []),
        getTelemetrySchedule().catch(() => null),
      ]);
      setTelemetryEndpoints(endpoints);
      setTelemetrySchedule(schedule);
    } catch (error) {
      console.error('Failed to load telemetry data:', error);
    }
  };

  const handleTelemetryToggle = async (enabled: boolean) => {
    const communityEndpoint = telemetryEndpoints.find(e => e.name === 'community');
    if (!communityEndpoint) return;

    setIsTogglingTelemetry(true);
    try {
      await updateTelemetryEndpoint(communityEndpoint.name, { enabled });
      await loadTelemetryData();
    } catch (error) {
      console.error('Failed to toggle telemetry:', error);
    } finally {
      setIsTogglingTelemetry(false);
    }
  };

  useEffect(() => {
    async function loadData() {
      try {
        const [containersData, hostsData, notifStatus] = await Promise.all([
          getContainers(),
          getHosts(),
          getNotificationStatus().catch(() => ({ unread_count: 0 })),
        ]);
        setContainers(containersData);
        setHosts(hostsData);
        setUnreadCount(notifStatus.unread_count);
      } catch (error) {
        console.error('Failed to load dashboard data:', error);
      } finally {
        setLoading(false);
      }
    }

    loadData();
    loadTelemetryData();

    // Refresh every 30 seconds
    const interval = setInterval(() => {
      loadData();
      loadTelemetryData();
    }, 30000);
    return () => clearInterval(interval);
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-[var(--text-tertiary)]">Loading...</div>
      </div>
    );
  }

  const running = containers.filter(c => c.state === 'running').length;
  const stopped = containers.filter(c => c.state === 'exited').length;

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Dashboard</h1>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Total Hosts" value={hosts.length} icon="🖥️" />
        <StatCard label="Total Containers" value={containers.length} icon="📦" />
        <StatCard
          label="Running"
          value={running}
          icon="▶️"
          color="text-[var(--success)]"
        />
        <StatCard
          label="Stopped"
          value={stopped}
          icon="⏹️"
          color="text-[var(--danger)]"
        />
      </div>

      {/* Charts and Lists */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <ContainersByState containers={containers} />
        <HostsList hosts={hosts} />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <RecentContainers containers={containers} />
        <CommunityAnalyticsCard
          endpoints={telemetryEndpoints}
          schedule={telemetrySchedule}
          onToggle={handleTelemetryToggle}
          isToggling={isTogglingTelemetry}
        />
      </div>
    </div>
  );
}
