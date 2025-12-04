/**
 * Plugin Management UI Functions
 *
 * Handles installation, updates, and management of external plugins
 * from the Settings page.
 */

/**
 * Install a plugin from a GitHub repository URL
 */
async function installPlugin() {
  const repoUrlInput = document.getElementById('pluginRepoUrl');
  const repoUrl = repoUrlInput.value.trim();

  if (!repoUrl) {
    showToast('Please enter a repository URL', 'error');
    return;
  }

  // Validate GitHub URL format
  if (!repoUrl.match(/^https:\/\/github\.com\/[\w-]+\/[\w-]+\/?$/)) {
    showToast('Invalid GitHub repository URL format', 'error');
    return;
  }

  try {
    showToast('Installing plugin...', 'info');

    const response = await fetch('/api/plugins/install', {
      method: 'POST',
      credentials: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        repository_url: repoUrl,
        version: 'latest',
      }),
    });

    const data = await response.json();

    if (!response.ok) {
      throw new Error(data.error || 'Failed to install plugin');
    }

    showToast('Plugin installed successfully!', 'success');
    repoUrlInput.value = '';

    // Refresh plugin list
    await refreshPluginList();

    // Reload plugin loader to pick up new plugin
    if (window.pluginLoader) {
      await window.pluginLoader.loadAllPlugins();
    }
  } catch (error) {
    console.error('Plugin installation error:', error);
    showToast(`Installation failed: ${error.message}`, 'error');
  }
}

/**
 * Refresh the list of installed plugins
 */
async function refreshPluginList() {
  const pluginListContainer = document.getElementById('pluginList');

  if (!pluginListContainer) {
    return;
  }

  try {
    pluginListContainer.innerHTML = '<div style="text-align: center; padding: 20px; color: #6c757d;">Loading plugins...</div>';

    const response = await fetch('/api/plugins', {
      credentials: 'same-origin',
    });

    if (!response.ok) {
      throw new Error('Failed to fetch plugins');
    }

    const plugins = await response.json();

    // Filter for external plugins
    const externalPlugins = plugins.filter(p => p.type === 'external');

    if (externalPlugins.length === 0) {
      pluginListContainer.innerHTML = `
        <div style="text-align: center; padding: 30px; color: #6c757d;">
          <p style="margin-bottom: 10px;">📦 No external plugins installed</p>
          <p style="font-size: 13px;">Install a plugin using the form above to get started.</p>
        </div>
      `;
      return;
    }

    // Render plugin cards
    pluginListContainer.innerHTML = externalPlugins.map(plugin => renderPluginCard(plugin)).join('');
  } catch (error) {
    console.error('Failed to load plugin list:', error);
    pluginListContainer.innerHTML = `
      <div style="text-align: center; padding: 30px; color: #dc3545;">
        <p>Failed to load plugins: ${error.message}</p>
        <button onclick="refreshPluginList()" class="btn btn-secondary" style="margin-top: 10px;">Retry</button>
      </div>
    `;
  }
}

/**
 * Render a plugin card
 */
