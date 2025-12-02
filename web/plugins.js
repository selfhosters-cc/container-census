// Plugin Manager for Container Census
// Handles loading plugins, rendering tabs, and displaying badges

class PluginManager {
    constructor() {
        this.plugins = [];
        this.tabs = [];
        this.badgeCache = new Map(); // Cache badges by container key
        this.initialized = false;
    }

    // Initialize plugin system
    async init() {
        try {
            await this.loadPlugins();
            await this.loadTabs();
            this.renderIntegrationsMenu();
            this.initialized = true;
            console.log('Plugin system initialized with', this.plugins.length, 'plugins');
        } catch (error) {
            console.error('Failed to initialize plugin system:', error);
        }
    }

    // Load all plugins from API
    async loadPlugins() {
        try {
            const response = await fetchWithAuth('/api/plugins');
            if (response.ok) {
                this.plugins = await response.json();
            } else {
                console.warn('Failed to load plugins:', response.status);
                this.plugins = [];
            }
        } catch (error) {
            console.error('Error loading plugins:', error);
            this.plugins = [];
        }
    }

    // Load all plugin tabs
    async loadTabs() {
        try {
            const response = await fetchWithAuth('/api/plugins/tabs');
            if (response.ok) {
                this.tabs = await response.json();
            } else {
                console.warn('Failed to load plugin tabs:', response.status);
                this.tabs = [];
            }
        } catch (error) {
            console.error('Error loading plugin tabs:', error);
            this.tabs = [];
        }
    }

    // Get badges for a container
    async getBadges(hostId, containerId) {
        const cacheKey = `${hostId}-${containerId}`;

        // Return from cache if available
        if (this.badgeCache.has(cacheKey)) {
            return this.badgeCache.get(cacheKey);
        }

        try {
            const response = await fetchWithAuth(`/api/plugins/badges?host_id=${hostId}&container_id=${containerId}`);
            if (response.ok) {
                const badges = await response.json();
                this.badgeCache.set(cacheKey, badges);
                return badges;
            }
        } catch (error) {
            console.error('Error loading badges:', error);
        }

        return [];
    }

    // Clear badge cache (call after data refresh)
    clearBadgeCache() {
        this.badgeCache.clear();
    }

    // Render integrations dropdown in navigation
    renderIntegrationsMenu() {
        const navContainer = document.querySelector('.sidebar-nav');
        if (!navContainer) {
            console.warn('Navigation container not found');
            return;
        }

        // Remove existing integrations menu if present
        const existing = document.getElementById('integrationsDropdown');
        if (existing) {
            existing.remove();
        }

        // Only show if there are tabs from plugins
        if (this.tabs.length === 0) {
            return;
        }

        // Create dropdown HTML
        const dropdown = document.createElement('div');
        dropdown.id = 'integrationsDropdown';
        dropdown.className = 'nav-dropdown';

        dropdown.innerHTML = `
            <button class="nav-item nav-dropdown-toggle" onclick="pluginManager.toggleIntegrationsMenu()">
                <span class="nav-icon">🔌</span>
                <span class="nav-label">Integrations</span>
                <span class="nav-dropdown-arrow">▸</span>
            </button>
            <div class="nav-dropdown-content" id="integrationsSubmenu">
                ${this.tabs.map(tab => `
                    <button class="nav-dropdown-item" data-plugin-tab="${tab.id}" onclick="pluginManager.showPluginTab('${tab.id}')">
                        <span class="nav-icon">${tab.icon || '📦'}</span>
                        <span class="nav-label">${escapeHtml(tab.label)}</span>
                    </button>
                `).join('')}
            </div>
        `;

        // Find the position to insert (before Settings/last items)
        const notificationsBtn = navContainer.querySelector('[data-tab="notifications"]');
        if (notificationsBtn) {
            navContainer.insertBefore(dropdown, notificationsBtn);
        } else {
            navContainer.appendChild(dropdown);
        }
    }

