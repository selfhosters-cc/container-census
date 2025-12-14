// Container types
export interface Container {
  id: string;
  name: string;
  image: string;
  image_id: string;
  image_tags?: string[];
  state: string;
  status: string;
  created: string;
  started_at?: string;
  host_id: number;
  host_name: string;
  ports: Port[];
  networks: string[];
  network_details?: NetworkDetail[];
  labels: Record<string, string>;
  cpu_percent?: number;
  memory_usage?: number;
  memory_limit?: number;
  memory_percent?: number;
  plugin_data?: Record<string, unknown>;
  update_available?: boolean;
}

export interface Port {
  private_port: number;
  public_port?: number;
  type: string;
}

export interface NetworkDetail {
  network_name: string;
  ip_address: string;
  aliases?: string[];
}

// Host types
export interface Host {
  id: number;
  name: string;
  address: string;
  host_type: string;
  enabled: boolean;
  api_token?: string;
  description?: string;
  collect_stats: boolean;
  enable_vulnerability_scanning?: boolean;
  created_at: string;
  updated_at: string;
  last_seen?: string;
  last_error?: string;
  container_count?: number;
  running_count?: number;
  agent_status?: string;
  agent_version?: string;
}

// Image types
export interface Image {
  id: string;
  name: string;
  tag: string;
  size: number;
  created: string;
  host_id: number;
  container_count: number;
}

// Vulnerability types
export interface SeverityCounts {
  critical: number;
  high: number;
  medium: number;
  low: number;
  unknown?: number;
}

export interface VulnerabilityScan {
  id?: number;
  image_id: string;
  image_name: string;
  scanned_at: string;
  scan_duration_ms?: number;
  success: boolean;
  error?: string;
  trivy_db_version?: string;
  total_vulnerabilities: number;
  severity_counts: SeverityCounts;
  host_ids?: number[];
  host_names?: string[];
  // Legacy flat fields for backward compatibility
  critical_count?: number;
  high_count?: number;
  medium_count?: number;
  low_count?: number;
}

export interface Vulnerability {
  vulnerability_id: string;
  pkg_name: string;
  installed_version: string;
  fixed_version?: string;
  severity: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'UNKNOWN';
  title: string;
  description: string;
}

export interface VulnerabilitySummaryData {
  total_images_scanned?: number;
  images_with_vulnerabilities?: number;
  total_vulnerabilities?: number;
  severity_counts?: {
    critical?: number;
    high?: number;
    medium?: number;
    low?: number;
  };
}

export interface VulnerabilitySummary {
  summary?: VulnerabilitySummaryData;
  total_images?: number;
  scanned_images?: number;
  images_with_vulnerabilities?: number;
  total_vulnerabilities?: number;
  critical_count?: number;
  high_count?: number;
  medium_count?: number;
  low_count?: number;
  queue_status?: {
    in_progress: number;
    pending: number;
  };
}

// Notification types
export interface NotificationChannel {
  id: number;
  name: string;
  type: 'webhook' | 'ntfy' | 'in_app';
  enabled: boolean;
  config: Record<string, unknown>;
  created_at: string;
}

export interface NotificationRule {
  id: number;
  name: string;
  enabled: boolean;
  event_types: string[];
  host_id?: number;
  container_pattern?: string;
  image_pattern?: string;
  cpu_threshold?: number;
  memory_threshold?: number;
  threshold_duration?: number;
  cooldown_period?: number;
  channel_ids: number[];
  created_at: string;
}

export interface NotificationLog {
  id: number;
  channel_id?: number;
  channel_name?: string;
  rule_id?: number;
  rule_name?: string;
  event_type: string;
  container_id?: string;
  container_name?: string;
  host_id?: number;
  host_name?: string;
  message: string;
  read: boolean;
  sent_at: string;
  success?: boolean;
  error?: string;
}