function renderPluginCard(plugin) {
  const statusColor = plugin.enabled ? '#28a745' : '#6c757d';
  const statusText = plugin.enabled ? 'Enabled' : 'Disabled';
  const statusIcon = plugin.enabled ? '✓' : '○';

  return `
    <div class="plugin-card" style="border: 1px solid #dee2e6; border-radius: 8px; padding: 20px; margin-bottom: 15px; background: white;">
      <div style="display: flex; justify-content: space-between; align-items: flex-start;">
        <div style="flex: 1;">
          <div style="display: flex; align-items: center; gap: 10px; margin-bottom: 8px;">
            <h4 style="margin: 0; font-size: 16px;">${escapeHtml(plugin.name)}</h4>
            <span style="display: inline-block; padding: 3px 10px; background: ${statusColor}; color: white; border-radius: 12px; font-size: 12px; font-weight: 500;">
              ${statusIcon} ${statusText}
            </span>
            ${plugin.version ? `<span style="color: #6c757d; font-size: 13px;">v${escapeHtml(plugin.version)}</span>` : ''}
          </div>
          <p style="margin: 0 0 10px 0; font-size: 14px; color: #6c757d;">
            ${escapeHtml(plugin.description || 'No description available')}
          </p>
          ${plugin.author ? `<div style="font-size: 13px; color: #6c757d; margin-bottom: 10px;">
            👤 ${escapeHtml(plugin.author)}
          </div>` : ''}
          ${plugin.permissions ? `<div style="font-size: 13px; color: #6c757d; margin-bottom: 10px;">
            🔐 Permissions: ${plugin.permissions.join(', ')}
          </div>` : ''}
        </div>
        <div style="display: flex; flex-direction: column; gap: 8px; min-width: 120px;">
          ${plugin.enabled ?
            `<button onclick="disablePlugin('${plugin.id}')" class="btn btn-secondary" style="font-size: 13px; padding: 6px 12px; width: 100%;">Disable</button>` :
            `<button onclick="enablePlugin('${plugin.id}')" class="btn btn-primary" style="font-size: 13px; padding: 6px 12px; width: 100%;">Enable</button>`
          }
          <button onclick="updatePlugin('${plugin.id}')" class="btn btn-secondary" style="font-size: 13px; padding: 6px 12px; width: 100%;">
            Update
          </button>
          <button onclick="showPluginLogs('${plugin.id}')" class="btn btn-secondary" style="font-size: 13px; padding: 6px 12px; width: 100%;">
            View Logs
          </button>
          <button onclick="uninstallPlugin('${plugin.id}', '${escapeHtml(plugin.name)}')" class="btn btn-danger" style="font-size: 13px; padding: 6px 12px; width: 100%;">
            Uninstall
          </button>
        </div>
      </div>
    </div>
  `;
}

/**
 * Enable a plugin
 */
async function enablePlugin(pluginId) {
  try {
    showToast('Enabling plugin...', 'info');

    const response = await fetch(`/api/plugins/${pluginId}/enable`, {
      method: 'PUT',
      credentials: 'same-origin',
    });

    if (!response.ok) {
      const data = await response.json();
      throw new Error(data.error || 'Failed to enable plugin');
    }

    showToast('Plugin enabled successfully!', 'success');
    await refreshPluginList();

    // Reload plugin loader
    if (window.pluginLoader) {
      await window.pluginLoader.loadAllPlugins();
    }
  } catch (error) {
    console.error('Enable plugin error:', error);
    showToast(`Failed to enable plugin: ${error.message}`, 'error');
  }
}

/**
 * Disable a plugin
 */
async function disablePlugin(pluginId) {
  try {
    showToast('Disabling plugin...', 'info');

    const response = await fetch(`/api/plugins/${pluginId}/disable`, {
      method: 'PUT',
      credentials: 'same-origin',
    });

    if (!response.ok) {
      const data = await response.json();
      throw new Error(data.error || 'Failed to disable plugin');
    }

    showToast('Plugin disabled successfully!', 'success');
    await refreshPluginList();

    // Unload plugin from loader
    if (window.pluginLoader) {
      window.pluginLoader.unloadPlugin(pluginId);
    }
  } catch (error) {
    console.error('Disable plugin error:', error);
    showToast(`Failed to disable plugin: ${error.message}`, 'error');
  }
}

/**
 * Update a plugin to the latest version
 */
async function updatePlugin(pluginId) {
  if (!confirm('Update this plugin to the latest version?')) {
    return;
  }

  try {
    showToast('Updating plugin...', 'info');

    const response = await fetch(`/api/plugins/${pluginId}/update`, {
      method: 'POST',
      credentials: 'same-origin',
    });

    if (!response.ok) {
      const data = await response.json();
      throw new Error(data.error || 'Failed to update plugin');
    }

    showToast('Plugin updated successfully!', 'success');
    await refreshPluginList();

    // Reload plugin
    if (window.pluginLoader) {
      await window.pluginLoader.reloadPlugin(pluginId);
    }
  } catch (error) {
    console.error('Update plugin error:', error);
    showToast(`Failed to update plugin: ${error.message}`, 'error');
  }
}

