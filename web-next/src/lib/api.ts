// API client for Container Census
// Uses session-based authentication

const API_BASE = '/api';

class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

async function fetchApi<T>(path: string, options?: RequestInit): Promise<T> {
  const url = `${API_BASE}${path}`;

  const response = await fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
    credentials: 'include', // Include session cookies
  });

  if (response.status === 401) {
    // Redirect to login if unauthorized
    window.location.href = '/login';
    throw new ApiError(401, 'Unauthorized');
  }

  if (!response.ok) {
    const text = await response.text();
    throw new ApiError(response.status, text || `HTTP ${response.status}`);
  }

  // Handle empty responses
  const text = await response.text();
  if (!text) return {} as T;

  return JSON.parse(text);
}

// Health
export const getHealth = () => fetchApi<import('@/types').HealthStatus>('/health');

// Containers
export const getContainers = () => fetchApi<import('@/types').Container[]>('/containers');
export const getContainerStats = (hostId: number, containerId: string, range: string) =>
  fetchApi<import('@/types').ContainerStatsPoint[]>(`/containers/${hostId}/${containerId}/stats?range=${range}`);
export const startContainer = (hostId: number, containerId: string) =>
  fetchApi<void>(`/containers/${hostId}/${containerId}/start`, { method: 'POST' });
export const stopContainer = (hostId: number, containerId: string) =>
  fetchApi<void>(`/containers/${hostId}/${containerId}/stop`, { method: 'POST' });
export const restartContainer = (hostId: number, containerId: string) =>
  fetchApi<void>(`/containers/${hostId}/${containerId}/restart`, { method: 'POST' });
export const removeContainer = (hostId: number, containerId: string) =>
  fetchApi<void>(`/containers/${hostId}/${containerId}`, { method: 'DELETE' });

// Hosts
export const getHosts = () => fetchApi<import('@/types').Host[]>('/hosts');
export const getHost = (id: number) => fetchApi<import('@/types').Host>(`/hosts/${id}`);
export const createHost = (host: Partial<import('@/types').Host>) =>
  fetchApi<import('@/types').Host>('/hosts', { method: 'POST', body: JSON.stringify(host) });
export const updateHost = (id: number, host: Partial<import('@/types').Host>) =>
  fetchApi<import('@/types').Host>(`/hosts/${id}`, { method: 'PUT', body: JSON.stringify(host) });
export const deleteHost = (id: number) =>
  fetchApi<void>(`/hosts/${id}`, { method: 'DELETE' });
export const scanHost = (id: number) =>
  fetchApi<void>(`/hosts/${id}/scan`, { method: 'POST' });

// Images
export const getImages = () => fetchApi<import('@/types').Image[]>('/images');

// Vulnerabilities
export const getVulnerabilitySummary = () =>
  fetchApi<import('@/types').VulnerabilitySummary>('/vulnerabilities/summary');
export const getVulnerabilityScans = (limit?: number) =>
  fetchApi<import('@/types').VulnerabilityScan[]>(`/vulnerabilities/scans${limit ? `?limit=${limit}` : ''}`);
export const getVulnerabilityDetails = (imageId: string) =>
  fetchApi<{ scan: import('@/types').VulnerabilityScan; vulnerabilities: import('@/types').Vulnerability[] }>(
    `/vulnerabilities/image/${encodeURIComponent(imageId)}`
  );
export const scanImage = (imageId: string) =>
  fetchApi<void>(`/vulnerabilities/scan/${encodeURIComponent(imageId)}`, { method: 'POST' });
export const scanAllImages = () =>
  fetchApi<void>('/vulnerabilities/scan-all', { method: 'POST' });
export const updateVulnerabilityDb = () =>
  fetchApi<void>('/vulnerabilities/update-db', { method: 'POST' });
export const getVulnerabilitySettings = () =>
  fetchApi<Record<string, string>>('/vulnerabilities/settings');
export const updateVulnerabilitySettings = (settings: Record<string, string>) =>
  fetchApi<void>('/vulnerabilities/settings', { method: 'PUT', body: JSON.stringify(settings) });

// Notifications
export const getNotificationChannels = () =>
  fetchApi<import('@/types').NotificationChannel[]>('/notifications/channels');
