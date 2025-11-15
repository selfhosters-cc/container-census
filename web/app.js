// State
let containers = [];
let hosts = [];
let activities = [];
let images = {};
let graphData = null;
let cy = null; // Cytoscape instance
let autoRefreshInterval = null;
let currentTab = 'dashboard';
let lifecycles = [];
let lastRefreshTime = null;
let lastRefreshInterval = null;
let statsModalRefreshInterval = null;
let isSidebarOpen = false;
let vulnerabilityCache = {}; // Cache vulnerability data by imageID
let vulnerabilityScansMap = {}; // Pre-loaded map of all scans to avoid 404s
let vulnerabilitySummary = null; // Cache overall summary

// Auth credentials (empty if auth is disabled)
let authUsername = '';
let authPassword = '';

// Helper function for authenticated fetch requests
async function fetchWithAuth(url, options = {}) {
    const headers = {
        ...options.headers,
        'Authorization': 'Basic ' + btoa(authUsername + ':' + authPassword)
    };

    return fetch(url, {
        ...options,
        headers
    });
}

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    setupEventListeners();
    initializeRouting();
    loadVersion();
    loadTelemetrySchedule();
    loadData();
    startAutoRefresh();
    updateLastRefreshIndicator();

    // Initialize notifications if function exists
    if (typeof initNotifications === 'function') {
        initNotifications();
    }

    // Setup help menu
    setupHelpMenu();

    // Check if onboarding tour should be shown
    checkAndShowOnboarding();
});

// URL Hash Routing
function initializeRouting() {
    // Load tab from URL hash on page load
    const hash = window.location.hash.slice(1); // Remove #
    if (hash && hash.startsWith('/')) {
        const tab = hash.slice(1); // Remove leading /
        const validTabs = ['dashboard', 'containers', 'monitoring', 'images', 'security', 'graph', 'hosts', 'history', 'activity', 'reports', 'notifications', 'settings'];
        if (validTabs.includes(tab)) {
            currentTab = tab;
            switchTab(tab, false); // Don't push to history on initial load
        } else {
            // Invalid hash, default to dashboard
            currentTab = 'dashboard';
            switchTab('dashboard', true);
        }
    } else {
        // No hash or invalid format, default to dashboard
        currentTab = 'dashboard';
        switchTab('dashboard', true);
    }

    // Listen for hash changes (back/forward buttons)
    window.addEventListener('hashchange', () => {
        const hash = window.location.hash.slice(1);
        if (hash && hash.startsWith('/')) {
            const tab = hash.slice(1);
            const validTabs = ['dashboard', 'containers', 'monitoring', 'images', 'security', 'graph', 'hosts', 'history', 'activity', 'reports', 'notifications', 'settings'];
            if (validTabs.includes(tab)) {
                currentTab = tab;
                switchTab(tab, false); // Don't push to history on hash change
            }
        }
    });
}

// Update URL hash without reloading
function updateURL(tab) {
    window.location.hash = '#/' + tab;
}

// Last refresh indicator
function updateLastRefreshIndicator() {
    const indicator = document.getElementById('lastUpdated');
    if (!indicator) return;

    if (lastRefreshInterval) {
        clearInterval(lastRefreshInterval);
    }

    function update() {
        if (lastRefreshTime) {
            const now = Date.now();
            const diff = Math.floor((now - lastRefreshTime) / 1000);

            if (diff < 60) {
                indicator.textContent = `Updated ${diff}s ago`;
            } else {
                const mins = Math.floor(diff / 60);
                indicator.textContent = `Updated ${mins}m ago`;
            }
        } else {
            indicator.textContent = 'Loading...';
        }
    }

    update();
    lastRefreshInterval = setInterval(update, 1000);
}

// Set last refresh time
function markRefresh() {
    lastRefreshTime = Date.now();
    const indicator = document.getElementById('lastUpdated');
    if (indicator) {
        indicator.classList.add('refreshing');
        setTimeout(() => indicator.classList.remove('refreshing'), 1000);
    }
}

// Toast notification system
function showToast(title, message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;

    const icons = {
        success: '✅',
        error: '❌',
        info: 'ℹ️',
        warning: '⚠️'
    };

    toast.innerHTML = `
        <span class="toast-icon">${icons[type] || icons.info}</span>
        <div class="toast-content">
            <div class="toast-title">${title}</div>
            <div class="toast-message">${message}</div>
        </div>
        <button class="toast-close" onclick="this.parentElement.remove()">×</button>
    `;

    document.body.appendChild(toast);

    // Trigger animation
    setTimeout(() => toast.classList.add('show'), 10);

    // Auto-remove after 5 seconds
    setTimeout(() => {
        toast.classList.remove('show');
        setTimeout(() => toast.remove(), 300);
    }, 5000);
}

// Update navigation badges
function updateNavigationBadges() {
    // Containers badge
    const containersBadge = document.getElementById('containersBadge');
    if (containersBadge && containers.length > 0) {
        containersBadge.textContent = containers.length;
    }

    // Running containers in monitoring badge
    const monitoringBadge = document.getElementById('monitoringBadge');
    const runningCount = containers.filter(c => c.state === 'running').length;
    if (monitoringBadge && runningCount > 0) {
        monitoringBadge.textContent = runningCount;
    }

    // Images badge
    const imagesBadge = document.getElementById('imagesBadge');
    if (imagesBadge && images) {
        const totalImages = Object.values(images).reduce((sum, imgs) => sum + imgs.length, 0);
        if (totalImages > 0) {
            imagesBadge.textContent = totalImages;
        }
    }

    // Hosts badge
    const hostsBadge = document.getElementById('hostsBadge');
    if (hostsBadge && hosts.length > 0) {
        hostsBadge.textContent = hosts.length;
    }

    // Activity badge
    const activityBadge = document.getElementById('activityBadge');
    if (activityBadge && activities.length > 0) {
        activityBadge.textContent = activities.length;
    }
}

// Sidebar toggle for mobile
function toggleSidebar() {
    const sidebar = document.querySelector('.sidebar');
    const body = document.body;
    isSidebarOpen = !isSidebarOpen;

    if (isSidebarOpen) {
        sidebar.classList.add('open');
        body.classList.add('sidebar-open');
    } else {
        sidebar.classList.remove('open');
        body.classList.remove('sidebar-open');
    }
}

// Filter state persistence
function saveFilterState() {
    const state = {
        search: document.getElementById('searchInput')?.value || '',
        hostFilter: document.getElementById('hostFilter')?.value || '',
        stateFilter: document.getElementById('stateFilter')?.value || ''
    };
    sessionStorage.setItem(`filters_${currentTab}`, JSON.stringify(state));
}

function restoreFilterState() {
    const stateStr = sessionStorage.getItem(`filters_${currentTab}`);

    const hostFilter = document.getElementById('hostFilter');
    const stateFilter = document.getElementById('stateFilter');

    if (!stateStr) {
        // Clear filters when switching tabs if no saved state
        // Note: searchInput is already cleared in switchTab()
        if (hostFilter) hostFilter.value = '';
        if (stateFilter) stateFilter.value = '';
        return;
    }

    try {
        const state = JSON.parse(stateStr);

        // Note: searchInput is always cleared in switchTab(), so we don't restore it
        if (hostFilter) hostFilter.value = state.hostFilter || '';
        if (stateFilter) stateFilter.value = state.stateFilter || '';

        // Don't apply filters here - let the tab's load function handle it
        // This prevents double-rendering when switching tabs
    } catch (e) {
        console.error('Error restoring filter state:', e);
    }
}

// Apply filters based on current tab
function applyCurrentFilters() {
    if (currentTab === 'containers') {
        filterContainers();
    } else if (currentTab === 'images') {
        filterImages();
    } else if (currentTab === 'monitoring') {
        filterMonitoring();
    } else if (currentTab === 'history') {
        filterHistory();
    }
}

// Keyboard shortcuts
function setupKeyboardShortcuts() {
    document.addEventListener('keydown', (e) => {
        // Ignore if user is typing in an input
        if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.isContentEditable) {
            // Allow '/' to focus search even when in input (if it's empty)
            if (e.key === '/' && e.target.value === '') {
                e.preventDefault();
                document.getElementById('searchInput')?.focus();
            }
            return;
        }

        // Tab switching with number keys
        if ((e.key >= '1' && e.key <= '9') || e.key === '0') {
            e.preventDefault();
            const tabs = ['dashboard', 'containers', 'monitoring', 'images', 'security', 'graph', 'hosts', 'history', 'activity', 'reports'];
            const tabIndex = e.key === '0' ? 9 : parseInt(e.key) - 1;
            if (tabs[tabIndex]) {
                switchTab(tabs[tabIndex]);
            }
        }

        // 'N' for notifications
        if (e.key === 'n' || e.key === 'N') {
            e.preventDefault();
            switchTab('notifications');
        }

        // '/' to focus search
        if (e.key === '/') {
            e.preventDefault();
            const searchInput = document.getElementById('searchInput');
            if (searchInput) {
                searchInput.focus();
                searchInput.select();
            }
        }

        // 'Escape' to close sidebar on mobile
        if (e.key === 'Escape' && isSidebarOpen) {
            toggleSidebar();
        }
    });
}

// Event Listeners
function setupEventListeners() {
    document.getElementById('scanBtn').addEventListener('click', triggerScan);
    document.getElementById('submitTelemetryBtn').addEventListener('click', submitTelemetry);
    document.getElementById('autoRefresh').addEventListener('change', handleAutoRefreshToggle);

    // Dashboard scan button
    const dashboardScanBtn = document.getElementById('dashboardScanBtn');
    if (dashboardScanBtn) {
        dashboardScanBtn.addEventListener('click', triggerScan);
    }

    const searchInput = document.getElementById('searchInput');
    const hostFilter = document.getElementById('hostFilter');
    const stateFilter = document.getElementById('stateFilter');

    if (searchInput) {
        searchInput.addEventListener('input', () => {
            applyCurrentFilters();
            saveFilterState();
        });
    }

    if (hostFilter) {
        hostFilter.addEventListener('change', () => {
            applyCurrentFilters();
            saveFilterState();
        });
    }

    if (stateFilter) {
        stateFilter.addEventListener('change', () => {
            applyCurrentFilters();
            saveFilterState();
        });
    }

    // Sidebar navigation
    document.querySelectorAll('.nav-item').forEach(btn => {
        btn.addEventListener('click', (e) => {
            const tab = e.currentTarget.dataset.tab;
            if (tab) switchTab(tab);
        });
    });

    // Mobile sidebar toggle
    const sidebarToggle = document.getElementById('sidebarToggle');
    if (sidebarToggle) {
        sidebarToggle.addEventListener('click', toggleSidebar);
    }

    // Mobile menu button (created via CSS ::before)
    document.body.addEventListener('click', (e) => {
        if (window.innerWidth <= 768) {
            const rect = { left: 15, top: 15, right: 60, bottom: 60 };
            if (e.clientX >= rect.left && e.clientX <= rect.right &&
                e.clientY >= rect.top && e.clientY <= rect.bottom) {
                toggleSidebar();
            }
        }
    });

    // Setup keyboard shortcuts
    setupKeyboardShortcuts();

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

    // Security tab handlers
    document.getElementById('scanAllImagesBtn')?.addEventListener('click', scanAllImages);
    document.getElementById('updateTrivyDBBtn')?.addEventListener('click', updateTrivyDB);
    document.getElementById('exportVulnerabilitiesBtn')?.addEventListener('click', exportVulnerabilities);
    document.getElementById('vulnerabilitySettingsBtn')?.addEventListener('click', openVulnerabilitySettingsModal);
    document.getElementById('securitySearchInput')?.addEventListener('input', filterSecurityScans);
    document.getElementById('securitySeverityFilter')?.addEventListener('change', filterSecurityScans);
    document.getElementById('securityStatusFilter')?.addEventListener('change', filterSecurityScans);

    // Vulnerability settings modal
    const vulnSettingsForm = document.getElementById('vulnerabilitySettingsForm');
    if (vulnSettingsForm) {
        vulnSettingsForm.addEventListener('submit', saveVulnerabilitySettings);
    }
    const vulnSettingsModal = document.getElementById('vulnerabilitySettingsModal');
    if (vulnSettingsModal) {
        vulnSettingsModal.addEventListener('click', (e) => {
            if (e.target.classList.contains('modal')) closeVulnerabilitySettingsModal();
        });
    }
}

// Tab Management
function switchTab(tab, updateHistory = true) {
    currentTab = tab;

    // Update URL hash
    if (updateHistory) {
        updateURL(tab);
    }

    // Update navigation items (new sidebar)
    document.querySelectorAll('.nav-item').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.tab === tab);
    });

    // Update tab content
    document.querySelectorAll('.tab-content').forEach(content => {
        content.classList.remove('active');
    });
    document.getElementById(`${tab}Tab`).classList.add('active');

    // Close mobile sidebar after selection
    if (window.innerWidth <= 768 && isSidebarOpen) {
        toggleSidebar();
    }

    // Show/hide and configure filters based on tab
    updateFiltersForTab(tab);

    // Clear search term when switching tabs
    const searchInput = document.getElementById('searchInput');
    if (searchInput) {
        searchInput.value = '';
    }

    // Restore filter state for this tab (but search is already cleared above)
    restoreFilterState();

    // Stop security tab polling when leaving
    if (tab !== 'security') {
        stopQueueStatusPolling();
    }

    // Auto-refresh data when switching to a tab
    if (tab === 'dashboard') {
        loadDashboard();
    } else if (tab === 'containers') {
        loadContainers();
    } else if (tab === 'monitoring') {
        loadMonitoringData();
    } else if (tab === 'images') {
        loadImages();
    } else if (tab === 'security') {
        loadSecurityTab();
    } else if (tab === 'hosts') {
        loadHosts().then(() => renderHosts(hosts));
    } else if (tab === 'graph') {
        loadGraph();
    } else if (tab === 'history') {
        loadContainerHistory();
    } else if (tab === 'activity') {
        loadActivityLog();
    } else if (tab === 'reports') {
        initializeReportsTab();
    } else if (tab === 'settings') {
        loadCollectors();
        loadScannerSettings();
        loadTelemetrySettings();
        loadImageUpdateSettings();
    }

    // Add pulse animation to nav item briefly
    const navItem = document.querySelector(`.nav-item[data-tab="${tab}"]`);
    if (navItem) {
        navItem.classList.add('pulse');
        setTimeout(() => navItem.classList.remove('pulse'), 2000);
    }
}

// Update filters visibility and configuration based on current tab
function updateFiltersForTab(tab) {
    const filtersBar = document.getElementById('filtersBar');
    const searchInput = document.getElementById('searchInput');
    const stateFilter = document.getElementById('stateFilter');

    // Tabs that support filtering
    const filterableTabs = ['containers', 'monitoring', 'images', 'history'];

    if (filterableTabs.includes(tab)) {
        filtersBar.style.display = 'flex';

        // Update placeholder based on tab
        if (tab === 'containers') {
            if (searchInput) searchInput.placeholder = 'Search containers...';
            if (stateFilter) stateFilter.style.display = 'block';
        } else if (tab === 'monitoring') {
            if (searchInput) searchInput.placeholder = 'Search running containers...';
            if (stateFilter) stateFilter.style.display = 'none';
        } else if (tab === 'images') {
            if (searchInput) searchInput.placeholder = 'Search images...';
            if (stateFilter) stateFilter.style.display = 'none';
        } else if (tab === 'history') {
            if (searchInput) searchInput.placeholder = 'Search container history...';
            if (stateFilter) stateFilter.style.display = 'none';
        }
    } else {
        // Hide filters for non-filterable tabs
        filtersBar.style.display = 'none';
    }
}

// Monitoring Tab
async function loadMonitoringData() {
    try {
        // Load both containers and hosts
        await Promise.all([loadContainers(), loadHosts()]);

        // Update stats and badges (since loadContainers doesn't do it on non-containers tab)
        updateStats();
        updateNavigationBadges();
        markRefresh();

        // Apply filters if any are active (this will call filterMonitoring and render)
        applyCurrentFilters();
    } catch (error) {
        console.error('Error loading monitoring data:', error);
        document.getElementById('monitoringGrid').innerHTML = '<div class="error">Failed to load monitoring data</div>';
    }
}

function renderMonitoringGrid(containersToRender) {
    const grid = document.getElementById('monitoringGrid');

    if (containersToRender.length === 0) {
        grid.innerHTML = '<div class="loading">No running containers found</div>';
        return;
    }

    grid.innerHTML = containersToRender.map((container, index) => {
        // Stats are available if memory_limit is set (since we removed omitempty, it's always present if stats were collected)
        const hasStats = container.memory_limit > 0;
        const cpuDisplay = hasStats ? container.cpu_percent.toFixed(1) + '%' : '-';
        const memoryMB = hasStats ? (container.memory_usage / 1024 / 1024).toFixed(0) : '-';
        const limitMB = hasStats ? (container.memory_limit / 1024 / 1024).toFixed(0) : '?';
        const memoryPercent = hasStats ? container.memory_percent.toFixed(1) + '%' : '-';

        const cardId = `monitoring-card-${index}`;
        const chartId = `monitoring-chart-${index}`;

        // Debug logging
        if (container.host_id !== 1) { // Log non-local containers
            console.log(`Container ${container.name} (host: ${container.host_name}, hostId: ${container.host_id}):`, {
                cpu_percent: container.cpu_percent,
                memory_usage: container.memory_usage,
                memory_limit: container.memory_limit,
                hasStats: hasStats,
                willRenderChart: hasStats
            });
        }

        return `
            <div class="monitoring-card" id="${cardId}">
                <div class="monitoring-card-header">
                    <div>
                        <div class="monitoring-card-title">${escapeHtml(container.name)}</div>
                        <div class="monitoring-card-host">📍 ${escapeHtml(container.host_name)}</div>
                        <div class="monitoring-card-host">🖼️ ${escapeHtml(container.image)}</div>
                    </div>
                </div>
                <div class="monitoring-card-stats">
                    <div class="monitoring-stat">
                        <div class="monitoring-stat-label">CPU Usage</div>
                        <div class="monitoring-stat-value">${cpuDisplay}</div>
                    </div>
                    <div class="monitoring-stat">
                        <div class="monitoring-stat-label">Memory</div>
                        <div class="monitoring-stat-value">${memoryMB} MB</div>
                        <div class="monitoring-stat-label" style="margin-top: 5px;">of ${limitMB} MB (${memoryPercent})</div>
                    </div>
                </div>
                ${hasStats ? `
                    <div class="monitoring-chart">
                        <canvas id="${chartId}"></canvas>
                        <div id="${chartId}-placeholder" style="display: none; text-align: center; color: #999; padding: 20px; font-size: 12px;">
                            Collecting data... Check back in a few minutes
                        </div>
                    </div>
                ` : ''}
                ${hasStats ? `
                    <button class="btn btn-primary stats-btn-${index}" data-index="${index}">
                        📊 View Detailed Stats
                    </button>
                ` : `
                    <button class="btn btn-secondary" disabled title="Stats collection not enabled or no data yet">
                        📊 No Stats Available
                    </button>
                `}
            </div>
        `;
    }).join('');

    // Add event listeners to stats buttons and render mini charts
    containersToRender.forEach((container, index) => {
        const hasStats = container.memory_limit > 0;
        if (hasStats) {
            const btn = document.querySelector(`.stats-btn-${index}`);
            if (btn) {
                btn.addEventListener('click', () => {
                    console.log('Opening stats modal for:', container.name, 'hostId:', container.host_id, 'containerId:', container.id);
                    openStatsModal(container.host_id, container.id, container.name);
                });
            }

            // Render mini sparkline chart
            renderMiniChart(`monitoring-chart-${index}`, container.host_id, container.id);
        }
    });
}

