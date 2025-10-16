// State
let containers = [];
let hosts = [];
let scanResults = [];
let images = {};
let autoRefreshInterval = null;
let currentTab = 'containers';

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    setupEventListeners();
    loadVersion();
    loadData();
    startAutoRefresh();
});

// Event Listeners
function setupEventListeners() {
    document.getElementById('refreshBtn').addEventListener('click', loadData);
    document.getElementById('scanBtn').addEventListener('click', triggerScan);
    document.getElementById('submitTelemetryBtn').addEventListener('click', submitTelemetry);
    document.getElementById('reloadConfigBtn').addEventListener('click', reloadConfig);
    document.getElementById('autoRefresh').addEventListener('change', handleAutoRefreshToggle);
    document.getElementById('searchInput').addEventListener('input', filterContainers);
    document.getElementById('hostFilter').addEventListener('change', filterContainers);
    document.getElementById('stateFilter').addEventListener('change', filterContainers);

    // Tab switching
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.addEventListener('click', (e) => switchTab(e.target.dataset.tab));
    });

    // Modal close on background click
    document.getElementById('logModal').addEventListener('click', (e) => {
        if (e.target.classList.contains('modal')) closeLogModal();
    });
    document.getElementById('confirmModal').addEventListener('click', (e) => {
        if (e.target.classList.contains('modal')) closeConfirmModal();
    });
    document.getElementById('confirmCancelBtn').addEventListener('click', closeConfirmModal);

    // Add Agent modal handlers
    const addAgentBtn = document.getElementById('addAgentBtn');
    const closeAddAgent = document.getElementById('closeAddAgent');
    const cancelAgentBtn = document.getElementById('cancelAgentBtn');
    const testAgentBtn = document.getElementById('testAgentBtn');
    const addAgentForm = document.getElementById('addAgentForm');
    const addAgentModal = document.getElementById('addAgentModal');

    if (addAgentBtn) addAgentBtn.addEventListener('click', openAddAgentModal);
    if (closeAddAgent) closeAddAgent.addEventListener('click', closeAddAgentModal);
    if (cancelAgentBtn) cancelAgentBtn.addEventListener('click', closeAddAgentModal);
    if (testAgentBtn) testAgentBtn.addEventListener('click', testAgentConnection);
    if (addAgentForm) addAgentForm.addEventListener('submit', handleAddAgent);
    if (addAgentModal) {
        addAgentModal.addEventListener('click', (e) => {
            if (e.target.classList.contains('modal')) closeAddAgentModal();
        });
    }
}

// Tab Management
function switchTab(tab) {
    currentTab = tab;

    // Update tab buttons
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.tab === tab);
    });

    // Update tab content
    document.querySelectorAll('.tab-content').forEach(content => {
        content.classList.remove('active');
    });
    document.getElementById(`${tab}Tab`).classList.add('active');

    // Load data for the tab
    if (tab === 'images' && Object.keys(images).length === 0) {
        loadImages();
    } else if (tab === 'hosts') {
        renderHosts(hosts);
    }
}

// Load version from API
async function loadVersion() {
    try {
        const response = await fetch('/api/health');
        const data = await response.json();
        if (data.version) {
            document.getElementById('versionBadge').textContent = 'v' + data.version;
        }
    } catch (error) {
        console.error('Error loading version:', error);
    }
}

// Auto-refresh
function startAutoRefresh() {
    const checkbox = document.getElementById('autoRefresh');
    if (checkbox.checked) {
        autoRefreshInterval = setInterval(() => {
            if (currentTab === 'containers') {
                loadContainers();
            } else if (currentTab === 'images') {
                loadImages();
            } else if (currentTab === 'scans') {
                loadScanResults();
            }
        }, 30000); // 30 seconds
    }
}

function stopAutoRefresh() {
    if (autoRefreshInterval) {
        clearInterval(autoRefreshInterval);
        autoRefreshInterval = null;
    }
}

function handleAutoRefreshToggle(e) {
    if (e.target.checked) {
        startAutoRefresh();
    } else {
        stopAutoRefresh();
    }
}

