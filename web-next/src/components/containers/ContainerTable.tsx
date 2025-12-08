'use client';

import { useState, useMemo, Fragment } from 'react';
import type { Container, ContainerLifecycleSummary } from '@/types';
import {
  formatPorts,
  formatMemory,
  formatUptime,
  formatLifetime,
  formatCpu,
  getStateBadgeClass,
  getStateIcon,
  extractImageTag,
} from '@/lib/containerUtils';
import InlineChart from './InlineChart';

interface ContainerTableProps {
  containers: Container[];
  lifecycleSummaries: Map<string, ContainerLifecycleSummary>;
  selectedContainers: Set<string>;
  onToggleSelection: (id: string) => void;
  onToggleSelectAll: () => void;
  onShowLogs: (container: Container) => void;
  onShowStats: (container: Container) => void;
  onShowHistory: (container: Container) => void;
  onAction: (container: Container, action: 'start' | 'stop' | 'restart' | 'remove') => void;
  onUpdate: (container: Container) => void;
}

type SortKey = 'name' | 'state' | 'host' | 'image' | 'uptime' | 'lifetime' | 'cpu' | 'memory';
type SortDirection = 'asc' | 'desc';

export default function ContainerTable({
  containers,
  lifecycleSummaries,
  selectedContainers,
  onToggleSelection,
  onToggleSelectAll,
  onShowLogs,
  onShowStats,
  onShowHistory,
  onAction,
  onUpdate,
}: ContainerTableProps) {
  const [sortConfig, setSortConfig] = useState<{ key: SortKey; direction: SortDirection } | null>(null);
  const [loadingActions, setLoadingActions] = useState<Map<string, string>>(new Map()); // Map<containerId, action>

  const handleAction = async (container: Container, action: 'start' | 'stop' | 'restart' | 'remove') => {
    // Set loading state
    setLoadingActions(prev => new Map(prev).set(container.id, action));

    try {
      // Call the parent handler
      await onAction(container, action);
    } finally {
      // Clear loading state
      setLoadingActions(prev => {
        const next = new Map(prev);
        next.delete(container.id);
        return next;
      });
    }
  };

  const handleSort = (key: SortKey) => {
    setSortConfig((prev) => {
      if (prev?.key === key) {
        // Toggle direction
        return { key, direction: prev.direction === 'asc' ? 'desc' : 'asc' };
      }
      // New sort key, default to ascending
      return { key, direction: 'asc' };
    });
  };

  const sortedContainers = useMemo(() => {
    if (!sortConfig) return containers;

    const sorted = [...containers].sort((a, b) => {
      let aVal: string | number | undefined;
      let bVal: string | number | undefined;

      switch (sortConfig.key) {
        case 'name':
          aVal = a.name.toLowerCase();
          bVal = b.name.toLowerCase();
          break;
        case 'state':
          // running > paused > exited
          const stateOrder: Record<string, number> = { running: 0, paused: 1, exited: 2, restarting: 3, dead: 4 };
          aVal = stateOrder[a.state] ?? 99;
          bVal = stateOrder[b.state] ?? 99;
          break;
        case 'host':
          aVal = a.host_name.toLowerCase();
          bVal = b.host_name.toLowerCase();
          break;
        case 'image':
          aVal = a.image.toLowerCase();
          bVal = b.image.toLowerCase();
          break;
        case 'uptime':
          // Calculate uptime in seconds
          aVal = a.started_at ? new Date().getTime() - new Date(a.started_at).getTime() : 0;
          bVal = b.started_at ? new Date().getTime() - new Date(b.started_at).getTime() : 0;
          break;
        case 'lifetime':
          // Get from lifecycle summary
          const aLifetime = lifecycleSummaries.get(`${a.id}-${a.host_id}`);
          const bLifetime = lifecycleSummaries.get(`${b.id}-${b.host_id}`);
          aVal = aLifetime?.first_seen ? new Date().getTime() - new Date(aLifetime.first_seen).getTime() : 0;
          bVal = bLifetime?.first_seen ? new Date().getTime() - new Date(bLifetime.first_seen).getTime() : 0;
          break;
        case 'cpu':
          aVal = a.cpu_percent ?? -1; // Sort nulls to end
          bVal = b.cpu_percent ?? -1;
          break;
        case 'memory':
          aVal = a.memory_usage ?? -1; // Sort nulls to end
          bVal = b.memory_usage ?? -1;
          break;
        default:
          return 0;
      }

      if (typeof aVal === 'string' && typeof bVal === 'string') {
        return sortConfig.direction === 'asc'
          ? aVal.localeCompare(bVal)
          : bVal.localeCompare(aVal);
      }

      if (typeof aVal === 'number' && typeof bVal === 'number') {
        return sortConfig.direction === 'asc' ? aVal - bVal : bVal - aVal;
      }

      return 0;
    });

    return sorted;
  }, [containers, sortConfig, lifecycleSummaries]);

  const SortHeader = ({ column, children }: { column: SortKey; children: React.ReactNode }) => (
    <th
      onClick={() => handleSort(column)}
      className="px-4 py-3 text-left cursor-pointer hover:bg-[var(--bg-hover)] transition-colors select-none"
    >
      <div className="flex items-center gap-1">
        {children}
        {sortConfig?.key === column && (
          <span className="text-xs">{sortConfig.direction === 'asc' ? '↑' : '↓'}</span>
        )}
      </div>
    </th>
  );

  return (
    <div className="table-container border border-[var(--border)] rounded-lg overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full min-w-[1200px]">
          <thead className="bg-[var(--bg-secondary)] border-b border-[var(--border)] sticky top-0 z-10">
            <tr>
              <th className="w-10 px-4 py-3">
                <input
                  type="checkbox"
                  checked={selectedContainers.size === containers.length && containers.length > 0}
                  onChange={onToggleSelectAll}
                  className="checkbox cursor-pointer"
                />
              </th>
              <SortHeader column="name">Name</SortHeader>
              <SortHeader column="state">State</SortHeader>
              <SortHeader column="host">Host</SortHeader>
              <SortHeader column="image">Image</SortHeader>
              <SortHeader column="uptime">Uptime</SortHeader>
              <SortHeader column="lifetime">
                <span className="hidden xl:inline">Lifetime</span>
                <span className="xl:hidden">Life</span>
              </SortHeader>
              <th className="px-4 py-3 text-left">
                <span className="hidden lg:inline">Ports</span>
                <span className="lg:hidden">P</span>
              </th>
              <SortHeader column="cpu">
                <span className="hidden lg:inline">CPU %</span>
                <span className="lg:hidden">CPU</span>
              </SortHeader>
              <SortHeader column="memory">
                <span className="hidden lg:inline">Memory</span>
                <span className="lg:hidden">Mem</span>
              </SortHeader>
              <th className="px-4 py-3 text-left">Actions</th>
            </tr>
          </thead>
          <tbody>
            {sortedContainers.length === 0 ? (
              <tr>
                <td colSpan={11} className="px-4 py-8 text-center text-[var(--text-secondary)]">
                  No containers found matching current filters
                </td>
              </tr>
            ) : (
              sortedContainers.map((container) => {
                const lifecycle = lifecycleSummaries.get(`${container.id}-${container.host_id}`);
                const isSelected = selectedContainers.has(container.id);
                const isRunning = container.state === 'running';

                return (
                  <Fragment key={`${container.id}-${container.host_id}`}>
                    {/* Main data row */}
                    <tr className="border-b border-[var(--border)] hover:bg-[var(--bg-hover)] transition-colors">
                      <td className="px-4 py-3">
                        <input
                          type="checkbox"
                          checked={isSelected}
                          onChange={() => onToggleSelection(container.id)}
                          className="checkbox cursor-pointer"
                        />
                      </td>
                      <td className="px-4 py-3 font-medium truncate max-w-[200px]" title={container.name}>
                        {container.name}
                      </td>
                      <td className="px-4 py-3">
                        <span
                          className={`inline-flex items-center gap-1 px-2 py-1 rounded text-xs font-medium ${getStateBadgeClass(
                            container.state
                          )}`}
                        >
                          <span>{getStateIcon(container.state)}</span>
                          <span>{container.state}</span>
                        </span>
                      </td>
                      <td className="px-4 py-3 truncate max-w-[150px]" title={container.host_name}>
                        {container.host_name}
                      </td>
                      <td className="px-4 py-3 truncate max-w-[250px]" title={container.image}>
                        <div className="flex items-center gap-2">
                          <span className="truncate">{container.image}</span>
                          {container.update_available && (
                            <button
                              onClick={() => onUpdate(container)}
                              className="text-xs bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400 px-2 py-0.5 rounded hover:bg-blue-200 dark:hover:bg-blue-900/50 transition-colors flex-shrink-0"
                              title="Update available"
                            >
                              ⬆️ Update
                            </button>
                          )}
                        </div>
                      </td>
                      <td className="px-4 py-3 text-sm">{formatUptime(container.started_at)}</td>
                      <td className="px-4 py-3 text-sm hidden xl:table-cell">
                        {lifecycle ? formatUptime(lifecycle.first_seen) : '-'}
                      </td>
                      <td className="px-4 py-3 text-sm hidden lg:table-cell truncate max-w-[180px]" title={formatPorts(container.ports, 10)}>
                        {formatPorts(container.ports)}
                      </td>
                      <td className="px-4 py-3 text-sm hidden lg:table-cell">{formatCpu(container.cpu_percent)}</td>
                      <td className="px-4 py-3 text-sm hidden lg:table-cell" title={formatMemory(container.memory_usage, container.memory_limit)}>
                        {formatMemory(container.memory_usage, container.memory_limit)}
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-1">
                          {loadingActions.get(container.id) ? (
                            <span className="text-xs text-[var(--text-secondary)] animate-pulse">
                              {loadingActions.get(container.id)}...
                            </span>
                          ) : (
                            <>
                              <button
                                onClick={() => onShowLogs(container)}
                                className="btn-icon"
                                title="View logs"
                              >
                                📄
                              </button>
                              {isRunning && (
                                <button
                                  onClick={() => onShowStats(container)}
                                  className="btn-icon"
                                  title="View stats"
                                >
                                  📊
                                </button>
                              )}
                              <button
                                onClick={() => onShowHistory(container)}
                                className="btn-icon"
                                title="View history"
                              >
                                📜
                              </button>
                              {isRunning ? (
                                <>
                                  <button
                                    onClick={() => handleAction(container, 'restart')}
                                    className="btn-icon"
                                    title="Restart"
                                  >
                                    🔄
                                  </button>
                                  <button
                                    onClick={() => handleAction(container, 'stop')}
                                    className="btn-icon"
                                    title="Stop"
                                  >
                                    ⏹️
                                  </button>
                                </>
                              ) : (
                                <>
                                  <button
                                    onClick={() => handleAction(container, 'start')}
                                    className="btn-icon"
                                    title="Start"
                                  >
                                    ▶️
                                  </button>
                                  <button
                                    onClick={() => handleAction(container, 'remove')}
                                    className="btn-icon text-red-600 dark:text-red-400"
                                    title="Remove"
                                  >
                                    🗑️
                                  </button>
                                </>
                              )}
                            </>
                          )}
                        </div>
                      </td>
                    </tr>

                    {/* Chart row - only for running containers */}
                    {isRunning && (
                      <tr className="border-b border-[var(--border)] bg-[var(--bg-primary)]">
                        <td colSpan={11} className="p-2">
                          <InlineChart hostId={container.host_id} containerId={container.id} compact={true} />
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