export interface NotificationSilence {
  id: number;
  host_id?: number;
  container_id?: string;
  container_name?: string;
  container_pattern?: string;
  reason?: string;
  expires_at?: string;
  created_at: string;
}

// Stats types
export interface ContainerStatsPoint {
  timestamp: string;
  cpu_percent: number;
  memory_usage: number;
  memory_limit: number;
}

// Plugin types
export interface PluginInfo {
  id: string;
  name: string;
  description: string;
  version: string;
  author?: string;
  homepage?: string;
  capabilities: string[] | null | undefined;
  built_in: boolean;
  enabled: boolean;
}

export interface PluginTab {
  id: string;
  label: string;
  icon: string;
  order: number;
  script_url?: string;
  init_func?: string;
}

export interface Badge {
  id: string;
  plugin_id: string;
  label: string;
  icon: string;
  color: string;
  tooltip?: string;
  link?: string;
  priority: number;
}

// Health check
export interface HealthStatus {
  status: string;
  version: string;
  build_time?: string;
  latest_version?: string;
  update_available?: boolean;
  release_url?: string;
}

export interface VersionCheckResponse {
  current_version: string;
  latest_version: string;
  update_available: boolean;
  release_url: string;
  checked_at: string;
  error?: string;
}

export interface DismissedVersionPreference {
  dismissed_version: string | null;
  dismiss_until_major: boolean;
}

// Dashboard stats
export interface DashboardStats {
  total_hosts: number;
  total_containers: number;
  running_containers: number;
  stopped_containers: number;
  paused_containers: number;
  total_images: number;
  critical_vulnerabilities: number;
  high_vulnerabilities: number;
  unread_notifications: number;
}

// Container lifecycle types
export interface ContainerLifecycleSummary {
  container_id: string;
  container_name: string;
  image: string;
  host_id: number;
  host_name: string;
  first_seen: string;
  last_seen: string;
  last_started?: string;  // Time container last entered running state
  current_state: string;
  state_changes: number;
  image_updates: number;
  restart_events: number;
  is_active: boolean;
  total_scans: number;
}

export interface ContainerLifecycleEvent {
  timestamp: string;
  event_type: 'first_seen' | 'started' | 'stopped' | 'paused' | 'resumed' |
              'restarted' | 'image_updated' | 'disappeared' | 'reappeared' |
              'state_change' | 'last_seen';
  old_state?: string;
  new_state?: string;
  old_image_tag?: string;
  new_image_tag?: string;
  old_image_sha?: string;
  new_image_sha?: string;
  description?: string;
  restart_count?: number;
}

// Security Plugin Types
export interface ScanProgress {
  in_progress: number;
  pending: number;
  total: number;
  current_scans: Array<{
    image_id: string;
    image_name: string;
    host_id: number;
    host_name: string;
    started_at: string;
  }>;
}

export interface ScanQueueStatus {
  queued: number;
  in_progress: number;
  completed_today: number;
  failed_today: number;
  queue_items: ScanJob[];
  total_workers: number;
  active_workers: number;
}

export interface ScanJob {
  image_id: string;
  image_name: string;
  host_id: number;
  host_name: string;
  queued_at: string;
  priority: number;
}

export interface TrivyHostStatus {
  host_id: number;
  host_name: string;
  trivy_version: string;
  db_version: string;
  last_updated: string;
  has_trivy: boolean;
}

export interface TrivyStatusResponse {
  hosts: TrivyHostStatus[];
}

export interface TrivySummary {
  with_trivy: number;
  without_trivy: number;
  disabled: number;
  total_agents: number;
}

export interface ScanAllResponse {
  message: string;
  queued_by_host: Record<string, number>;
  total_queued: number;
}

export interface UpdateDBResponse {
  results: Array<{
    host_id: number;
    host_name: string;
    success: boolean;
    error?: string;
  }>;
}

// Auth types
export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  success: boolean;
  error?: string;
}