// Data Loading
async function loadData() {
    try {
        await Promise.all([
            loadHosts(),
            loadContainers(),
            loadScanResults()
        ]);
        updateStats();
        updateHostFilter();

        if (currentTab === 'images') {
            await loadImages();
        } else if (currentTab === 'hosts') {
            renderHosts(hosts);
        }
    } catch (error) {
        console.error('Error loading data:', error);
    }
}

async function loadHosts() {
    try {
        const response = await fetch('/api/hosts');
        hosts = await response.json();
    } catch (error) {
        console.error('Error loading hosts:', error);
        hosts = [];
    }
}

async function loadContainers() {
    try {
        const response = await fetch('/api/containers');
        containers = await response.json() || [];
        renderContainers(containers);
        updateStats();
    } catch (error) {
        console.error('Error loading containers:', error);
        containers = [];
        document.getElementById('containersBody').innerHTML =
            '<tr><td colspan="8" class="error">Failed to load containers</td></tr>';
    }
}

async function loadImages() {
    try {
        const response = await fetch('/api/images');
        images = await response.json() || {};
        renderImages(images);
    } catch (error) {
        console.error('Error loading images:', error);
        images = {};
        document.getElementById('imagesBody').innerHTML =
            '<tr><td colspan="7" class="error">Failed to load images</td></tr>';
    }
}

async function loadScanResults() {
    try {
        const response = await fetch('/api/scan/results?limit=10');
        scanResults = await response.json() || [];
        renderScanResults(scanResults);
        updateStats();
    } catch (error) {
        console.error('Error loading scan results:', error);
        scanResults = [];
        document.getElementById('scanResultsBody').innerHTML =
            '<tr><td colspan="5" class="error">Failed to load scan results</td></tr>';
    }
}

async function triggerScan() {
    const btn = document.getElementById('scanBtn');
    btn.disabled = true;
    btn.textContent = 'Scanning...';

    try {
        const response = await fetch('/api/scan', { method: 'POST' });
        if (response.ok) {
            // Wait a bit then refresh
            setTimeout(() => {
                loadData();
                btn.disabled = false;
                btn.textContent = 'Trigger Scan';
            }, 2000);
        }
    } catch (error) {
        console.error('Error triggering scan:', error);
        btn.disabled = false;
        btn.textContent = 'Trigger Scan';
    }
}

async function submitTelemetry() {
    const btn = document.getElementById('submitTelemetryBtn');
    btn.disabled = true;
    btn.textContent = 'Submitting...';

    try {
        const response = await fetch('/api/telemetry/submit', { method: 'POST' });
        if (response.ok) {
            const data = await response.json();
            showNotification(data.message || 'Telemetry submitted successfully', 'success');
        } else {
            const error = await response.json();
            showNotification('Failed to submit telemetry: ' + (error.error || 'Unknown error'), 'error');
        }
    } catch (error) {
        console.error('Error submitting telemetry:', error);
        showNotification('Failed to submit telemetry: ' + error.message, 'error');
    } finally {
        btn.disabled = false;
        btn.textContent = 'Submit Telemetry';
    }
}

async function reloadConfig() {
    const btn = document.getElementById('reloadConfigBtn');
    btn.disabled = true;
    btn.textContent = 'Reloading...';

    try {
        const response = await fetch('/api/config/reload', { method: 'POST' });
        if (response.ok) {
            const data = await response.json();
            let message = data.message;
            if (data.added > 0 || data.updated > 0) {
                message += ` - Added: ${data.added}, Updated: ${data.updated}`;
            }
            if (data.errors && data.errors.length > 0) {
                showNotification(message + ' (with errors)', 'error');
                console.error('Config reload errors:', data.errors);
            } else {
                showNotification(message, 'success');
            }
            // Refresh data after config reload
            setTimeout(() => loadData(), 1000);
        } else {
            const error = await response.json();
            showNotification('Failed to reload config: ' + error.error, 'error');
        }
    } catch (error) {
        console.error('Error reloading config:', error);
        showNotification('Failed to reload config', 'error');
    } finally {
        btn.disabled = false;
        btn.textContent = 'Reload Config';
    }
}

// Container Management Actions
async function startContainer(hostId, containerId, containerName) {
    try {
        const response = await fetch(`/api/containers/${hostId}/${containerId}/start`, {
            method: 'POST'
        });

        if (response.ok) {
            showNotification(`Container "${containerName}" started successfully`, 'success');
            await loadContainers();
        } else {
            const error = await response.json();
            showNotification(`Failed to start container: ${error.error}`, 'error');
        }
    } catch (error) {
        console.error('Error starting container:', error);
        showNotification('Failed to start container', 'error');
    }
}