export const createNotificationChannel = (channel: Partial<import('@/types').NotificationChannel>) =>
  fetchApi<import('@/types').NotificationChannel>('/notifications/channels', { method: 'POST', body: JSON.stringify(channel) });
export const updateNotificationChannel = (id: number, channel: Partial<import('@/types').NotificationChannel>) =>
  fetchApi<import('@/types').NotificationChannel>(`/notifications/channels/${id}`, { method: 'PUT', body: JSON.stringify(channel) });
export const deleteNotificationChannel = (id: number) =>
  fetchApi<void>(`/notifications/channels/${id}`, { method: 'DELETE' });
export const testNotificationChannel = (id: number) =>
  fetchApi<{ success: boolean; error?: string }>(`/notifications/channels/${id}/test`, { method: 'POST' });

export const getNotificationRules = () =>
  fetchApi<import('@/types').NotificationRule[]>('/notifications/rules');
export const createNotificationRule = (rule: Partial<import('@/types').NotificationRule>) =>
  fetchApi<import('@/types').NotificationRule>('/notifications/rules', { method: 'POST', body: JSON.stringify(rule) });
export const updateNotificationRule = (id: number, rule: Partial<import('@/types').NotificationRule>) =>
  fetchApi<import('@/types').NotificationRule>(`/notifications/rules/${id}`, { method: 'PUT', body: JSON.stringify(rule) });
export const deleteNotificationRule = (id: number) =>
  fetchApi<void>(`/notifications/rules/${id}`, { method: 'DELETE' });

export const getNotificationLog = (limit?: number, unread?: boolean) => {
  const params = new URLSearchParams();
  if (limit) params.set('limit', String(limit));
  if (unread) params.set('unread', 'true');
  return fetchApi<import('@/types').NotificationLog[]>(`/notifications/log?${params}`);
};
export const markNotificationRead = (id: number) =>
  fetchApi<void>(`/notifications/log/${id}/read`, { method: 'PUT' });
export const markAllNotificationsRead = () =>
  fetchApi<void>('/notifications/log/read-all', { method: 'POST' });
export const clearOldNotifications = () =>
  fetchApi<void>('/notifications/log/clear', { method: 'DELETE' });

export const getNotificationSilences = () =>
  fetchApi<import('@/types').NotificationSilence[]>('/notifications/silences');
export const createNotificationSilence = (silence: Partial<import('@/types').NotificationSilence>) =>
  fetchApi<import('@/types').NotificationSilence>('/notifications/silences', { method: 'POST', body: JSON.stringify(silence) });
export const deleteNotificationSilence = (id: number) =>
  fetchApi<void>(`/notifications/silences/${id}`, { method: 'DELETE' });

export const getNotificationStatus = () =>
  fetchApi<{ unread_count: number; rule_count: number; channel_count: number; rate_limit_remaining: number }>(
    '/notifications/status'
  );

// Plugins
export const getPlugins = () => fetchApi<import('@/types').PluginInfo[]>('/plugins');
export const getPluginTabs = () => fetchApi<import('@/types').PluginTab[]>('/plugins/tabs');
export const getPluginBadges = (hostId: number, containerId: string) =>
  fetchApi<import('@/types').Badge[]>(`/plugins/badges?host_id=${hostId}&container_id=${containerId}`);
export const enablePlugin = (id: string) =>
  fetchApi<void>(`/plugins/${id}/enable`, { method: 'PUT' });
export const disablePlugin = (id: string) =>
  fetchApi<void>(`/plugins/${id}/disable`, { method: 'PUT' });
export const getPluginSettings = (id: string) =>
  fetchApi<Record<string, string>>(`/plugins/${id}/settings`);
export const updatePluginSettings = (id: string, settings: Record<string, string>) =>
  fetchApi<void>(`/plugins/${id}/settings`, { method: 'PUT', body: JSON.stringify(settings) });

// Plugin-specific API (for plugin tabs)
export const fetchPluginApi = <T>(pluginId: string, path: string, options?: RequestInit) =>
  fetchApi<T>(`/p/${pluginId}${path}`, options);

// Metrics (Prometheus format)
export const getMetrics = async () => {
  const response = await fetch('/metrics');
  return response.text();
};