// Render mini sparkline chart for monitoring cards
async function renderMiniChart(canvasId, hostId, containerId) {
    try {
        // Fetch last hour of stats for sparkline
        const url = `/api/containers/${hostId}/${containerId}/stats?range=1h`;
        console.log(`Fetching stats from: ${url}`);
        const response = await fetch(url);
        if (!response.ok) {
            console.error(`Failed to fetch stats for ${canvasId}: ${response.status} ${response.statusText}`);
            return;
        }

        const stats = await response.json();
        console.log(`Stats for ${canvasId}:`, stats ? stats.length : 'null', 'data points');

        const canvas = document.getElementById(canvasId);
        const placeholder = document.getElementById(`${canvasId}-placeholder`);

        if (!canvas) return;

        if (!stats || stats.length === 0) {
            console.warn(`No stats data available for ${canvasId} - showing placeholder`);
            // Hide canvas and show placeholder message
            canvas.style.display = 'none';
            if (placeholder) {
                placeholder.style.display = 'block';
            }
            return;
        }

        // Hide placeholder if data exists
        if (placeholder) {
            placeholder.style.display = 'none';
        }
        canvas.style.display = 'block';

        // Destroy existing chart if it exists to avoid "Canvas is already in use" error
        const existingChart = Chart.getChart(canvasId);
        if (existingChart) {
            existingChart.destroy();
        }

        const ctx = canvas.getContext('2d');

        // Take last 20 points for sparkline
        const recentStats = stats.slice(-20);
        const cpuData = recentStats.map(s => s.cpu_percent || 0);
        const memoryData = recentStats.map(s => (s.memory_usage || 0) / 1024 / 1024);

        new Chart(ctx, {
            type: 'line',
            data: {
                labels: recentStats.map(() => ''),
                datasets: [
                    {
                        label: 'CPU %',
                        data: cpuData,
                        borderColor: 'rgb(75, 192, 192)',
                        backgroundColor: 'rgba(75, 192, 192, 0.1)',
                        borderWidth: 2,
                        pointRadius: 0,
                        tension: 0.4,
                        yAxisID: 'y'
                    },
                    {
                        label: 'Memory MB',
                        data: memoryData,
                        borderColor: 'rgb(255, 99, 132)',
                        backgroundColor: 'rgba(255, 99, 132, 0.1)',
                        borderWidth: 2,
                        pointRadius: 0,
                        tension: 0.4,
                        yAxisID: 'y1'
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                interaction: {
                    mode: 'index',
                    intersect: false
                },
                plugins: {
                    legend: {
                        display: true,
                        position: 'top',
                        labels: {
                            boxWidth: 12,
                            padding: 8,
                            font: {
                                size: 11
                            }
                        }
                    },
                    tooltip: {
                        enabled: true,
                        mode: 'index',
                        intersect: false,
                        callbacks: {
                            label: function(context) {
                                let label = context.dataset.label || '';
                                if (label) {
                                    label += ': ';
                                }
                                if (context.parsed.y !== null) {
                                    label += context.parsed.y.toFixed(2);
                                    if (context.dataset.yAxisID === 'y') {
                                        label += '%';
                                    } else {
                                        label += ' MB';
                                    }
                                }
                                return label;
                            }
                        }
                    }
                },
                scales: {
                    x: {
                        display: false
                    },
                    y: {
                        display: true,
                        beginAtZero: true,
                        position: 'left',
                        title: {
                            display: true,
                            text: 'CPU %',
                            font: {
                                size: 10
                            }
                        },
                        ticks: {
                            font: {
                                size: 9
                            }
                        }
                    },
                    y1: {
                        display: true,
                        beginAtZero: true,
                        position: 'right',
                        title: {
                            display: true,
                            text: 'Memory MB',
                            font: {
                                size: 10
                            }
                        },
                        ticks: {
                            font: {
                                size: 9
                            }
                        },
                        grid: {
                            drawOnChartArea: false
                        }
                    }
                }
            }
        });
    } catch (error) {
        console.error('Error rendering mini chart:', error);
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
        updateNavigationBadges();
        markRefresh();

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
        const data = await response.json();
        hosts = Array.isArray(data) ? data : [];
    } catch (error) {
        console.error('Error loading hosts:', error);
        hosts = [];
    }
}

async function loadContainers() {
    try {
        const response = await fetch('/api/containers');
        const data = await response.json();
        const allContainers = Array.isArray(data) ? data : [];

        // Filter to only show containers from enabled hosts
        const enabledHostIds = new Set((hosts || []).filter(h => h.enabled).map(h => h.id));
        containers = allContainers.filter(c => enabledHostIds.has(c.host_id));

        // Only render/filter if we're on the containers tab
        // Other tabs (like monitoring) will handle their own rendering
        if (currentTab === 'containers') {
            filterContainers();
            updateStats();
            updateNavigationBadges();
            markRefresh();
        }
    } catch (error) {
        console.error('Error loading containers:', error);
        containers = [];
        if (currentTab === 'containers') {
            document.getElementById('containersBody').innerHTML =
                '<tr><td colspan="8" class="error">Failed to load containers</td></tr>';
        }
    }
}

async function loadImages() {
    try {
        const response = await fetch('/api/images');
        images = await response.json() || {};

        // Apply filters if any are active
        applyCurrentFilters();

        updateNavigationBadges();
        markRefresh();
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
        const data = await response.json();
        activities = Array.isArray(data) ? data : [];
        renderActivityLog(activities);
        updateStats();
        updateNavigationBadges();
        markRefresh();
    } catch (error) {
        console.error('Error loading activity log:', error);
        activities = [];
        document.getElementById('activityLogBody').innerHTML =
            '<tr><td colspan="6" class="error">Failed to load activity log</td></tr>';
    }
}

async function triggerScan() {
    const btn = document.getElementById('scanBtn');
    const btnIcon = document.getElementById('scanBtnIcon');

    btn.disabled = true;
    btn.classList.add('scanning');
    if (btnIcon) btnIcon.classList.add('spinning');

    showToast('Scan Started', 'Scanning all configured hosts...', 'info');

    const resetButton = () => {
        btn.disabled = false;
        btn.classList.remove('scanning');
        if (btnIcon) btnIcon.classList.remove('spinning');
    };

    try {
        const startTime = Date.now();
        const response = await fetch('/api/scan', { method: 'POST' });

        if (response.ok) {
            // Wait 3 seconds then refresh data once and reset button
            setTimeout(async () => {
                await loadData();
                resetButton();
                const duration = ((Date.now() - startTime) / 1000).toFixed(1);
                showToast('Scan Complete', `Scan finished in ${duration}s`, 'success');
            }, 3000);
        } else {
            resetButton();
            throw new Error('Scan request failed');
        }
    } catch (error) {
        console.error('Error triggering scan:', error);
        resetButton();
        showToast('Scan Failed', 'Failed to trigger scan: ' + error.message, 'error');
    }
}

async function submitTelemetry() {
    const btn = document.getElementById('submitTelemetryBtn');
    btn.disabled = true;
    btn.classList.add('submitting');

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
        btn.classList.remove('submitting');
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
            // Trigger a scan to get updated state
            setTimeout(async () => {
                await fetch('/api/scan', { method: 'POST' });
                await loadData();
            }, 2000);
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
                    // Trigger a scan to get updated state
                    setTimeout(async () => {
                        await fetch('/api/scan', { method: 'POST' });
                        await loadData();
                    }, 2000);
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
                    // Trigger a scan to get updated state
                    setTimeout(async () => {
                        await fetch('/api/scan', { method: 'POST' });
                        await loadData();
                    }, 2000);
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

// Track current log view state for refresh
let currentLogView = {
    hostId: null,
    containerName: null
};

async function viewLogs(hostId, containerName, displayName) {
    // Store current view for refresh
    currentLogView.hostId = hostId;
    currentLogView.containerName = containerName;

    document.getElementById('logContainerName').textContent = displayName || containerName;
    document.getElementById('logContent').textContent = 'Loading logs...';
    document.getElementById('logModal').classList.add('show');

    try {
        // Use container name instead of ID for reliability after updates
        const response = await fetch(`/api/containers/${hostId}/${encodeURIComponent(containerName)}/logs?tail=500`);

        if (response.ok) {
            const data = await response.json();
            document.getElementById('logContent').textContent = data.logs || 'No logs available';
        } else {
            const error = await response.json();
            document.getElementById('logContent').textContent = `Error: ${error.error}`;
        }
    } catch (error) {
        console.error('Error loading logs:', error);
        document.getElementById('logContent').textContent = 'Failed to load logs: ' + error.message;
    }
}

// Refresh logs for currently viewed container
async function refreshLogs() {
    if (!currentLogView.hostId || !currentLogView.containerName) {
        showNotification('No logs currently loaded', 'warning');
        return;
    }

    const displayName = document.getElementById('logContainerName').textContent;
    document.getElementById('logContent').textContent = 'Refreshing logs...';

    try {
        const response = await fetch(`/api/containers/${currentLogView.hostId}/${encodeURIComponent(currentLogView.containerName)}/logs?tail=500`);

        if (response.ok) {
            const data = await response.json();
            document.getElementById('logContent').textContent = data.logs || 'No logs available';
            showNotification('Logs refreshed', 'success');
        } else {
            const error = await response.json();
            document.getElementById('logContent').textContent = `Error: ${error.error}`;
            showNotification('Failed to refresh logs', 'error');
        }
    } catch (error) {
        console.error('Error refreshing logs:', error);
        document.getElementById('logContent').textContent = 'Failed to refresh logs: ' + error.message;
        showNotification('Failed to refresh logs', 'error');
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
    const container = document.getElementById('containersBody');

    if (containersToRender.length === 0) {
        container.innerHTML = '<div class="loading">No containers found</div>';
        return;
    }

    container.innerHTML = containersToRender.map(cont => {
        const isRunning = cont.state === 'running';
        const isStopped = cont.state === 'exited';
        const isPaused = cont.state === 'paused';
        const hasStats = cont.cpu_percent > 0 || cont.memory_usage > 0;

        // Format CPU
        let cpuDisplay = '-';
        if (cont.cpu_percent > 0) {
            cpuDisplay = cont.cpu_percent.toFixed(1) + '%';
        }

        // Format Memory
        let memoryDisplay = '-';
        let memoryPercent = '';
        if (cont.memory_usage > 0) {
            const memoryMB = (cont.memory_usage / 1024 / 1024).toFixed(0);
            const limitMB = cont.memory_limit > 0 ? (cont.memory_limit / 1024 / 1024).toFixed(0) : '?';
            memoryDisplay = `${memoryMB} / ${limitMB} MB`;
            if (cont.memory_percent > 0) {
                memoryPercent = ` (${cont.memory_percent.toFixed(1)}%)`;
            }
        }

        // State icon
        const stateIcon = isRunning ? '✅' : isStopped ? '⏸️' : isPaused ? '⏸️' : '❓';
        const createdTime = formatDate(cont.created);
        const statusText = cont.status || '-';

        return `
        <div class="container-card-modern ${cont.state}">
            <div class="container-card-header-modern">
                <div class="container-card-left">
                    <div class="container-status-indicator ${cont.state}">
                        ${stateIcon}
                    </div>
                    <div class="container-card-info">
                        <div class="container-card-name">${escapeHtml(cont.name)}</div>
                        <div class="container-card-meta">
                            <span class="meta-item">📍 ${escapeHtml(cont.host_name)}</span>
                            <span class="meta-item">⏱️ ${createdTime}</span>
                            <span class="state-badge state-${cont.state}">${cont.state}</span>
                        </div>
                    </div>
                </div>
                <div class="container-card-actions">
                    ${hasStats && isRunning ? `
                        <button class="btn btn-sm btn-stats" onclick="openStatsModal(${cont.host_id}, '${escapeAttr(cont.id)}', '${escapeAttr(cont.name)}')" title="View Stats">
                            📊 Stats
                        </button>
                    ` : ''}
                    ${hasStats && isRunning ? `
                        <button class="btn btn-sm btn-timeline" onclick="viewContainerTimeline(${cont.host_id}, '${escapeAttr(cont.id)}', '${escapeAttr(cont.name)}')" title="View Timeline">
                            📅 Timeline
                        </button>
                    ` : ''}
                </div>
            </div>

            <div class="container-card-content">
                <div class="container-detail-row">
                    <span class="detail-label">🖼️ Image</span>
                    <code class="detail-value image-value" title="${escapeHtml(cont.image)}">${escapeHtml(cont.image)}</code>
                    ${cont.update_available ? '<span class="update-badge">⬆️ Update Available</span>' : ''}
                    ${(cont.image.endsWith(':latest') || !cont.image.includes(':')) && isRunning ? `
                        <button class="btn btn-xs btn-primary" onclick="checkContainerUpdate(${cont.host_id}, '${escapeAttr(cont.name)}', '${escapeAttr(cont.name)}')" title="Check for updates">
                            🔍 Check
                        </button>
                    ` : ''}
                    ${cont.update_available ? `
                        <button class="btn btn-xs btn-success" onclick="updateContainer(${cont.host_id}, '${escapeAttr(cont.name)}', '${escapeAttr(cont.name)}', '${escapeAttr(cont.image)}')" title="Update image">
                            ⬆️ Update
                        </button>
                    ` : ''}
                </div>

                ${statusText !== '-' ? `
                <div class="container-detail-row">
                    <span class="detail-label">📝 Status</span>
                    <span class="detail-value">${escapeHtml(statusText)}</span>
                </div>
                ` : ''}

                ${cont.ports && cont.ports.length > 0 && cont.ports.some(p => p.public_port > 0) ? `
                <div class="container-detail-row">
                    <span class="detail-label">🔌 Ports</span>
                    <span class="detail-value">${formatPorts(cont.ports)}</span>
                </div>
                ` : ''}

                <div class="container-metrics-grid">
                    ${hasStats ? `
                    <div class="metric-box">
                        <div class="metric-icon">💻</div>
                        <div class="metric-content">
                            <div class="metric-label">CPU Usage</div>
                            <div class="metric-value">${cpuDisplay}</div>
                        </div>
                    </div>

                    <div class="metric-box">
                        <div class="metric-icon">💾</div>
                        <div class="metric-content">
                            <div class="metric-label">Memory Usage</div>
                            <div class="metric-value">${memoryDisplay}${memoryPercent}</div>
                        </div>
                    </div>
                    ` : '<div class="metric-box"><div class="metric-content"><div class="metric-label">No resource metrics available</div></div></div>'}
                </div>

                <div class="container-actions-row">
                    ${isRunning ? `
                        <button class="btn btn-sm btn-warning" onclick="stopContainer(${cont.host_id}, '${escapeAttr(cont.id)}', '${escapeAttr(cont.name)}')">
                            ⏹ Stop
                        </button>
                        <button class="btn btn-sm btn-warning" onclick="restartContainer(${cont.host_id}, '${escapeAttr(cont.id)}', '${escapeAttr(cont.name)}')">
                            🔄 Restart
                        </button>
                    ` : ''}
                    ${isStopped ? `
                        <button class="btn btn-sm btn-success" onclick="startContainer(${cont.host_id}, '${escapeAttr(cont.id)}', '${escapeAttr(cont.name)}')">
                            ▶ Start
                        </button>
                        <button class="btn btn-sm btn-danger" onclick="removeContainer(${cont.host_id}, '${escapeAttr(cont.id)}', '${escapeAttr(cont.name)}')">
                            🗑 Remove
                        </button>
                    ` : ''}
                    <button class="btn btn-sm btn-primary" onclick="viewLogs(${cont.host_id}, '${escapeAttr(cont.name)}', '${escapeAttr(cont.name)}')">
                        📋 Logs
                    </button>
                </div>
            </div>
        </div>
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
        tbody.innerHTML = '<tr><td colspan="8" class="loading">No hosts configured</td></tr>';
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

        const statsCollectionBadge = host.collect_stats
            ? '<span class="badge badge-success" style="cursor: pointer;" onclick="toggleStatsCollection(' + host.id + ', false)" title="Click to disable stats collection">✓ Enabled</span>'
            : '<span class="badge badge-secondary" style="cursor: pointer;" onclick="toggleStatsCollection(' + host.id + ', true)" title="Click to enable stats collection">Disabled</span>';

        return `
        <tr>
            <td><strong>${escapeHtml(host.name)}</strong></td>
            <td>${typeIcon} ${escapeHtml(hostType)}</td>
            <td><code>${escapeHtml(host.address)}</code></td>
            <td>${statusBadge}</td>
            <td>${statsCollectionBadge}</td>
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

async function toggleStatsCollection(hostId, enable) {
    try {
        const host = hosts.find(h => h.id === hostId);
        if (!host) return;

        const response = await fetch(`/api/hosts/${hostId}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ ...host, collect_stats: enable })
        });

        if (response.ok) {
            showNotification(`Stats collection ${enable ? 'enabled' : 'disabled'} successfully`, 'success');
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
    const safeHosts = hosts || [];
    const safeContainers = containers || [];
    const safeActivities = activities || [];

    document.getElementById('totalHosts').textContent = safeHosts.length;
    document.getElementById('totalContainers').textContent = safeContainers.length;

    const running = safeContainers.filter(c => c.state === 'running').length;
    document.getElementById('runningContainers').textContent = running;

    // Find most recent scan activity
    const scanActivities = safeActivities.filter(a => a.type === 'scan');
    if (scanActivities.length > 0) {
        const lastScan = new Date(scanActivities[0].timestamp);
        document.getElementById('lastScan').textContent = formatTimeAgo(lastScan);
    } else {
        document.getElementById('lastScan').textContent = 'Never';
    }

    // Update vulnerability stats (if available)
    updateVulnerabilityStats();
}

// Update vulnerability statistics in sidebar
async function updateVulnerabilityStats() {
    const criticalElem = document.getElementById('criticalVulns');

    if (!criticalElem) return;

    // If we don't have summary yet, fetch it
    if (!vulnerabilitySummary) {
        try {
            vulnerabilitySummary = await loadVulnerabilitySummary();
        } catch (error) {
            console.error('Error loading vulnerability summary:', error);
            criticalElem.textContent = '-';
            return;
        }
    }

    if (vulnerabilitySummary && vulnerabilitySummary.summary) {
        const s = vulnerabilitySummary.summary;
        const critical = s.severity_counts?.critical || 0;

        criticalElem.textContent = critical;

        // Add visual indication for high counts
        if (critical > 0) {
            criticalElem.style.fontWeight = 'bold';
        } else {
            criticalElem.style.fontWeight = 'normal';
        }
    }
}

function updateHostFilter() {
    // Update both the main host filter and the monitoring tab host filter
    const selects = ['hostFilter', 'monitoringHostFilter'];

    selects.forEach(selectId => {
        const select = document.getElementById(selectId);
        if (select) {
            const currentValue = select.value;

            select.innerHTML = '<option value="">All Hosts</option>' +
                hosts.map(host => `<option value="${host.id}">${escapeHtml(host.name)}</option>`).join('');

            select.value = currentValue;
        }
    });
}

// Filtering
function filterContainers() {
    const searchTerm = document.getElementById('searchInput')?.value.toLowerCase() || '';
    const hostFilter = document.getElementById('hostFilter')?.value || '';
    const stateFilter = document.getElementById('stateFilter')?.value || '';

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

    // Load vulnerability badges asynchronously
    loadAllVulnerabilityBadges();
}

function filterImages() {
    const searchTerm = document.getElementById('searchInput')?.value.toLowerCase() || '';
    const hostFilter = document.getElementById('hostFilter')?.value || '';

    if (!images || Object.keys(images).length === 0) {
        return;
    }

    // Filter images data structure
    const filteredImages = {};
    for (const [hostName, hostData] of Object.entries(images)) {
        const hostId = hostData.host_id;
        const matchesHost = hostFilter === '' || hostId?.toString() === hostFilter;

        if (matchesHost) {
            const filteredHostImages = hostData.images.filter(img => {
                const matchesSearch = searchTerm === '' ||
                    (img.repository && img.repository.toLowerCase().includes(searchTerm)) ||
                    (img.tag && img.tag.toLowerCase().includes(searchTerm)) ||
                    (img.id && img.id.toLowerCase().includes(searchTerm));

                return matchesSearch;
            });

            if (filteredHostImages.length > 0) {
                filteredImages[hostName] = {
                    ...hostData,
                    images: filteredHostImages
                };
            }
        }
    }

    renderImages(filteredImages);
}

function filterMonitoring() {
    const searchTerm = document.getElementById('searchInput')?.value.toLowerCase() || '';
    const hostFilter = document.getElementById('hostFilter')?.value || '';

    // Get running containers from enabled hosts
    const enabledHostIds = new Set(hosts.filter(h => h.enabled).map(h => h.id));
    let runningContainers = containers.filter(c =>
        c.state === 'running' && enabledHostIds.has(c.host_id)
    );

    // Apply filters
    runningContainers = runningContainers.filter(container => {
        const matchesSearch = searchTerm === '' ||
            container.name.toLowerCase().includes(searchTerm) ||
            container.image.toLowerCase().includes(searchTerm) ||
            container.host_name.toLowerCase().includes(searchTerm);

        const matchesHost = hostFilter === '' || container.host_id.toString() === hostFilter;

        return matchesSearch && matchesHost;
    });

    renderMonitoringGrid(runningContainers);
}

function filterHistory() {
    const searchTerm = document.getElementById('searchInput')?.value.toLowerCase() || '';
    const hostFilter = document.getElementById('hostFilter')?.value || '';

    if (!lifecycles || lifecycles.length === 0) {
        return;
    }

    // Apply filters to lifecycles
    const filteredLifecycles = lifecycles.filter(lifecycle => {
        const matchesSearch = searchTerm === '' ||
            lifecycle.container_name.toLowerCase().includes(searchTerm) ||
            lifecycle.image.toLowerCase().includes(searchTerm) ||
            lifecycle.host_name.toLowerCase().includes(searchTerm);

        const matchesHost = hostFilter === '' || lifecycle.host_id.toString() === hostFilter;

        return matchesSearch && matchesHost;
    });

    renderContainerHistory(filteredLifecycles);
    updateHistoryStats(filteredLifecycles);
}

// Modal Functions
function closeLogModal() {
    document.getElementById('logModal').classList.remove('show');
    clearLogSearch();
}

// Log Search Functionality
let logSearchMatches = [];
let currentMatchIndex = -1;
let originalLogContent = '';

function searchLogs(direction) {
    const searchInput = document.getElementById('logSearchInput');
    const logContent = document.getElementById('logContent');
    const searchStatus = document.getElementById('logSearchStatus');
    const searchTerm = searchInput.value.trim();

    if (!searchTerm) {
        searchStatus.textContent = '';
        return;
    }

    // If this is a new search, find all matches
    if (originalLogContent === '') {
        originalLogContent = logContent.textContent;
    }

    // Find all matches (case-insensitive)
    const lines = originalLogContent.split('\n');
    logSearchMatches = [];

    lines.forEach((line, lineIndex) => {
        const lowerLine = line.toLowerCase();
        const lowerTerm = searchTerm.toLowerCase();
        let index = 0;

        while ((index = lowerLine.indexOf(lowerTerm, index)) !== -1) {
            logSearchMatches.push({ lineIndex, charIndex: index, length: searchTerm.length });
            index += searchTerm.length;
        }
    });

    if (logSearchMatches.length === 0) {
        searchStatus.textContent = 'No matches';
        logContent.innerHTML = escapeHtml(originalLogContent);
        return;
    }

    // Navigate through matches
    if (direction === 'next') {
        currentMatchIndex = (currentMatchIndex + 1) % logSearchMatches.length;
    } else if (direction === 'prev') {
        currentMatchIndex = currentMatchIndex <= 0 ? logSearchMatches.length - 1 : currentMatchIndex - 1;
    } else {
        currentMatchIndex = 0;
    }

    // Update status
    searchStatus.textContent = `${currentMatchIndex + 1} of ${logSearchMatches.length}`;

    // Highlight all matches and mark current one
    highlightMatches(lines, searchTerm);

    // Scroll to current match
    scrollToCurrentMatch();
}

function highlightMatches(lines, searchTerm) {
    const logContent = document.getElementById('logContent');
    const lowerTerm = searchTerm.toLowerCase();

    let html = '';
    let globalMatchIndex = 0;

    lines.forEach((line, lineIndex) => {
        let highlightedLine = '';
        let lastIndex = 0;
        const lowerLine = line.toLowerCase();

        let index = 0;
        while ((index = lowerLine.indexOf(lowerTerm, index)) !== -1) {
            // Add text before match
            highlightedLine += escapeHtml(line.substring(lastIndex, index));

            // Add highlighted match
            const isCurrent = globalMatchIndex === currentMatchIndex;
            const matchClass = isCurrent ? 'current-match' : '';
            const matchId = isCurrent ? ' id="current-log-match"' : '';
            highlightedLine += `<mark class="${matchClass}"${matchId}>${escapeHtml(line.substring(index, index + searchTerm.length))}</mark>`;

            lastIndex = index + searchTerm.length;
            index = lastIndex;
            globalMatchIndex++;
        }

        // Add remaining text
        highlightedLine += escapeHtml(line.substring(lastIndex));
        html += highlightedLine + '\n';
    });

    logContent.innerHTML = html;
}

function scrollToCurrentMatch() {
    const currentMatch = document.getElementById('current-log-match');
    if (currentMatch) {
        currentMatch.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
}

function clearLogSearch() {
    const searchInput = document.getElementById('logSearchInput');
    const logContent = document.getElementById('logContent');
    const searchStatus = document.getElementById('logSearchStatus');

    searchInput.value = '';
    searchStatus.textContent = '';

    if (originalLogContent) {
        logContent.textContent = originalLogContent;
    }

    logSearchMatches = [];
    currentMatchIndex = -1;
    originalLogContent = '';
}

// Add event listener for Enter key in search input
document.addEventListener('DOMContentLoaded', function() {
    const searchInput = document.getElementById('logSearchInput');
    if (searchInput) {
        searchInput.addEventListener('keydown', function(e) {
            if (e.key === 'Enter') {
                e.preventDefault();
                searchLogs(e.shiftKey ? 'prev' : 'next');
            } else if (e.key === 'Escape') {
                clearLogSearch();
            }
        });

        // Trigger new search when input changes
        searchInput.addEventListener('input', function() {
            originalLogContent = '';
            currentMatchIndex = -1;
            if (this.value.trim()) {
                searchLogs('next');
            } else {
                clearLogSearch();
            }
        });
    }
});

function showConfirmDialog(title, message, onConfirm, type = 'warning') {
    document.getElementById('confirmTitle').textContent = title;
    document.getElementById('confirmMessage').innerHTML = message;
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
        description: document.getElementById('agentDescription').value,
        collect_stats: document.getElementById('agentCollectStats').checked
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
        // Load from new database-first settings API
        const response = await fetch('/api/settings');
        const settings = await response.json();

        const intervalHours = settings.telemetry?.interval_hours || 168;
        const dropdown = document.getElementById('telemetryFrequency');
        if (dropdown) {
            dropdown.value = intervalHours.toString();
            console.log('Loaded telemetry frequency from database:', intervalHours, 'hours');
        }
    } catch (error) {
        console.error('Failed to load telemetry settings:', error);
    }
}

async function loadScannerSettings() {
    try {
        // Load from new database-first settings API
        const response = await fetch('/api/settings');
        const settings = await response.json();

        const intervalSeconds = settings.scanner?.interval_seconds || 300;
        const dropdown = document.getElementById('scanInterval');
        if (dropdown) {
            dropdown.value = intervalSeconds.toString();
            console.log('Loaded scanner interval from database:', intervalSeconds, 'seconds');
        }
    } catch (error) {
        console.error('Failed to load scanner settings:', error);
    }
}

async function saveScanInterval() {
    const status = document.getElementById('scanIntervalSaveStatus');
    const intervalSeconds = parseInt(document.getElementById('scanInterval').value);

    status.textContent = 'Saving...';
    status.className = 'save-status-inline saving';

    try {
        // Load current settings first
        const currentResponse = await fetch('/api/settings');
        const currentSettings = await currentResponse.json();

        // Update only the scanner interval, preserve other settings
        const updatedSettings = {
            scanner: {
                interval_seconds: intervalSeconds,
                timeout_seconds: currentSettings.scanner?.timeout_seconds || 30
            },
            telemetry: {
                interval_hours: currentSettings.telemetry?.interval_hours || 168
            },
            notification: currentSettings.notification || {
                rate_limit_max: 100,
                rate_limit_batch_interval: 600,
                threshold_duration: 120,
                cooldown_period: 300
            }
        };

        const response = await fetch('/api/settings', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(updatedSettings)
        });

        if (response.ok) {
            status.textContent = '✓ Saved & Reloaded';
            status.className = 'save-status-inline success';
            showNotification('Scan interval updated successfully (hot-reloaded)', 'success');
        } else {
            const error = await response.json();
            status.textContent = '✗ Failed';
            status.className = 'save-status-inline error';
            showNotification('Failed to update scan interval: ' + (error.error || 'Unknown error'), 'error');
        }
    } catch (error) {
        status.textContent = '✗ Error';
        status.className = 'save-status-inline error';
        console.error('Failed to save scan interval:', error);
    }

    setTimeout(() => {
        status.textContent = '';
        status.className = 'save-status-inline';
    }, 3000);
}

async function saveTelemetryFrequency() {
    const status = document.getElementById('frequencySaveStatus');
    const intervalHours = parseInt(document.getElementById('telemetryFrequency').value);

    status.textContent = 'Saving...';
    status.className = 'save-status-inline saving';

    try {
        // Load current settings first
        const currentResponse = await fetch('/api/settings');
        const currentSettings = await currentResponse.json();

        // Update only the telemetry interval, preserve other settings
        const updatedSettings = {
            scanner: {
                interval_seconds: currentSettings.scanner?.interval_seconds || 300,
                timeout_seconds: currentSettings.scanner?.timeout_seconds || 30
            },
            telemetry: {
                interval_hours: intervalHours
            },
            notification: currentSettings.notification || {
                rate_limit_max: 100,
                rate_limit_batch_interval: 600,
                threshold_duration: 120,
                cooldown_period: 300
            }
        };

        const response = await fetch('/api/settings', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(updatedSettings)
        });

        if (response.ok) {
            status.textContent = '✓ Saved & Reloaded';
            status.className = 'save-status-inline success';
            showNotification('Telemetry frequency updated successfully (hot-reloaded)', 'success');
        } else {
            const error = await response.json();
            status.textContent = '✗ Failed';
            status.className = 'save-status-inline error';
            showNotification('Failed to update telemetry frequency: ' + (error.error || 'Unknown error'), 'error');
        }
    } catch (error) {
        status.textContent = '✗ Error';
        status.className = 'save-status-inline error';
        console.error('Failed to save telemetry frequency:', error);
    }

    setTimeout(() => {
        status.textContent = '';
        status.className = 'save-status-inline';
    }, 3000);
}

// Initialize settings when switching to settings tab
document.addEventListener('DOMContentLoaded', () => {
    // Load settings immediately on page load
    loadScannerSettings();
    loadTelemetrySettings();

    // Load settings when settings tab is clicked
    const settingsTab = document.querySelector('[data-tab="settings"]');
    if (settingsTab) {
        settingsTab.addEventListener('click', () => {
            setTimeout(() => {
                loadScannerSettings();
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
        const response = await fetch('/api/telemetry/endpoints', {
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
        const response = await fetch(`/api/telemetry/endpoints/${encodeURIComponent(name)}`, {
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
        const response = await fetch(`/api/telemetry/endpoints/${encodeURIComponent(name)}`, {
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
        const response = await fetch('/api/containers/lifecycle?limit=200');
        lifecycles = await response.json() || [];

        // Apply filters if any are active
        applyCurrentFilters();
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
        <div class="history-card-modern ${lifecycle.is_active ? 'active' : 'inactive'}">
            <div class="history-card-header-modern">
                <div class="history-card-left">
                    <div class="history-status-indicator ${lifecycle.is_active ? 'active' : 'inactive'}">
                        ${lifecycle.is_active ? '✅' : '⏸️'}
                    </div>
                    <div class="history-card-info">
                        <div class="history-card-name">${escapeHtml(lifecycle.container_name)}</div>
                        <div class="history-card-meta">
                            <span class="meta-item">📍 ${escapeHtml(lifecycle.host_name)}</span>
                            <span class="meta-item">⏱️ ${lifetime}</span>
                        </div>
                    </div>
                </div>
                <button class="btn btn-primary btn-timeline" onclick="viewContainerTimeline(${lifecycle.host_id}, '${escapeAttr(lifecycle.container_id)}', '${escapeAttr(lifecycle.container_name)}')" title="View detailed timeline">
                    <span class="timeline-icon">📅</span>
                    <span class="timeline-text">View Timeline</span>
                </button>
            </div>

            <div class="history-card-content">
                <div class="history-detail-row">
                    <span class="detail-label">🖼️ Image</span>
                    <code class="detail-value image-value" title="${escapeHtml(lifecycle.image)}">${escapeHtml(lifecycle.image)}</code>
                </div>

                <div class="history-metrics-grid">
                    <div class="metric-box">
                        <div class="metric-icon">👁️</div>
                        <div class="metric-content">
                            <div class="metric-label">First Seen</div>
                            <div class="metric-value">${formatTimeAgo(firstSeen)}</div>
                            <div class="metric-subtext">${formatDateTime(lifecycle.first_seen)}</div>
                        </div>
                    </div>

                    <div class="metric-box">
                        <div class="metric-icon">🕐</div>
                        <div class="metric-content">
                            <div class="metric-label">Last Seen</div>
                            <div class="metric-value">${formatTimeAgo(lastSeen)}</div>
                            <div class="metric-subtext">${formatDateTime(lifecycle.last_seen)}</div>
                        </div>
                    </div>

                    <div class="metric-box ${stateChanges > 5 ? 'metric-warning' : ''}">
                        <div class="metric-icon">🔄</div>
                        <div class="metric-content">
                            <div class="metric-label">State Changes</div>
                            <div class="metric-value">${stateChanges}</div>
                        </div>
                    </div>

                    <div class="metric-box ${imageUpdates > 0 ? 'metric-info' : ''}">
                        <div class="metric-icon">⬆️</div>
                        <div class="metric-content">
                            <div class="metric-label">Image Updates</div>
                            <div class="metric-value">${imageUpdates}</div>
                        </div>
                    </div>

                    <div class="metric-box ${restartEvents > 10 ? 'metric-alert' : restartEvents > 0 ? 'metric-warning' : ''}">
                        <div class="metric-icon">🔁</div>
                        <div class="metric-content">
                            <div class="metric-label">Restarts</div>
                            <div class="metric-value">${restartEvents}</div>
                        </div>
                    </div>
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

// Stats Modal
let statsCharts = { cpu: null, memory: null };
let currentStatsContainer = null;
let currentStatsRange = '1h';

function openStatsModal(hostId, containerId, containerName) {
    console.log('openStatsModal called with:', { hostId, containerId, containerName });

    currentStatsContainer = { hostId, containerId, containerName };
    currentStatsRange = '1h';

    const modal = document.getElementById('statsModal');
    const nameElement = document.getElementById('statsContainerName');

    if (!modal) {
        console.error('Stats modal element not found!');
        return;
    }

    if (!nameElement) {
        console.error('Stats container name element not found!');
        return;
    }

    nameElement.textContent = containerName;

    // Use the 'show' class instead of style.display
    modal.classList.add('show');
    modal.style.display = ''; // Clear any inline style

    console.log('Modal displayed with show class');

    // Reset range buttons
    document.querySelectorAll('.stats-range-btn').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.range === '1h');
    });

    // Add click handlers for range buttons
    document.querySelectorAll('.stats-range-btn').forEach(btn => {
        btn.onclick = () => changeStatsRange(btn.dataset.range);
    });

    loadStatsData();
}

function closeStatsModal() {
    const modal = document.getElementById('statsModal');
    if (modal) {
        modal.classList.remove('show');
    }

    // Destroy charts
    if (statsCharts.cpu) {
        statsCharts.cpu.destroy();
        statsCharts.cpu = null;
    }
    if (statsCharts.memory) {
        statsCharts.memory.destroy();
        statsCharts.memory = null;
    }

    currentStatsContainer = null;
}

function changeStatsRange(range) {
    currentStatsRange = range;

    // Update active button
    document.querySelectorAll('.stats-range-btn').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.range === range);
    });

    loadStatsData();
}

async function loadStatsData() {
    if (!currentStatsContainer) {
        console.error('No current stats container set');
        return;
    }

    const { hostId, containerId } = currentStatsContainer;
    const url = `/api/containers/${hostId}/${containerId}/stats?range=${currentStatsRange}`;

    console.log('Loading stats from:', url);

    try {
        const response = await fetch(url);
        console.log('Stats response status:', response.status);

        if (!response.ok) {
            const errorText = await response.text();
            console.error('Stats API error:', errorText);
            throw new Error(`Failed to load stats: ${response.status} ${errorText}`);
        }

        const stats = await response.json();
        console.log('Stats data received:', stats);

        if (!stats || !Array.isArray(stats) || stats.length === 0) {
            document.getElementById('statsMessage').textContent = 'No stats data available for this time range. Stats collection may need more time to gather data.';
            document.getElementById('statsMessage').className = 'loading';
            document.getElementById('statsMessage').style.display = 'block';
            document.getElementById('statsChartArea').style.display = 'none';
            return;
        }

        // Hide message and show charts
        document.getElementById('statsMessage').style.display = 'none';
        document.getElementById('statsChartArea').style.display = 'block';

        renderStatsCharts(stats);
        updateStatsSummary(stats);
    } catch (error) {
        console.error('Error loading stats:', error);
        document.getElementById('statsMessage').textContent = `Failed to load stats data: ${error.message}`;
        document.getElementById('statsMessage').className = 'error';
        document.getElementById('statsMessage').style.display = 'block';
        document.getElementById('statsChartArea').style.display = 'none';
    }
}

function renderStatsCharts(stats) {
    // Destroy existing charts
    if (statsCharts.cpu) statsCharts.cpu.destroy();
    if (statsCharts.memory) statsCharts.memory.destroy();

    // Prepare data
    const labels = stats.map(s => new Date(s.timestamp).toLocaleString());
    const cpuData = stats.map(s => s.cpu_percent || 0);
    const memoryData = stats.map(s => (s.memory_usage || 0) / 1024 / 1024); // Convert to MB
    const memoryLimitData = stats.map(s => (s.memory_limit || 0) / 1024 / 1024);

    // CPU Chart
    const cpuCanvas = document.getElementById('cpuChart');
    const cpuCtx = cpuCanvas.getContext('2d');
    statsCharts.cpu = new Chart(cpuCtx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [{
                label: 'CPU %',
                data: cpuData,
                borderColor: 'rgb(75, 192, 192)',
                backgroundColor: 'rgba(75, 192, 192, 0.2)',
                tension: 0.4,
                fill: true
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                title: {
                    display: true,
                    text: 'CPU Usage Over Time'
                },
                legend: {
                    display: false
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    max: 100,
                    title: {
                        display: true,
                        text: 'CPU %'
                    }
                },
                x: {
                    ticks: {
                        maxTicksLimit: 10
                    }
                }
            }
        }
    });

    // Memory Chart
    const memoryCanvas = document.getElementById('memoryChart');
    const memoryCtx = memoryCanvas.getContext('2d');
    const datasets = [{
        label: 'Memory Usage (MB)',
        data: memoryData,
        borderColor: 'rgb(255, 99, 132)',
        backgroundColor: 'rgba(255, 99, 132, 0.2)',
        tension: 0.4,
        fill: true
    }];

    // Add memory limit line if available
    const hasLimit = memoryLimitData.some(l => l > 0);
    if (hasLimit) {
        datasets.push({
            label: 'Memory Limit (MB)',
            data: memoryLimitData,
            borderColor: 'rgb(255, 159, 64)',
            backgroundColor: 'rgba(255, 159, 64, 0.1)',
            borderDash: [5, 5],
            tension: 0,
            fill: false
        });
    }

    statsCharts.memory = new Chart(memoryCtx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: datasets
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                title: {
                    display: true,
                    text: 'Memory Usage Over Time'
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    title: {
                        display: true,
                        text: 'Memory (MB)'
                    }
                },
                x: {
                    ticks: {
                        maxTicksLimit: 10
                    }
                }
            }
        }
    });
}

function updateStatsSummary(stats) {
    const cpuValues = stats.map(s => s.cpu_percent || 0).filter(v => v > 0);
    const memoryValues = stats.map(s => s.memory_usage || 0).filter(v => v > 0);

    // CPU stats
    const avgCpu = cpuValues.length > 0 ? cpuValues.reduce((a, b) => a + b, 0) / cpuValues.length : 0;
    const maxCpu = cpuValues.length > 0 ? Math.max(...cpuValues) : 0;

    document.getElementById('avgCpu').textContent = avgCpu.toFixed(1) + '%';
    document.getElementById('maxCpu').textContent = maxCpu.toFixed(1) + '%';

    // Memory stats
    const avgMemory = memoryValues.length > 0 ? memoryValues.reduce((a, b) => a + b, 0) / memoryValues.length : 0;
    const maxMemory = memoryValues.length > 0 ? Math.max(...memoryValues) : 0;

    const formatMemory = (bytes) => {
        const mb = bytes / 1024 / 1024;
        if (mb > 1024) {
            return (mb / 1024).toFixed(2) + ' GB';
        }
        return mb.toFixed(0) + ' MB';
    };

    document.getElementById('avgMemory').textContent = formatMemory(avgMemory);
    document.getElementById('maxMemory').textContent = formatMemory(maxMemory);
}

// Close modal when clicking outside
document.getElementById('statsModal')?.addEventListener('click', (e) => {
    if (e.target.classList.contains('modal')) closeStatsModal();
});

// ==================== REPORTS TAB ====================

let currentReport = null;
let changesTimelineChart = null;

// Initialize reports tab
function initializeReportsTab() {
    // Set default date range to last 7 days
    const end = new Date();
    const start = new Date(end - 7 * 24 * 60 * 60 * 1000);

    document.getElementById('reportStartDate').value = formatDateTimeLocal(start);
    document.getElementById('reportEndDate').value = formatDateTimeLocal(end);

    // Load hosts for filter
    loadHostsForReportFilter();

    // Set up event listeners
    setupReportEventListeners();
}

// Set up event listeners for reports tab
function setupReportEventListeners() {
    document.getElementById('generateReportBtn').addEventListener('click', generateReport);
    document.getElementById('report7d').addEventListener('click', () => setReportRange(7));
    document.getElementById('report30d').addEventListener('click', () => setReportRange(30));
    document.getElementById('report90d').addEventListener('click', () => setReportRange(90));
    document.getElementById('exportReportBtn').addEventListener('click', exportReport);
}

// Navigate to History tab with container filter
function goToContainerHistory(containerName, hostId) {
    // Switch to history tab
    switchTab('history');

    // Set the search filter to the container name
    const searchInput = document.getElementById('searchInput');
    if (searchInput) {
        searchInput.value = containerName;
    }

    // Set the host filter if provided
    const hostFilter = document.getElementById('hostFilter');
    if (hostFilter && hostId) {
        hostFilter.value = hostId.toString();
    }

    // Apply the filters
    setTimeout(() => {
        applyCurrentFilters();
    }, 100);
}

// Format date for datetime-local input
function formatDateTimeLocal(date) {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    const hours = String(date.getHours()).padStart(2, '0');
    const minutes = String(date.getMinutes()).padStart(2, '0');
    return `${year}-${month}-${day}T${hours}:${minutes}`;
}

// Load hosts for report filter dropdown
async function loadHostsForReportFilter() {
    try {
        const response = await fetch('/api/hosts');
        const data = await response.json();

        const select = document.getElementById('reportHostFilter');
        select.innerHTML = '<option value="">All Hosts</option>';

        data.forEach(host => {
            const option = document.createElement('option');
            option.value = host.id;
            option.textContent = host.name;
            select.appendChild(option);
        });
    } catch (error) {
        console.error('Failed to load hosts for report filter:', error);
    }
}

// Set report date range preset
function setReportRange(days) {
    const end = new Date();
    const start = new Date(end - days * 24 * 60 * 60 * 1000);

    document.getElementById('reportStartDate').value = formatDateTimeLocal(start);
    document.getElementById('reportEndDate').value = formatDateTimeLocal(end);
}

// Generate report
async function generateReport() {
    const startInput = document.getElementById('reportStartDate').value;
    const endInput = document.getElementById('reportEndDate').value;
    const hostFilter = document.getElementById('reportHostFilter').value;

    if (!startInput || !endInput) {
        alert('Please select both start and end dates');
        return;
    }

    const start = new Date(startInput).toISOString();
    const end = new Date(endInput).toISOString();

    // Show loading, hide results and empty state
    document.getElementById('reportLoading').style.display = 'block';
    document.getElementById('reportResults').style.display = 'none';
    document.getElementById('reportEmptyState').style.display = 'none';

    try {
        let url = `/api/reports/changes?start=${encodeURIComponent(start)}&end=${encodeURIComponent(end)}`;
        if (hostFilter) {
            url += `&host_id=${hostFilter}`;
        }

        const response = await fetch(url);
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${await response.text()}`);
        }

        currentReport = await response.json();
        renderReport(currentReport);

        // Hide loading, show results
        document.getElementById('reportLoading').style.display = 'none';
        document.getElementById('reportResults').style.display = 'block';
    } catch (error) {
        console.error('Failed to generate report:', error);
        alert('Failed to generate report: ' + error.message);
        document.getElementById('reportLoading').style.display = 'none';
        document.getElementById('reportEmptyState').style.display = 'block';
    }
}

// Render report
function renderReport(report) {
    // Render summary cards
    renderReportSummary(report.summary);

    // Render timeline chart
    renderTimelineChart(report);

    // Render details sections
    renderNewContainers(report.new_containers);
    renderRemovedContainers(report.removed_containers);
    renderImageUpdates(report.image_updates);
    renderStateChanges(report.state_changes);
    renderTopRestarted(report.top_restarted);
}

// Render summary cards
function renderReportSummary(summary) {
    const cardsHTML = `
        <div class="stat-card">
            <div class="stat-icon">🖥️</div>
            <div class="stat-content">
                <div class="stat-value">${summary.total_hosts}</div>
                <div class="stat-label">Total Hosts</div>
            </div>
        </div>
        <div class="stat-card">
            <div class="stat-icon">📦</div>
            <div class="stat-content">
                <div class="stat-value">${summary.total_containers}</div>
                <div class="stat-label">Total Containers</div>
            </div>
        </div>
        <div class="stat-card">
            <div class="stat-icon">🆕</div>
            <div class="stat-content">
                <div class="stat-value">${summary.new_containers}</div>
                <div class="stat-label">New Containers</div>
            </div>
        </div>
        <div class="stat-card">
            <div class="stat-icon">❌</div>
            <div class="stat-content">
                <div class="stat-value">${summary.removed_containers}</div>
                <div class="stat-label">Removed</div>
            </div>
        </div>
        <div class="stat-card">
            <div class="stat-icon">🔄</div>
            <div class="stat-content">
                <div class="stat-value">${summary.image_updates}</div>
                <div class="stat-label">Image Updates</div>
            </div>
        </div>
        <div class="stat-card">
            <div class="stat-icon">🔀</div>
            <div class="stat-content">
                <div class="stat-value">${summary.state_changes}</div>
                <div class="stat-label">State Changes</div>
            </div>
        </div>
    `;

    document.getElementById('reportSummaryCards').innerHTML = cardsHTML;
}

// Render timeline chart
function renderTimelineChart(report) {
    // Destroy existing chart if it exists
    if (changesTimelineChart) {
        changesTimelineChart.destroy();
    }

    // Aggregate changes by day
    const changesByDay = {};

    // Helper to get day key
    const getDayKey = (timestamp) => {
        const date = new Date(timestamp);
        return date.toISOString().split('T')[0];
    };

    // Count new containers
    report.new_containers.forEach(c => {
        const day = getDayKey(c.timestamp);
        if (!changesByDay[day]) changesByDay[day] = { new: 0, removed: 0, imageUpdates: 0, stateChanges: 0 };
        changesByDay[day].new++;
    });

    // Count removed containers
    report.removed_containers.forEach(c => {
        const day = getDayKey(c.timestamp);
        if (!changesByDay[day]) changesByDay[day] = { new: 0, removed: 0, imageUpdates: 0, stateChanges: 0 };
        changesByDay[day].removed++;
    });

    // Count image updates
    report.image_updates.forEach(u => {
        const day = getDayKey(u.updated_at);
        if (!changesByDay[day]) changesByDay[day] = { new: 0, removed: 0, imageUpdates: 0, stateChanges: 0 };
        changesByDay[day].imageUpdates++;
    });

    // Count state changes
    report.state_changes.forEach(s => {
        const day = getDayKey(s.changed_at);
        if (!changesByDay[day]) changesByDay[day] = { new: 0, removed: 0, imageUpdates: 0, stateChanges: 0 };
        changesByDay[day].stateChanges++;
    });

    // Sort days
    const days = Object.keys(changesByDay).sort();

    const ctx = document.getElementById('changesTimelineChart').getContext('2d');
    changesTimelineChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: days.map(d => new Date(d).toLocaleDateString()),
            datasets: [
                {
                    label: 'New Containers',
                    data: days.map(d => changesByDay[d].new),
                    borderColor: '#2ecc71',
                    backgroundColor: 'rgba(46, 204, 113, 0.1)',
                    tension: 0.4
                },
                {
                    label: 'Removed Containers',
                    data: days.map(d => changesByDay[d].removed),
                    borderColor: '#e74c3c',
                    backgroundColor: 'rgba(231, 76, 60, 0.1)',
                    tension: 0.4
                },
                {
                    label: 'Image Updates',
                    data: days.map(d => changesByDay[d].imageUpdates),
                    borderColor: '#3498db',
                    backgroundColor: 'rgba(52, 152, 219, 0.1)',
                    tension: 0.4
                },
                {
                    label: 'State Changes',
                    data: days.map(d => changesByDay[d].stateChanges),
                    borderColor: '#f39c12',
                    backgroundColor: 'rgba(243, 156, 18, 0.1)',
                    tension: 0.4
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            plugins: {
                legend: {
                    display: true,
                    position: 'bottom'
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    ticks: {
                        stepSize: 1
                    }
                }
            }
        }
    });
}

// Render new containers table
function renderNewContainers(containers) {
    document.getElementById('newContainersCount').textContent = containers.length;

    if (containers.length === 0) {
        document.getElementById('newContainersTable').innerHTML = '<p class="empty-message">No new containers in this period</p>';
        return;
    }

    const tableHTML = `
        <table class="report-table">
            <thead>
                <tr>
                    <th>Container Name</th>
                    <th>Image</th>
                    <th>Host</th>
                    <th>First Seen</th>
                    <th>State</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>
                ${containers.map(c => `
                    <tr>
                        <td>
                            <code class="container-link" onclick="goToContainerHistory('${escapeHtml(c.container_name)}', ${c.host_id})" title="View in History">
                                ${escapeHtml(c.container_name)} 🔗
                            </code>
                            ${c.is_transient ? '<span class="transient-badge" title="This container appeared and disappeared within the reporting period">⚡ Transient</span>' : ''}
                        </td>
                        <td>${escapeHtml(c.image)}</td>
                        <td>${escapeHtml(c.host_name)}</td>
                        <td>${formatDateTime(c.timestamp)}</td>
                        <td><span class="status-badge status-${c.state}">${c.state}</span></td>
                        <td>
                            <button class="btn-icon" onclick="openStatsModal(${c.host_id}, '${escapeHtml(c.container_id)}', '${escapeHtml(c.container_name)}')" title="View Stats & Timeline">
                                📊
                            </button>
                            <button class="btn-icon" onclick="viewContainerTimeline(${c.host_id}, '${escapeHtml(c.container_id)}', '${escapeHtml(c.container_name)}')" title="View Lifecycle Timeline">
                                📜
                            </button>
                        </td>
                    </tr>
                `).join('')}
            </tbody>
        </table>
    `;

    document.getElementById('newContainersTable').innerHTML = tableHTML;
}

// Render removed containers table
function renderRemovedContainers(containers) {
    document.getElementById('removedContainersCount').textContent = containers.length;

    if (containers.length === 0) {
        document.getElementById('removedContainersTable').innerHTML = '<p class="empty-message">No removed containers in this period</p>';
        return;
    }

    const tableHTML = `
        <table class="report-table">
            <thead>
                <tr>
                    <th>Container Name</th>
                    <th>Image</th>
                    <th>Host</th>
                    <th>Last Seen</th>
                    <th>Final State</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>
                ${containers.map(c => `
                    <tr>
                        <td>
                            <code class="container-link" onclick="goToContainerHistory('${escapeHtml(c.container_name)}', ${c.host_id})" title="View in History">
                                ${escapeHtml(c.container_name)} 🔗
                            </code>
                            ${c.is_transient ? '<span class="transient-badge" title="This container appeared and disappeared within the reporting period">⚡ Transient</span>' : ''}
                        </td>
                        <td>${escapeHtml(c.image)}</td>
                        <td>${escapeHtml(c.host_name)}</td>
                        <td>${formatDateTime(c.timestamp)}</td>
                        <td><span class="status-badge status-${c.state}">${c.state}</span></td>
                        <td>
                            <button class="btn-icon" onclick="openStatsModal(${c.host_id}, '${escapeHtml(c.container_id)}', '${escapeHtml(c.container_name)}')" title="View Stats & Timeline">
                                📊
                            </button>
                            <button class="btn-icon" onclick="viewContainerTimeline(${c.host_id}, '${escapeHtml(c.container_id)}', '${escapeHtml(c.container_name)}')" title="View Lifecycle Timeline">
                                📜
                            </button>
                        </td>
                    </tr>
                `).join('')}
            </tbody>
        </table>
    `;

    document.getElementById('removedContainersTable').innerHTML = tableHTML;
}

// Render image updates table
function renderImageUpdates(updates) {
    document.getElementById('imageUpdatesCount').textContent = updates.length;

    if (updates.length === 0) {
        document.getElementById('imageUpdatesTable').innerHTML = '<p class="empty-message">No image updates in this period</p>';
        return;
    }

    const tableHTML = `
        <table class="report-table">
            <thead>
                <tr>
                    <th>Container Name</th>
                    <th>Host</th>
                    <th>Old Image</th>
                    <th>New Image</th>
                    <th>Updated At</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>
                ${updates.map(u => `
                    <tr>
                        <td>
                            <code class="container-link" onclick="goToContainerHistory('${escapeHtml(u.container_name)}', ${u.host_id})" title="View in History">
                                ${escapeHtml(u.container_name)} 🔗
                            </code>
                        </td>
                        <td>${escapeHtml(u.host_name)}</td>
                        <td>${escapeHtml(u.old_image)}<br><small>${u.old_image_id.substring(0, 12)}</small></td>
                        <td>${escapeHtml(u.new_image)}<br><small>${u.new_image_id.substring(0, 12)}</small></td>
                        <td>${formatDateTime(u.updated_at)}</td>
                        <td>
                            <button class="btn-icon" onclick="openStatsModal(${u.host_id}, '${escapeHtml(u.container_id)}', '${escapeHtml(u.container_name)}')" title="View Stats & Timeline">
                                📊
                            </button>
                            <button class="btn-icon" onclick="viewContainerTimeline(${u.host_id}, '${escapeHtml(u.container_id)}', '${escapeHtml(u.container_name)}')" title="View Lifecycle Timeline">
                                📜
                            </button>
                        </td>
                    </tr>
                `).join('')}
            </tbody>
        </table>
    `;

    document.getElementById('imageUpdatesTable').innerHTML = tableHTML;
}

// Render state changes table
function renderStateChanges(changes) {
    document.getElementById('stateChangesCount').textContent = changes.length;

    if (changes.length === 0) {
        document.getElementById('stateChangesTable').innerHTML = '<p class="empty-message">No state changes in this period</p>';
        return;
    }

    const tableHTML = `
        <table class="report-table">
            <thead>
                <tr>
                    <th>Container Name</th>
                    <th>Host</th>
                    <th>Old State</th>
                    <th>New State</th>
                    <th>Changed At</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>
                ${changes.map(s => `
                    <tr>
                        <td>
                            <code class="container-link" onclick="goToContainerHistory('${escapeHtml(s.container_name)}', ${s.host_id})" title="View in History">
                                ${escapeHtml(s.container_name)} 🔗
                            </code>
                        </td>
                        <td>${escapeHtml(s.host_name)}</td>
                        <td><span class="status-badge status-${s.old_state}">${s.old_state}</span></td>
                        <td><span class="status-badge status-${s.new_state}">${s.new_state}</span></td>
                        <td>${formatDateTime(s.changed_at)}</td>
                        <td>
                            <button class="btn-icon" onclick="openStatsModal(${s.host_id}, '${escapeHtml(s.container_id)}', '${escapeHtml(s.container_name)}')" title="View Stats & Timeline">
                                📊
                            </button>
                            <button class="btn-icon" onclick="viewContainerTimeline(${s.host_id}, '${escapeHtml(s.container_id)}', '${escapeHtml(s.container_name)}')" title="View Lifecycle Timeline">
                                📜
                            </button>
                        </td>
                    </tr>
                `).join('')}
            </tbody>
        </table>
    `;

    document.getElementById('stateChangesTable').innerHTML = tableHTML;
}

// Render top restarted containers table
function renderTopRestarted(containers) {
    document.getElementById('topRestartedCount').textContent = containers.length;

    if (containers.length === 0) {
        document.getElementById('topRestartedTable').innerHTML = '<p class="empty-message">No active containers in this period</p>';
        return;
    }

    const tableHTML = `
        <table class="report-table">
            <thead>
                <tr>
                    <th>Container Name</th>
                    <th>Image</th>
                    <th>Host</th>
                    <th>Activity Count</th>
                    <th>Current State</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>
                ${containers.map(r => `
                    <tr>
                        <td>
                            <code class="container-link" onclick="goToContainerHistory('${escapeHtml(r.container_name)}', ${r.host_id})" title="View in History">
                                ${escapeHtml(r.container_name)} 🔗
                            </code>
                        </td>
                        <td>${escapeHtml(r.image)}</td>
                        <td>${escapeHtml(r.host_name)}</td>
                        <td>${r.restart_count}</td>
                        <td><span class="status-badge status-${r.current_state}">${r.current_state}</span></td>
                        <td>
                            <button class="btn-icon" onclick="openStatsModal(${r.host_id}, '${escapeHtml(r.container_id)}', '${escapeHtml(r.container_name)}')" title="View Stats & Timeline">
                                📊
                            </button>
                            <button class="btn-icon" onclick="viewContainerTimeline(${r.host_id}, '${escapeHtml(r.container_id)}', '${escapeHtml(r.container_name)}')" title="View Lifecycle Timeline">
                                📜
                            </button>
                        </td>
                    </tr>
                `).join('')}
            </tbody>
        </table>
    `;

    document.getElementById('topRestartedTable').innerHTML = tableHTML;
}

// Toggle report section visibility
window.toggleReportSection = function(section) {
    const sectionElement = document.getElementById(`${section}Section`);
    const isVisible = sectionElement.style.display !== 'none';
    sectionElement.style.display = isVisible ? 'none' : 'block';

    // Toggle collapse icon
    const header = sectionElement.previousElementSibling;
    const icon = header.querySelector('.collapse-icon');
    if (icon) {
        icon.textContent = isVisible ? '▶' : '▼';
    }
};

// Export report as JSON
function exportReport() {
    if (!currentReport) {
        alert('No report to export. Please generate a report first.');
        return;
    }

    const dataStr = JSON.stringify(currentReport, null, 2);
    const dataBlob = new Blob([dataStr], { type: 'application/json' });
    const url = URL.createObjectURL(dataBlob);

    const link = document.createElement('a');
    link.href = url;
    link.download = `container-census-report-${new Date().toISOString().split('T')[0]}.json`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
}

// Helper: Escape HTML
function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Helper: Format date/time
function formatDateTime(timestamp) {
    if (!timestamp) return '-';
    const date = new Date(timestamp);
    return date.toLocaleString();
}

// ===== Vulnerability Scanning =====

// Fetch vulnerability scan for an image
async function getVulnerabilityScan(imageID) {
    // Check cache first
    if (vulnerabilityCache[imageID]) {
        return vulnerabilityCache[imageID];
    }

    // If not in cache, try loading from the pre-loaded scans map
    if (vulnerabilityScansMap && vulnerabilityScansMap[imageID]) {
        vulnerabilityCache[imageID] = vulnerabilityScansMap[imageID];
        return vulnerabilityScansMap[imageID];
    }

    // Mark as null in cache to avoid repeated 404 requests
    vulnerabilityCache[imageID] = null;
    return null;
}

// Pre-load all vulnerability scans to avoid 404 requests
async function preloadVulnerabilityScans() {
    try {
        const response = await fetch('/api/vulnerabilities/scans?limit=1000', {
            headers: {
                'Authorization': 'Basic ' + btoa(authUsername + ':' + authPassword)
            }
        });
        if (response.ok) {
            const scans = await response.json();
            // Build a map of imageID -> scan data
            vulnerabilityScansMap = {};
            scans.forEach(scan => {
                vulnerabilityScansMap[scan.image_id] = {
                    scan: scan,
                    vulnerabilities: scan.vulnerabilities || []
                };
            });
            return vulnerabilityScansMap;
        }
    } catch (error) {
        console.error('Error preloading vulnerability scans:', error);
    }
    return {};
}

// Fetch vulnerability summary (all images)
// Note: loadVulnerabilitySummary() is defined later in the file with security-enabled check

// Generate vulnerability badge HTML
function getVulnerabilityBadgeHTML(scan) {
    if (!scan) {
        // No scan found
        return '<span class="vulnerability-badge not-scanned" title="Not scanned">🛡️ Not Scanned</span>';
    }

    if (!scan.scan.success) {
        // Check if it's a remote image (not available for scanning)
        const error = scan.scan.error || '';
        if (error.includes('image not available for scanning') || error.includes('not available')) {
            return '<span class="vulnerability-badge remote" title="Remote image - not available for scanning">🌐 Remote</span>';
        }
        // Other scan failures
        return '<span class="vulnerability-badge not-scanned" title="Scan failed">⚠️ Scan Failed</span>';
    }

    const counts = scan.scan.severity_counts || {};
    const total = scan.scan.total_vulnerabilities || 0;
    const critical = counts.critical || 0;
    const high = counts.high || 0;
    const medium = counts.medium || 0;
    const low = counts.low || 0;

    if (total === 0) {
        return '<span class="vulnerability-badge clean" title="No vulnerabilities found">✓ Clean</span>';
    }

    // Determine severity class based on highest severity found
    let badgeClass = 'low';
    let icon = '🛡️';
    if (critical > 0) {
        badgeClass = 'critical';
        icon = '🚨';
    } else if (high > 0) {
        badgeClass = 'high';
        icon = '⚠️';
    } else if (medium > 0) {
        badgeClass = 'medium';
        icon = '⚡';
    }

    // Format badge text
    let badgeText = `${icon} ${total}`;
    if (critical > 0 || high > 0) {
        badgeText += ` (${critical}C ${high}H)`;
    }

    const titleParts = [];
    if (critical > 0) titleParts.push(`${critical} Critical`);
    if (high > 0) titleParts.push(`${high} High`);
    if (medium > 0) titleParts.push(`${medium} Medium`);
    if (low > 0) titleParts.push(`${low} Low`);
    const title = `Total: ${total} vulnerabilities - ${titleParts.join(', ')}`;

    return `<span class="vulnerability-badge ${badgeClass}" title="${title}">${badgeText}</span>`;
}

// Add vulnerability badge to container card (called asynchronously)
async function addVulnerabilityBadge(containerElement, imageID) {
    const scan = await getVulnerabilityScan(imageID);
    const badgeHTML = getVulnerabilityBadgeHTML(scan, imageID);

    // Find the image row in the container card
    const imageRow = containerElement.querySelector('.detail-value.image-value');
    if (imageRow && imageRow.parentElement) {
        // Add badge after the image name
        const badgeContainer = document.createElement('span');
        badgeContainer.innerHTML = badgeHTML;
        const badge = badgeContainer.firstChild;

        // Make badge clickable if it has vulnerabilities
        if (scan && scan.scan && scan.scan.success) {
            const imageName = scan.scan.image_name || imageID;
            badge.style.cursor = 'pointer';
            badge.onclick = () => viewVulnerabilityDetails(imageID, imageName);
        }

        imageRow.parentElement.appendChild(badge);
    }
}

// Load vulnerability badges for all visible containers
async function loadAllVulnerabilityBadges() {
    const containerCards = document.querySelectorAll('.container-card-modern');

    // Get the current filtered containers being displayed
    const searchTerm = document.getElementById('searchInput')?.value.toLowerCase() || '';
    const hostFilter = document.getElementById('hostFilter')?.value || '';
    const stateFilter = document.getElementById('stateFilter')?.value || '';

    const filtered = containers.filter(container => {
        const matchesSearch = searchTerm === '' ||
            container.name.toLowerCase().includes(searchTerm) ||
            container.image.toLowerCase().includes(searchTerm) ||
            container.host_name.toLowerCase().includes(searchTerm);

        const matchesHost = hostFilter === '' || container.host_id.toString() === hostFilter;
        const matchesState = stateFilter === '' || container.state === stateFilter;

        return matchesSearch && matchesHost && matchesState;
    });

    // Pre-load all vulnerability scans to avoid 404 errors
    await preloadVulnerabilityScans();

    // Now add badges to each card
    containerCards.forEach((card, index) => {
        if (filtered[index] && filtered[index].image_id) {
            addVulnerabilityBadge(card, filtered[index].image_id);
        }
    });
}

// ===== Security Tab =====

let allVulnerabilityScans = [];
let securityChart = null;
let scanningImages = new Set(); // Track images currently being scanned

// Load the security tab
async function loadSecurityTab() {
    try {
        // Load summary and all scans in parallel
        const [summary, scans] = await Promise.all([
            loadVulnerabilitySummary(),
            loadAllVulnerabilityScans()
        ]);

        allVulnerabilityScans = scans || [];

        // Update summary cards
        updateSecuritySummaryCards(summary, allVulnerabilityScans);

        // Render security chart
        renderSecurityChart(summary);

        // Render vulnerability trends chart
        renderVulnerabilityTrendsChart(allVulnerabilityScans);

        // Update queue status
        updateQueueStatus(summary?.queue_status);

        // Update scan count badge (use allVulnerabilityScans for total, not filtered)
        const scanCountBadge = document.getElementById('scanCountBadge');
        if (scanCountBadge && allVulnerabilityScans) {
            scanCountBadge.textContent = `${allVulnerabilityScans.length} scan${allVulnerabilityScans.length !== 1 ? 's' : ''}`;
        }

        // Render scans table
        filterSecurityScans();

        // Start periodic queue status updates (every 3 seconds)
        startQueueStatusPolling();

    } catch (error) {
        console.error('Error loading security tab:', error);
    }
}

// Poll queue status periodically to update button states
let queueStatusInterval = null;
function startQueueStatusPolling() {
    // Clear existing interval
    if (queueStatusInterval) {
        clearInterval(queueStatusInterval);
    }

    // Poll every 3 seconds
    queueStatusInterval = setInterval(async () => {
        try {
            const summary = await loadVulnerabilitySummary();
            updateQueueStatus(summary?.queue_status);
        } catch (error) {
            console.error('Error polling queue status:', error);
        }
    }, 3000);
}

// Stop polling when leaving security tab
function stopQueueStatusPolling() {
    if (queueStatusInterval) {
        clearInterval(queueStatusInterval);
        queueStatusInterval = null;
    }
}

// Load all vulnerability scans
async function loadAllVulnerabilityScans() {
    try {
        const response = await fetch('/api/vulnerabilities/scans?limit=1000');
        if (response.ok) {
            return await response.json();
        }
    } catch (error) {
        console.error('Error fetching vulnerability scans:', error);
    }
    return [];
}

// Update security summary cards
function updateSecuritySummaryCards(summary, scans) {
    if (!summary) {
        document.getElementById('totalScannedImages').textContent = '-';
        document.getElementById('totalCriticalVulns').textContent = '-';
        document.getElementById('totalHighVulns').textContent = '-';
        document.getElementById('atRiskImages').textContent = '-';
        return;
    }

    // Handle both wrapped (summary.summary) and direct summary objects
    const s = summary.summary || summary;
    const totalScans = scans ? scans.length : 0;
    const uniqueImages = s.total_images_scanned || 0;

    // Show both unique images and total scans for clarity
    const displayText = totalScans > 0 ? `${uniqueImages} (${totalScans} scans)` : `${uniqueImages}`;
    document.getElementById('totalScannedImages').textContent = displayText;
    document.getElementById('totalCriticalVulns').textContent = s.severity_counts?.critical || 0;
    document.getElementById('totalHighVulns').textContent = s.severity_counts?.high || 0;
    document.getElementById('atRiskImages').textContent = s.images_with_vulnerabilities || 0;
}

// Render security severity chart
function renderSecurityChart(summary) {
    const ctx = document.getElementById('vulnerabilitySeverityChart');
    if (!ctx) return;

    // Handle both wrapped (summary.summary) and direct summary objects
    const s = summary?.summary || summary || {};
    const severityCounts = s.severity_counts || {};
    const data = {
        labels: ['Critical', 'High', 'Medium', 'Low'],
        datasets: [{
            data: [
                severityCounts.critical || 0,
                severityCounts.high || 0,
                severityCounts.medium || 0,
                severityCounts.low || 0
            ],
            backgroundColor: [
                '#ff1744',  // Critical
                '#ff9800',  // High
                '#ffc107',  // Medium
                '#4caf50'   // Low
            ],
            borderWidth: 2,
            borderColor: 'white'
        }]
    };

    if (securityChart) {
        securityChart.destroy();
    }

    securityChart = new Chart(ctx, {
        type: 'doughnut',
        data: data,
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    position: 'bottom',
                    labels: {
                        color: '#333',
                        font: { size: 13 },
                        padding: 12,
                        usePointStyle: true
                    }
                },
                title: {
                    display: false
                }
            }
        }
    });
}

// Global variable for trends chart
let trendsChart = null;

// Render vulnerability trends chart
function renderVulnerabilityTrendsChart(scans) {
    const ctx = document.getElementById('vulnerabilityTrendsChart');
    if (!ctx) return;

    try {
        // Use provided scans data
        if (!scans || scans.length === 0) {
            console.log('No scan data available for trends chart');
            return;
        }

        // Group scans by date (last 30 days) and calculate aggregates
        const now = new Date();
        const thirtyDaysAgo = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000);

        const dailyData = {};

        scans.forEach(scan => {
            if (!scan.success || !scan.scanned_at) return;

            const scanDate = new Date(scan.scanned_at);
            if (scanDate < thirtyDaysAgo) return;

            const dateKey = scanDate.toISOString().split('T')[0];

            if (!dailyData[dateKey]) {
                dailyData[dateKey] = {
                    critical: 0,
                    high: 0,
                    medium: 0,
                    low: 0,
                    total: 0,
                    count: 0
                };
            }

            const counts = scan.severity_counts || {};
            dailyData[dateKey].critical += counts.critical || 0;
            dailyData[dateKey].high += counts.high || 0;
            dailyData[dateKey].medium += counts.medium || 0;
            dailyData[dateKey].low += counts.low || 0;
            dailyData[dateKey].total += scan.total_vulnerabilities || 0;
            dailyData[dateKey].count++;
        });

        // Sort dates and create labels
        const sortedDates = Object.keys(dailyData).sort();
        const labels = sortedDates.map(date => {
            const d = new Date(date);
            return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
        });

        const criticalData = sortedDates.map(date => dailyData[date].critical);
        const highData = sortedDates.map(date => dailyData[date].high);
        const mediumData = sortedDates.map(date => dailyData[date].medium);
        const lowData = sortedDates.map(date => dailyData[date].low);

        if (trendsChart) {
            trendsChart.destroy();
        }

        trendsChart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: labels,
                datasets: [
                    {
                        label: 'Critical',
                        data: criticalData,
                        borderColor: '#ff1744',
                        backgroundColor: 'rgba(255, 23, 68, 0.1)',
                        borderWidth: 2,
                        fill: true,
                        tension: 0.4
                    },
                    {
                        label: 'High',
                        data: highData,
                        borderColor: '#ff9800',
                        backgroundColor: 'rgba(255, 152, 0, 0.1)',
                        borderWidth: 2,
                        fill: true,
                        tension: 0.4
                    },
                    {
                        label: 'Medium',
                        data: mediumData,
                        borderColor: '#ffc107',
                        backgroundColor: 'rgba(255, 193, 7, 0.1)',
                        borderWidth: 2,
                        fill: true,
                        tension: 0.4
                    },
                    {
                        label: 'Low',
                        data: lowData,
                        borderColor: '#4caf50',
                        backgroundColor: 'rgba(76, 175, 80, 0.1)',
                        borderWidth: 2,
                        fill: true,
                        tension: 0.4
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                interaction: {
                    mode: 'index',
                    intersect: false
                },
                plugins: {
                    legend: {
                        position: 'bottom',
                        labels: {
                            color: '#333',
                            font: { size: 13 },
                            padding: 12,
                            usePointStyle: true
                        }
                    },
                    title: {
                        display: false
                    },
                    tooltip: {
                        backgroundColor: 'rgba(0, 0, 0, 0.8)',
                        padding: 12,
                        titleFont: { size: 14, weight: 'bold' },
                        bodyFont: { size: 13 },
                        callbacks: {
                            footer: function(context) {
                                let total = 0;
                                context.forEach(item => {
                                    total += item.parsed.y;
                                });
                                return 'Total: ' + total;
                            }
                        }
                    }
                },
                scales: {
                    x: {
                        grid: {
                            display: false
                        },
                        ticks: {
                            color: '#666',
                            font: { size: 11 }
                        }
                    },
                    y: {
                        beginAtZero: true,
                        grid: {
                            color: 'rgba(0, 0, 0, 0.05)'
                        },
                        ticks: {
                            color: '#666',
                            font: { size: 11 },
                            precision: 0
                        }
                    }
                }
            }
        });
    } catch (error) {
        console.error('Error rendering trends chart:', error);
    }
}