async function stopContainer(hostId, containerId, containerName) {
    showConfirmDialog(
        'Stop Container',
        `Are you sure you want to stop "${containerName}"?`,
        async () => {
            try {
                const response = await fetch(`/api/containers/${hostId}/${containerId}/stop`, {
                    method: 'POST'
                });

                if (response.ok) {
                    showNotification(`Container "${containerName}" stopped successfully`, 'success');
                    await loadContainers();
                } else {
                    const error = await response.json();
                    showNotification(`Failed to stop container: ${error.error}`, 'error');
                }
            } catch (error) {
                console.error('Error stopping container:', error);
                showNotification('Failed to stop container', 'error');
            }
        }
    );
}

async function restartContainer(hostId, containerId, containerName) {
    showConfirmDialog(
        'Restart Container',
        `Are you sure you want to restart "${containerName}"?`,
        async () => {
            try {
                const response = await fetch(`/api/containers/${hostId}/${containerId}/restart`, {
                    method: 'POST'
                });

                if (response.ok) {
                    showNotification(`Container "${containerName}" restarted successfully`, 'success');
                    await loadContainers();
                } else {
                    const error = await response.json();
                    showNotification(`Failed to restart container: ${error.error}`, 'error');
                }
            } catch (error) {
                console.error('Error restarting container:', error);
                showNotification('Failed to restart container', 'error');
            }
        }
    );
}

async function removeContainer(hostId, containerId, containerName) {
    showConfirmDialog(
        'Remove Container',
        `Are you sure you want to remove "${containerName}"? This action cannot be undone.`,
        async () => {
            try {
                const response = await fetch(`/api/containers/${hostId}/${containerId}?force=true`, {
                    method: 'DELETE'
                });

                if (response.ok) {
                    showNotification(`Container "${containerName}" removed successfully`, 'success');
                    await loadContainers();
                } else {
                    const error = await response.json();
                    showNotification(`Failed to remove container: ${error.error}`, 'error');
                }
            } catch (error) {
                console.error('Error removing container:', error);
                showNotification('Failed to remove container', 'error');
            }
        },
        'danger'
    );
}

async function viewLogs(hostId, containerId, containerName) {
    document.getElementById('logContainerName').textContent = containerName;
    document.getElementById('logContent').textContent = 'Loading logs...';
    document.getElementById('logModal').classList.add('show');

    try {
        const response = await fetch(`/api/containers/${hostId}/${containerId}/logs?tail=500`);

        if (response.ok) {
            const data = await response.json();
            document.getElementById('logContent').textContent = data.logs || 'No logs available';
        } else {
            const error = await response.json();
            document.getElementById('logContent').textContent = `Error: ${error.error}`;
        }
    } catch (error) {
        console.error('Error loading logs:', error);
        document.getElementById('logContent').textContent = 'Failed to load logs';
    }
}

// Image Management Actions
async function removeImage(hostId, imageId, imageName) {
    showConfirmDialog(
        'Remove Image',
        `Are you sure you want to remove image "${imageName}"?`,
        async () => {
            try {
                const response = await fetch(`/api/images/${hostId}/${encodeURIComponent(imageId)}?force=true`, {
                    method: 'DELETE'
                });

                if (response.ok) {
                    showNotification(`Image "${imageName}" removed successfully`, 'success');
                    await loadImages();
                } else {
                    const error = await response.json();
                    showNotification(`Failed to remove image: ${error.error}`, 'error');
                }
            } catch (error) {
                console.error('Error removing image:', error);
                showNotification('Failed to remove image', 'error');
            }
        },
        'danger'
    );
}

async function pruneImages(hostId, hostName) {
    showConfirmDialog(
        'Prune Images',
        `Are you sure you want to prune all unused images on "${hostName}"? This will remove all dangling images.`,
        async () => {
            try {
                const response = await fetch(`/api/images/host/${hostId}/prune`, {
                    method: 'POST'
                });

                if (response.ok) {
                    const data = await response.json();
                    const sizeMB = (data.space_reclaimed / (1024 * 1024)).toFixed(2);
                    showNotification(`Images pruned successfully. Space reclaimed: ${sizeMB} MB`, 'success');
                    await loadImages();
                } else {
                    const error = await response.json();
                    showNotification(`Failed to prune images: ${error.error}`, 'error');
                }
            } catch (error) {
                console.error('Error pruning images:', error);
                showNotification('Failed to prune images', 'error');
            }
        }
    );
}

