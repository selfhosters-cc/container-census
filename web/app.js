// State
let containers = [];
let hosts = [];
let activities = [];
let images = {};
let graphData = null;
let cy = null; // Cytoscape instance
let autoRefreshInterval = null;
let currentTab = 'containers';
let lifecycles = [];

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    setupEventListeners();
    loadVersion();
    loadTelemetrySchedule();
    loadData();
    startAutoRefresh();
});

// Event Listeners
function setupEventListeners() {
    document.getElementById('scanBtn').addEventListener('click', triggerScan);
    document.getElementById('submitTelemetryBtn').addEventListener('click', submitTelemetry);
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

    // Graph filter handlers
    document.getElementById('showNetworks')?.addEventListener('change', applyGraphFilters);
    document.getElementById('showVolumes')?.addEventListener('change', applyGraphFilters);
    document.getElementById('showDepends')?.addEventListener('change', applyGraphFilters);
    document.getElementById('showLinks')?.addEventListener('change', applyGraphFilters);

    // Graph display option handlers
    document.getElementById('colorByProject')?.addEventListener('change', applyGraphFilters);
    document.getElementById('hideEdgeLabels')?.addEventListener('change', toggleEdgeLabels);

    // Activity log filter
    document.getElementById('activityTypeFilter')?.addEventListener('change', loadActivityLog);

    // History filter
    document.getElementById('historyHostFilter')?.addEventListener('change', loadContainerHistory);

    // Graph selector handlers
    document.getElementById('composeProjectSelect')?.addEventListener('change', handleComposeProjectChange);
    document.getElementById('networkSelect')?.addEventListener('change', handleNetworkChange);
    document.getElementById('layoutSelect')?.addEventListener('change', handleLayoutChange);

    // Graph search handler
    document.getElementById('graphSearch')?.addEventListener('input', handleGraphSearch);

    // Graph zoom control handlers
    document.getElementById('zoomInBtn')?.addEventListener('click', zoomIn);
    document.getElementById('zoomOutBtn')?.addEventListener('click', zoomOut);
    document.getElementById('zoomResetBtn')?.addEventListener('click', zoomReset);
    document.getElementById('fitGraphBtn')?.addEventListener('click', fitGraph);
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

    // Auto-refresh data when switching to a tab
    if (tab === 'containers') {
        loadContainers();
    } else if (tab === 'images') {
        loadImages();
    } else if (tab === 'hosts') {
        loadHosts().then(() => renderHosts(hosts));
    } else if (tab === 'graph') {
        loadGraph();
    } else if (tab === 'history') {
        loadContainerHistory();
    } else if (tab === 'activity') {
        loadActivityLog();
    } else if (tab === 'settings') {
        loadCollectors();
    }
}

// Load version from API
async function loadVersion() {
    try {
        const response = await fetch('/api/health');
        const data = await response.json();
        const badge = document.getElementById('versionBadge');

        if (data.version) {
            if (data.update_available && data.latest_version) {
                // Show update indicator
                badge.innerHTML = `v${data.version} → v${data.latest_version} <span style="font-size: 1.2em;">⬆️</span>`;
                badge.style.cursor = 'pointer';
                badge.title = 'Click to view update';
                badge.onclick = () => {
                    if (data.release_url) {
                        window.open(data.release_url, '_blank');
                    }
                };

                // Log update notification
                console.log(`🎉 Container Census update available: v${data.version} → v${data.latest_version}`);
                console.log(`   Download: ${data.release_url || 'https://github.com/selfhosters-cc/container-census/releases'}`);
            } else {
                // No update available
                badge.textContent = 'v' + data.version;
                badge.style.cursor = 'default';
                badge.title = 'Current version';
                badge.onclick = null;
            }
        }
    } catch (error) {
        console.error('Error loading version:', error);
    }
}

// Load telemetry schedule from API
async function loadTelemetrySchedule() {
    try {
        const response = await fetch('/api/telemetry/schedule');
        const data = await response.json();
        const scheduleDiv = document.getElementById('telemetrySchedule');

        if (data.enabled_endpoints === 0) {
            scheduleDiv.innerHTML = '<small style="color: #999;">No automatic telemetry (no endpoints configured)</small>';
            return;
        }

        if (data.next_submission) {
            const nextDate = new Date(data.next_submission);
            const now = new Date();
            const diffMs = nextDate - now;
            const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
            const diffMins = Math.floor((diffMs % (1000 * 60 * 60)) / (1000 * 60));

            let timeStr = '';
            if (diffMs < 0) {
                timeStr = 'overdue';
            } else if (diffHours < 1) {
                timeStr = `in ${diffMins} minutes`;
            } else if (diffHours < 24) {
                timeStr = `in ${diffHours} hour${diffHours > 1 ? 's' : ''}`;
            } else {
                const diffDays = Math.floor(diffHours / 24);
                timeStr = `in ${diffDays} day${diffDays > 1 ? 's' : ''}`;
            }

            const endpointText = data.enabled_endpoints === 1 ? 'endpoint' : 'endpoints';
            scheduleDiv.innerHTML = `<small style="color: #999;">Next telemetry: ${timeStr} to ${data.enabled_endpoints} ${endpointText}</small>`;
        } else if (data.message) {
            scheduleDiv.innerHTML = `<small style="color: #999;">${data.message}</small>`;
        }
    } catch (error) {
        console.error('Error loading telemetry schedule:', error);
    }
}