// Update queue status
function updateQueueStatus(queueStatus) {
    const queueDiv = document.getElementById('securityQueueStatus');
    if (!queueDiv) return;

    if (!queueStatus) {
        queueDiv.style.display = 'none';
        scanningImages.clear();
        return;
    }

    // Update set of images currently being scanned
    scanningImages.clear();
    if (queueStatus.queue_items && Array.isArray(queueStatus.queue_items)) {
        queueStatus.queue_items.forEach(item => {
            if (item.image_id) {
                scanningImages.add(item.image_id);
            }
        });
    }

    // Show status if there's any activity OR worker info
    const hasActivity = queueStatus.in_progress > 0 || queueStatus.queued > 0;
    const hasWorkerInfo = queueStatus.total_workers && queueStatus.total_workers > 0;

    if (!hasActivity && !hasWorkerInfo) {
        queueDiv.style.display = 'none';
        return;
    }

    queueDiv.style.display = 'flex';
    const statusText = document.getElementById('queueStatusText');

    let text = '';
    if (hasWorkerInfo) {
        text = `${queueStatus.total_workers} workers (${queueStatus.active_workers || 0} active)`;
        if (hasActivity) {
            text += ` - ${queueStatus.in_progress} scanning, ${queueStatus.queued} queued`;
        }
    } else if (hasActivity) {
        text = `${queueStatus.in_progress} scanning, ${queueStatus.queued} queued`;
    }

    statusText.textContent = text;

    // Re-render table to update button states
    filterSecurityScans();
}