    // Toggle integrations dropdown
    toggleIntegrationsMenu() {
        const dropdown = document.getElementById('integrationsDropdown');
        if (dropdown) {
            dropdown.classList.toggle('open');
        }
    }

    // Show a plugin tab
    async showPluginTab(pluginId) {
        // Close dropdown
        const dropdown = document.getElementById('integrationsDropdown');
        if (dropdown) {
            dropdown.classList.remove('open');
        }

        // Update active states
        document.querySelectorAll('.nav-item').forEach(btn => btn.classList.remove('active'));
        document.querySelectorAll('.nav-dropdown-item').forEach(btn => btn.classList.remove('active'));

        const activeBtn = document.querySelector(`[data-plugin-tab="${pluginId}"]`);
        if (activeBtn) {
            activeBtn.classList.add('active');
        }

        // Find the tab definition
        const tab = this.tabs.find(t => t.id === pluginId);
        if (!tab) {
            console.error('Plugin tab not found:', pluginId);
            return;
        }

        // Hide all existing tab contents
        document.querySelectorAll('.tab-content').forEach(tc => tc.classList.remove('active'));

        // Get or create plugin tab container
        let tabContent = document.getElementById(`plugin-tab-${pluginId}`);
        if (!tabContent) {
            tabContent = document.createElement('div');
            tabContent.id = `plugin-tab-${pluginId}`;
            tabContent.className = 'tab-content';
            document.querySelector('.main-content').appendChild(tabContent);
        }

        tabContent.classList.add('active');

        // Load tab content
        try {
            tabContent.innerHTML = '<div class="loading-spinner">Loading...</div>';

            // Fetch tab content from plugin API
            // Plugin routes are under /api/p/{pluginId}/ to avoid conflict with /api/plugins/{id} management routes
            const response = await fetchWithAuth(`/api/p/${pluginId}/tab`);
            if (response.ok) {
                const html = await response.text();
                tabContent.innerHTML = html;

                // Load external script if specified, then call init function
                if (tab.script_url) {
                    await this.loadPluginScript(tab.script_url, tab.init_func);
                } else {
                    // Execute any inline scripts in the loaded content
                    this.executeScripts(tabContent);
                }
            } else {
                tabContent.innerHTML = `<div class="error-message">Failed to load plugin tab: ${response.status}</div>`;
            }
        } catch (error) {
            console.error('Error loading plugin tab:', error);
            tabContent.innerHTML = `<div class="error-message">Error loading plugin: ${error.message}</div>`;
        }

        // Update URL hash
        window.location.hash = `/plugin/${pluginId}`;
    }

    // Load an external plugin script and call its init function
    async loadPluginScript(scriptUrl, initFunc) {
        return new Promise((resolve, reject) => {
            // Check if script is already loaded
            const existingScript = document.querySelector(`script[src="${scriptUrl}"]`);
            if (existingScript) {
                // Script already loaded, just call init function
                if (initFunc && typeof window[initFunc] === 'function') {
                    try {
                        window[initFunc]();
                    } catch (error) {
                        console.error('Error calling plugin init function:', error);
                    }
                }
                resolve();
                return;
            }

            // Load the script
            const script = document.createElement('script');
            script.src = scriptUrl;
            script.onload = () => {
                console.log('Plugin script loaded:', scriptUrl);
                // Call init function if specified
                if (initFunc && typeof window[initFunc] === 'function') {
                    try {
                        window[initFunc]();
                    } catch (error) {
                        console.error('Error calling plugin init function:', error);
                    }
                }
                resolve();
            };
            script.onerror = (error) => {
                console.error('Failed to load plugin script:', scriptUrl, error);
                reject(error);
            };
            document.head.appendChild(script);
        });
    }

