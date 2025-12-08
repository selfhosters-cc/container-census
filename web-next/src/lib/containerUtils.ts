import type { Port } from '@/types';

/**
 * Format container uptime from started timestamp
 */
export function formatUptime(startedAt: string | undefined, endAt?: string): string {
  if (!startedAt) return '-';

  const end = endAt ? new Date(endAt) : new Date();
  const started = new Date(startedAt);

  if (isNaN(started.getTime()) || started.getFullYear() < 2000) {
    return '-';
  }

  const diff = end.getTime() - started.getTime();
  if (diff < 0) return '-';

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

/**
 * Extract image tag from image string or image tags array
 */
export function extractImageTag(image: string, imageTags?: string[]): string {
  if (imageTags && imageTags.length > 0) {
    const tag = imageTags[0];
    const parts = tag.split(':');
    return parts[parts.length - 1] || 'latest';
  }
  const parts = image.split(':');
  return parts[parts.length - 1] || 'latest';
}

/**
 * Format container ports for display
 */
export function formatPorts(ports: Port[] | undefined, maxDisplay = 3): string {
  if (!ports || ports.length === 0) return '-';

  const formatted = ports.slice(0, maxDisplay).map(p => {
    if (p.public_port) {
      return `${p.public_port}:${p.private_port}`;
    }
    return `${p.private_port}`;
  });

  if (ports.length > maxDisplay) {
    formatted.push(`+${ports.length - maxDisplay} more`);
  }

  return formatted.join(', ');
}

/**
 * Format memory usage with limit and percentage
 */
export function formatMemory(usage?: number, limit?: number): string {
  if (!usage) return '-';

  const usageMB = Math.round(usage / 1024 / 1024);

  if (!limit) return `${usageMB}M`;

  const limitMB = Math.round(limit / 1024 / 1024);
  const percent = Math.round((usage / limit) * 100);

  return `${usageMB}M / ${limitMB}M (${percent}%)`;
}

/**
 * Format lifetime duration string
 */
export function formatLifetime(lifetime?: string): string {
  return lifetime || '-';
}

/**
 * Get Tailwind CSS classes for state badge
 */
export function getStateBadgeClass(state: string): string {
  const stateClasses: Record<string, string> = {
    running: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
    exited: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-400',
    paused: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
    restarting: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400',
    dead: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
    created: 'bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-400',
    removing: 'bg-orange-100 text-orange-800 dark:bg-orange-900/30 dark:text-orange-400',
  };

  return stateClasses[state.toLowerCase()] || stateClasses.exited;
}

/**
 * Get emoji icon for container state
 */
export function getStateIcon(state: string): string {
  const icons: Record<string, string> = {
    running: '🟢',
    exited: '🔴',
    paused: '⏸️',
    restarting: '🔄',
    dead: '💀',
    created: '⚪',
    removing: '🗑️',
  };

  return icons[state.toLowerCase()] || '⚪';
}

/**
 * Format CPU percentage
 */
export function formatCpu(cpuPercent?: number): string {
  if (cpuPercent === undefined || cpuPercent === null) return '-';
  return `${cpuPercent.toFixed(1)}%`;
}

/**
 * Format memory percentage
 */
export function formatMemoryPercent(memoryPercent?: number): string {
  if (memoryPercent === undefined || memoryPercent === null) return '-';
  return `${memoryPercent.toFixed(1)}%`;
}
