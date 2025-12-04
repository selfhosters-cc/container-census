/**
 * Container Census Plugin SDK
 *
 * Provides a convenient API for external plugins to interact with
 * the Census server and integrate with the frontend.
 *
 * @version 1.0.0
 */

class CensusPluginSDK {
  /**
   * @param {string} pluginId - The unique plugin identifier
   * @param {HTMLElement} container - The container element for the plugin UI
   */
  constructor(pluginId, container) {
    this.pluginId = pluginId;
    this.container = container;
    this.apiBaseUrl = '/api';
    this.pluginApiBaseUrl = `/api/p/${pluginId}`;
    this.eventListeners = new Map();
  }

  /**
   * Fetch wrapper for plugin API routes
   * Automatically handles authentication via session cookies
   *
   * @param {string} path - Path relative to /api/p/{pluginId}/
   * @param {RequestInit} options - Fetch options
   * @returns {Promise<Response>}
   */
  async fetch(path, options = {}) {
    const url = `${this.pluginApiBaseUrl}${path}`;

    const defaultOptions = {
      credentials: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    };

    return fetch(url, { ...defaultOptions, ...options });
  }

  /**
   * Fetch data from Census API endpoints
   *
   * @param {string} path - Path relative to /api/
   * @param {RequestInit} options - Fetch options
   * @returns {Promise<Response>}
   */
  async censusAPI(path, options = {}) {
    const url = `${this.apiBaseUrl}${path}`;

    const defaultOptions = {
      credentials: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    };

    return fetch(url, { ...defaultOptions, ...options });
  }

  /**
   * Get all containers
   *
   * @returns {Promise<Array>}
   */
  async getContainers() {
    const response = await this.censusAPI('/containers');
    if (!response.ok) {
      throw new Error(`Failed to fetch containers: ${response.statusText}`);
    }
    return response.json();
  }

  /**
   * Get containers for a specific host
   *
   * @param {number} hostId - Host ID
   * @returns {Promise<Array>}
   */
  async getContainersByHost(hostId) {
    const response = await this.censusAPI(`/containers/host/${hostId}`);
    if (!response.ok) {
      throw new Error(`Failed to fetch containers for host ${hostId}: ${response.statusText}`);
    }
    return response.json();
  }

  /**
   * Get all hosts
   *
   * @returns {Promise<Array>}
   */
  async getHosts() {
    const response = await this.censusAPI('/hosts');
    if (!response.ok) {
      throw new Error(`Failed to fetch hosts: ${response.statusText}`);
    }
    return response.json();
  }

  /**
   * Get container graph data
   *
   * @returns {Promise<Object>}
   */
  async getContainerGraph() {
    const response = await this.censusAPI('/containers/graph');
    if (!response.ok) {
      throw new Error(`Failed to fetch container graph: ${response.statusText}`);
    }
    return response.json();
  }

  /**
   * Get plugin-specific data from storage
   *
   * @param {string} key - Data key
   * @returns {Promise<any>}
   */
  async getPluginData(key) {
    const response = await this.fetch(`/data/${key}`);
    if (response.status === 404) {
      return null;
    }
    if (!response.ok) {
      throw new Error(`Failed to get plugin data for key ${key}: ${response.statusText}`);
    }
    return response.json();
  }

  /**
   * Set plugin-specific data in storage
   *
   * @param {string} key - Data key
   * @param {any} value - Data value (will be JSON stringified)
   * @returns {Promise<void>}
   */
  async setPluginData(key, value) {
    const response = await this.fetch(`/data/${key}`, {
      method: 'PUT',
      body: JSON.stringify(value),
    });
    if (!response.ok) {
      throw new Error(`Failed to set plugin data for key ${key}: ${response.statusText}`);
    }
  }

  /**
   * Delete plugin-specific data from storage
   *
   * @param {string} key - Data key
   * @returns {Promise<void>}
   */
  async deletePluginData(key) {
    const response = await this.fetch(`/data/${key}`, {
      method: 'DELETE',
    });
    if (!response.ok) {
      throw new Error(`Failed to delete plugin data for key ${key}: ${response.statusText}`);
    }
  }

  /**
   * Show a toast notification
   *
   * @param {string} message - Notification message
   * @param {'success' | 'error' | 'info' | 'warning'} type - Notification type
   */
  showToast(message, type = 'info') {
    if (typeof window.showToast === 'function') {
      window.showToast(message, type);
    } else {
      console.log(`[${type.toUpperCase()}] ${message}`);
    }
  }

  /**
   * Navigate to a different tab
   *
   * @param {string} tabId - Tab ID to navigate to
   */
  navigateToTab(tabId) {
    if (typeof window.navigateToTab === 'function') {
      window.navigateToTab(tabId);
    } else {
      console.warn('Tab navigation not available');
    }
  }

  /**
   * Subscribe to Census system events
   *
   * @param {string} eventType - Event type to subscribe to
   * @param {Function} callback - Event handler function
   * @returns {Function} Unsubscribe function
   */
  on(eventType, callback) {
    if (!this.eventListeners.has(eventType)) {
      this.eventListeners.set(eventType, new Set());
    }

    this.eventListeners.get(eventType).add(callback);

    // Return unsubscribe function
    return () => {
      const listeners = this.eventListeners.get(eventType);
      if (listeners) {
        listeners.delete(callback);
      }
    };
  }

