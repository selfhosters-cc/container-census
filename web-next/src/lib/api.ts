// API client for Container Census
// Uses session-based authentication

export const API_BASE = '/api';

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
    // Clear auth state and redirect to login
    localStorage.removeItem('census-auth');
    if (window.location.pathname !== '/login') {
      window.location.href = '/login';
    }
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
export const getTrivySummary = () =>
  fetchApi<import('@/types').TrivySummary>('/hosts/trivy-summary');
export const getAgentInfo = (hostId: number) =>
  fetchApi<{ has_trivy: boolean; trivy_version?: string }>(`/hosts/agent/${hostId}/info`);

// Images
export const getImages = () => fetchApi<import('@/types').Image[]>('/images');

// Vulnerabilities (Security Plugin)
export const getVulnerabilitySummary = () =>
  fetchApi<import('@/types').VulnerabilitySummary>('/p/security/summary');
export const getVulnerabilityScans = (limit?: number) =>
  fetchApi<import('@/types').VulnerabilityScan[]>(`/p/security/scans${limit ? `?limit=${limit}` : ''}`);
export const getVulnerabilityDetails = (imageId: string) =>
  fetchApi<{ scan: import('@/types').VulnerabilityScan; vulnerabilities: import('@/types').Vulnerability[] }>(
    `/p/security/image/${encodeURIComponent(imageId)}`
  );
export const scanImage = (imageId: string) =>
  fetchApi<void>(`/p/security/scan/${encodeURIComponent(imageId)}`, { method: 'POST' });
export const scanAllImages = (hostIds?: number[]) =>
  fetchApi<import('@/types').ScanAllResponse>('/p/security/scan-all', {
    method: 'POST',
    body: hostIds ? JSON.stringify({ host_ids: hostIds }) : undefined,
  });
export const getScanProgress = () =>
  fetchApi<import('@/types').ScanProgress>('/p/security/progress');
export const getScanQueue = () =>
  fetchApi<import('@/types').ScanQueueStatus>('/p/security/queue');
export const getTrivyStatus = () =>
  fetchApi<import('@/types').TrivyStatusResponse>('/p/security/trivy-status');
export const updateVulnerabilityDb = (hostIds?: number[]) =>
  fetchApi<import('@/types').UpdateDBResponse>('/p/security/update-db', {
    method: 'POST',
    body: hostIds ? JSON.stringify({ host_ids: hostIds }) : undefined,
  });
export const getVulnerabilitySettings = () =>
  fetchApi<Record<string, string>>('/p/security/settings');
export const updateVulnerabilitySettings = (settings: Record<string, string>) =>
  fetchApi<void>('/p/security/settings', { method: 'PUT', body: JSON.stringify(settings) });

// Notifications
export const getNotificationChannels = () =>
  fetchApi<import('@/types').NotificationChannel[]>('/notifications/channels');
export const createNotificationChannel = (channel: Partial<import('@/types').NotificationChannel>) =>
  fetchApi<import('@/types').NotificationChannel>('/notifications/channels', { method: 'POST', body: JSON.stringify(channel) });
export const updateNotificationChannel = (id: number, channel: Partial<import('@/types').NotificationChannel>) =>
  fetchApi<import('@/types').NotificationChannel>(`/notifications/channels/${id}`, { method: 'PUT', body: JSON.stringify(channel) });
export const deleteNotificationChannel = (id: number) =>
  fetchApi<void>(`/notifications/channels/${id}`, { method: 'DELETE' });
export const testNotificationChannel = async (id: number): Promise<{ success: boolean; error?: string }> => {
  const result = await fetchApi<{ status: string; message: string }>(`/notifications/channels/${id}/test`, { method: 'POST' });
  return { success: result.status === 'success', error: result.status !== 'success' ? result.message : undefined };
};

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
  return fetchApi<import('@/types').NotificationLog[]>(`/notifications/logs?${params}`);
};
export const markNotificationRead = (id: number) =>
  fetchApi<void>(`/notifications/logs/${id}/read`, { method: 'PUT' });
export const markAllNotificationsRead = () =>
  fetchApi<void>('/notifications/logs/read-all', { method: 'PUT' });
export const clearOldNotifications = () =>
  fetchApi<void>('/notifications/logs/clear', { method: 'DELETE' });

export const getNotificationSilences = () =>
  fetchApi<import('@/types').NotificationSilence[]>('/notifications/silences');
export const createNotificationSilence = (silence: Record<string, unknown>) =>
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

// Scan
export const triggerScan = () =>
  fetchApi<void>('/scan', { method: 'POST' });

// Telemetry
export const submitTelemetry = () =>
  fetchApi<void>('/telemetry/submit', { method: 'POST' });