// Filter security scans table
function filterSecurityScans() {
    const searchTerm = document.getElementById('securitySearchInput')?.value.toLowerCase() || '';
    const severityFilter = document.getElementById('securitySeverityFilter')?.value || '';
    const statusFilter = document.getElementById('securityStatusFilter')?.value || '';

    const filtered = allVulnerabilityScans.filter(scan => {
        const matchesSearch = searchTerm === '' ||
            scan.image_name.toLowerCase().includes(searchTerm) ||
            scan.image_id.toLowerCase().includes(searchTerm);

        let matchesSeverity = true;
        if (severityFilter) {
            if (severityFilter === 'clean') {
                matchesSeverity = scan.total_vulnerabilities === 0 && scan.success;
            } else if (severityFilter === 'critical') {
                matchesSeverity = (scan.severity_counts?.critical || 0) > 0;
            } else if (severityFilter === 'high') {
                matchesSeverity = (scan.severity_counts?.high || 0) > 0;
            } else if (severityFilter === 'medium') {
                matchesSeverity = (scan.severity_counts?.medium || 0) > 0;
            } else if (severityFilter === 'low') {
                matchesSeverity = (scan.severity_counts?.low || 0) > 0;
            }
        }

        let matchesStatus = true;
        if (statusFilter) {
            const error = scan.error || '';
            if (statusFilter === 'scanned') {
                matchesStatus = scan.success;
            } else if (statusFilter === 'remote') {
                matchesStatus = !scan.success && (error.includes('image not available for scanning') || error.includes('not available'));
            } else if (statusFilter === 'failed') {
                matchesStatus = !scan.success && !(error.includes('image not available for scanning') || error.includes('not available'));
            }
        }

        return matchesSearch && matchesSeverity && matchesStatus;
    });

    renderSecurityScansTable(filtered);
}

