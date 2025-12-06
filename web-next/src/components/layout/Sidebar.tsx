'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useState, useEffect } from 'react';
import { getPluginTabs, getNotificationStatus, getHealth } from '@/lib/api';
import type { PluginTab, HealthStatus } from '@/types';

interface NavItem {
  href: string;
  label: string;
  icon: string;
  badge?: number;
}

const mainNavItems: NavItem[] = [
  { href: '/', label: 'Dashboard', icon: '📊' },
  { href: '/containers', label: 'Containers', icon: '📦' },
  { href: '/hosts', label: 'Hosts', icon: '🖥️' },
  { href: '/security', label: 'Security', icon: '🛡️' },
];

const bottomNavItems: NavItem[] = [
  { href: '/notifications', label: 'Notifications', icon: '🔔' },
  { href: '/settings', label: 'Settings', icon: '⚙️' },
];

export default function Sidebar() {
  const pathname = usePathname();
  const [pluginTabs, setPluginTabs] = useState<PluginTab[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [health, setHealth] = useState<HealthStatus | null>(null);

  useEffect(() => {
    // Load plugin tabs
    getPluginTabs()
      .then(setPluginTabs)
      .catch(console.error);

    // Load notification status
    getNotificationStatus()
      .then((status) => setUnreadCount(status.unread_count))
      .catch(console.error);

    // Load health/version info
    getHealth()
      .then(setHealth)
      .catch(console.error);

    // Refresh notification count periodically
    const interval = setInterval(() => {
      getNotificationStatus()
        .then((status) => setUnreadCount(status.unread_count))
        .catch(console.error);
    }, 30000);

    return () => clearInterval(interval);
  }, []);

  const isActive = (href: string) => {
    if (href === '/') return pathname === '/';
    return pathname.startsWith(href);
  };

  return (
    <aside className="w-64 bg-[var(--bg-secondary)] border-r border-[var(--border)] flex flex-col h-screen">
      {/* Logo/Header */}
      <div className="p-4 border-b border-[var(--border)]">
        <h1 className="text-xl font-bold text-[var(--text-primary)] flex items-center gap-2">
          <span>📦</span>
          <span>Container Census</span>
        </h1>
        {health && (
          <div className="mt-2 text-sm text-[var(--text-tertiary)]">
            {health.update_available ? (
              <a
                href={health.release_url}
                target="_blank"
                rel="noopener noreferrer"
                className="text-[var(--accent)] hover:underline"
              >
                v{health.version} → v{health.latest_version} ⬆️
              </a>
            ) : (
              <span>v{health.version}</span>
            )}
          </div>
        )}
      </div>

      {/* Navigation */}
      <nav className="flex-1 overflow-y-auto p-2">
        {/* Main navigation */}
        <div className="space-y-1">
          {mainNavItems.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
                isActive(item.href)
                  ? 'bg-[var(--accent)] text-white'
                  : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
              }`}
            >
              <span className="text-lg">{item.icon}</span>
              <span>{item.label}</span>
            </Link>
          ))}
        </div>

        {/* Integrations section - always expanded */}
        {pluginTabs.length > 0 && (
          <div className="mt-4">
            <div className="px-3 py-2 text-xs font-semibold text-[var(--text-tertiary)] uppercase tracking-wider">
              Integrations
            </div>
            <div className="space-y-1">
              {pluginTabs.map((tab) => (
                <Link
                  key={tab.id}
                  href={`/integrations/${tab.id}`}
                  className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
                    pathname === `/integrations/${tab.id}`
                      ? 'bg-[var(--accent)] text-white'
                      : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
                  }`}
                >
                  <span className="text-lg">{tab.icon}</span>
                  <span>{tab.label}</span>
                </Link>
              ))}
              <Link
                href="/integrations"
                className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
                  pathname === '/integrations'
                    ? 'bg-[var(--accent)] text-white'
                    : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
                }`}
              >
                <span className="text-lg">⚙️</span>
                <span>Manage Plugins</span>
              </Link>
            </div>
          </div>
        )}

        {/* Bottom navigation */}
        <div className="mt-4 pt-4 border-t border-[var(--border)] space-y-1">
          {bottomNavItems.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
                isActive(item.href)
                  ? 'bg-[var(--accent)] text-white'
                  : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
              }`}
            >
              <span className="text-lg">{item.icon}</span>
              <span>{item.label}</span>
              {item.href === '/notifications' && unreadCount > 0 && (
                <span className="ml-auto bg-[var(--danger)] text-white text-xs px-2 py-0.5 rounded-full">
                  {unreadCount}
                </span>
              )}
            </Link>
          ))}
        </div>
      </nav>
    </aside>
  );
}
