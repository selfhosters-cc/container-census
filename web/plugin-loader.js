/**
 * Container Census Plugin Loader
 *
 * Dynamically loads and initializes external plugins from their
 * JavaScript bundles and CSS files.
 */

class PluginLoader {
  constructor() {
    this.loadedPlugins = new Map(); // pluginId -> PluginInstance
    this.loadedScripts = new Set();
    this.loadedStylesheets = new Set();
  }

  /**
   * Load all external plugins
   *
   * @returns {Promise<void>}
   */
  async loadAllPlugins() {
    try {
      const response = await fetch('/api/plugins', {
        credentials: 'same-origin',
      });

      if (!response.ok) {
        throw new Error(`Failed to fetch plugins: ${response.statusText}`);
      }

      const plugins = await response.json();

      // Filter for external plugins that have frontend bundles
      const externalPlugins = plugins.filter(
        p => p.type === 'external' && p.enabled && p.frontend_bundle
      );

      // Load all plugins in parallel
      await Promise.all(
        externalPlugins.map(plugin => this.loadPlugin(plugin))
      );

      console.log(`Loaded ${externalPlugins.length} external plugin(s)`);
    } catch (error) {
      console.error('Failed to load plugins:', error);
    }
  }

  /**
   * Load a single plugin
   *
   * @param {Object} pluginMetadata - Plugin metadata from API
   * @returns {Promise<void>}
   */
  async loadPlugin(pluginMetadata) {
    const { id, name, frontend_bundle, frontend_css, tab_config } = pluginMetadata;

    try {
      console.log(`Loading plugin: ${name} (${id})`);

      // Load CSS if available
      if (frontend_css && !this.loadedStylesheets.has(frontend_css)) {
        await this.loadStylesheet(frontend_css);
        this.loadedStylesheets.add(frontend_css);
      }

      // Load JavaScript bundle
      if (!this.loadedScripts.has(frontend_bundle)) {
        await this.loadScript(frontend_bundle);
        this.loadedScripts.add(frontend_bundle);
      }

      // Store plugin instance
      this.loadedPlugins.set(id, {
        metadata: pluginMetadata,
        initialized: false,
      });

      console.log(`Successfully loaded plugin: ${name}`);
    } catch (error) {
      console.error(`Failed to load plugin ${name} (${id}):`, error);
      throw error;
    }
  }

  /**
   * Initialize a plugin in its container
   *
   * @param {string} pluginId - Plugin ID
   * @param {HTMLElement} container - Container element
   * @returns {Promise<void>}
   */
  async initializePlugin(pluginId, container) {
    const plugin = this.loadedPlugins.get(pluginId);

    if (!plugin) {
      throw new Error(`Plugin ${pluginId} is not loaded`);
    }

    if (plugin.initialized) {
      console.warn(`Plugin ${pluginId} is already initialized`);
      return;
    }

    const { metadata } = plugin;
    const initFunction = metadata.frontend_init_function || `init${this.toPascalCase(pluginId)}`;

    // Check if init function exists
    if (typeof window[initFunction] !== 'function') {
      throw new Error(
        `Plugin init function ${initFunction} not found. ` +
        `Make sure the plugin bundle exports this function to window.`
      );
    }

    // Create SDK instance
    const sdk = new window.CensusPluginSDK(pluginId, container);

    try {
      // Call plugin init function
      await window[initFunction](container, sdk);

      plugin.initialized = true;
      plugin.sdk = sdk;

      console.log(`Initialized plugin: ${metadata.name}`);
    } catch (error) {
      console.error(`Failed to initialize plugin ${pluginId}:`, error);
      throw error;
    }
  }

  /**
   * Get plugin tabs for navigation
   *
   * @returns {Array<Object>}
   */
  getPluginTabs() {
    const tabs = [];

    this.loadedPlugins.forEach((plugin, id) => {
      const { metadata } = plugin;

      if (metadata.tab_config) {
        const tabConfig = typeof metadata.tab_config === 'string'
          ? JSON.parse(metadata.tab_config)
          : metadata.tab_config;

        tabs.push({
          id: tabConfig.id || `plugin-${id}`,
          label: tabConfig.label || metadata.name,
          icon: tabConfig.icon || '🔌',
          order: tabConfig.order || 100,
          pluginId: id,
          type: 'plugin',
        });
      }
    });

    // Sort by order
    return tabs.sort((a, b) => a.order - b.order);
  }