  /**
   * Emit an event (called internally by Census)
   * @private
   */
  _emit(eventType, data) {
    const listeners = this.eventListeners.get(eventType);
    if (listeners) {
      listeners.forEach(callback => {
        try {
          callback(data);
        } catch (error) {
          console.error(`Error in event listener for ${eventType}:`, error);
        }
      });
    }
  }

  /**
   * Create a DOM element with attributes and children
   *
   * @param {string} tag - HTML tag name
   * @param {Object} attrs - Element attributes
   * @param {...(string|Node)} children - Child elements or text
   * @returns {HTMLElement}
   */
  createElement(tag, attrs = {}, ...children) {
    const element = document.createElement(tag);

    Object.entries(attrs).forEach(([key, value]) => {
      if (key === 'className') {
        element.className = value;
      } else if (key === 'style' && typeof value === 'object') {
        Object.assign(element.style, value);
      } else if (key.startsWith('on') && typeof value === 'function') {
        const eventName = key.slice(2).toLowerCase();
        element.addEventListener(eventName, value);
      } else {
        element.setAttribute(key, value);
      }
    });

    children.forEach(child => {
      if (typeof child === 'string') {
        element.appendChild(document.createTextNode(child));
      } else if (child instanceof Node) {
        element.appendChild(child);
      }
    });

    return element;
  }

  /**
   * Clear the plugin container
   */
  clearContainer() {
    while (this.container.firstChild) {
      this.container.removeChild(this.container.firstChild);
    }
  }

  /**
   * Render content to the plugin container
   *
   * @param {Node|string} content - Content to render
   */
  render(content) {
    this.clearContainer();

    if (typeof content === 'string') {
      this.container.innerHTML = content;
    } else if (content instanceof Node) {
      this.container.appendChild(content);
    }
  }

  /**
   * Load an external library (lazy loading)
   *
   * @param {string} url - Script URL
   * @returns {Promise<void>}
   */
  loadScript(url) {
    return new Promise((resolve, reject) => {
      // Check if already loaded
      if (document.querySelector(`script[src="${url}"]`)) {
        resolve();
        return;
      }

      const script = document.createElement('script');
      script.src = url;
      script.onload = () => resolve();
      script.onerror = () => reject(new Error(`Failed to load script: ${url}`));
      document.head.appendChild(script);
    });
  }

  /**
   * Load an external stylesheet
   *
   * @param {string} url - Stylesheet URL
   * @returns {Promise<void>}
   */
  loadStylesheet(url) {
    return new Promise((resolve, reject) => {
      // Check if already loaded
      if (document.querySelector(`link[href="${url}"]`)) {
        resolve();
        return;
      }

      const link = document.createElement('link');
      link.rel = 'stylesheet';
      link.href = url;
      link.onload = () => resolve();
      link.onerror = () => reject(new Error(`Failed to load stylesheet: ${url}`));
      document.head.appendChild(link);
    });
  }

  /**
   * Format a date string
   *
   * @param {string|Date} date - Date to format
   * @returns {string}
   */
  formatDate(date) {
    if (typeof window.formatDate === 'function') {
      return window.formatDate(date);
    }

    const d = typeof date === 'string' ? new Date(date) : date;
    return d.toLocaleString();
  }

  /**
   * Format bytes to human-readable size
   *
   * @param {number} bytes - Bytes to format
   * @returns {string}
   */
  formatBytes(bytes) {
    if (bytes === 0) return '0 B';

    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));

    return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`;
  }

  /**
   * Get container status color
   *
   * @param {string} status - Container status
   * @returns {string} CSS color
   */
  getStatusColor(status) {
    const colors = {
      running: '#10b981',
      exited: '#6b7280',
      paused: '#f59e0b',
      restarting: '#3b82f6',
      dead: '#ef4444',
      created: '#8b5cf6',
    };

    return colors[status] || '#6b7280';
  }

  /**
   * Create a loading spinner element
   *
   * @returns {HTMLElement}
   */
  createLoadingSpinner() {
    return this.createElement('div', {
      className: 'loading-spinner',
      style: {
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        padding: '40px',
      }
    }, 'Loading...');
  }

  /**
   * Create an error message element
   *
   * @param {string} message - Error message
   * @returns {HTMLElement}
   */
  createErrorMessage(message) {
    return this.createElement('div', {
      className: 'error-message',
      style: {
        color: '#ef4444',
        padding: '20px',
        border: '1px solid #ef4444',
        borderRadius: '4px',
        backgroundColor: '#fee2e2',
      }
    }, `Error: ${message}`);
  }
}

// Export for CommonJS and ES modules
if (typeof module !== 'undefined' && module.exports) {
  module.exports = CensusPluginSDK;
}

// Also expose globally for browser usage
if (typeof window !== 'undefined') {
  window.CensusPluginSDK = CensusPluginSDK;
}