// Render security scans table
function renderSecurityScansTable(scans) {
    const tbody = document.getElementById('securityScansBody');
    if (!tbody) return;

    if (scans.length === 0) {
        tbody.innerHTML = '<tr><td colspan="9" class="loading">No scans found</td></tr>';
        return;
    }

    tbody.innerHTML = scans.map(scan => {
        const counts = scan.severity_counts || {};
        const total = scan.total_vulnerabilities || 0;
        const critical = counts.critical || 0;
        const high = counts.high || 0;
        const medium = counts.medium || 0;
        const low = counts.low || 0;
        const scannedTime = formatTimeAgo(new Date(scan.scanned_at));

        // Determine status badge
        let statusBadge = '';
        if (!scan.success) {
            const error = scan.error || '';
            if (error.includes('image not available for scanning') || error.includes('not available')) {
                statusBadge = '<span class="vulnerability-badge remote" title="Remote image - not available for scanning">🌐 Remote</span>';
            } else {
                statusBadge = '<span class="vulnerability-badge not-scanned" title="Scan failed">⚠️ Failed</span>';
            }
        } else if (total === 0) {
            statusBadge = '<span class="vulnerability-badge clean" title="No vulnerabilities found">✓ Clean</span>';
        } else if (critical > 0) {
            statusBadge = '<span class="vulnerability-badge critical" title="Has critical vulnerabilities">🚨 Critical</span>';
        } else if (high > 0) {
            statusBadge = '<span class="vulnerability-badge high" title="Has high vulnerabilities">⚠️ High</span>';
        } else {
            statusBadge = '<span class="vulnerability-badge medium" title="Has vulnerabilities">⚡ Vuln</span>';
        }

        // Check if this image is currently being scanned
        const isScanning = scan.image_id && scanningImages.has(scan.image_id);
        const rescanBtnClass = 'btn btn-sm btn-secondary';
        const rescanBtnDisabled = isScanning ? 'disabled' : '';
        const rescanBtnText = isScanning ? '⏳ Scanning...' : '🔄 Rescan';

        // Determine row class based on highest severity
        let rowClass = '';
        if (critical > 0) rowClass = 'severity-critical';
        else if (high > 0) rowClass = 'severity-high';
        else if (medium > 0) rowClass = 'severity-medium';

        return `
            <tr class="${rowClass}">
                <td><code>${escapeHtml(scan.image_name)}</code></td>
                <td>${statusBadge}</td>
                <td>${total}</td>
                <td><span class="severity-badge critical">${critical}</span></td>
                <td><span class="severity-badge high">${high}</span></td>
                <td><span class="severity-badge medium">${medium}</span></td>
                <td><span class="severity-badge low">${low}</span></td>
                <td>${scannedTime}</td>
                <td>
                    <button class="btn btn-sm btn-primary" onclick="viewVulnerabilityDetails('${escapeAttr(scan.image_id)}', '${escapeAttr(scan.image_name)}')">
                        🔍 Details
                    </button>
                    <button class="${rescanBtnClass}" onclick="rescanImage('${escapeAttr(scan.image_id)}', '${escapeAttr(scan.image_name)}')" ${rescanBtnDisabled}>
                        ${rescanBtnText}
                    </button>
                </td>
            </tr>
        `;
    }).join('');
}