// Rendering
function renderContainers(containersToRender) {
    const tbody = document.getElementById('containersBody');

    if (containersToRender.length === 0) {
        tbody.innerHTML = '<tr><td colspan="8" class="loading">No containers found</td></tr>';
        return;
    }

    tbody.innerHTML = containersToRender.map(container => {
        const isRunning = container.state === 'running';
        const isStopped = container.state === 'exited';

        return `
        <tr>
            <td><strong>${escapeHtml(container.host_name)}</strong></td>
            <td>${escapeHtml(container.name)}</td>
            <td><code>${escapeHtml(container.image)}</code></td>
            <td><span class="state-badge state-${container.state}">${container.state}</span></td>
            <td>${escapeHtml(container.status)}</td>
            <td class="port-list">${formatPorts(container.ports)}</td>
            <td class="time-ago">${formatDate(container.created)}</td>
            <td class="actions">
                ${isRunning ? `
                    <button class="btn-icon btn-stop" onclick="stopContainer(${container.host_id}, '${escapeAttr(container.id)}', '${escapeAttr(container.name)}')" title="Stop">⏹</button>
                    <button class="btn-icon btn-restart" onclick="restartContainer(${container.host_id}, '${escapeAttr(container.id)}', '${escapeAttr(container.name)}')" title="Restart">⟳</button>
                ` : ''}
                ${isStopped ? `
                    <button class="btn-icon btn-start" onclick="startContainer(${container.host_id}, '${escapeAttr(container.id)}', '${escapeAttr(container.name)}')" title="Start">▶</button>
                ` : ''}
                <button class="btn-icon btn-logs" onclick="viewLogs(${container.host_id}, '${escapeAttr(container.id)}', '${escapeAttr(container.name)}')" title="View Logs">📋</button>
                ${isStopped ? `
                    <button class="btn-icon btn-delete" onclick="removeContainer(${container.host_id}, '${escapeAttr(container.id)}', '${escapeAttr(container.name)}')" title="Remove">🗑</button>
                ` : ''}
            </td>
        </tr>
        `;
    }).join('');
}

function renderImages(imagesData) {
    const tbody = document.getElementById('imagesBody');

    try {
        const allImages = [];
        for (const [hostName, hostData] of Object.entries(imagesData)) {
            const hostId = hostData.host_id;
            const images = hostData.images || [];

            images.forEach(img => {
                allImages.push({
                    hostId,
                    hostName,
                    ...img
                });
            });
        }

        if (allImages.length === 0) {
            tbody.innerHTML = '<tr><td colspan="7" class="loading">No images found</td></tr>';
            return;
        }

    // Group by host to add prune button
    const hostButtons = {};
    for (const [hostName, hostData] of Object.entries(imagesData)) {
        const hostId = hostData.host_id;
        hostButtons[hostName] = `
            <button class="btn btn-sm btn-warning" onclick="pruneImages(${hostId}, '${escapeAttr(hostName)}')">
                Prune Unused Images
            </button>
        `;
    }

    // Add prune buttons above table
    const imagesSection = document.querySelector('.images-section h2');
    let pruneContainer = document.querySelector('.prune-buttons');
    if (!pruneContainer) {
        pruneContainer = document.createElement('div');
        pruneContainer.className = 'prune-buttons';
        imagesSection.parentNode.insertBefore(pruneContainer, imagesSection.nextSibling);
    }
    pruneContainer.innerHTML = Object.values(hostButtons).join(' ');

    tbody.innerHTML = allImages.map(img => {
        const repoTags = (img.RepoTags && img.RepoTags.length > 0) ? img.RepoTags : ['<none>:<none>'];
        const tagParts = repoTags[0].split(':');
        const tag = tagParts.pop() || 'none';
        const repo = tagParts.join(':') || 'none';
        const imageId = img.Id ? img.Id.replace('sha256:', '').substring(0, 12) : 'unknown';
        const sizeMB = (img.Size / (1024 * 1024)).toFixed(2);
        const created = new Date(img.Created * 1000);

        return `
        <tr>
            <td><strong>${escapeHtml(img.hostName || 'unknown')}</strong></td>
            <td><code>${escapeHtml(repo)}</code></td>
            <td><code>${escapeHtml(tag)}</code></td>
            <td><code>${imageId}</code></td>
            <td>${sizeMB} MB</td>
            <td class="time-ago">${formatDate(created.toISOString())}</td>
            <td class="actions">
                <button class="btn-icon btn-delete" onclick="removeImage(${img.hostId}, '${escapeAttr(img.Id || '')}', '${escapeAttr(repoTags[0] || '')}')" title="Remove">🗑</button>
            </td>
        </tr>
        `;
    }).join('');
    } catch (error) {
        console.error('Error rendering images:', error);
        tbody.innerHTML = '<tr><td colspan="7" class="error">Error rendering images. Check console for details.</td></tr>';
    }
}

