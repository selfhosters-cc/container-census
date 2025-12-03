'use client';

import { useState, useEffect, useRef } from 'react';
import Link from 'next/link';
import { getHealth, getNotificationStatus, getNotificationLog, markAllNotificationsRead } from '@/lib/api';
import type { HealthStatus, NotificationLog } from '@/types';

interface HeaderProps {
  onScan: () => void;
  onTelemetry: () => void;
}

export default function Header({ onScan, onTelemetry }: HeaderProps) {
  const [health, setHealth] = useState<HealthStatus | null>(null);
  const [unreadCount, setUnreadCount] = useState(0);
  const [notifications, setNotifications] = useState<NotificationLog[]>([]);
  const [showNotifications, setShowNotifications] = useState(false);
  const [showHelp, setShowHelp] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [scanToasts, setScanToasts] = useState<{ id: number; message: string; type: 'info' | 'success' | 'error' }[]>([]);
  const toastIdCounter = useRef(0);
  const notificationRef = useRef<HTMLDivElement>(null);
  const helpRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    // Load health and notification status
    getHealth().then(setHealth).catch(console.error);
    getNotificationStatus()
      .then(status => setUnreadCount(status.unread_count))
      .catch(console.error);
    getNotificationLog(10)
      .then(setNotifications)
      .catch(console.error);

    // Refresh notification count periodically
    const interval = setInterval(() => {
      getNotificationStatus()
        .then(status => setUnreadCount(status.unread_count))
        .catch(console.error);
    }, 30000);

    return () => clearInterval(interval);
  }, []);

  // Close dropdowns when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (notificationRef.current && !notificationRef.current.contains(event.target as Node)) {
        setShowNotifications(false);
      }
      if (helpRef.current && !helpRef.current.contains(event.target as Node)) {
        setShowHelp(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const addToast = (message: string, type: 'info' | 'success' | 'error', duration: number = 3000) => {
    const id = toastIdCounter.current++;
    setScanToasts(prev => [...prev, { id, message, type }]);

    setTimeout(() => {
      setScanToasts(prev => prev.filter(toast => toast.id !== id));
    }, duration);

    return id;
  };

  const removeToast = (id: number) => {
    setScanToasts(prev => prev.filter(toast => toast.id !== id));
  };

  const handleScan = async () => {
    setScanning(true);
    const scanningToastId = addToast('Scanning all hosts...', 'info', 60000); // Long duration, will be replaced
    try {
      await onScan();
      removeToast(scanningToastId); // Remove scanning toast immediately
      addToast('Scan completed successfully!', 'success', 3000);
    } catch (error) {
      removeToast(scanningToastId); // Remove scanning toast immediately
      addToast('Scan failed. Please try again.', 'error', 5000);
    } finally {
      setScanning(false);
    }
  };

  const handleMarkAllRead = async () => {
    try {
      await markAllNotificationsRead();
      setUnreadCount(0);
      setNotifications(prev => prev.map(n => ({ ...n, read: true })));
    } catch (error) {
      console.error('Failed to mark notifications as read:', error);
    }
  };

  return (
    <header className="h-14 bg-[var(--bg-secondary)] border-b border-[var(--border)] flex items-center justify-between px-4">
      {/* Left side - Logo */}
      <div className="flex items-center gap-3">
        <Link href="/" className="flex items-center gap-2 text-lg font-bold hover:opacity-80">
          <span>Census</span>
        </Link>
        {health && (
          <span className="text-xs px-2 py-1 rounded bg-[var(--bg-tertiary)] text-[var(--text-tertiary)]">
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
              `v${health.version}`
            )}
          </span>
        )}
      </div>

      {/* Right side - Action buttons */}
      <div className="flex items-center gap-1">
        {/* Scan button */}
        <button
          onClick={handleScan}
          disabled={scanning}
          className="p-2 rounded hover:bg-[var(--bg-tertiary)] transition-colors disabled:opacity-50"
          title="Trigger Scan"
        >
          <span className={scanning ? 'animate-spin inline-block' : ''}>{scanning ? '⏳' : '🔄'}</span>
        </button>

        {/* Telemetry button */}
        <button
          onClick={onTelemetry}
          className="p-2 rounded hover:bg-[var(--bg-tertiary)] transition-colors"
          title="Submit Telemetry"
        >
          <span>📡</span>
        </button>

        {/* Help dropdown */}
        <div ref={helpRef} className="relative">
          <button
            onClick={() => setShowHelp(!showHelp)}
            className="p-2 rounded hover:bg-[var(--bg-tertiary)] transition-colors"
            title="Help"
          >
            <span>❓</span>
          </button>
          {showHelp && (
            <div className="absolute right-0 top-full mt-2 w-48 bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg shadow-lg z-50 overflow-hidden">
              <a
                href="https://github.com/selfhosters-cc/container-census"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 px-4 py-2 hover:bg-[var(--bg-tertiary)] text-sm"
              >
                <span>📚</span> Documentation
              </a>
              <a
                href="https://github.com/selfhosters-cc/container-census/issues"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 px-4 py-2 hover:bg-[var(--bg-tertiary)] text-sm"
              >
                <span>💬</span> Give Feedback
              </a>
            </div>
          )}
        </div>

        {/* Settings link */}
        <Link
          href="/settings"
          className="p-2 rounded hover:bg-[var(--bg-tertiary)] transition-colors"
          title="Settings"
        >
          <span>⚙️</span>
        </Link>

        {/* Notifications dropdown */}
        <div ref={notificationRef} className="relative">
          <button
            onClick={() => setShowNotifications(!showNotifications)}
            className="p-2 rounded hover:bg-[var(--bg-tertiary)] transition-colors relative"
            title="Notifications"
          >
            <span>🔔</span>
            {unreadCount > 0 && (
              <span className="absolute -top-1 -right-1 bg-[var(--danger)] text-white text-xs px-1.5 py-0.5 rounded-full min-w-[18px] text-center">
                {unreadCount > 99 ? '99+' : unreadCount}
              </span>
            )}
          </button>
          {showNotifications && (
            <div className="absolute right-0 top-full mt-2 w-80 bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg shadow-lg z-50 overflow-hidden">
              <div className="flex items-center justify-between px-4 py-2 border-b border-[var(--border)]">
                <h4 className="font-medium">Notifications</h4>
                {unreadCount > 0 && (
                  <button
                    onClick={handleMarkAllRead}
                    className="text-xs text-[var(--accent)] hover:underline"
                  >
                    Mark all read
                  </button>
                )}
              </div>
              <div className="max-h-64 overflow-y-auto">
                {notifications.length === 0 ? (
                  <div className="px-4 py-8 text-center text-[var(--text-tertiary)] text-sm">
                    No notifications
                  </div>
                ) : (
                  notifications.map(n => (
                    <div
                      key={n.id}
                      className={`px-4 py-2 border-b border-[var(--border)] last:border-0 ${!n.read ? 'bg-[var(--bg-tertiary)]/50' : ''}`}
                    >
                      <div className="text-sm">{n.message}</div>
                      <div className="text-xs text-[var(--text-tertiary)] mt-1">
                        {new Date(n.sent_at).toLocaleString()}
                      </div>
                    </div>
                  ))
                )}
              </div>
              <div className="px-4 py-2 border-t border-[var(--border)]">
                <Link
                  href="/notifications"
                  className="text-xs text-[var(--accent)] hover:underline"
                  onClick={() => setShowNotifications(false)}
                >
                  View all notifications
                </Link>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Toast Notifications Stack */}
      <div className="fixed bottom-4 right-4 z-50 flex flex-col-reverse gap-2 pointer-events-none">
        {scanToasts.map((toast, index) => (
          <div
            key={toast.id}
            className={`px-4 py-3 rounded-lg shadow-lg pointer-events-auto transition-all duration-300 ease-in-out ${
              toast.type === 'success' ? 'bg-green-600 text-white' :
              toast.type === 'error' ? 'bg-red-600 text-white' :
              'bg-blue-600 text-white'
            }`}
            style={{
              animation: 'slideInRight 0.3s ease-out',
              opacity: 1,
            }}
          >
            <div className="flex items-center gap-2">
              {toast.type === 'info' && <span className="animate-spin">⏳</span>}
              {toast.type === 'success' && <span>✅</span>}
              {toast.type === 'error' && <span>❌</span>}
              <span>{toast.message}</span>
            </div>
          </div>
        ))}
      </div>

      <style jsx>{`
        @keyframes slideInRight {
          from {
            transform: translateX(100%);
            opacity: 0;
          }
          to {
            transform: translateX(0);
            opacity: 1;
          }
        }
      `}</style>
    </header>
  );
}