// Trigger scan for all images
async function scanAllImages() {
    try {
        const response = await fetch('/api/vulnerabilities/scan-all', {
            method: 'POST',
            headers: { 'Authorization': 'Basic ' + btoa(authUsername + ':' + authPassword) }
        });
        if (response.ok) {
            const data = await response.json();
            showNotification(`Queued ${data.images_queued} images for scanning`, 'success');
            // Reload security tab after a short delay
            setTimeout(loadSecurityTab, 2000);
        } else {
            const error = await response.json();
            showNotification(`Failed to queue scans: ${error.error}`, 'error');
        }
    } catch (error) {
        console.error('Error triggering scan-all:', error);
        showNotification('Failed to queue scans', 'error');
    }
}

// Trigger scan for a specific image
async function rescanImage(imageID, imageName) {
    try {
        const response = await fetch(`/api/vulnerabilities/scan/${encodeURIComponent(imageID)}`, {
            method: 'POST',
            headers: { 'Authorization': 'Basic ' + btoa(authUsername + ':' + authPassword) }
        });
        if (response.ok) {
            showNotification(`Queued ${imageName} for scanning`, 'success');
            // Add to scanning set and update UI immediately
            scanningImages.add(imageID);
            renderSecurityScansTable(allVulnerabilityScans);

            // Update the queue status
            const summary = await loadVulnerabilitySummary();
            updateQueueStatus(summary?.queue_status);

            // Poll for scan completion
            pollForScanCompletion(imageID);
        } else {
            const error = await response.json();
            showNotification(`Failed to queue scan: ${error.error}`, 'error');
        }
    } catch (error) {
        console.error('Error triggering scan:', error);
        showNotification('Failed to queue scan', 'error');
    }
}

// Poll for scan completion and refresh data when done
async function pollForScanCompletion(imageID) {
    const maxAttempts = 60; // Poll for up to 10 minutes (60 * 10s)
    let attempts = 0;

    const pollInterval = setInterval(async () => {
        attempts++;

        // Check queue status
        const summary = await loadVulnerabilitySummary();
        updateQueueStatus(summary?.queue_status);

        // Check if this image is still in the queue
        const stillScanning = scanningImages.has(imageID);

        if (!stillScanning || attempts >= maxAttempts) {
            clearInterval(pollInterval);

            // Reload scan data to show updated results
            await preloadVulnerabilityScans();
            if (currentTab === 'security') {
                filterSecurityScans();
            }

            // Clear vulnerability scan cache for this image
            if (vulnScanCache.has(imageID)) {
                vulnScanCache.delete(imageID);
            }
        }
    }, 10000); // Poll every 10 seconds
}

// Update Trivy database
async function updateTrivyDB() {
    try {
        showNotification('Updating Trivy database... This may take a few minutes.', 'info');
        const response = await fetch('/api/vulnerabilities/update-db', {
            method: 'POST',
            headers: { 'Authorization': 'Basic ' + btoa(authUsername + ':' + authPassword) }
        });
        if (response.ok) {
            showNotification('Trivy database updated successfully', 'success');
        } else {
            const error = await response.json();
            showNotification(`Failed to update database: ${error.error}`, 'error');
        }
    } catch (error) {
        console.error('Error updating Trivy DB:', error);
        showNotification('Failed to update database', 'error');
    }
}

// View vulnerability details
async function viewVulnerabilityDetails(imageID, imageName) {
    document.getElementById('vulnDetailsImageName').textContent = imageName;
    document.getElementById('vulnerabilityDetailsModal').classList.add('show');
    document.getElementById('vulnDetailsContent').innerHTML = '<div class="loading">Loading vulnerabilities...</div>';

    try {
        const response = await fetch(`/api/vulnerabilities/image/${encodeURIComponent(imageID)}`, {
            headers: {
                'Authorization': 'Basic ' + btoa(authUsername + ':' + authPassword)
            }
        });
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }

        const data = await response.json();
        renderVulnerabilityDetails(data);
    } catch (error) {
        console.error('Error loading vulnerability details:', error);
        document.getElementById('vulnDetailsContent').innerHTML = `<div class="error">Failed to load vulnerability details: ${error.message}</div>`;
    }
}

function closeVulnerabilityDetailsModal() {
    document.getElementById('vulnerabilityDetailsModal').classList.remove('show');
}

function renderVulnerabilityDetails(data) {
    if (!data || !data.vulnerabilities || data.vulnerabilities.length === 0) {
        document.getElementById('vulnDetailsContent').innerHTML = '<p class="empty-message">No vulnerabilities found for this image.</p>';
        return;
    }

    const vulns = data.vulnerabilities;

    // Group by severity
    const bySeverity = {
        CRITICAL: vulns.filter(v => v.severity === 'CRITICAL'),
        HIGH: vulns.filter(v => v.severity === 'HIGH'),
        MEDIUM: vulns.filter(v => v.severity === 'MEDIUM'),
        LOW: vulns.filter(v => v.severity === 'LOW'),
        UNKNOWN: vulns.filter(v => v.severity === 'UNKNOWN' || !v.severity)
    };

    const html = `
        <div class="vuln-details-summary">
            <div class="vuln-stat">
                <span class="severity-badge severity-critical">${bySeverity.CRITICAL.length}</span> Critical
            </div>
            <div class="vuln-stat">
                <span class="severity-badge severity-high">${bySeverity.HIGH.length}</span> High
            </div>
            <div class="vuln-stat">
                <span class="severity-badge severity-medium">${bySeverity.MEDIUM.length}</span> Medium
            </div>
            <div class="vuln-stat">
                <span class="severity-badge severity-low">${bySeverity.LOW.length}</span> Low
            </div>
        </div>

        ${['CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'UNKNOWN'].map(severity => {
            const items = bySeverity[severity];
            if (items.length === 0) return '';

            return `
                <div class="vuln-severity-section">
                    <h3><span class="severity-badge severity-${severity.toLowerCase()}">${severity}</span> (${items.length})</h3>
                    <table class="vuln-table">
                        <thead>
                            <tr>
                                <th>CVE ID</th>
                                <th>Package</th>
                                <th>Installed</th>
                                <th>Fixed In</th>
                                <th>Title</th>
                            </tr>
                        </thead>
                        <tbody>
                            ${items.map(v => `
                                <tr>
                                    <td>
                                        <a href="https://nvd.nist.gov/vuln/detail/${escapeHtml(v.vulnerability_id)}" target="_blank" rel="noopener">
                                            ${escapeHtml(v.vulnerability_id)}
                                        </a>
                                    </td>
                                    <td><code>${escapeHtml(v.pkg_name)}</code></td>
                                    <td><code>${escapeHtml(v.installed_version || 'N/A')}</code></td>
                                    <td><code>${escapeHtml(v.fixed_version || 'Not Fixed')}</code></td>
                                    <td class="vuln-title">${escapeHtml(v.title || 'No description')}</td>
                                </tr>
                            `).join('')}
                        </tbody>
                    </table>
                </div>
            `;
        }).join('')}
    `;

    document.getElementById('vulnDetailsContent').innerHTML = html;
}

// Export vulnerabilities (placeholder)
function exportVulnerabilities() {
    showNotification('Export functionality coming soon!', 'info');
    // TODO: Export vulnerability data as CSV/JSON
}

// ===== Vulnerability Settings Modal =====

let currentVulnerabilitySettings = null;

// Open vulnerability settings modal
async function openVulnerabilitySettingsModal() {
    try {
        const response = await fetch('/api/vulnerabilities/settings', {
            headers: {
                'Authorization': 'Basic ' + btoa(authUsername + ':' + authPassword)
            }
        });
        if (response.ok) {
            currentVulnerabilitySettings = await response.json();
            populateVulnerabilitySettingsForm(currentVulnerabilitySettings);
            document.getElementById('vulnerabilitySettingsModal').classList.add('show');
        } else {
            showNotification('Failed to load vulnerability settings', 'error');
        }
    } catch (error) {
        console.error('Error loading vulnerability settings:', error);
        showNotification('Failed to load vulnerability settings', 'error');
    }
}

// Close vulnerability settings modal
function closeVulnerabilitySettingsModal() {
    document.getElementById('vulnerabilitySettingsModal').classList.remove('show');
}

// Populate vulnerability settings form
function populateVulnerabilitySettingsForm(settings) {
    document.getElementById('vulnEnabled').checked = settings.enabled || false;
    document.getElementById('vulnAutoScan').checked = settings.auto_scan_new_images || false;
    document.getElementById('vulnWorkerPoolSize').value = settings.worker_pool_size || 5;
    document.getElementById('vulnScanTimeout').value = settings.scan_timeout_minutes || 10;
    document.getElementById('vulnMaxQueueSize').value = settings.max_queue_size || 100;
    document.getElementById('vulnCacheTTL').value = settings.cache_ttl_hours || 24;
    document.getElementById('vulnRescanInterval').value = settings.rescan_interval_hours || 168;
    document.getElementById('vulnDBUpdateInterval').value = settings.db_update_interval_hours || 24;
    document.getElementById('vulnRetentionDays').value = settings.retention_days || 90;
    document.getElementById('vulnDetailedRetentionDays').value = settings.detailed_retention_days || 30;
    document.getElementById('vulnAlertCritical').checked = settings.alert_on_critical || false;
    document.getElementById('vulnAlertHigh').checked = settings.alert_on_high || false;
    document.getElementById('vulnCacheDir').value = settings.cache_dir || '/app/data/.trivy';
}

// Save vulnerability settings
async function saveVulnerabilitySettings(event) {
    event.preventDefault();

    const settings = {
        enabled: document.getElementById('vulnEnabled').checked,
        auto_scan_new_images: document.getElementById('vulnAutoScan').checked,
        worker_pool_size: parseInt(document.getElementById('vulnWorkerPoolSize').value),
        scan_timeout_minutes: parseInt(document.getElementById('vulnScanTimeout').value),
        max_queue_size: parseInt(document.getElementById('vulnMaxQueueSize').value),
        cache_ttl_hours: parseInt(document.getElementById('vulnCacheTTL').value),
        rescan_interval_hours: parseInt(document.getElementById('vulnRescanInterval').value),
        db_update_interval_hours: parseInt(document.getElementById('vulnDBUpdateInterval').value),
        retention_days: parseInt(document.getElementById('vulnRetentionDays').value),
        detailed_retention_days: parseInt(document.getElementById('vulnDetailedRetentionDays').value),
        alert_on_critical: document.getElementById('vulnAlertCritical').checked,
        alert_on_high: document.getElementById('vulnAlertHigh').checked,
        cache_dir: document.getElementById('vulnCacheDir').value
    };

    try {
        const response = await fetch('/api/vulnerabilities/settings', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(settings)
        });

        if (response.ok) {
            showNotification('Vulnerability settings saved successfully', 'success');
            closeVulnerabilitySettingsModal();
        } else {
            const error = await response.json();
            showNotification(`Failed to save settings: ${error.error}`, 'error');
        }
    } catch (error) {
        console.error('Error saving vulnerability settings:', error);
        showNotification('Failed to save settings', 'error');
    }
}

// ============================================
// Onboarding and Help Functions
// ============================================

// Global onboarding tour instance
let onboardingTourInstance = null;

// Start the onboarding tour
async function startOnboardingTour() {
    if (!window.OnboardingTour) {
        showToast('Error', 'Onboarding tour not loaded', 'error');
        return;
    }

    if (!onboardingTourInstance) {
        onboardingTourInstance = new OnboardingTour();
    }

    await onboardingTourInstance.start();
    closeHelpMenu();
}