function renderScanResults(results) {
    const tbody = document.getElementById('scanResultsBody');

    if (results.length === 0) {
        tbody.innerHTML = '<tr><td colspan="5" class="loading">No scan results yet</td></tr>';
        return;
    }

    tbody.innerHTML = results.map(result => {
        const duration = new Date(result.completed_at) - new Date(result.started_at);
        const durationText = `${(duration / 1000).toFixed(1)}s`;

        return `
            <tr>
                <td><strong>${escapeHtml(result.host_name)}</strong></td>
                <td class="time-ago">${formatDateTime(result.started_at)}</td>
                <td>${durationText}</td>
                <td class="${result.success ? 'scan-success' : 'scan-failed'}">
                    ${result.success ? '✓ Success' : '✗ Failed'}
                    ${result.error ? `<br><small>${escapeHtml(result.error)}</small>` : ''}
                </td>
                <td>${result.containers_found}</td>
            </tr>
        `;
    }).join('');
}

function renderHosts(hostsData) {
    const tbody = document.getElementById('hostsBody');

    if (!hostsData || hostsData.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" class="loading">No hosts configured</td></tr>';
        return;
    }

    tbody.innerHTML = hostsData.map(host => {
        const statusBadge = host.enabled
            ? (host.host_type === 'agent'
                ? (host.agent_status === 'online' ? '<span class="badge badge-success">Online</span>' : '<span class="badge badge-warning">Offline</span>')
                : '<span class="badge badge-success">Enabled</span>')
            : '<span class="badge badge-secondary">Disabled</span>';

        const lastSeen = host.last_seen ? formatDate(host.last_seen) : '-';
        const hostType = host.host_type || 'unknown';
        const typeIcon = {
            'agent': '🤖',
            'unix': '🐳',
            'tcp': '🌐',
            'ssh': '🔐',
            'unknown': '❓'
        }[hostType] || '❓';

        return `
        <tr>
            <td><strong>${escapeHtml(host.name)}</strong></td>
            <td>${typeIcon} ${escapeHtml(hostType)}</td>
            <td><code>${escapeHtml(host.address)}</code></td>
            <td>${statusBadge}</td>
            <td>${escapeHtml(host.description || '-')}</td>
            <td class="time-ago">${lastSeen}</td>
            <td class="actions">
                ${host.enabled
                    ? `<button class="btn-icon btn-warning" onclick="toggleHost(${host.id}, false)" title="Disable">⏸</button>`
                    : `<button class="btn-icon btn-success" onclick="toggleHost(${host.id}, true)" title="Enable">▶</button>`
                }
                <button class="btn-icon btn-delete" onclick="deleteHost(${host.id}, '${escapeAttr(host.name)}')" title="Delete">🗑</button>
            </td>
        </tr>
        `;
    }).join('');
}