// Auto-refresh
function startAutoRefresh() {
    const checkbox = document.getElementById('autoRefresh');
    if (checkbox.checked) {
        autoRefreshInterval = setInterval(() => {
            // Always refresh telemetry schedule
            loadTelemetrySchedule();

            if (currentTab === 'containers') {
                loadContainers();
            } else if (currentTab === 'images') {
                loadImages();
            } else if (currentTab === 'activity') {
                loadActivityLog();
            } else if (currentTab === 'settings') {
                loadCollectors(); // Auto-refresh telemetry status
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
            loadActivityLog()
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

async function loadActivityLog() {
    try {
        const activityType = document.getElementById('activityTypeFilter')?.value || 'all';
        const response = await fetch(`/api/activity-log?limit=50&type=${activityType}`);
        activities = await response.json() || [];
        renderActivityLog(activities);
        updateStats();
    } catch (error) {
        console.error('Error loading activity log:', error);
        document.getElementById('activityLogBody').innerHTML =
            '<tr><td colspan="6" class="error">Failed to load activity log</td></tr>';
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

    // Add visual indicator to collector items
    const collectorItems = document.querySelectorAll('.collector-item');
    collectorItems.forEach(item => {
        item.classList.add('submitting');
    });

    try {
        const response = await fetch('/api/telemetry/submit', { method: 'POST' });
        if (response.ok) {
            const data = await response.json();
            showNotification(data.message || 'Telemetry submitted successfully', 'success');

            // Wait a moment for submission to complete, then refresh status
            setTimeout(async () => {
                if (currentTab === 'settings') {
                    await loadCollectors();
                }
            }, 1500);
        } else {
            const error = await response.json();
            showNotification('Failed to submit telemetry: ' + (error.error || 'Unknown error'), 'error');

            // Remove submitting state on error
            collectorItems.forEach(item => {
                item.classList.remove('submitting');
            });
        }
    } catch (error) {
        console.error('Error submitting telemetry:', error);
        showNotification('Failed to submit telemetry: ' + error.message, 'error');

        // Remove submitting state on error
        collectorItems.forEach(item => {
            item.classList.remove('submitting');
        });
    } finally {
        btn.disabled = false;
        btn.textContent = 'Submit Telemetry';
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

                    // Immediately remove from local state
                    containers = containers.filter(c => !(c.host_id === hostId && c.id === containerId));

                    // Update UI immediately
                    if (currentTab === 'containers') {
                        filterContainers(); // This will re-render with current filters
                    }

                    // Update stats
                    updateStats();

                    // Trigger a scan in the background to sync the database
                    fetch('/api/scan', { method: 'POST' }).catch(err =>
                        console.log('Background scan triggered:', err)
                    );
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

                    // Immediately remove from local state
                    for (const [hostName, hostData] of Object.entries(images)) {
                        if (hostData.host_id === hostId) {
                            images[hostName].images = (hostData.images || []).filter(img => img.Id !== imageId);
                            break;
                        }
                    }

                    // Update UI immediately
                    if (currentTab === 'images') {
                        renderImages(images);
                    }

                    // Trigger a scan in the background to sync the database
                    fetch('/api/scan', { method: 'POST' }).catch(err =>
                        console.log('Background scan triggered:', err)
                    );
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
                Prune Unused Images (${escapeHtml(hostName)})
            </button>
        `;
    }

    // Add prune buttons above table (one button per host)
    const imagesSection = document.querySelector('.images-section h2');
    let pruneContainer = document.querySelector('.prune-buttons');
    if (!pruneContainer) {
        pruneContainer = document.createElement('div');
        pruneContainer.className = 'prune-buttons';
        imagesSection.parentNode.insertBefore(pruneContainer, imagesSection.nextSibling);
    }
    // Update buttons - will show one "Prune Unused Images" button per host
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

function renderActivityLog(activities) {
    const tbody = document.getElementById('activityLogBody');

    if (!activities || activities.length === 0) {
        tbody.innerHTML = '<tr><td colspan="6" class="loading">No activity logged yet</td></tr>';
        return;
    }

    tbody.innerHTML = activities.map(activity => {
        const durationText = `${activity.duration.toFixed(2)}s`;
        const typeIcon = activity.type === 'scan' ? '🔍' : '📊';
        const typeLabel = activity.type === 'scan' ? 'Scan' : 'Telemetry';

        // Build details based on activity type
        let details = '';
        if (activity.type === 'scan') {
            details = `${activity.details.containers_found || 0} containers`;
        } else {
            const parts = [];
            if (activity.details.hosts_count) parts.push(`${activity.details.hosts_count} hosts`);
            if (activity.details.containers_count) parts.push(`${activity.details.containers_count} containers`);
            if (activity.details.images_count) parts.push(`${activity.details.images_count} images`);
            details = parts.join(', ');
        }

        return `
            <tr class="activity-${activity.type}">
                <td>${typeIcon} <strong>${typeLabel}</strong></td>
                <td><strong>${escapeHtml(activity.target)}</strong></td>
                <td class="time-ago">${formatDateTime(activity.timestamp)}</td>
                <td>${durationText}</td>
                <td class="${activity.success ? 'scan-success' : 'scan-failed'}">
                    ${activity.success ? '✓ Success' : '✗ Failed'}
                    ${activity.error ? `<br><small>${escapeHtml(activity.error)}</small>` : ''}
                </td>
                <td><small>${details}</small></td>
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
        let statusBadge;
        if (!host.enabled) {
            statusBadge = '<span class="badge badge-secondary">Disabled</span>';
        } else if (host.host_type === 'agent') {
            if (host.agent_status === 'online') {
                statusBadge = '<span class="badge badge-success">Online</span>';
            } else if (host.agent_status === 'auth_failed') {
                statusBadge = '<span class="badge badge-error" title="API token mismatch">Auth Failed</span>';
            } else {
                statusBadge = '<span class="badge badge-warning">Offline</span>';
            }
        } else {
            statusBadge = '<span class="badge badge-success">Enabled</span>';
        }

        // For agents, show precise datetime; for others, show relative time
        const lastSeen = host.last_seen
            ? (host.host_type === 'agent' ? formatDateTime(host.last_seen) : formatDate(host.last_seen))
            : '-';

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

    // Find most recent scan activity
    const scanActivities = activities.filter(a => a.type === 'scan');
    if (scanActivities.length > 0) {
        const lastScan = new Date(scanActivities[0].timestamp);
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
    if (!dateStr) return '-';

    const date = new Date(dateStr);

    // Check if date is valid
    if (isNaN(date.getTime())) return '-';

    // Check if date is zero/epoch or in the far future/past (invalid)
    const year = date.getFullYear();
    if (year < 1970 || year > 2100) return '-';

    const now = new Date();
    const diffMs = now - date;

    // If date is in the future, return '-'
    if (diffMs < 0) return '-';

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

// Settings Management
async function loadTelemetrySettings() {
    try {
        // Get current config to load frequency
        const response = await fetch('/api/config');
        const config = await response.json();

        // API returns IntervalHours (capital I) not interval_hours
        const intervalHours = config.Telemetry?.IntervalHours || 168;
        const dropdown = document.getElementById('telemetryFrequency');
        if (dropdown) {
            dropdown.value = intervalHours.toString();
            console.log('Loaded telemetry frequency:', intervalHours, 'Set dropdown to:', dropdown.value);
        }
    } catch (error) {
        console.error('Failed to load telemetry settings:', error);
    }
}

async function saveTelemetryFrequency() {
    const status = document.getElementById('frequencySaveStatus');
    const intervalHours = parseInt(document.getElementById('telemetryFrequency').value);

    status.textContent = 'Saving...';
    status.className = 'save-status-inline saving';

    try {
        // First, get current config to preserve community endpoint state
        const configResponse = await fetch('/api/config');
        const config = await configResponse.json();

        const communityEndpoint = config.telemetry?.endpoints?.find(e =>
            e.url === 'https://cc-telemetry.selfhosters.cc/api/ingest'
        );
        const isCommunityEnabled = communityEndpoint ? communityEndpoint.enabled : false;

        // Now save with preserved community state
        const response = await fetch('/api/config/telemetry', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                interval_hours: intervalHours,
                community_endpoint: isCommunityEnabled  // Preserve current state
            })
        });

        if (response.ok) {
            status.textContent = '✓ Saved';
            status.className = 'save-status-inline success';
            showNotification('Submission frequency updated successfully', 'success');
        } else {
            const error = await response.json();
            status.textContent = '✗ Failed';
            status.className = 'save-status-inline error';
        }
    } catch (error) {
        status.textContent = '✗ Error';
        status.className = 'save-status-inline error';
        console.error('Failed to save frequency:', error);
    }

    setTimeout(() => {
        status.textContent = '';
        status.className = 'save-status-inline';
    }, 3000);
}

// Initialize settings when switching to settings tab
document.addEventListener('DOMContentLoaded', () => {
    // Load settings immediately on page load
    loadTelemetrySettings();

    // Load settings when settings tab is clicked
    const settingsTab = document.querySelector('[data-tab="settings"]');
    if (settingsTab) {
        settingsTab.addEventListener('click', () => {
            setTimeout(() => {
                loadTelemetrySettings();
                loadCollectors();
            }, 100);
        });
    }
});

// Custom Collectors Management

async function loadCollectors() {
    try {
        // Fetch telemetry status which includes all endpoints with status info
        const [statusResponse, debugResponse] = await Promise.all([
            fetch('/api/telemetry/status'),
            fetch('/api/telemetry/debug-enabled')
        ]);

        if (!statusResponse.ok) {
            console.error('Failed to fetch telemetry status, status:', statusResponse.status);
            throw new Error('Failed to load collectors');
        }

        const collectors = await statusResponse.json();
        const debugInfo = debugResponse.ok ? await debugResponse.json() : { debug_enabled: false };

        console.log('Loaded collectors with status:', collectors);
        console.log('Debug mode:', debugInfo.debug_enabled);
        renderCollectors(collectors, debugInfo.debug_enabled);
    } catch (error) {
        console.error('Error loading collectors:', error);
        showNotification('Failed to load collectors', 'error');
    }
}

function renderCollectors(collectors, debugEnabled = false) {
    const collectorsList = document.getElementById('collectorsList');

    // Separate community and custom collectors
    const communityCollector = collectors.find(c => c.name === 'community');
    const customCollectors = collectors.filter(c => c.name !== 'community');

    let html = '';

    // Render community collector if exists
    if (communityCollector) {
        const lastSuccess = communityCollector.last_success ? new Date(communityCollector.last_success) : null;
        const lastFailure = communityCollector.last_failure ? new Date(communityCollector.last_failure) : null;
        const statusText = formatTelemetryStatus(lastSuccess, lastFailure);
        const statusClass = getStatusClass(lastSuccess, lastFailure);

        html += `
            <div class="collector-item community-collector" style="background: #f8f9fa; border: 2px solid #667eea; margin-bottom: 20px; padding: 20px;">
                <div style="display: flex; justify-content: space-between; align-items: flex-start;">
                    <div style="flex: 1;">
                        <div class="collector-name" style="font-size: 16px; margin-bottom: 8px;">
                            <strong>📊 Community Collector</strong>
                            <span class="collector-status ${communityCollector.enabled ? 'enabled' : 'disabled'}">
                                ${communityCollector.enabled ? 'Enabled' : 'Disabled'}
                            </span>
                        </div>
                        <div class="collector-url" style="margin: 8px 0; color: #666; font-size: 13px;">${escapeHtml(communityCollector.url)}</div>
                        <p style="margin: 10px 0; color: #555; font-size: 14px;">
                            Help improve Container Census by sharing anonymous usage statistics.
                        </p>

                        <div class="telemetry-info" style="display: grid; grid-template-columns: 1fr 1fr; gap: 15px; margin: 15px 0; padding: 15px; background: white; border-radius: 6px;">
                            <div class="info-column">
                                <h4 style="margin: 0 0 8px 0; font-size: 13px; color: #2e7d32;">✓ What gets shared:</h4>
                                <ul style="margin: 0; padding-left: 20px; font-size: 12px; color: #666;">
                                    <li>Container Census version</li>
                                    <li>Number of containers and hosts</li>
                                    <li>Popular container images (names only)</li>
                                    <li>Container registry distribution</li>
                                    <li>Geographic region (timezone-based)</li>
                                </ul>
                            </div>
                            <div class="info-column">
                                <h4 style="margin: 0 0 8px 0; font-size: 13px; color: #c62828;">✗ What is NOT shared:</h4>
                                <ul style="margin: 0; padding-left: 20px; font-size: 12px; color: #666;">
                                    <li>Host names or IP addresses</li>
                                    <li>Container names or env variables</li>
                                    <li>Any credentials or secrets</li>
                                    <li>Personal information</li>
                                </ul>
                            </div>
                        </div>

                        ${statusText ? `<div class="telemetry-status ${statusClass}">${statusText}</div>` : ''}
                        ${lastFailure && communityCollector.last_failure_reason ?
                            `<div class="telemetry-error" title="${escapeHtml(communityCollector.last_failure_reason)}">
                                ⚠ ${escapeHtml(communityCollector.last_failure_reason.substring(0, 80))}${communityCollector.last_failure_reason.length > 80 ? '...' : ''}
                            </div>` : ''}
                        ${debugEnabled && lastFailure ?
                            `<div style="margin-top: 10px;">
                                <button class="btn btn-sm btn-secondary" onclick="resetCircuitBreaker('${escapeAttr(communityCollector.name)}')" style="font-size: 12px;">
                                    🔧 Reset Circuit Breaker
                                </button>
                            </div>` : ''}
                    </div>
                    <div style="margin-left: 20px;">
                        <button class="btn ${communityCollector.enabled ? 'btn-warning' : 'btn-primary'}"
                                onclick="toggleCollector('${escapeAttr(communityCollector.name)}', ${!communityCollector.enabled})"
                                style="min-width: 100px; white-space: nowrap;">
                            ${communityCollector.enabled ? 'Disable' : 'Enable'}
                        </button>
                    </div>
                </div>
            </div>
        `;
    }

    // Add separator before custom collectors
    if (customCollectors.length > 0) {
        html += '<h4 style="margin: 30px 0 15px 0; color: #666;">Custom Collectors</h4>';
    }

    if (customCollectors.length === 0) {
        html += '<p style="color: #666; font-style: italic; margin-top: 20px;">No custom collectors configured.</p>';
    } else {
        html += customCollectors.map(collector => {
            const lastSuccess = collector.last_success ? new Date(collector.last_success) : null;
            const lastFailure = collector.last_failure ? new Date(collector.last_failure) : null;
            const statusText = formatTelemetryStatus(lastSuccess, lastFailure);
            const statusClass = getStatusClass(lastSuccess, lastFailure);

            return `
            <div class="collector-item">
                <div class="collector-info">
                    <div class="collector-name">
                        ${escapeHtml(collector.name)}
                        <span class="collector-status ${collector.enabled ? 'enabled' : 'disabled'}">
                            ${collector.enabled ? 'Enabled' : 'Disabled'}
                        </span>
                    </div>
                    <div class="collector-url">${escapeHtml(collector.url)}</div>
                    ${collector.api_key ? '<div style="font-size: 12px; color: #999;">🔑 API Key configured</div>' : ''}
                    ${statusText ? `<div class="telemetry-status ${statusClass}">${statusText}</div>` : ''}
                    ${lastFailure && collector.last_failure_reason ?
                        `<div class="telemetry-error" title="${escapeHtml(collector.last_failure_reason)}">
                            ⚠ ${escapeHtml(collector.last_failure_reason.substring(0, 60))}${collector.last_failure_reason.length > 60 ? '...' : ''}
                        </div>` : ''}
                    ${debugEnabled && lastFailure ?
                        `<div style="margin-top: 8px;">
                            <button class="btn btn-sm" onclick="resetCircuitBreaker('${escapeAttr(collector.name)}')" style="font-size: 11px; padding: 4px 8px;">
                                🔧 Reset Circuit Breaker
                            </button>
                        </div>` : ''}
                </div>
                <div class="collector-actions">
                    <button class="btn btn-sm btn-secondary" onclick="toggleCollector('${escapeAttr(collector.name)}', ${!collector.enabled})">
                        ${collector.enabled ? 'Disable' : 'Enable'}
                    </button>
                    <button class="btn btn-sm btn-danger" onclick="deleteCollector('${escapeAttr(collector.name)}')">
                        Delete
                    </button>
                </div>
            </div>
            `;
        }).join('');
    }

    collectorsList.innerHTML = html;
}

function formatTelemetryStatus(lastSuccess, lastFailure) {
    if (!lastSuccess && !lastFailure) {
        return 'No telemetry submitted yet';
    }

    if (!lastFailure || (lastSuccess && lastSuccess > lastFailure)) {
        return `✓ Last success: ${formatTimeAgo(lastSuccess)}`;
    } else {
        return `✗ Last failure: ${formatTimeAgo(lastFailure)}`;
    }
}

function getStatusClass(lastSuccess, lastFailure) {
    if (!lastSuccess && !lastFailure) {
        return 'status-unknown';
    }

    if (!lastFailure || (lastSuccess && lastSuccess > lastFailure)) {
        return 'status-success';
    } else {
        return 'status-error';
    }
}

async function testCollector() {
    const url = document.getElementById('collectorURL').value.trim();
    const apiKey = document.getElementById('collectorAPIKey').value.trim();
    const status = document.getElementById('collectorSaveStatus');

    if (!url) {
        status.textContent = '✗ URL is required to test';
        status.className = 'save-status-inline error';
        setTimeout(() => status.textContent = '', 3000);
        return;
    }

    status.textContent = 'Testing connection...';
    status.className = 'save-status-inline saving';

    const testData = { url };
    if (apiKey) {
        testData.api_key = apiKey;
    }

    try {
        const response = await fetch('/api/telemetry/test-endpoint', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(testData)
        });

        const result = await response.json();

        if (response.ok) {
            status.textContent = '✓ Connection successful!';
            status.className = 'save-status-inline success';
        } else {
            status.textContent = '✗ ' + (result.error || 'Connection failed');
            status.className = 'save-status-inline error';
        }
    } catch (error) {
        status.textContent = '✗ Connection failed: ' + error.message;
        status.className = 'save-status-inline error';
    }

    setTimeout(() => status.textContent = '', 5000);
}

async function addCollector() {
    const name = document.getElementById('collectorName').value.trim();
    const url = document.getElementById('collectorURL').value.trim();
    const apiKey = document.getElementById('collectorAPIKey').value.trim();
    const enabled = document.getElementById('collectorEnabled').checked;
    const status = document.getElementById('collectorSaveStatus');

    // Validate inputs
    if (!name) {
        status.textContent = '✗ Name is required';
        status.className = 'save-status-inline error';
        setTimeout(() => status.textContent = '', 3000);
        return;
    }

    if (!url) {
        status.textContent = '✗ URL is required';
        status.className = 'save-status-inline error';
        setTimeout(() => status.textContent = '', 3000);
        return;
    }

    // Show saving status
    status.textContent = 'Saving...';
    status.className = 'save-status-inline saving';

    const endpoint = {
        name,
        url,
        enabled
    };

    if (apiKey) {
        endpoint.api_key = apiKey;
    }

    try {
        const response = await fetch('/api/config/telemetry/endpoints', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(endpoint)
        });

        if (response.ok) {
            status.textContent = '✓ Collector added successfully';
            status.className = 'save-status-inline success';

            // Clear form
            document.getElementById('collectorName').value = '';
            document.getElementById('collectorURL').value = '';
            document.getElementById('collectorAPIKey').value = '';
            document.getElementById('collectorEnabled').checked = true;

            // Reload collectors list
            await loadCollectors();

            showNotification('Collector added successfully', 'success');
        } else {
            const error = await response.json();
            status.textContent = '✗ Failed: ' + (error.error || 'Unknown error');
            status.className = 'save-status-inline error';
            showNotification('Failed to add collector', 'error');
        }
    } catch (error) {
        status.textContent = '✗ Error: ' + error.message;
        status.className = 'save-status-inline error';
        showNotification('Error adding collector', 'error');
    }

    // Clear status after 3 seconds
    setTimeout(() => {
        status.textContent = '';
        status.className = 'save-status-inline';
    }, 3000);
}

async function toggleCollector(name, enabled) {
    try {
        const response = await fetch(`/api/config/telemetry/endpoints/${encodeURIComponent(name)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ enabled })
        });

        if (response.ok) {
            await loadCollectors();
            showNotification(`Collector ${enabled ? 'enabled' : 'disabled'} successfully`, 'success');
        } else {
            const error = await response.json();
            showNotification('Failed to update collector: ' + (error.error || 'Unknown error'), 'error');
        }
    } catch (error) {
        showNotification('Error updating collector', 'error');
    }
}

async function deleteCollector(name) {
    if (!confirm(`Are you sure you want to delete the collector "${name}"?`)) {
        return;
    }

    try {
        const response = await fetch(`/api/config/telemetry/endpoints/${encodeURIComponent(name)}`, {
            method: 'DELETE'
        });

        if (response.ok) {
            await loadCollectors();
            showNotification('Collector deleted successfully', 'success');
        } else {
            const error = await response.json();
            showNotification('Failed to delete collector: ' + (error.error || 'Unknown error'), 'error');
        }
    } catch (error) {
        showNotification('Error deleting collector', 'error');
    }
}

async function resetCircuitBreaker(name) {
    try {
        const response = await fetch(`/api/telemetry/reset-circuit-breaker/${encodeURIComponent(name)}`, {
            method: 'POST'
        });

        if (response.ok) {
            await loadCollectors();
            showNotification('Circuit breaker reset successfully - endpoint will retry on next submission', 'success');
        } else {
            const error = await response.json();
            showNotification('Failed to reset circuit breaker: ' + (error.error || 'Unknown error'), 'error');
        }
    } catch (error) {
        showNotification('Error resetting circuit breaker', 'error');
    }
}

// Container History Functions

async function loadContainerHistory() {
    try {
        const hostFilter = document.getElementById('historyHostFilter')?.value || '';
        const url = hostFilter ? `/api/containers/lifecycle?limit=200&host_id=${hostFilter}` : '/api/containers/lifecycle?limit=200';

        const response = await fetch(url);
        lifecycles = await response.json() || [];

        // Update host filter dropdown if hosts are loaded
        updateHistoryHostFilter();

        // Render the history table
        renderContainerHistory(lifecycles);

        // Update stats
        updateHistoryStats(lifecycles);
    } catch (error) {
        console.error('Error loading container history:', error);
        document.getElementById('historyBody').innerHTML =
            '<div class="error">Failed to load container history</div>';
    }
}

function updateHistoryHostFilter() {
    const select = document.getElementById('historyHostFilter');
    if (!select || hosts.length === 0) return;

    const currentValue = select.value;
    select.innerHTML = '<option value="">All Hosts</option>' +
        hosts.map(host => `<option value="${host.id}">${escapeHtml(host.name)}</option>`).join('');
    select.value = currentValue;
}

function updateHistoryStats(lifecycles) {
    const total = lifecycles.length;
    const active = lifecycles.filter(l => l.is_active).length;
    const inactive = total - active;

    document.getElementById('historyTotalContainers').textContent = total;
    document.getElementById('historyActiveContainers').textContent = active;
    document.getElementById('historyInactiveContainers').textContent = inactive;
}

function renderContainerHistory(lifecycles) {
    const container = document.getElementById('historyBody');

    if (!lifecycles || lifecycles.length === 0) {
        container.innerHTML = '<div class="loading">No container history available</div>';
        return;
    }

    container.innerHTML = lifecycles.map(lifecycle => {
        const firstSeen = new Date(lifecycle.first_seen);
        const lastSeen = new Date(lifecycle.last_seen);
        const lifetime = formatDuration(lastSeen - firstSeen);

        const statusBadge = lifecycle.is_active
            ? '<span class="state-badge state-running">Active</span>'
            : '<span class="state-badge state-exited">Inactive</span>';

        // State changes includes the initial detection (first_seen) + actual state changes
        const stateChanges = 1 + (lifecycle.state_changes || 0);
        const imageUpdates = lifecycle.image_updates || 0;
        const restartEvents = lifecycle.restart_events || 0;

        return `
        <div class="history-card">
            <div class="history-card-header">
                <div class="history-card-title">
                    <strong>${escapeHtml(lifecycle.container_name)}</strong>
                    <span class="history-card-host">${escapeHtml(lifecycle.host_name)}</span>
                </div>
                <div class="history-card-actions">
                    ${statusBadge}
                    <button class="btn-icon btn-info" onclick="viewContainerTimeline(${lifecycle.host_id}, '${escapeAttr(lifecycle.container_id)}', '${escapeAttr(lifecycle.container_name)}')" title="View Timeline">📅</button>
                </div>
            </div>
            <div class="history-card-body">
                <div class="history-card-row">
                    <div class="history-card-label">Image:</div>
                    <div class="history-card-value"><code title="${escapeHtml(lifecycle.image)}">${escapeHtml(lifecycle.image)}</code></div>
                </div>
                <div class="history-card-row">
                    <div class="history-card-label">First Seen:</div>
                    <div class="history-card-value">${formatDateTime(lifecycle.first_seen)}</div>
                    <div class="history-card-label">Last Seen:</div>
                    <div class="history-card-value">${formatDateTime(lifecycle.last_seen)}</div>
                </div>
                <div class="history-card-row">
                    <div class="history-card-label">Lifetime:</div>
                    <div class="history-card-value">${lifetime}</div>
                    <div class="history-card-label">State Changes:</div>
                    <div class="history-card-value">${stateChanges}</div>
                    <div class="history-card-label">Image Updates:</div>
                    <div class="history-card-value">${imageUpdates}</div>
                </div>
            </div>
        </div>
        `;
    }).join('');
}

async function viewContainerTimeline(hostId, containerId, containerName) {
    document.getElementById('timelineContainerName').textContent = containerName;
    document.getElementById('timelineContent').innerHTML = '<div class="loading">Loading timeline...</div>';
    document.getElementById('timelineModal').classList.add('show');

    try {
        const response = await fetch(`/api/containers/lifecycle/${hostId}/${encodeURIComponent(containerName)}`);
        const events = await response.json();

        if (!events || events.length === 0) {
            document.getElementById('timelineContent').innerHTML = '<p>No lifecycle events found for this container.</p>';
            return;
        }

        renderTimeline(events);
    } catch (error) {
        console.error('Error loading timeline:', error);
        document.getElementById('timelineContent').innerHTML = '<p class="error">Failed to load timeline events</p>';
    }
}

function renderTimeline(events) {
    if (!events || events.length === 0) {
        document.getElementById('timelineContent').innerHTML = '<p>No lifecycle events found.</p>';
        return;
    }

    // Calculate summary statistics
    const firstEvent = events[0];
    const lastEvent = events[events.length - 1];
    // State changes includes first_seen + actual state transitions
    const actualStateChanges = events.filter(e => e.event_type === 'started' || e.event_type === 'stopped' || e.event_type === 'state_change').length;
    const stateChanges = 1 + actualStateChanges; // +1 for first_seen
    const imageUpdates = events.filter(e => e.event_type === 'image_updated').length;

    // Extract scan count from last_seen event if present
    let totalScans = 'N/A';
    if (lastEvent && lastEvent.event_type === 'last_seen') {
        const match = lastEvent.description.match(/seen (\d+) times/);
        if (match) {
            totalScans = parseInt(match[1]);
        }
    }

    // Calculate duration
    const firstTime = new Date(firstEvent.timestamp);
    const lastTime = new Date(lastEvent.timestamp);
    const durationMs = lastTime - firstTime;
    const durationDays = Math.floor(durationMs / (1000 * 60 * 60 * 24));
    const durationText = durationDays > 0 ? `${durationDays} days` : 'same day';

    // Determine status
    const isActive = lastEvent.new_state === 'running';
    const statusText = isActive ? 'Active (running)' : lastEvent.new_state === 'exited' ? 'Inactive (stopped)' : 'Unknown';
    const statusClass = isActive ? 'badge-success' : 'badge-warning';

    // Build summary banner
    const summaryHTML = `
        <div class="timeline-summary" style="background: #f8f9fa; border: 1px solid #dee2e6; border-radius: 8px; padding: 16px; margin-bottom: 20px;">
            <div style="display: flex; align-items: center; margin-bottom: 12px;">
                <span style="font-size: 24px; margin-right: 10px;">📊</span>
                <div>
                    <strong style="font-size: 16px;">Container History Summary</strong>
                    <div style="color: #666; font-size: 13px; margin-top: 4px;">
                        ${formatDate(firstEvent.timestamp)} to ${formatDate(lastEvent.timestamp)} (${durationText})
                    </div>
                </div>
            </div>
            <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px; font-size: 14px;">
                <div><strong>Total Observations:</strong> ${totalScans}</div>
                <div><strong>State Changes:</strong> ${stateChanges}</div>
                <div><strong>Image Updates:</strong> ${imageUpdates}</div>
                <div><strong>Current Status:</strong> <span class="badge ${statusClass}">${statusText}</span></div>
            </div>
        </div>
    `;

    const timelineHTML = events.map(event => {
        const eventIcon = getEventIcon(event.event_type);
        const eventClass = getEventClass(event.event_type);

        let details = '';
        if (event.old_state && event.new_state) {
            details = `<span class="state-badge state-${event.old_state}">${event.old_state}</span> → <span class="state-badge state-${event.new_state}">${event.new_state}</span>`;
        } else if (event.old_image_tag && event.new_image_tag) {
            // New format: show both tag and SHA
            details = `<code>${event.old_image_tag}</code> <span class="text-muted">(${event.old_image_sha})</span> → <code>${event.new_image_tag}</code> <span class="text-muted">(${event.new_image_sha})</span>`;
        } else if (event.old_image && event.new_image) {
            // Fallback to old format for backward compatibility
            details = `<code>${event.old_image}</code> → <code>${event.new_image}</code>`;
        } else if (event.restart_count) {
            details = `<strong>${event.restart_count} restart(s)</strong>`;
        }

        return `
        <div class="timeline-event ${eventClass}">
            <div class="timeline-marker">${eventIcon}</div>
            <div class="timeline-content-box">
                <div class="timeline-time">${formatDateTime(event.timestamp)}</div>
                <div class="timeline-description">
                    <strong>${event.description}</strong>
                    ${details ? `<div class="timeline-details">${details}</div>` : ''}
                </div>
            </div>
        </div>
        `;
    }).join('');

    document.getElementById('timelineContent').innerHTML = `${summaryHTML}<div class="timeline">${timelineHTML}</div>`;
}

function getEventIcon(eventType) {
    const icons = {
        'first_seen': '🎉',
        'started': '▶️',
        'stopped': '⏹️',
        'paused': '⏸️',
        'resumed': '▶️',
        'restarted': '⟳',
        'image_updated': '📦',
        'disappeared': '👻',
        'reappeared': '✨',
        'state_change': '🔄',
        'last_seen': '📍'
    };
    return icons[eventType] || '•';
}

function getEventClass(eventType) {
    const classes = {
        'first_seen': 'event-success',
        'started': 'event-success',
        'stopped': 'event-warning',
        'paused': 'event-info',
        'resumed': 'event-success',
        'restarted': 'event-warning',
        'image_updated': 'event-info',
        'disappeared': 'event-error',
        'reappeared': 'event-success',
        'state_change': 'event-info',
        'last_seen': 'event-info'
    };
    return classes[eventType] || 'event-default';
}

function formatDuration(ms) {
    const seconds = Math.floor(ms / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);

    if (days > 0) {
        return `${days}d ${hours % 24}h`;
    } else if (hours > 0) {
        return `${hours}h ${minutes % 60}m`;
    } else if (minutes > 0) {
        return `${minutes}m`;
    } else {
        return `${seconds}s`;
    }
}

function closeTimelineModal() {
    document.getElementById('timelineModal').classList.remove('show');
}

// Graph Visualization Functions

async function loadGraph() {
    const container = document.getElementById('graphContainer');
    container.innerHTML = '<div class="graph-loading">Loading graph...</div>';

    try {
        const response = await fetch('/api/containers/graph');
        graphData = await response.json();
        renderGraph(graphData);
    } catch (error) {
        console.error('Error loading graph:', error);
        container.innerHTML = '<div class="graph-error">Failed to load container graph</div>';
    }
}

function renderGraph(data) {
    const container = document.getElementById('graphContainer');

    if (!data.nodes || data.nodes.length === 0) {
        container.innerHTML = '<div class="graph-empty">No containers to display</div>';
        return;
    }

    // Clear loading message
    container.innerHTML = '';

    // Build lists for dropdowns
    buildGraphDropdowns(data);

    // Count edge types
    updateEdgeCounts(data.edges);

    // Assign colors to compose projects
    const composeProjects = [...new Set(data.nodes.map(n => n.compose_project).filter(p => p))];
    const projectColors = {};
    const colors = ['#3498db', '#9b59b6', '#e67e22', '#1abc9c', '#e74c3c', '#f39c12', '#2ecc71', '#34495e'];
    composeProjects.forEach((project, i) => {
        projectColors[project] = colors[i % colors.length];
    });

    // Build Cytoscape elements
    const elements = {
        nodes: data.nodes.map(node => ({
            data: {
                id: node.id,
                label: node.name,
                nodeType: node.node_type || 'container',
                state: node.state,
                image: node.image,
                host: node.host_name,
                composeProject: node.compose_project || '',
                projectColor: projectColors[node.compose_project] || null
            }
        })),
        edges: data.edges.map(edge => ({
            data: {
                id: `${edge.source}-${edge.target}-${edge.type}`,
                source: edge.source,
                target: edge.target,
                label: edge.label,
                type: edge.type
            }
        }))
    };

    // Initialize Cytoscape
    cy = cytoscape({
        container: container,
        elements: elements,
        style: [
            // Node styles
            {
                selector: 'node',
                style: {
                    'label': 'data(label)',
                    'text-valign': 'center',
                    'text-halign': 'center',
                    'background-color': '#95a5a6',
                    'color': '#fff',
                    'text-outline-color': '#2c3e50',
                    'text-outline-width': 2,
                    'font-size': '12px',
                    'width': 50,
                    'height': 50,
                    'shape': 'ellipse'
                }
            },
            // Network nodes - different shape and color
            {
                selector: 'node[nodeType="network"]',
                style: {
                    'shape': 'diamond',
                    'background-color': '#3498db',
                    'border-width': 3,
                    'border-color': '#2980b9',
                    'width': 60,
                    'height': 60,
                    'font-size': '11px',
                    'font-weight': 'bold'
                }
            },
            {
                selector: 'node[state="running"]',
                style: {
                    'background-color': '#2ecc71',
                    'border-width': 3,
                    'border-color': '#27ae60'
                }
            },
            {
                selector: 'node[state="exited"]',
                style: {
                    'background-color': '#95a5a6',
                    'border-width': 3,
                    'border-color': '#7f8c8d'
                }
            },
            {
                selector: 'node[state="paused"]',
                style: {
                    'background-color': '#f39c12',
                    'border-width': 3,
                    'border-color': '#e67e22'
                }
            },
            {
                selector: 'node.project-colored',
                style: {
                    'background-color': 'data(projectColor)',
                    'border-color': 'data(projectColor)',
                    'border-width': 4
                }
            },
            {
                selector: 'node.dimmed',
                style: {
                    'opacity': 0.2
                }
            },
            {
                selector: 'node.highlighted',
                style: {
                    'border-width': 6,
                    'border-color': '#f1c40f',
                    'z-index': 999
                }
            },
            {
                selector: 'node:selected',
                style: {
                    'border-width': 5,
                    'border-color': '#3498db'
                }
            },
            // Edge styles
            {
                selector: 'edge',
                style: {
                    'curve-style': 'bezier',
                    'target-arrow-shape': 'none',
                    'line-color': '#bdc3c7',
                    'width': 2,
                    'label': 'data(label)',
                    'font-size': '10px',
                    'text-rotation': 'autorotate',
                    'text-margin-y': -10,
                    'color': '#34495e',
                    'text-background-color': '#fff',
                    'text-background-opacity': 0.8,
                    'text-background-padding': '3px'
                }
            },
            {
                selector: 'edge[type="network"]',
                style: {
                    'line-color': '#3498db',
                    'width': 3
                }
            },
            {
                selector: 'edge[type="volume"]',
                style: {
                    'line-color': '#e74c3c',
                    'width': 3
                }
            },
            {
                selector: 'edge[type="depends"]',
                style: {
                    'line-color': '#16a085',
                    'width': 3,
                    'target-arrow-shape': 'triangle',
                    'target-arrow-color': '#16a085',
                    'curve-style': 'bezier'
                }
            },
            {
                selector: 'edge[type="link"]',
                style: {
                    'line-color': '#9b59b6',
                    'width': 2,
                    'target-arrow-shape': 'triangle'
                }
            },
            {
                selector: 'edge:selected',
                style: {
                    'width': 4,
                    'line-color': '#2c3e50'
                }
            },
            {
                selector: 'edge.dimmed',
                style: {
                    'opacity': 0.15
                }
            },
            {
                selector: 'edge.no-label',
                style: {
                    'label': ''
                }
            }
        ],
        layout: {
            name: 'cose',
            animate: true,
            animationDuration: 1000,
            idealEdgeLength: 100,
            nodeOverlap: 20,
            refresh: 20,
            fit: true,
            padding: 30,
            randomize: false,
            componentSpacing: 100,
            nodeRepulsion: 400000,
            edgeElasticity: 100,
            nestingFactor: 5,
            gravity: 80,
            numIter: 1000,
            initialTemp: 200,
            coolingFactor: 0.95,
            minTemp: 1.0
        },
        minZoom: 0.1,
        maxZoom: 5,
        wheelSensitivity: 0.1  // Slower, more controlled zoom with mouse wheel
    });

    // Add event handlers
    cy.on('tap', 'node', function(evt) {
        const node = evt.target;
        const data = node.data();

        if (data.nodeType === 'network') {
            // Show network node info
            const connectedContainers = cy.edges(`[source="${data.id}"], [target="${data.id}"]`).length;
            showGraphInfo(`
                <strong>🌐 Network: ${data.label}</strong><br>
                Host: ${data.host}<br>
                Connected Containers: ${connectedContainers}
            `);
        } else {
            // Show container node info
            showGraphInfo(`
                <strong>${data.label}</strong><br>
                Host: ${data.host}<br>
                Image: ${data.image}<br>
                State: <span class="state-badge state-${data.state}">${data.state}</span><br>
                ${data.composeProject ? `Compose Project: ${data.composeProject}<br>` : ''}
            `);
        }
    });

    cy.on('tap', 'edge', function(evt) {
        const edge = evt.target;
        const data = edge.data();
        const sourceNode = cy.getElementById(data.source).data();
        const targetNode = cy.getElementById(data.target).data();

        let typeDescription = '';
        switch(data.type) {
            case 'network': typeDescription = 'Network Connection'; break;
            case 'volume': typeDescription = 'Shared Volume'; break;
            case 'depends': typeDescription = 'Dependency'; break;
            case 'link': typeDescription = 'Container Link'; break;
            default: typeDescription = 'Connection';
        }

        showGraphInfo(`
            <strong>${typeDescription}</strong><br>
            From: ${sourceNode.label}<br>
            To: ${targetNode.label}<br>
            ${data.label}
        `);
    });

    cy.on('tap', function(evt) {
        if (evt.target === cy) {
            showGraphInfo('<p>Click on containers or connections to see details</p>');
        }
    });

    // Apply initial filters
    applyGraphFilters();
}

function applyGraphFilters() {
    if (!cy) return;

    const showNetworks = document.getElementById('showNetworks').checked;
    const showVolumes = document.getElementById('showVolumes').checked;
    const showDepends = document.getElementById('showDepends').checked;
    const showLinks = document.getElementById('showLinks').checked;
    const colorByProject = document.getElementById('colorByProject').checked;

    // Apply color-by-project styling
    if (colorByProject) {
        cy.nodes().forEach(node => {
            if (node.data('projectColor')) {
                node.addClass('project-colored');
            }
        });
    } else {
        cy.nodes().removeClass('project-colored');
    }

    // Show/hide edges based on filters
    cy.edges().forEach(edge => {
        const type = edge.data('type');

        // Check if edge should be visible based on type filters
        let visibleByType = true;
        if (type === 'network' && !showNetworks) visibleByType = false;
        if (type === 'volume' && !showVolumes) visibleByType = false;
        if (type === 'depends' && !showDepends) visibleByType = false;
        if (type === 'link' && !showLinks) visibleByType = false;

        // Check if edge is dimmed by project/network selector
        const isDimmed = edge.hasClass('dimmed');

        // Show edge if it passes type filter and is not dimmed
        // OR if we're re-enabling a type (even if dimmed, show it)
        if (visibleByType && !isDimmed) {
            edge.show();
        } else if (!visibleByType) {
            edge.hide();
        } else if (visibleByType && isDimmed) {
            // Show but keep dimmed
            edge.show();
        }
    });
}

function showGraphInfo(html) {
    const infoDiv = document.getElementById('graphInfo');
    infoDiv.innerHTML = html;
}

// Graph zoom control functions
function zoomIn() {
    if (!cy) return;
    const currentZoom = cy.zoom();
    const newZoom = currentZoom * 1.2; // 20% increase
    cy.zoom({
        level: newZoom,
        renderedPosition: {
            x: cy.width() / 2,
            y: cy.height() / 2
        }
    });
}

function zoomOut() {
    if (!cy) return;
    const currentZoom = cy.zoom();
    const newZoom = currentZoom * 0.8; // 20% decrease
    cy.zoom({
        level: newZoom,
        renderedPosition: {
            x: cy.width() / 2,
            y: cy.height() / 2
        }
    });
}

function zoomReset() {
    if (!cy) return;
    cy.zoom(1);
    cy.center();
}

function fitGraph() {
    if (!cy) return;
    cy.fit(null, 30); // Fit all elements with 30px padding
}

// Helper functions for graph enhancements

function buildGraphDropdowns(data) {
    // Build compose project dropdown
    const composeProjects = [...new Set(data.nodes.map(n => n.compose_project).filter(p => p))].sort();
    const composeSelect = document.getElementById('composeProjectSelect');
    composeSelect.innerHTML = '<option value="">Compose: All Projects</option>';
    composeProjects.forEach(project => {
        const optGroup1 = document.createElement('optgroup');
        optGroup1.label = project;
        optGroup1.innerHTML = `
            <option value="highlight:${project}">Highlight: ${project}</option>
            <option value="isolate:${project}">Isolate: ${project}</option>
        `;
        composeSelect.appendChild(optGroup1);
    });

    // Build network dropdown - get network names from network nodes
    const networks = [...new Set(data.nodes.filter(n => n.node_type === 'network').map(n => n.name))].sort();
    const networkSelect = document.getElementById('networkSelect');
    networkSelect.innerHTML = '<option value="">Networks: Show All</option>';
    networks.forEach(network => {
        const optGroup = document.createElement('optgroup');
        optGroup.label = network;
        optGroup.innerHTML = `
            <option value="highlight:${network}">Highlight: ${network}</option>
            <option value="isolate:${network}">Isolate: ${network}</option>
        `;
        networkSelect.appendChild(optGroup);
    });
}

function updateEdgeCounts(edges) {
    const counts = {
        network: 0,
        volume: 0,
        depends: 0,
        link: 0
    };

    edges.forEach(edge => {
        if (counts.hasOwnProperty(edge.type)) {
            counts[edge.type]++;
        }
    });

    document.getElementById('networkCount').textContent = `(${counts.network})`;
    document.getElementById('volumeCount').textContent = `(${counts.volume})`;
    document.getElementById('dependsCount').textContent = `(${counts.depends})`;
    document.getElementById('linksCount').textContent = `(${counts.link})`;
}

function handleComposeProjectChange(event) {
    if (!cy) return;

    const value = event.target.value;

    // Reset all nodes and edges
    cy.nodes().removeClass('dimmed highlighted').show();
    cy.edges().removeClass('dimmed').show();

    if (!value) {
        applyGraphFilters();
        return;
    }

    const [mode, project] = value.split(':');

    if (mode === 'highlight') {
        // Dim non-matching nodes
        cy.nodes().forEach(node => {
            if (node.data('composeProject') !== project) {
                node.addClass('dimmed');
            }
        });
        // Dim edges not connected to this project
        cy.edges().forEach(edge => {
            const source = cy.getElementById(edge.data('source'));
            const target = cy.getElementById(edge.data('target'));
            if (source.data('composeProject') !== project && target.data('composeProject') !== project) {
                edge.addClass('dimmed');
            }
        });
    } else if (mode === 'isolate') {
        // Hide non-matching nodes
        cy.nodes().forEach(node => {
            if (node.data('composeProject') !== project) {
                node.hide();
            }
        });
        // Hide edges where both ends are not in project
        cy.edges().forEach(edge => {
            const source = cy.getElementById(edge.data('source'));
            const target = cy.getElementById(edge.data('target'));
            if (source.data('composeProject') !== project || target.data('composeProject') !== project) {
                edge.hide();
            }
        });
        // Fit to show isolated project
        setTimeout(() => cy.fit(null, 30), 100);
    }

    applyGraphFilters();
}

function handleNetworkChange(event) {
    if (!cy) return;

    const value = event.target.value;

    // Reset all
    cy.nodes().removeClass('dimmed highlighted').show();
    cy.edges().removeClass('dimmed').show();

    if (!value) {
        applyGraphFilters();
        return;
    }

    const [mode, network] = value.split(':');

    // Find the network node with this name
    let networkNodeId = null;
    cy.nodes().forEach(node => {
        if (node.data('nodeType') === 'network' && node.data('label') === network) {
            networkNodeId = node.id();
        }
    });

    if (!networkNodeId) {
        applyGraphFilters();
        return;
    }

    // Find all container nodes connected to this network node
    const connectedContainerIds = new Set();
    cy.edges().forEach(edge => {
        if (edge.data('type') === 'network' &&
            (edge.data('source') === networkNodeId || edge.data('target') === networkNodeId)) {
            // Add the other end (the container)
            const containerId = edge.data('source') === networkNodeId ?
                edge.data('target') : edge.data('source');
            connectedContainerIds.add(containerId);
        }
    });

    // Also add the network node itself to the set of nodes to keep visible
    connectedContainerIds.add(networkNodeId);

    if (mode === 'highlight') {
        // Dim nodes not connected to this network
        cy.nodes().forEach(node => {
            if (!connectedContainerIds.has(node.id())) {
                node.addClass('dimmed');
            }
        });
        // Dim edges not connected to this network node
        cy.edges().forEach(edge => {
            if (!(edge.data('source') === networkNodeId || edge.data('target') === networkNodeId)) {
                edge.addClass('dimmed');
            }
        });
    } else if (mode === 'isolate') {
        // Hide nodes not connected to this network
        cy.nodes().forEach(node => {
            if (!connectedContainerIds.has(node.id())) {
                node.hide();
            }
        });
        // Hide edges not connected to this network node
        cy.edges().forEach(edge => {
            if (!(edge.data('source') === networkNodeId || edge.data('target') === networkNodeId)) {
                edge.hide();
            }
        });
        setTimeout(() => cy.fit(null, 30), 100);
    }

    applyGraphFilters();
}

function handleLayoutChange(event) {
    if (!cy) return;

    const layoutName = event.target.value;
    let layoutOptions = { name: layoutName, animate: true, animationDuration: 500 };

    // Customize options for different layouts
    if (layoutName === 'dagre') {
        layoutOptions.rankDir = 'TB'; // Top to bottom
        layoutOptions.nodeSep = 50;
        layoutOptions.rankSep = 100;
    } else if (layoutName === 'cose') {
        layoutOptions.idealEdgeLength = 100;
        layoutOptions.nodeOverlap = 20;
        layoutOptions.refresh = 20;
        layoutOptions.fit = true;
        layoutOptions.padding = 30;
        layoutOptions.randomize = false;
        layoutOptions.componentSpacing = 100;
        layoutOptions.nodeRepulsion = 400000;
        layoutOptions.edgeElasticity = 100;
        layoutOptions.nestingFactor = 5;
        layoutOptions.gravity = 80;
        layoutOptions.numIter = 1000;
        layoutOptions.initialTemp = 200;
        layoutOptions.coolingFactor = 0.95;
        layoutOptions.minTemp = 1.0;
    } else if (layoutName === 'circle') {
        layoutOptions.radius = 250;
    } else if (layoutName === 'grid') {
        layoutOptions.rows = Math.ceil(Math.sqrt(cy.nodes().length));
    } else if (layoutName === 'concentric') {
        layoutOptions.concentric = node => node.degree();
        layoutOptions.levelWidth = () => 2;
    }

    const layout = cy.layout(layoutOptions);
    layout.run();
}

function handleGraphSearch(event) {
    if (!cy) return;

    const searchTerm = event.target.value.toLowerCase().trim();

    // Reset all highlighting
    cy.nodes().removeClass('highlighted');

    if (!searchTerm) {
        return;
    }

    // Find matching nodes
    const matchingNodes = cy.nodes().filter(node => {
        return node.data('label').toLowerCase().includes(searchTerm);
    });

    if (matchingNodes.length > 0) {
        // Highlight matches
        matchingNodes.addClass('highlighted');

        // Center on first match
        const firstMatch = matchingNodes[0];
        cy.animate({
            center: { eles: firstMatch },
            zoom: Math.max(cy.zoom(), 1.5)
        }, {
            duration: 500
        });

        // Update info
        if (matchingNodes.length === 1) {
            showGraphInfo(`Found: <strong>${firstMatch.data('label')}</strong>`);
        } else {
            showGraphInfo(`Found ${matchingNodes.length} containers matching "${searchTerm}"`);
        }
    } else {
        showGraphInfo(`No containers found matching "${searchTerm}"`);
    }
}

function toggleEdgeLabels() {
    if (!cy) return;

    const hideLabels = document.getElementById('hideEdgeLabels').checked;

    if (hideLabels) {
        cy.edges().addClass('no-label');
    } else {
        cy.edges().removeClass('no-label');
    }
}
