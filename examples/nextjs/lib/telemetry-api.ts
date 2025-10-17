/**
 * Telemetry API Client
 *
 * This module provides a type-safe client for fetching telemetry data
 * from the Container Census telemetry collector API.
 *
 * Environment Variables:
 * - TELEMETRY_API_URL: Base URL of the telemetry collector (required)
 * - TELEMETRY_API_KEY: API key for authentication (required)
 */

// API Response Types
export interface ImageCount {
  image: string;
  count: number;
}

export interface Growth {
  date: string;
  installations: number;
  avg_containers: number;
}

export interface Summary {
  installations: number;
  total_submissions: number;
  total_containers: number;
  total_hosts: number;
  total_agents: number;
  unique_images: number;
}

export interface RegistryCount {
  registry: string;
  count: number;
}

export interface VersionCount {
  version: string;
  installations: number;
}

export interface HeatmapData {
  day_of_week: number;  // 0=Sunday, 6=Saturday
  hour_of_day: number;   // 0-23
  report_count: number;
}

export interface IntervalCount {
  interval: number;      // seconds
  installations: number;
}

export interface GeographyData {
  timezone: string;
  installations: number;
  region: string;
}

export interface SubmissionEvent {
  id: number;
  installation_id: string;
  event_type: 'new' | 'update';
  timestamp: string;
  containers: number;
  hosts: number;
}

// API Client Configuration
interface TelemetryAPIConfig {
  baseURL: string;
  apiKey: string;
}

class TelemetryAPIError extends Error {
  constructor(
    message: string,
    public status?: number,
    public response?: any
  ) {
    super(message);
    this.name = 'TelemetryAPIError';
  }
}

/**
 * Telemetry API Client
 *
 * Usage:
 * ```ts
 * const api = new TelemetryAPI({
 *   baseURL: process.env.TELEMETRY_API_URL!,
 *   apiKey: process.env.TELEMETRY_API_KEY!
 * });
 *
 * const summary = await api.getSummary();
 * ```
 */
export class TelemetryAPI {
  private config: TelemetryAPIConfig;

  constructor(config: TelemetryAPIConfig) {
    this.config = config;
  }

  /**
   * Make an authenticated API request
   */
  private async request<T>(endpoint: string, params?: Record<string, any>): Promise<T> {
    const url = new URL(endpoint, this.config.baseURL);

    // Add query parameters
    if (params) {
      Object.entries(params).forEach(([key, value]) => {
        if (value !== undefined && value !== null) {
          url.searchParams.append(key, String(value));
        }
      });
    }

    const response = await fetch(url.toString(), {
      method: 'GET',
      headers: {
        'X-API-Key': this.config.apiKey,
        'Content-Type': 'application/json',
      },
      // Cache for 5 minutes in production
      next: { revalidate: 300 },
    });

    if (!response.ok) {
      const error = await response.text().catch(() => 'Unknown error');
      throw new TelemetryAPIError(
        `API request failed: ${response.statusText}`,
        response.status,
        error
      );
    }

    return response.json();
  }

  /**
   * Get top images by usage
   */
  async getTopImages(params?: { limit?: number; days?: number }): Promise<ImageCount[]> {
    return this.request<ImageCount[]>('/api/stats/top-images', params);
  }

  /**
   * Get growth metrics over time
   */
  async getGrowth(params?: { days?: number }): Promise<Growth[]> {
    return this.request<Growth[]>('/api/stats/growth', params);
  }

  /**
   * Get summary statistics
   */
  async getSummary(): Promise<Summary> {
    return this.request<Summary>('/api/stats/summary');
  }

  /**
   * Get registry distribution
   */
  async getRegistries(params?: { days?: number }): Promise<RegistryCount[]> {
    return this.request<RegistryCount[]>('/api/stats/registries', params);
  }

  /**
   * Get version distribution
   */
  async getVersions(): Promise<VersionCount[]> {
    return this.request<VersionCount[]>('/api/stats/versions');
  }

  /**
   * Get activity heatmap data
   */
  async getActivityHeatmap(params?: { days?: number }): Promise<HeatmapData[]> {
    return this.request<HeatmapData[]>('/api/stats/activity-heatmap', params);
  }

  /**
   * Get scan interval distribution
   */
  async getScanIntervals(): Promise<IntervalCount[]> {
    return this.request<IntervalCount[]>('/api/stats/scan-intervals');
  }

  /**
   * Get geographic distribution
   */
  async getGeography(): Promise<GeographyData[]> {
    return this.request<GeographyData[]>('/api/stats/geography');
  }

  /**
   * Get recent submission events
   */
  async getRecentEvents(params?: { limit?: number; since?: number }): Promise<SubmissionEvent[]> {
    return this.request<SubmissionEvent[]>('/api/stats/recent-events', params);
  }

  /**
   * Get installation count
   */
  async getInstallations(params?: { days?: number }): Promise<{ total_installations: number; period_days: number }> {
    return this.request('/api/stats/installations', params);
  }
}

/**
 * Create a singleton API client instance
 * This should only be used in Server Components
 */
export function createTelemetryAPI(): TelemetryAPI {
  const baseURL = process.env.TELEMETRY_API_URL;
  const apiKey = process.env.TELEMETRY_API_KEY;

  if (!baseURL) {
    throw new Error('TELEMETRY_API_URL environment variable is not set');
  }

  if (!apiKey) {
    throw new Error('TELEMETRY_API_KEY environment variable is not set');
  }

  return new TelemetryAPI({ baseURL, apiKey });
}

/**
 * Convenience function for making one-off API calls
 * Use this in Server Components for simplicity
 */
export async function fetchTelemetryData<T>(
  endpoint: string,
  params?: Record<string, any>
): Promise<T> {
  const api = createTelemetryAPI();
  return api.request<T>(endpoint, params);
}