    // Execute scripts in loaded content
    executeScripts(container) {
        const scripts = container.querySelectorAll('script');
        scripts.forEach(script => {
            // Remove the original script tag first to prevent double execution
            script.remove();

            if (script.src) {
                // External scripts - load asynchronously
                const newScript = document.createElement('script');
                newScript.src = script.src;
                document.head.appendChild(newScript);
            } else {
                // Inline scripts - execute synchronously using eval in global scope
                // This ensures functions assigned to window are available for onclick handlers
                try {
                    // Use indirect eval to execute in global scope
                    const globalEval = eval;
                    globalEval(script.textContent);
                } catch (error) {
                    console.error('Error executing plugin script:', error);
                }
            }
        });
    }

    // Render badges for a container element
    renderBadges(badges) {
        if (!badges || badges.length === 0) {
            return '';
        }

        return badges.map(badge => {
            const colorClass = badge.color ? `badge-${badge.color}` : '';
            const clickHandler = badge.click_url ? `onclick="window.open('${escapeAttr(badge.click_url)}', '_blank')"` : '';
            const cursorStyle = badge.click_url ? 'cursor: pointer;' : '';

            return `
                <span class="plugin-badge ${colorClass}"
                      title="${escapeAttr(badge.tooltip || '')}"
                      style="${cursorStyle}"
                      ${clickHandler}>
                    ${badge.icon ? `<span class="badge-icon">${badge.icon}</span>` : ''}
                    <span class="badge-label">${escapeHtml(badge.label)}</span>
                </span>
            `;
        }).join('');
    }

    // Get plugin settings
    async getPluginSettings(pluginId) {
        try {
            const response = await fetchWithAuth(`/api/plugins/${pluginId}/settings`);
            if (response.ok) {
                return await response.json();
            }
        } catch (error) {
            console.error('Error loading plugin settings:', error);
        }
        return {};
    }

    // Save plugin settings
    async savePluginSettings(pluginId, settings) {
        try {
            const response = await fetchWithAuth(`/api/plugins/${pluginId}/settings`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(settings)
            });
            return response.ok;
        } catch (error) {
            console.error('Error saving plugin settings:', error);
            return false;
        }
    }

    // Enable a plugin
    async enablePlugin(pluginId) {
        try {
            const response = await fetchWithAuth(`/api/plugins/${pluginId}/enable`, {
                method: 'PUT'
            });
            if (response.ok) {
                await this.loadPlugins();
                await this.loadTabs();
                this.renderIntegrationsMenu();
                return true;
            }
        } catch (error) {
            console.error('Error enabling plugin:', error);
        }
        return false;
    }

    // Disable a plugin
    async disablePlugin(pluginId) {
        try {
            const response = await fetchWithAuth(`/api/plugins/${pluginId}/disable`, {
                method: 'PUT'
            });
            if (response.ok) {
                await this.loadPlugins();
                await this.loadTabs();
                this.renderIntegrationsMenu();
                return true;
            }
        } catch (error) {
            console.error('Error disabling plugin:', error);
        }
        return false;
    }

    // Get all plugins info
    getPlugins() {
        return this.plugins;
    }

    // Check if a plugin is enabled
    isPluginEnabled(pluginId) {
        const plugin = this.plugins.find(p => p.id === pluginId);
        return plugin ? plugin.enabled !== false : false;
    }
}

// Utility: Format uptime from started_at timestamp
function formatUptime(startedAt) {
    if (!startedAt) return '';

    const now = new Date();
    const started = new Date(startedAt);

    // Check for invalid date
    if (isNaN(started.getTime()) || started.getFullYear() < 2000) {
        return '';
    }

    const diff = now - started;
    if (diff < 0) return '';

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

// Global plugin manager instance
let pluginManager = new PluginManager();

// Initialize when DOM is ready (after app.js initialization)
document.addEventListener('DOMContentLoaded', () => {
    // Delay initialization to ensure app.js has loaded
    setTimeout(() => {
        pluginManager.init();
    }, 100);
});

// Handle plugin tab routing
window.addEventListener('hashchange', () => {
    const hash = window.location.hash.slice(1);
    if (hash && hash.startsWith('/plugin/')) {
        const pluginId = hash.replace('/plugin/', '');
        pluginManager.showPluginTab(pluginId);
    }
});