async function toggleHost(hostId, enable) {
    try {
        const host = hosts.find(h => h.id === hostId);
        if (!host) return;

        const response = await fetch(`/api/hosts/${hostId}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ ...host, enabled: enable })
        });

        if (response.ok) {
            showNotification(`Host ${enable ? 'enabled' : 'disabled'} successfully`, 'success');
            loadData();
        } else {
            const error = await response.json();
            showNotification('Error: ' + (error.error || 'Failed to update host'), 'error');
        }
    } catch (error) {
        showNotification('Error: ' + error.message, 'error');
    }
}

async function deleteHost(hostId, hostName) {
    if (!confirm(`Are you sure you want to delete host "${hostName}"?\n\nThis will remove all associated container history.`)) {
        return;
    }

    try {
        const response = await fetch(`/api/hosts/${hostId}`, {
            method: 'DELETE'
        });

        if (response.ok) {
            showNotification(`Host "${hostName}" deleted successfully`, 'success');
            loadData();
        } else {
            const error = await response.json();
            showNotification('Error: ' + (error.error || 'Failed to delete host'), 'error');
        }
    } catch (error) {
        showNotification('Error: ' + error.message, 'error');
    }
}

function updateStats() {
    document.getElementById('totalHosts').textContent = hosts.length;
    document.getElementById('totalContainers').textContent = containers.length;

    const running = containers.filter(c => c.state === 'running').length;
    document.getElementById('runningContainers').textContent = running;

    if (scanResults.length > 0) {
        const lastScan = new Date(scanResults[0].started_at);
        document.getElementById('lastScan').textContent = formatTimeAgo(lastScan);
    } else {
        document.getElementById('lastScan').textContent = 'Never';
    }
}

function updateHostFilter() {
    const select = document.getElementById('hostFilter');
    const currentValue = select.value;

    select.innerHTML = '<option value="">All Hosts</option>' +
        hosts.map(host => `<option value="${host.id}">${escapeHtml(host.name)}</option>`).join('');

    select.value = currentValue;
}

// Filtering
function filterContainers() {
    const searchTerm = document.getElementById('searchInput').value.toLowerCase();
    const hostFilter = document.getElementById('hostFilter').value;
    const stateFilter = document.getElementById('stateFilter').value;

    const filtered = containers.filter(container => {
        const matchesSearch = searchTerm === '' ||
            container.name.toLowerCase().includes(searchTerm) ||
            container.image.toLowerCase().includes(searchTerm) ||
            container.host_name.toLowerCase().includes(searchTerm);

        const matchesHost = hostFilter === '' || container.host_id.toString() === hostFilter;
        const matchesState = stateFilter === '' || container.state === stateFilter;

        return matchesSearch && matchesHost && matchesState;
    });

    renderContainers(filtered);
}

// Modal Functions
function closeLogModal() {
    document.getElementById('logModal').classList.remove('show');
}

function showConfirmDialog(title, message, onConfirm, type = 'warning') {
    document.getElementById('confirmTitle').textContent = title;
    document.getElementById('confirmMessage').textContent = message;
    document.getElementById('confirmModal').classList.add('show');

    const okBtn = document.getElementById('confirmOkBtn');
    okBtn.className = type === 'danger' ? 'btn btn-danger' : 'btn btn-warning';

    okBtn.onclick = () => {
        closeConfirmModal();
        onConfirm();
    };
}

function closeConfirmModal() {
    document.getElementById('confirmModal').classList.remove('show');
}

// Notification
function showNotification(message, type = 'info') {
    // Create notification element
    const notification = document.createElement('div');
    notification.className = `notification notification-${type}`;
    notification.textContent = message;

    // Add to page
    document.body.appendChild(notification);

    // Show with animation
    setTimeout(() => notification.classList.add('show'), 10);

    // Remove after 5 seconds
    setTimeout(() => {
        notification.classList.remove('show');
        setTimeout(() => notification.remove(), 300);
    }, 5000);
}

// Formatting Helpers
function formatPorts(ports) {
    if (!ports || ports.length === 0) return '-';

    return ports
        .filter(p => p.public_port > 0)
        .map(p => `${p.public_port}:${p.private_port}/${p.type}`)
        .join('<br>') || '-';
}

function formatDate(dateStr) {
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now - date;
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

    if (diffDays === 0) return 'Today';
    if (diffDays === 1) return 'Yesterday';
    if (diffDays < 7) return `${diffDays} days ago`;
    if (diffDays < 30) return `${Math.floor(diffDays / 7)} weeks ago`;
    if (diffDays < 365) return `${Math.floor(diffDays / 30)} months ago`;
    return `${Math.floor(diffDays / 365)} years ago`;
}

function formatDateTime(dateStr) {
    const date = new Date(dateStr);
    return date.toLocaleString();
}

function formatTimeAgo(date) {
    const now = new Date();
    const diffMs = now - date;
    const diffMins = Math.floor(diffMs / (1000 * 60));

    if (diffMins < 1) return 'Just now';
    if (diffMins === 1) return '1 min ago';
    if (diffMins < 60) return `${diffMins} mins ago`;

    const diffHours = Math.floor(diffMins / 60);
    if (diffHours === 1) return '1 hour ago';
    if (diffHours < 24) return `${diffHours} hours ago`;

    const diffDays = Math.floor(diffHours / 24);
    if (diffDays === 1) return '1 day ago';
    return `${diffDays} days ago`;
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function escapeAttr(text) {
    return text.replace(/'/g, "\\'").replace(/"/g, '&quot;');
}

// Add Agent Host Modal Functions

function openAddAgentModal() {
    console.log('Opening add agent modal...');
    const modal = document.getElementById('addAgentModal');
    const form = document.getElementById('addAgentForm');
    const result = document.getElementById('agentTestResult');

    if (!modal) {
        console.error('Modal element not found!');
        return;
    }

    modal.classList.add('show');
    form.reset();
    result.style.display = 'none';
    console.log('Modal opened');
}

function closeAddAgentModal() {
    const modal = document.getElementById('addAgentModal');
    if (modal) {
        modal.classList.remove('show');
    }
}

async function testAgentConnection() {
    const address = document.getElementById('agentAddress').value;
    const token = document.getElementById('agentToken').value;
    const testBtn = document.getElementById('testAgentBtn');
    const result = document.getElementById('agentTestResult');

    if (!address || !token) {
        result.className = 'alert alert-error';
        result.textContent = 'Please enter both address and token';
        result.style.display = 'block';
        return;
    }

    testBtn.disabled = true;
    testBtn.textContent = 'Testing...';

    try {
        const response = await fetch('/api/hosts/agent/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ address, agent_token: token })
        });

        const data = await response.json();

        if (data.success) {
            result.className = 'alert alert-success';
            result.textContent = '✓ Connection successful! Agent is reachable.';
        } else {
            result.className = 'alert alert-error';
            result.textContent = '✗ Connection failed: ' + (data.error || 'Unknown error');
        }
        result.style.display = 'block';
    } catch (error) {
        result.className = 'alert alert-error';
        result.textContent = '✗ Error: ' + error.message;
        result.style.display = 'block';
    } finally {
        testBtn.disabled = false;
        testBtn.textContent = 'Test Connection';
    }
}

async function handleAddAgent(e) {
    e.preventDefault();

    const addressInput = document.getElementById('agentAddress');
    const address = addressInput.value.trim();

    // Validate address format
    const validProtocols = /^(https?|agent):\/\/.+/;
    if (!validProtocols.test(address)) {
        const result = document.getElementById('agentTestResult');
        result.className = 'alert alert-error';
        result.textContent = 'Invalid address format. Must start with http://, https://, or agent:// followed by hostname/IP and optional port (e.g., http://192.168.1.100:9876)';
        result.style.display = 'block';
        addressInput.focus();
        return;
    }

    const data = {
        name: document.getElementById('agentName').value,
        address: address,
        agent_token: document.getElementById('agentToken').value,
        description: document.getElementById('agentDescription').value
    };

    const saveBtn = document.getElementById('saveAgentBtn');
    const result = document.getElementById('agentTestResult');

    saveBtn.disabled = true;
    saveBtn.textContent = 'Adding...';

    try {
        const response = await fetch('/api/hosts/agent', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });

        if (response.ok) {
            showNotification('Agent host added successfully!', 'success');
            closeAddAgentModal();
            loadData(); // Refresh the data
        } else {
            const error = await response.json();
            result.className = 'alert alert-error';
            result.textContent = 'Error: ' + (error.error || 'Failed to add agent');
            result.style.display = 'block';
        }
    } catch (error) {
        result.className = 'alert alert-error';
        result.textContent = 'Error: ' + error.message;
        result.style.display = 'block';
    } finally {
        saveBtn.disabled = false;
        saveBtn.textContent = 'Add Agent';
    }
}
