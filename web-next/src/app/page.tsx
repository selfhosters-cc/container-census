'use client';

import { useEffect, useState } from 'react';
import { getContainers, getHosts, getVulnerabilitySummary, getNotificationStatus } from '@/lib/api';
import type { Container, Host, VulnerabilitySummary } from '@/types';

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

export default function DashboardPage() {
  const [containers, setContainers] = useState<Container[]>([]);
  const [hosts, setHosts] = useState<Host[]>([]);
  const [vulnSummary, setVulnSummary] = useState<VulnerabilitySummary | null>(null);
  const [unreadCount, setUnreadCount] = useState(0);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function loadData() {
      try {
        const [containersData, hostsData, vulnData, notifStatus] = await Promise.all([
          getContainers(),
          getHosts(),
          getVulnerabilitySummary().catch(() => null),
          getNotificationStatus().catch(() => ({ unread_count: 0 })),
        ]);
        setContainers(containersData);
        setHosts(hostsData);
        setVulnSummary(vulnData);
        setUnreadCount(notifStatus.unread_count);
      } catch (error) {
        console.error('Failed to load dashboard data:', error);
      } finally {
        setLoading(false);
      }
    }

    loadData();

    // Refresh every 30 seconds
    const interval = setInterval(loadData, 30000);
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

      {/* Vulnerability Stats */}
      {vulnSummary && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          <StatCard
            label="Critical Vulnerabilities"
            value={vulnSummary.critical_count ?? 0}
            icon="🚨"
            color={(vulnSummary.critical_count ?? 0) > 0 ? 'text-[var(--danger)]' : 'text-[var(--text-primary)]'}
          />
          <StatCard
            label="High Vulnerabilities"
            value={vulnSummary.high_count ?? 0}
            icon="⚠️"
            color={(vulnSummary.high_count ?? 0) > 0 ? 'text-[var(--warning)]' : 'text-[var(--text-primary)]'}
          />
          <StatCard
            label="Scanned Images"
            value={`${vulnSummary.scanned_images ?? 0}/${vulnSummary.total_images ?? 0}`}
            icon="🔍"
          />
          <StatCard
            label="Unread Notifications"
            value={unreadCount}
            icon="🔔"
            color={unreadCount > 0 ? 'text-[var(--accent)]' : 'text-[var(--text-primary)]'}
          />
        </div>
      )}

      {/* Charts and Lists */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <ContainersByState containers={containers} />
        <HostsList hosts={hosts} />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <RecentContainers containers={containers} />
      </div>
    </div>
  );
}