// Initialize onboarding check on load
async function checkAndShowOnboarding() {
    // Wait a bit for page to fully load
    setTimeout(async () => {
        if (window.OnboardingTour) {
            const shouldShow = await OnboardingTour.shouldShow();
            if (shouldShow) {
                startOnboardingTour();
            }
        }
    }, 1000);
}

// Show changelog modal
async function showChangelogModal() {
    const modal = document.getElementById('changelogModal');
    const content = document.getElementById('changelogContent');

    if (!modal || !content) return;

    modal.classList.add('show');
    content.innerHTML = '<div class="loading">Loading changelog...</div>';

    try {
        const response = await fetch('/api/changelog');
        if (response.ok) {
            const markdown = await response.text();
            content.innerHTML = renderMarkdownChangelog(markdown);
        } else {
            content.innerHTML = '<div class="error">Changelog not available</div>';
        }
    } catch (error) {
        console.error('Error loading changelog:', error);
        content.innerHTML = '<div class="error">Failed to load changelog</div>';
    }

    closeHelpMenu();
}

// Close changelog modal
function closeChangelogModal() {
    const modal = document.getElementById('changelogModal');
    if (modal) {
        modal.classList.remove('show');
    }
}

// Simple markdown to HTML converter for changelog
function renderMarkdownChangelog(markdown) {
    let html = markdown
        // Headers
        .replace(/^### (.*$)/gim, '<h3>$1</h3>')
        .replace(/^## (.*$)/gim, '<h2>$1</h2>')
        .replace(/^# (.*$)/gim, '<h1>$1</h1>')
        // Bold
        .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
        // Links
        .replace(/\[([^\]]+)\]\(([^\)]+)\)/g, '<a href="$2" target="_blank">$1</a>')
        // Lists
        .replace(/^\* (.+)$/gim, '<li>$1</li>')
        .replace(/^- (.+)$/gim, '<li>$1</li>')
        // Paragraphs
        .replace(/\n\n/g, '</p><p>')
        // Code blocks
        .replace(/`([^`]+)`/g, '<code>$1</code>');

    // Wrap lists
    html = html.replace(/(<li>.*<\/li>)/s, '<ul>$1</ul>');

    // Wrap in paragraph if not already wrapped
    if (!html.startsWith('<h1>') && !html.startsWith('<h2>')) {
        html = '<p>' + html + '</p>';
    }

    return '<div class="changelog-rendered">' + html + '</div>';
}

// Help menu dropdown handling
function setupHelpMenu() {
    const helpBtn = document.getElementById('helpMenuBtn');
    const helpDropdown = document.getElementById('helpDropdown');

    if (helpBtn && helpDropdown) {
        helpBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            helpDropdown.classList.toggle('show');
        });

        // Close when clicking outside
        document.addEventListener('click', (e) => {
            if (!helpBtn.contains(e.target) && !helpDropdown.contains(e.target)) {
                helpDropdown.classList.remove('show');
            }
        });
    }
}

function closeHelpMenu() {
    const helpDropdown = document.getElementById('helpDropdown');
    if (helpDropdown) {
        helpDropdown.classList.remove('show');
    }
}


// ===== DASHBOARD FUNCTIONS =====

async function loadDashboard() {
    try {
        // Load all required data in parallel with individual error handling
        const results = await Promise.allSettled([
            loadContainers(),
            loadHosts(),
            loadVulnerabilitySummary()
        ]);

        // Log any failures but don't stop rendering
        results.forEach((result, index) => {
            if (result.status === 'rejected') {
                const names = ['loadContainers', 'loadHosts', 'loadVulnerabilitySummary'];
                console.error(`${names[index]} failed:`, result.reason);
            }
        });

        // Render all dashboard sections (await async ones to catch errors)
        renderDashboardMetrics();
        renderDashboardHostStatus();
        renderDashboardResourceStatus();
        await renderDashboardRecentActivity();
        await renderDashboardSecurity();
        await renderDashboardTelemetry();

        markRefresh();
    } catch (error) {
        console.error('Error loading dashboard:', error);
        showToast('Error', 'Failed to load dashboard data', 'error');
    }
}

function renderDashboardMetrics() {
    const safeHosts = hosts || [];
    const safeContainers = containers || [];

    // Total hosts
    const totalHosts = safeHosts.length;
    document.getElementById('dashTotalHosts').textContent = totalHosts;

    // Running containers
    const runningContainers = safeContainers.filter(c => c.state === 'running').length;
    document.getElementById('dashRunningContainers').textContent = runningContainers;

    // Total containers
    const totalContainers = safeContainers.length;
    document.getElementById('dashTotalContainers').textContent = totalContainers;
}

function renderDashboardHostStatus() {
    const container = document.getElementById('dashHostStatus');
    const safeHosts = hosts || [];

    if (safeHosts.length === 0) {
        container.innerHTML = '<p class="text-secondary">No hosts configured. <a href="#" onclick="switchTab(\'hosts\', true)">Add a host</a> to get started.</p>';
        return;
    }

    // Separate local host from agents
    const localHost = safeHosts.find(h => h.host_type === 'unix');
    const agents = safeHosts.filter(h => h.host_type === 'agent');

    // Agent status counts
    const onlineAgents = agents.filter(h => h.agent_status === 'online').length;
    const offlineAgents = agents.filter(h => h.agent_status === 'offline' || h.agent_status === 'auth_failed').length;
    const unknownAgents = agents.filter(h => h.agent_status === 'unknown').length;

    let html = '<div style="display: flex; flex-direction: column; gap: 1rem;">';

    // Local host status
    if (localHost) {
        const isOnline = localHost.enabled !== false; // Local host is online if enabled
        html += `
            <div>
                <div style="font-size: 0.75rem; font-weight: 600; color: var(--text-secondary); text-transform: uppercase; margin-bottom: 0.5rem;">Local Host</div>
                <div class="status-indicator ${isOnline ? 'online' : 'offline'}">
                    <span style="width: 8px; height: 8px; background: currentColor; border-radius: 50%; display: inline-block;"></span>
                    <span>${isOnline ? 'Online' : 'Offline'}</span>
                </div>
            </div>
        `;
    }

    // Agents status
    if (agents.length > 0 || localHost) {
        html += `<div style="border-top: 1px solid var(--border); padding-top: 0.5rem;">`;
    }

    html += `<div style="font-size: 0.75rem; font-weight: 600; color: var(--text-secondary); text-transform: uppercase; margin-bottom: 0.5rem;">Agents</div>`;

    if (agents.length === 0) {
        html += `<div class="text-secondary" style="font-size: 0.875rem;">0 agents configured</div>`;
    } else {
        // Show online agents
        if (onlineAgents > 0) {
            html += `
                <div class="status-indicator online">
                    <span style="width: 8px; height: 8px; background: currentColor; border-radius: 50%; display: inline-block;"></span>
                    <span>${onlineAgents} Online</span>
                </div>
            `;
        }

        // Show offline agents
        if (offlineAgents > 0) {
            html += `
                <div class="status-indicator offline">
                    <span style="width: 8px; height: 8px; background: currentColor; border-radius: 50%; display: inline-block;"></span>
                    <span>${offlineAgents} Offline</span>
                </div>
            `;
        }

        // Show unknown agents
        if (unknownAgents > 0) {
            html += `
                <div class="status-indicator warning">
                    <span style="width: 8px; height: 8px; background: currentColor; border-radius: 50%; display: inline-block;"></span>
                    <span>${unknownAgents} Unknown</span>
                </div>
            `;
        }

        // If all agents are online, show compact version
        if (offlineAgents === 0 && unknownAgents === 0 && onlineAgents === agents.length) {
            // Already showing the online count above
        }
    }

    if (agents.length > 0 || localHost) {
        html += `</div>`;
    }

    html += `
            <div style="margin-top: 0.5rem; padding-top: 0.5rem; border-top: 1px solid var(--border);">
                <button onclick="switchTab('hosts', true)" class="btn btn-sm btn-secondary" style="width: 100%;">
                    Manage Hosts
                </button>
            </div>
        </div>
    `;

    container.innerHTML = html;
}

async function renderDashboardSecurity() {
    const container = document.getElementById('dashSecurityContent');
    const toggle = document.getElementById('dashSecurityEnabled');

    try {
        // Load vulnerability settings to check if enabled
        const settingsResponse = await fetchWithAuth('/api/vulnerabilities/settings');
        if (!settingsResponse.ok) {
            console.error('Failed to load vulnerability settings:', settingsResponse.status);
            container.innerHTML = '<p class="text-secondary">Security scanning unavailable</p>';
            return;
        }
        const settings = await settingsResponse.json();

        const isEnabled = settings.enabled;
        toggle.checked = isEnabled;

        // Setup toggle event listener
        toggle.onchange = async (e) => {
            const newState = e.target.checked;
            await toggleSecurityScanning(newState);
        };

        if (!isEnabled) {
            container.innerHTML = `
                <div style="padding: 1rem; background: rgba(245, 158, 11, 0.1); border-radius: var(--radius); border: 1px solid var(--warning);">
                    <p style="color: var(--warning); font-weight: 600; margin-bottom: 0.5rem;">Security Scanning Disabled</p>
                    <p style="font-size: 0.875rem; color: var(--text-secondary);">Enable security scanning to detect vulnerabilities in your container images using Trivy.</p>
                </div>
            `;
            return;
        }

        // If enabled, show vulnerability summary
        if (!vulnerabilitySummary) {
            container.innerHTML = '<p class="text-secondary">Loading security data...</p>';
            return;
        }

        const severityCounts = vulnerabilitySummary.severity_counts || vulnerabilitySummary;
        const { critical = 0, high = 0, medium = 0, low = 0 } = severityCounts;
        const total = critical + high + medium + low;

        if (total === 0) {
            container.innerHTML = `
                <div style="text-align: center; padding: 1rem;">
                    <div style="font-size: 3rem; margin-bottom: 0.5rem;">✅</div>
                    <p style="color: var(--success); font-weight: 600;">No vulnerabilities detected</p>
                    <p class="text-secondary" style="font-size: 0.875rem; margin-top: 0.5rem;">All scanned images are clean</p>
                </div>
            `;
            return;
        }

        container.innerHTML = `
            <div style="display: flex; flex-direction: column; gap: 0.75rem;">
                ${critical > 0 ? `
                <div style="display: flex; justify-content: space-between; align-items: center;">
                    <span style="color: var(--danger); font-weight: 600;">Critical</span>
                    <span style="background: rgba(239, 68, 68, 0.1); color: var(--danger); padding: 0.25rem 0.75rem; border-radius: var(--radius); font-weight: 600;">${critical}</span>
                </div>
                ` : ''}
                ${high > 0 ? `
                <div style="display: flex; justify-content: space-between; align-items: center;">
                    <span style="color: var(--warning); font-weight: 600;">High</span>
                    <span style="background: rgba(245, 158, 11, 0.1); color: var(--warning); padding: 0.25rem 0.75rem; border-radius: var(--radius); font-weight: 600;">${high}</span>
                </div>
                ` : ''}
                ${medium > 0 ? `
                <div style="display: flex; justify-content: space-between; align-items: center;">
                    <span style="color: var(--info); font-weight: 600;">Medium</span>
                    <span style="background: rgba(59, 130, 246, 0.1); color: var(--info); padding: 0.25rem 0.75rem; border-radius: var(--radius); font-weight: 600;">${medium}</span>
                </div>
                ` : ''}
                <div style="margin-top: 0.5rem; padding-top: 0.5rem; border-top: 1px solid var(--border);">
                    <p style="font-size: 0.875rem; color: var(--text-secondary);">
                        <strong>${total}</strong> total vulnerabilities found across scanned images
                    </p>
                </div>
            </div>
        `;
    } catch (error) {
        console.error('Error loading security status:', error);
        container.innerHTML = `<p class="text-secondary" style="padding: 1rem; text-align: center;">Security status unavailable</p>`;
    }
}

async function toggleSecurityScanning(newState) {
    const container = document.getElementById('dashSecurityContent');
    const toggle = document.getElementById('dashSecurityEnabled');

    try {
        if (newState) {
            // Show loading message
            container.innerHTML = `
                <div style="padding: 1rem; background: rgba(59, 130, 246, 0.1); border-radius: var(--radius); border: 1px solid var(--info);">
                    <p style="color: var(--info); font-weight: 600; margin-bottom: 0.5rem;">🔄 Enabling Security Scanning...</p>
                    <p style="font-size: 0.875rem; color: var(--text-secondary);">Downloading Trivy vulnerability database. This may take a few minutes...</p>
                </div>
            `;

            // First, update the Trivy database
            const updateResponse = await fetchWithAuth('/api/vulnerabilities/update-db', {
                method: 'POST'
            });

            if (!updateResponse.ok) {
                throw new Error('Failed to update Trivy database');
            }

            showToast('Success', 'Trivy database updated successfully', 'success');

            // Then enable security scanning
            const settingsResponse = await fetchWithAuth('/api/vulnerabilities/settings');
            const settings = await settingsResponse.json();
            settings.enabled = true;

            const updateSettingsResponse = await fetchWithAuth('/api/vulnerabilities/settings', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(settings)
            });

            if (!updateSettingsResponse.ok) {
                throw new Error('Failed to enable security scanning');
            }

            showToast('Success', 'Security scanning enabled', 'success');

            // Reload vulnerability summary and refresh dashboard
            await loadVulnerabilitySummary();
            await renderDashboardSecurity();

        } else {
            // Disable security scanning
            const settingsResponse = await fetchWithAuth('/api/vulnerabilities/settings');
            const settings = await settingsResponse.json();
            settings.enabled = false;

            const updateSettingsResponse = await fetchWithAuth('/api/vulnerabilities/settings', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(settings)
            });

            if (!updateSettingsResponse.ok) {
                throw new Error('Failed to disable security scanning');
            }

            showToast('Success', 'Security scanning disabled', 'success');
            await renderDashboardSecurity();
        }

    } catch (error) {
        console.error('Error toggling security scanning:', error);
        showToast('Error', 'Failed to toggle security scanning: ' + error.message, 'error');

        // Revert toggle state
        toggle.checked = !newState;

        // Show error message
        container.innerHTML = `
            <div style="padding: 1rem; background: rgba(239, 68, 68, 0.1); border-radius: var(--radius); border: 1px solid var(--danger);">
                <p style="color: var(--danger); font-weight: 600; margin-bottom: 0.5rem;">❌ Error</p>
                <p style="font-size: 0.875rem; color: var(--text-secondary);">${escapeHtml(error.message)}</p>
            </div>
        `;
    }
}

function renderDashboardResourceStatus() {
    const container = document.getElementById('dashResourceStatus');

    const runningContainers = containers.filter(c => c.state === 'running');
    const containersWithStats = runningContainers.filter(c => c.memory_limit > 0);

    if (containersWithStats.length === 0) {
        container.innerHTML = '<p class="text-secondary">No resource stats available. Enable stats collection in host settings.</p>';
        return;
    }

    // Find top consumers
    const topCPU = [...containersWithStats].sort((a, b) => b.cpu_percent - a.cpu_percent).slice(0, 3);

    container.innerHTML = `
        <div style="display: flex; flex-direction: column; gap: 1rem;">
            <div>
                <h4 style="font-size: 0.875rem; font-weight: 600; margin-bottom: 0.5rem; color: var(--text-secondary);">Top CPU Usage</h4>
                ${topCPU.map(c => `
                    <div style="display: flex; justify-content: space-between; align-items: center; padding: 0.5rem 0;">
                        <span style="font-size: 0.875rem; color: var(--text-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 150px;">${escapeHtml(c.name)}</span>
                        <span style="font-weight: 600; color: ${c.cpu_percent > 80 ? 'var(--danger)' : c.cpu_percent > 50 ? 'var(--warning)' : 'var(--success)'};">${c.cpu_percent.toFixed(1)}%</span>
                    </div>
                `).join('')}
            </div>
            <div style="padding-top: 0.5rem; border-top: 1px solid var(--border);">
                <button onclick="switchTab('monitoring', true)" class="btn btn-sm btn-secondary" style="width: 100%;">
                    View All
                </button>
            </div>
        </div>
    `;
}

async function renderDashboardRecentActivity() {
    const container = document.getElementById('dashRecentActivity');

    try {
        // Fetch recent activity
        const response = await fetchWithAuth('/api/activity-log?limit=10');

        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }

        const data = await response.json();
        const activities = Array.isArray(data) ? data : [];

        if (activities.length === 0) {
            container.innerHTML = '<p class="text-secondary" style="padding: 1rem; text-align: center;">No recent activity</p>';
            return;
        }

        container.innerHTML = activities.map(activity => {
            const icon = activity.type === 'scan' ? '🔄' : '📡';
            const status = activity.success ? 'Success' : 'Failed';
            const statusColor = activity.success ? 'var(--success)' : 'var(--danger)';
            const timestamp = new Date(activity.timestamp).toLocaleString();

            return `
                <div style="display: flex; align-items: flex-start; gap: 1rem; padding: 0.75rem 0; border-bottom: 1px solid var(--border-light);">
                    <div style="font-size: 1.5rem;">${icon}</div>
                    <div style="flex: 1;">
                        <div style="font-weight: 600; color: var(--text-primary);">${escapeHtml(activity.type === 'scan' ? 'Scan' : 'Telemetry')}: ${escapeHtml(activity.target || 'All Hosts')}</div>
                        <div style="font-size: 0.8125rem; color: var(--text-secondary); margin-top: 0.25rem;">${timestamp}</div>
                    </div>
                    <div style="font-size: 0.875rem; font-weight: 600; color: ${statusColor};">${status}</div>
                </div>
            `;
        }).join('');
    } catch (error) {
        console.error('Error loading activity:', error);
        container.innerHTML = `<p class="text-secondary" style="padding: 1rem; text-align: center;">Activity log unavailable</p>`;
    }
}

async function renderDashboardTelemetry() {
    const container = document.getElementById('dashTelemetryContent');
    const toggle = document.getElementById('dashTelemetryEnabled');

    try {
        // Load telemetry endpoints
        const response = await fetchWithAuth('/api/telemetry/endpoints');
        if (!response.ok) {
            console.error('Failed to load telemetry endpoints:', response.status);
            container.innerHTML = '<p class="text-secondary">Telemetry status unavailable</p>';
            return;
        }
        const endpoints = await response.json();

        // Check if community endpoint is enabled
        const communityEndpoint = endpoints.find(e => e.name === 'community');
        const isEnabled = communityEndpoint && communityEndpoint.enabled;

        toggle.checked = isEnabled;

        if (!isEnabled) {
            container.innerHTML = `
                <div style="padding: 1rem; background: rgba(245, 158, 11, 0.1); border-radius: var(--radius); border: 1px solid var(--warning);">
                    <p style="color: var(--warning); font-weight: 600; margin-bottom: 0.5rem;">Telemetry Disabled</p>
                    <p style="font-size: 0.875rem; color: var(--text-secondary);">Enable telemetry to contribute anonymous usage statistics and help improve Container Census.</p>
                </div>
            `;
        } else {
            // Load telemetry schedule
            const schedResponse = await fetchWithAuth('/api/telemetry/schedule');
            const schedule = await schedResponse.json();

            container.innerHTML = `
                <div style="display: flex; flex-direction: column; gap: 0.75rem;">
                    <div style="display: flex; justify-content: space-between; align-items: center;">
                        <span style="color: var(--text-secondary);">Status</span>
                        <span class="status-indicator online">Active</span>
                    </div>
                    <div style="display: flex; justify-content: space-between; align-items: center;">
                        <span style="color: var(--text-secondary);">Next Submission</span>
                        <span style="font-weight: 600;">${schedule.next_submission ? new Date(schedule.next_submission).toLocaleString() : 'Unknown'}</span>
                    </div>
                    <div style="display: flex; justify-content: space-between; align-items: center;">
                        <span style="color: var(--text-secondary);">Frequency</span>
                        <span style="font-weight: 600;">${schedule.interval_hours ? schedule.interval_hours + 'h' : 'Unknown'}</span>
                    </div>
                </div>
            `;
        }

        // Add toggle event listener
        toggle.addEventListener('change', async (e) => {
            const newState = e.target.checked;
            await toggleTelemetry(newState);
        });

    } catch (error) {
        console.error('Error loading telemetry status:', error);
        container.innerHTML = '<p class="text-secondary">Failed to load telemetry status</p>';
    }
}

async function toggleTelemetry(newState) {
    const container = document.getElementById('dashTelemetryContent');
    const toggle = document.getElementById('dashTelemetryEnabled');

    try {
        // Show loading message
        container.innerHTML = `
            <div style="padding: 1rem; text-align: center;">
                <p style="color: var(--text-secondary);">${newState ? 'Enabling' : 'Disabling'} telemetry...</p>
            </div>
        `;

        // Load telemetry endpoints
        const response = await fetchWithAuth('/api/telemetry/endpoints');
        if (!response.ok) {
            throw new Error('Failed to load telemetry endpoints');
        }
        const endpoints = await response.json();

        // Find community endpoint
        const communityEndpoint = endpoints.find(e => e.name === 'community');
        if (!communityEndpoint) {
            throw new Error('Community endpoint not found');
        }

        // Update the endpoint
        const updateResponse = await fetchWithAuth(`/api/telemetry/endpoints/${encodeURIComponent(communityEndpoint.name)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                enabled: newState
            })
        });

        if (!updateResponse.ok) {
            const error = await updateResponse.json();
            throw new Error(error.error || 'Failed to update telemetry');
        }

        showToast('Success', `Telemetry ${newState ? 'enabled' : 'disabled'}`, 'success');

        // Reload the telemetry section
        await renderDashboardTelemetry();

    } catch (error) {
        console.error('Error toggling telemetry:', error);
        showToast('Error', 'Failed to toggle telemetry: ' + error.message, 'error');
        // Revert toggle
        toggle.checked = !newState;
        container.innerHTML = '<p class="text-secondary">Failed to update telemetry</p>';
    }
}