  /**
   * Emit an event to all loaded plugins
   *
   * @param {string} eventType - Event type
   * @param {any} data - Event data
   */
  emitEvent(eventType, data) {
    this.loadedPlugins.forEach((plugin) => {
      if (plugin.sdk && plugin.initialized) {
        plugin.sdk._emit(eventType, data);
      }
    });
  }

  /**
   * Unload a plugin
   *
   * @param {string} pluginId - Plugin ID
   */
  unloadPlugin(pluginId) {
    const plugin = this.loadedPlugins.get(pluginId);

    if (!plugin) {
      return;
    }

    // Clear container if SDK exists
    if (plugin.sdk) {
      plugin.sdk.clearContainer();
    }

    this.loadedPlugins.delete(pluginId);

    console.log(`Unloaded plugin: ${pluginId}`);
  }

  /**
   * Reload a plugin (useful for development)
   *
   * @param {string} pluginId - Plugin ID
   * @param {HTMLElement} container - Container element
   * @returns {Promise<void>}
   */
  async reloadPlugin(pluginId, container) {
    // Unload first
    this.unloadPlugin(pluginId);

    // Remove from loaded scripts/stylesheets to force reload
    const plugin = this.loadedPlugins.get(pluginId);
    if (plugin) {
      this.loadedScripts.delete(plugin.metadata.frontend_bundle);
      if (plugin.metadata.frontend_css) {
        this.loadedStylesheets.delete(plugin.metadata.frontend_css);
      }
    }

    // Fetch fresh metadata and reload
    const response = await fetch(`/api/plugins/${pluginId}`, {
      credentials: 'same-origin',
    });

    if (!response.ok) {
      throw new Error(`Failed to fetch plugin metadata: ${response.statusText}`);
    }

    const metadata = await response.json();
    await this.loadPlugin(metadata);

    if (container) {
      await this.initializePlugin(pluginId, container);
    }
  }

  /**
   * Check if a plugin is loaded
   *
   * @param {string} pluginId - Plugin ID
   * @returns {boolean}
   */
  isPluginLoaded(pluginId) {
    return this.loadedPlugins.has(pluginId);
  }

  /**
   * Check if a plugin is initialized
   *
   * @param {string} pluginId - Plugin ID
   * @returns {boolean}
   */
  isPluginInitialized(pluginId) {
    const plugin = this.loadedPlugins.get(pluginId);
    return plugin ? plugin.initialized : false;
  }

  /**
   * Load a JavaScript file
   *
   * @param {string} url - Script URL
   * @returns {Promise<void>}
   * @private
   */
  loadScript(url) {
    return new Promise((resolve, reject) => {
      // Check if already loaded
      const existing = document.querySelector(`script[src="${url}"]`);
      if (existing) {
        resolve();
        return;
      }

      const script = document.createElement('script');
      script.src = url;
      script.async = true;

      script.onload = () => {
        console.log(`Loaded script: ${url}`);
        resolve();
      };

      script.onerror = () => {
        reject(new Error(`Failed to load script: ${url}`));
      };

      document.head.appendChild(script);
    });
  }

  /**
   * Load a CSS file
   *
   * @param {string} url - Stylesheet URL
   * @returns {Promise<void>}
   * @private
   */
  loadStylesheet(url) {
    return new Promise((resolve, reject) => {
      // Check if already loaded
      const existing = document.querySelector(`link[href="${url}"]`);
      if (existing) {
        resolve();
        return;
      }

      const link = document.createElement('link');
      link.rel = 'stylesheet';
      link.href = url;

      link.onload = () => {
        console.log(`Loaded stylesheet: ${url}`);
        resolve();
      };

      link.onerror = () => {
        reject(new Error(`Failed to load stylesheet: ${url}`));
      };

      document.head.appendChild(link);
    });
  }

  /**
   * Convert kebab-case to PascalCase
   *
   * @param {string} str - Input string
   * @returns {string}
   * @private
   */
  toPascalCase(str) {
    return str
      .split(/[-_]/)
      .map(word => word.charAt(0).toUpperCase() + word.slice(1))
      .join('');
  }
}

// Create global plugin loader instance
window.pluginLoader = new PluginLoader();

// Load plugins when DOM is ready
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', () => {
    window.pluginLoader.loadAllPlugins();
  });
} else {
  // DOM already loaded
  window.pluginLoader.loadAllPlugins();
}

// Export for modules
if (typeof module !== 'undefined' && module.exports) {
  module.exports = PluginLoader;
}