/**
 * Uninstall a plugin
 */
async function uninstallPlugin(pluginId, pluginName) {
  if (!confirm(`Are you sure you want to uninstall "${pluginName}"?\n\nThis will remove all plugin files and data.`)) {
    return;
  }

  try {
    showToast('Uninstalling plugin...', 'info');

    const response = await fetch(`/api/plugins/${pluginId}/uninstall`, {
      method: 'DELETE',
      credentials: 'same-origin',
    });

    if (!response.ok) {
      const data = await response.json();
      throw new Error(data.error || 'Failed to uninstall plugin');
    }

    showToast('Plugin uninstalled successfully!', 'success');
    await refreshPluginList();

    // Unload plugin from loader
    if (window.pluginLoader) {
      window.pluginLoader.unloadPlugin(pluginId);
    }
  } catch (error) {
    console.error('Uninstall plugin error:', error);
    showToast(`Failed to uninstall plugin: ${error.message}`, 'error');
  }
}

/**
 * Show plugin logs in a modal
 */
async function showPluginLogs(pluginId) {
  try {
    const response = await fetch(`/api/plugins/${pluginId}/logs`, {
      credentials: 'same-origin',
    });

    if (!response.ok) {
      const data = await response.json();
      throw new Error(data.error || 'Failed to fetch logs');
    }

    const logsData = await response.json();

    // Create modal
    const modal = document.createElement('div');
    modal.className = 'modal';
    modal.style.display = 'block';
    modal.innerHTML = `
      <div class="modal-content modal-large">
        <div class="modal-header">
          <h3>Plugin Logs: ${pluginId}</h3>
          <button class="close-btn" onclick="this.closest('.modal').remove()">&times;</button>
        </div>
        <div class="modal-body">
          <div style="margin-bottom: 20px;">
            <h4 style="margin-bottom: 10px;">Standard Output</h4>
            <pre style="background: #f8f9fa; padding: 15px; border-radius: 4px; max-height: 300px; overflow-y: auto; font-size: 12px; line-height: 1.4;">${logsData.stdout && logsData.stdout.length > 0 ? escapeHtml(logsData.stdout.join('\n')) : 'No stdout logs'}</pre>
          </div>
          <div>
            <h4 style="margin-bottom: 10px;">Standard Error</h4>
            <pre style="background: #fff3cd; padding: 15px; border-radius: 4px; max-height: 300px; overflow-y: auto; font-size: 12px; line-height: 1.4; color: #856404;">${logsData.stderr && logsData.stderr.length > 0 ? escapeHtml(logsData.stderr.join('\n')) : 'No stderr logs'}</pre>
          </div>
        </div>
      </div>
    `;

    document.body.appendChild(modal);

    // Close on background click
    modal.addEventListener('click', (e) => {
      if (e.target === modal) {
        modal.remove();
      }
    });
  } catch (error) {
    console.error('Show logs error:', error);
    showToast(`Failed to show logs: ${error.message}`, 'error');
  }
}

/**
 * Escape HTML to prevent XSS
 */
function escapeHtml(text) {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

// Load plugin list when settings tab is opened
if (typeof window !== 'undefined') {
  // Refresh plugin list when switching to settings tab
  const originalSwitchTab = window.switchTab;
  if (typeof originalSwitchTab === 'function') {
    window.switchTab = function(tabName, skipHistory) {
      const result = originalSwitchTab.call(this, tabName, skipHistory);
      if (tabName === 'settings') {
        setTimeout(() => refreshPluginList(), 100);
      }
      return result;
    };
  }

  // Initial load if on settings tab
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
      setTimeout(() => {
        const settingsTab = document.getElementById('settingsTab');
        if (settingsTab && !settingsTab.classList.contains('hidden')) {
          refreshPluginList();
        }
      }, 500);
    });
  }
}