async function loadVulnerabilitySummary() {
    try {
        // First check if security scanning is enabled
        const settingsResponse = await fetchWithAuth('/api/vulnerabilities/settings');
        const settings = await settingsResponse.json();

        if (!settings.enabled) {
            // Security scanning is disabled
            vulnerabilitySummary = null;

            // Update sidebar stats to show N/A or hide
            const criticalVulnsEl = document.getElementById('criticalVulns');
            if (criticalVulnsEl) {
                criticalVulnsEl.textContent = '-';
                criticalVulnsEl.style.fontWeight = '600';
            }
            return null;
        }

        // Load summary if enabled
        const response = await fetchWithAuth('/api/vulnerabilities/summary');
        const data = await response.json();

        // Extract summary from response (might be nested in 'summary' field)
        vulnerabilitySummary = data.summary || data;

        // Update sidebar stats
        const criticalVulnsEl = document.getElementById('criticalVulns');
        if (criticalVulnsEl) {
            const criticalCount = vulnerabilitySummary.severity_counts?.critical || 0;
            criticalVulnsEl.textContent = criticalCount;
            criticalVulnsEl.style.fontWeight = criticalCount > 0 ? '700' : '600';
        }

        // Return the full data object (includes summary and queue_status)
        return data;
    } catch (error) {
        console.error('Error loading vulnerability summary:', error);
        vulnerabilitySummary = { severity_counts: { critical: 0, high: 0, medium: 0, low: 0 } };
        return vulnerabilitySummary;
    }
}

// ======= IMPORT/EXPORT FUNCTIONS =======

async function exportSettings() {
    try {
        const response = await fetchWithAuth('/api/settings/export');

        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }

        // Get the YAML content
        const yamlContent = await response.text();

        // Create a blob and download it
        const blob = new Blob([yamlContent], { type: 'application/x-yaml' });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = 'container-census-config.yaml';
        document.body.appendChild(a);
        a.click();
        window.URL.revokeObjectURL(url);
        document.body.removeChild(a);

        showToast('Success', 'Settings exported successfully', 'success');
    } catch (error) {
        console.error('Error exporting settings:', error);
        showToast('Error', 'Failed to export settings: ' + error.message, 'error');
    }
}

async function handleImportFile(event) {
    const file = event.target.files[0];
    if (!file) return;

    const statusEl = document.getElementById('importStatus');
    statusEl.textContent = '⏳ Importing...';
    statusEl.className = 'save-status-inline';

    try {
        const formData = new FormData();
        formData.append('file', file);

        const response = await fetchWithAuth('/api/settings/import', {
            method: 'POST',
            body: formData
        });

        const result = await response.json();

        if (response.ok) {
            statusEl.textContent = '✓ Import successful';
            statusEl.className = 'save-status-inline success';
            showToast('Success', 'Settings imported successfully. Reloading...', 'success');

            // Reload the page after 2 seconds to apply new settings
            setTimeout(() => {
                window.location.reload();
            }, 2000);
        } else {
            statusEl.textContent = '✗ Import failed';
            statusEl.className = 'save-status-inline error';
            showToast('Error', 'Failed to import settings: ' + (result.error || 'Unknown error'), 'error');
        }
    } catch (error) {
        console.error('Error importing settings:', error);
        statusEl.textContent = '✗ Error';
        statusEl.className = 'save-status-inline error';
        showToast('Error', 'Error importing settings: ' + error.message, 'error');
    }

    // Clear the file input
    event.target.value = '';

    // Clear status after 5 seconds
    setTimeout(() => {
        statusEl.textContent = '';
        statusEl.className = 'save-status-inline';
    }, 5000);
}

// ======= DANGER ZONE FUNCTIONS =======

async function resetAllSettings() {
    if (!confirm('⚠️ Are you sure you want to reset ALL settings to defaults?\n\nThis will:\n- Delete all system settings\n- Delete all telemetry endpoints\n- Trigger auto-import from config.yaml if available\n\nThis action cannot be undone.')) {
        return;
    }

    try {
        const response = await fetch('/api/settings/reset', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            }
        });

        const result = await response.json();

        if (response.ok) {
            showNotification(result.message, 'success');
            // Reload the page after 2 seconds to trigger auto-import
            setTimeout(() => {
                window.location.reload();
            }, 2000);
        } else {
            showNotification('Failed to reset settings: ' + (result.error || result.message), 'error');
        }
    } catch (error) {
        console.error('Error resetting settings:', error);
        showNotification('Error resetting settings: ' + error.message, 'error');
    }
}

async function clearContainerHistory() {
    if (!confirm('⚠️ Are you sure you want to clear container history?\n\nThis will:\n- Delete all historical container scan data\n- Keep only the most recent snapshot\n- Clear historical charts and trends\n\nThis action cannot be undone.')) {
        return;
    }

    try {
        const response = await fetch('/api/settings/clear-history', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            }
        });

        const result = await response.json();

        if (response.ok) {
            showNotification(result.message, 'success');
        } else {
            showNotification('Failed to clear history: ' + (result.error || result.message), 'error');
        }
    } catch (error) {
        console.error('Error clearing history:', error);
        showNotification('Error clearing history: ' + error.message, 'error');
    }
}

async function clearVulnerabilities() {
    if (!confirm('⚠️ Are you sure you want to clear all vulnerability data?\n\nThis will:\n- Delete all vulnerability scan results\n- Delete all CVE data\n- Images will be rescanned on next scheduled scan\n\nThis action cannot be undone.')) {
        return;
    }

    try {
        const response = await fetch('/api/settings/clear-vulnerabilities', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            }
        });

        const result = await response.json();

        if (response.ok) {
            showNotification(result.message, 'success');
            // Reload vulnerability tab if currently viewing it
            const currentTab = document.querySelector('.tab-content.active')?.id;
            if (currentTab === 'vulnerabilityTab') {
                loadVulnerabilitySummary();
            }
        } else {
            showNotification('Failed to clear vulnerabilities: ' + (result.error || result.message), 'error');
        }
    } catch (error) {
        console.error('Error clearing vulnerabilities:', error);
        showNotification('Error clearing vulnerabilities: ' + error.message, 'error');
    }
}

async function clearActivityLog() {
    if (!confirm('⚠️ Are you sure you want to clear the activity log?\n\nThis will:\n- Delete all lifecycle events\n- Delete all container state change history\n- Clear the activity tab\n\nNew events will be logged as they occur. This action cannot be undone.')) {
        return;
    }

    try {
        const response = await fetch('/api/settings/clear-activity', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            }
        });

        const result = await response.json();

        if (response.ok) {
            showNotification(result.message, 'success');
            // Reload activity tab if currently viewing it
            const currentTab = document.querySelector('.tab-content.active')?.id;
            if (currentTab === 'activity') {
                loadActivityLog();
            }
        } else {
            showNotification('Failed to clear activity log: ' + (result.error || result.message), 'error');
        }
    } catch (error) {
        console.error('Error clearing activity log:', error);
        showNotification('Error clearing activity log: ' + error.message, 'error');
    }
}

async function nuclearReset() {
    // First confirmation
    if (!confirm('💀 NUCLEAR OPTION: DELETE EVERYTHING 💀\n\nThis will permanently delete:\n- ALL settings\n- ALL container history\n- ALL vulnerability scans\n- ALL activity logs\n- ALL hosts\n- ALL telemetry endpoints\n- ALL notifications\n\nYour database will be completely reset to a fresh installation state.\n\nAre you ABSOLUTELY SURE?')) {
        return;
    }

    // Second confirmation to prevent accidents
    const confirmation = prompt('Type "DELETE EVERYTHING" (in all caps) to confirm nuclear reset:');
    if (confirmation !== 'DELETE EVERYTHING') {
        showNotification('Nuclear reset cancelled', 'info');
        return;
    }

    try {
        const response = await fetch('/api/settings/nuclear-reset', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            }
        });

        const result = await response.json();

        if (response.ok) {
            showNotification('💀 Nuclear reset complete. Reloading in 3 seconds...', 'success');
            // Reload the page after 3 seconds
            setTimeout(() => {
                window.location.reload();
            }, 3000);
        } else {
            showNotification('Failed to perform nuclear reset: ' + (result.error || result.message), 'error');
        }
    } catch (error) {
        console.error('Error performing nuclear reset:', error);
        showNotification('Error performing nuclear reset: ' + error.message, 'error');
    }
}

// ===== Image Update Functions =====

// Check if a single container has an image update available
async function checkContainerUpdate(hostId, containerId, containerName) {
    try {
        const response = await fetch(`/api/containers/${hostId}/${containerId}/check-update`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            }
        });

        const result = await response.json();

        if (response.ok) {
            if (result.available) {
                showNotification(`Update available for ${containerName}`, 'success');
            } else if (result.message) {
                showNotification(result.message, 'info');
            } else {
                showNotification(`${containerName} is up to date`, 'info');
            }

            // Reload containers to update UI badges
            await loadData();
        } else {
            showNotification('Failed to check for update: ' + (result.error || 'Unknown error'), 'error');
        }
    } catch (error) {
        console.error('Error checking for update:', error);
        showNotification('Error checking for update: ' + error.message, 'error');
    }
}

// Check all :latest containers for updates
async function checkAllUpdates() {
    // Get all containers with :latest tag
    const latestContainers = containers.filter(c =>
        c.image.endsWith(':latest') || (!c.image.includes(':') && c.state === 'running')
    );

    if (latestContainers.length === 0) {
        showNotification('No containers with :latest tag found', 'info');
        return;
    }

    showNotification(`Checking ${latestContainers.length} container(s) for updates...`, 'info');

    const containerList = latestContainers.map(c => ({
        host_id: c.host_id,
        container_id: c.id
    }));

    try {
        const response = await fetch('/api/containers/bulk-check-updates', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ containers: containerList })
        });

        const results = await response.json();

        if (response.ok) {
            let availableCount = 0;
            for (const key in results) {
                if (results[key].available) {
                    availableCount++;
                }
            }

            if (availableCount > 0) {
                showNotification(`${availableCount} update(s) available`, 'success');
            } else {
                showNotification('All containers are up to date', 'info');
            }

            // Reload containers to update UI badges
            await loadData();
        } else {
            showNotification('Failed to check for updates', 'error');
        }
    } catch (error) {
        console.error('Error checking for updates:', error);
        showNotification('Error checking for updates: ' + error.message, 'error');
    }
}

// Update a single container (pull new image and recreate)
async function updateContainer(hostId, containerId, containerName, imageName) {
    // Show confirmation dialog with dry-run preview
    showConfirmDialog(
        'Update Container',
        `
        <div style="text-align: left;">
            <p><strong>Container:</strong> ${escapeHtml(containerName)}</p>
            <p><strong>Image:</strong> ${escapeHtml(imageName)}</p>
            <p style="margin-top: 15px;">This will:</p>
            <ul style="margin: 10px 0;">
                <li>Pull the latest <code>${escapeHtml(imageName)}</code> image</li>
                <li>Stop and remove the current container</li>
                <li>Create a new container with the same configuration</li>
                <li>Start the new container</li>
            </ul>
            <p style="margin-top: 15px; padding: 10px; background-color: #fff3cd; border-radius: 4px;">
                ⚠️ <strong>Note:</strong> The old image will be kept for rollback. Container configuration (env vars, volumes, ports, networks) will be preserved.
            </p>
            <p style="margin-top: 10px; color: #856404;">
                Non-volume data will be lost. Ensure important data is in volumes!
            </p>
        </div>
        `,
        async () => {
            // Show progress modal
            showProgressModal('Updating Container', 'Pulling new image...');

            try {
                // First, do a dry-run to preview
                updateProgressModal('Validating container configuration...');
                const dryRunResponse = await fetch(`/api/containers/${hostId}/${containerId}/update?dry_run=true`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    }
                });

                const dryRunResult = await dryRunResponse.json();

                if (!dryRunResponse.ok) {
                    hideProgressModal();
                    showNotification('Dry-run failed: ' + (dryRunResult.error || 'Unknown error'), 'error');
                    return;
                }

                // Now perform the actual update
                updateProgressModal(`Pulling latest ${imageName} image...`);

                const response = await fetch(`/api/containers/${hostId}/${containerId}/update`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    }
                });

                const result = await response.json();

                if (response.ok && result.success) {
                    updateProgressModal('Container updated! Refreshing data...');

                    // Immediate refresh to get new container data
                    await loadData();

                    hideProgressModal();
                    showNotification(`Container ${containerName} updated successfully! New ID: ${result.new_container_id?.substring(0, 12)}`, 'success');
                } else {
                    hideProgressModal();
                    showNotification('Failed to update container: ' + (result.error || 'Unknown error'), 'error');
                }
            } catch (error) {
                hideProgressModal();
                console.error('Error updating container:', error);
                showNotification('Error updating container: ' + error.message, 'error');
            }
        },
        'warning'
    );
}

// Load image update settings
async function loadImageUpdateSettings() {
    try {
        const response = await fetch('/api/image-updates/settings');
        const settings = await response.json();

        if (response.ok) {
            // Populate settings form
            document.getElementById('autoCheckEnabled').checked = settings.auto_check_enabled;
            document.getElementById('checkIntervalHours').value = settings.check_interval_hours;
            document.getElementById('onlyCheckLatestTags').checked = settings.only_check_latest_tags;
        }
    } catch (error) {
        console.error('Error loading image update settings:', error);
    }
}

// Save image update settings
async function saveImageUpdateSettings() {
    const settings = {
        auto_check_enabled: document.getElementById('autoCheckEnabled').checked,
        check_interval_hours: parseInt(document.getElementById('checkIntervalHours').value),
        only_check_latest_tags: document.getElementById('onlyCheckLatestTags').checked
    };

    const statusEl = document.getElementById('imageUpdateSaveStatus');

    try {
        const response = await fetch('/api/image-updates/settings', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(settings)
        });

        const result = await response.json();

        if (response.ok) {
            statusEl.textContent = '✓ Settings saved successfully';
            statusEl.style.color = 'green';
            setTimeout(() => { statusEl.textContent = ''; }, 3000);
        } else {
            statusEl.textContent = '✗ Failed to save: ' + (result.error || 'Unknown error');
            statusEl.style.color = 'red';
        }
    } catch (error) {
        console.error('Error saving settings:', error);
        statusEl.textContent = '✗ Error: ' + error.message;
        statusEl.style.color = 'red';
    }
}

// Show progress modal
function showProgressModal(title, message) {
    const modal = document.getElementById('progressModal');
    if (!modal) {
        // Create modal if it doesn't exist
        const modalHtml = `
            <div id="progressModal" class="modal">
                <div class="modal-content">
                    <h2 id="progressTitle">Progress</h2>
                    <p id="progressMessage">Please wait...</p>
                    <div class="progress-bar">
                        <div class="progress-bar-fill"></div>
                    </div>
                </div>
            </div>
        `;
        document.body.insertAdjacentHTML('beforeend', modalHtml);
    }

    document.getElementById('progressTitle').textContent = title;
    document.getElementById('progressMessage').textContent = message;
    document.getElementById('progressModal').classList.add('show');
}

// Update progress modal message
function updateProgressModal(message) {
    const messageEl = document.getElementById('progressMessage');
    if (messageEl) {
        messageEl.textContent = message;
    }
}

// Hide progress modal
function hideProgressModal() {
    const modal = document.getElementById('progressModal');
    if (modal) {
        modal.classList.remove('show');
    }
}