export const getTelemetryEndpoints = () =>
  fetchApi<import('@/types').TelemetryEndpoint[]>('/telemetry/endpoints');
export const updateTelemetryEndpoint = (name: string, data: { enabled: boolean }) =>
  fetchApi<void>(`/telemetry/endpoints/${encodeURIComponent(name)}`, { method: 'PUT', body: JSON.stringify(data) });
export const createTelemetryEndpoint = (endpoint: import('@/types').TelemetryEndpointCreate) =>
  fetchApi<void>('/telemetry/endpoints', { method: 'POST', body: JSON.stringify(endpoint) });
export const deleteTelemetryEndpoint = (name: string) =>
  fetchApi<void>(`/telemetry/endpoints/${encodeURIComponent(name)}`, { method: 'DELETE' });
export const getTelemetryStatus = () =>
  fetchApi<import('@/types').TelemetryEndpoint[]>('/telemetry/status');
export const getTelemetrySchedule = () =>
  fetchApi<import('@/types').TelemetrySchedule>('/telemetry/schedule');
export const testTelemetryEndpoint = (url: string, apiKey?: string) =>
  fetchApi<void>('/telemetry/test-endpoint', { method: 'POST', body: JSON.stringify({ url, api_key: apiKey }) });

// Container logs
export const getContainerLogs = (hostId: number, containerId: string, tail?: number) =>
  fetchApi<{ logs: string }>(`/containers/${hostId}/${containerId}/logs${tail ? `?tail=${tail}` : ''}`);

// Container updates (uses container name, not ID)
export const checkContainerUpdate = (hostId: number, containerName: string) =>
  fetchApi<{ available: boolean; message?: string }>(`/containers/${hostId}/${encodeURIComponent(containerName)}/check-update`, { method: 'POST' });
export const updateContainer = (hostId: number, containerName: string) =>
  fetchApi<{ success: boolean; message?: string; new_container_id?: string }>(`/containers/${hostId}/${encodeURIComponent(containerName)}/update`, { method: 'POST' });

// Streaming container update with progress
export const updateContainerWithProgress = (hostId: number, containerName: string) =>
  fetchApi<{ job_id: string }>(`/containers/${hostId}/${encodeURIComponent(containerName)}/update?stream_progress=true`, { method: 'POST' });

// Pull progress types
export interface PullProgress {
  status: string;
  image_name: string;
  host_name: string;
  layers: Record<string, { id: string; status: string; current: number; total: number }>;
  layer_count: number;
  completed_layers: number;
  total_bytes: number;
  downloaded_bytes: number;
  overall_percent: number;
  message: string;
  error?: string;
}

// Bulk update operations
export const bulkCheckUpdates = (containers: Array<{ host_id: number; container_id: string }>) =>
  fetchApi<{ job_id: string }>('/containers/bulk-check-updates', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ containers }),
  });

export const bulkUpdate = (containers: Array<{ host_id: number; container_id: string }>) =>
  fetchApi<Record<string, { success: boolean; error?: string; new_container_id?: string }>>('/containers/bulk-update', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ containers }),
  });

// Vulnerability trends
export const getVulnerabilityTrends = () =>
  fetchApi<{ date: string; critical: number; high: number; medium: number; low: number }[]>('/vulnerabilities/trends');

// Container lifecycle
export const getContainerLifecycleEvents = (hostId: number, containerName: string) =>
  fetchApi<import('@/types').ContainerLifecycleEvent[]>(
    `/containers/lifecycle/${hostId}/${encodeURIComponent(containerName)}`
  );

export const getContainerLifecycleSummaries = (limit: number = 200, hostId?: number) => {
  const params = new URLSearchParams({ limit: limit.toString() });
  if (hostId) params.append('host_id', hostId.toString());
  return fetchApi<import('@/types').ContainerLifecycleSummary[]>(`/containers/lifecycle?${params}`);
};

// Version checking via server (which calls telemetry collector)
export const checkVersion = () =>
  fetchApi<import('@/types').VersionCheckResponse>('/version/check', { method: 'POST' });

// Dismissed version preferences
export const getDismissedVersion = () =>
  fetchApi<import('@/types').DismissedVersionPreference>('/preferences/dismissed-version');

export const dismissVersion = (version: string, dismissUntilMajor: boolean = false) =>
  fetchApi('/preferences/dismiss-version', {
    method: 'POST',
    body: JSON.stringify({ version, dismiss_until_major: dismissUntilMajor })
  });

export const clearDismissedVersion = () =>
  fetchApi('/preferences/dismissed-version', { method: 'DELETE' });
