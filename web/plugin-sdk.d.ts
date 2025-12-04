/**
 * Container Census Plugin SDK
 * TypeScript declarations
 */

export interface Container {
  id: string;
  name: string;
  image: string;
  image_id: string;
  status: 'running' | 'exited' | 'paused' | 'restarting' | 'dead' | 'created';
  state: string;
  created: string;
  started_at: string;
  host_id: number;
  host_name: string;
  labels?: Record<string, string>;
  ports?: Array<{
    private_port: number;
    public_port?: number;
    type: string;
    ip?: string;
  }>;
  volumes?: Array<{
    name: string;
    destination: string;
    rw: boolean;
  }>;
  networks?: string[];
  cpu_percent?: number;
  memory_usage?: number;
  memory_limit?: number;
  memory_percent?: number;
}

export interface Host {
  id: number;
  name: string;
  address: string;
  host_type: string;
  description?: string;
  enabled: boolean;
  collect_stats: boolean;
  last_seen?: string;
  agent_version?: string;
  agent_status?: string;
  container_count?: number;
  running_count?: number;
}

export interface GraphNode {
  id: string;
  label: string;
  type: 'container' | 'network' | 'volume';
  status?: string;
  host_id?: number;
}

export interface GraphEdge {
  from: string;
  to: string;
  type: 'network' | 'volume' | 'link' | 'depends';
  label?: string;
}

export interface ContainerGraph {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export type ToastType = 'success' | 'error' | 'info' | 'warning';

export interface PluginSDKOptions {
  pluginId: string;
  container: HTMLElement;
}

export declare class CensusPluginSDK {
  pluginId: string;
  container: HTMLElement;
  apiBaseUrl: string;
  pluginApiBaseUrl: string;

  constructor(pluginId: string, container: HTMLElement);

  /**
   * Fetch wrapper for plugin API routes
   */
  fetch(path: string, options?: RequestInit): Promise<Response>;

  /**
   * Fetch data from Census API endpoints
   */
  censusAPI(path: string, options?: RequestInit): Promise<Response>;

  /**
   * Get all containers
   */
  getContainers(): Promise<Container[]>;

  /**
   * Get containers for a specific host
   */
  getContainersByHost(hostId: number): Promise<Container[]>;

  /**
   * Get all hosts
   */
  getHosts(): Promise<Host[]>;

  /**
   * Get container graph data
   */
  getContainerGraph(): Promise<ContainerGraph>;

  /**
   * Get plugin-specific data from storage
   */
  getPluginData<T = any>(key: string): Promise<T | null>;

  /**
   * Set plugin-specific data in storage
   */
  setPluginData(key: string, value: any): Promise<void>;

  /**
   * Delete plugin-specific data from storage
   */
  deletePluginData(key: string): Promise<void>;

  /**
   * Show a toast notification
   */
  showToast(message: string, type?: ToastType): void;

  /**
   * Navigate to a different tab
   */
  navigateToTab(tabId: string): void;

  /**
   * Subscribe to Census system events
   */
  on(eventType: string, callback: (data: any) => void): () => void;

  /**
   * Create a DOM element with attributes and children
   */
  createElement<K extends keyof HTMLElementTagNameMap>(
    tag: K,
    attrs?: Record<string, any>,
    ...children: (string | Node)[]
  ): HTMLElementTagNameMap[K];

  /**
   * Clear the plugin container
   */
  clearContainer(): void;

  /**
   * Render content to the plugin container
   */
  render(content: Node | string): void;

  /**
   * Load an external library (lazy loading)
   */
  loadScript(url: string): Promise<void>;

  /**
   * Load an external stylesheet
   */
  loadStylesheet(url: string): Promise<void>;

  /**
   * Format a date string
   */
  formatDate(date: string | Date): string;

  /**
   * Format bytes to human-readable size
   */
  formatBytes(bytes: number): string;

  /**
   * Get container status color
   */
  getStatusColor(status: string): string;

  /**
   * Create a loading spinner element
   */
  createLoadingSpinner(): HTMLElement;

  /**
   * Create an error message element
   */
  createErrorMessage(message: string): HTMLElement;
}

export default CensusPluginSDK;
