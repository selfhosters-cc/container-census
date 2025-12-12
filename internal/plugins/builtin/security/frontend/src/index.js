// Security Plugin Frontend
// This file contains all security/vulnerability scanning functionality

import './styles.css';

// Global state
let vulnerabilityCache = {};
let vulnerabilityScansMap = {};
let vulnerabilityChartInstance = null;
let trendsChart = null;
let securityRefreshInterval = null;

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
        const response = await fetch('/api/p/security/scans?limit=1000');
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

    return `<span class="vulnerability-badge ${badgeClass}" title="${title}" onclick="window.viewVulnerabilityDetails('${scan.scan.image_id}')">${badgeText}</span>`;
}

// View vulnerability details (navigate to Security tab)
window.viewVulnerabilityDetails = function(imageID) {
    // Switch to security tab
    window.switchTab('security');

    // Wait for tab to load, then filter/scroll to image
    setTimeout(() => {
        const searchInput = document.getElementById('securitySearch');
        if (searchInput) {
            searchInput.value = imageID;
            searchInput.dispatchEvent(new Event('input'));
        }
    }, 100);
};

// Load and render vulnerability summary
async function loadVulnerabilitySummary() {
    try {
        const response = await fetch('/api/p/security/summary');
        if (!response.ok) {
            if (response.status === 404) {
                // Security plugin not available
                document.getElementById('securityTab').innerHTML = '<div class="no-data">Vulnerability scanning not available</div>';
                return;
            }
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const data = await response.json();
        renderVulnerabilitySummary(data.summary, data.queue_status);

        // Load scans table
        await loadVulnerabilityScans();
    } catch (error) {
        console.error('Error loading vulnerability summary:', error);
        document.getElementById('securityTab').innerHTML = '<div class="no-data">Error loading vulnerability data</div>';
    }
}

// Render vulnerability summary cards and chart
function renderVulnerabilitySummary(summary, queueStatus) {
    // Update summary cards by their IDs (matching the injected HTML structure)
    const { total_images_scanned, images_with_vulnerabilities, severity_counts } = summary;
    const counts = severity_counts || {};

    // Update card values
    const totalScannedEl = document.getElementById('totalScannedImages');
    const totalCriticalEl = document.getElementById('totalCriticalVulns');
    const totalHighEl = document.getElementById('totalHighVulns');
    const atRiskEl = document.getElementById('atRiskImages');

    if (totalScannedEl) totalScannedEl.textContent = total_images_scanned || 0;
    if (totalCriticalEl) totalCriticalEl.textContent = counts.critical || 0;
    if (totalHighEl) totalHighEl.textContent = counts.high || 0;
    if (atRiskEl) atRiskEl.textContent = images_with_vulnerabilities || 0;

    // Update queue status banner
    const queueStatusBanner = document.getElementById('securityQueueStatus');
    const queueStatusText = document.getElementById('queueStatusText');

    if (queueStatusBanner && queueStatusText) {
        if (queueStatus && (queueStatus.in_progress > 0 || queueStatus.pending > 0)) {
            queueStatusBanner.style.display = 'flex';
            queueStatusText.textContent = `${queueStatus.in_progress} active, ${queueStatus.pending} pending (${queueStatus.completed} completed, ${queueStatus.failed} failed)`;
        } else {
            queueStatusBanner.style.display = 'none';
        }
    }

    // Render charts
    renderVulnerabilityChart(counts);
}

// Render vulnerability severity chart
function renderVulnerabilityChart(counts) {
    const canvas = document.getElementById('vulnerabilitySeverityChart');
    if (!canvas) return;

    const ctx = canvas.getContext('2d');

    // Destroy existing chart if it exists
    if (vulnerabilityChartInstance) {
        vulnerabilityChartInstance.destroy();
    }

    const data = {
        labels: ['Critical', 'High', 'Medium', 'Low'],
        datasets: [{
            data: [
                counts.critical || 0,
                counts.high || 0,
                counts.medium || 0,
                counts.low || 0
            ],
            backgroundColor: [
                'rgba(255, 82, 82, 0.8)',
                'rgba(255, 171, 0, 0.8)',
                'rgba(255, 235, 59, 0.8)',
                'rgba(76, 175, 80, 0.8)'
            ],
            borderColor: [
                'rgba(255, 82, 82, 1)',
                'rgba(255, 171, 0, 1)',
                'rgba(255, 235, 59, 1)',
                'rgba(76, 175, 80, 1)'
            ],
            borderWidth: 1
        }]
    };

    vulnerabilityChartInstance = new Chart(ctx, {
        type: 'doughnut',
        data: data,
        options: {
            responsive: true,
            maintainAspectRatio: true,
            plugins: {
                legend: {
                    display: true,
                    position: 'bottom'
                },
                title: {
                    display: true,
                    text: 'Vulnerability Severity Distribution'
                }
            }
        }
    });
}

// Render vulnerability trends chart (last 30 days)
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
                            color: 'var(--text-secondary)',
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
                            color: 'var(--text-tertiary)',
                            font: { size: 11 }
                        }
                    },
                    y: {
                        beginAtZero: true,
                        grid: {
                            color: 'rgba(100, 100, 100, 0.1)'
                        },
                        ticks: {
                            color: 'var(--text-tertiary)',
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

// Load and render vulnerability scans table
async function loadVulnerabilityScans() {
    try {
        const response = await fetch('/api/p/security/scans?limit=1000');
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);

        const scans = await response.json();
        renderVulnerabilityScans(scans);
        renderVulnerabilityTrendsChart(scans);
    } catch (error) {
        console.error('Error loading vulnerability scans:', error);
    }
}

// Store scans globally for filtering
let allVulnerabilityScans = [];

// Render vulnerability scans table
function renderVulnerabilityScans(scans) {
    const tbody = document.getElementById('securityScansBody');
    if (!tbody) return;

    // Deduplicate by image_id (keep most recent scan)
    const uniqueScans = [];
    const seenImages = new Set();

    // Sort by scanned_at descending to get most recent first
    const sortedScans = [...(scans || [])].sort((a, b) => {
        const dateA = new Date(a.scanned_at);
        const dateB = new Date(b.scanned_at);
        return dateB - dateA;
    });

    for (const scan of sortedScans) {
        if (!seenImages.has(scan.image_id)) {
            seenImages.add(scan.image_id);
            uniqueScans.push(scan);
        }
    }

    // Store scans globally for filtering
    allVulnerabilityScans = uniqueScans;

    // Populate host filter dropdown
    populateHostFilter(uniqueScans);

    // Update scan count badge
    const scanCountBadge = document.getElementById('scanCountBadge');
    if (scanCountBadge) {
        scanCountBadge.textContent = `${uniqueScans.length} images`;
    }

    if (!uniqueScans || uniqueScans.length === 0) {
        tbody.innerHTML = `<tr><td colspan="9" class="no-data">No vulnerability scans found</td></tr>`;
        return;
    }

    tbody.innerHTML = uniqueScans.map(scan => {
        const counts = scan.severity_counts || {};
        const total = scan.total_vulnerabilities || 0;
        const scannedAt = new Date(scan.scanned_at).toLocaleString();

        let statusBadge = '';
        let severityClass = '';
        if (!scan.success) {
            statusBadge = '❌ Failed';
            severityClass = 'failed';
        } else if (total === 0) {
            statusBadge = '✅ Clean';
            severityClass = 'clean';
        } else if (counts.critical > 0) {
            statusBadge = '🚨 Critical';
            severityClass = 'critical';
        } else if (counts.high > 0) {
            statusBadge = '⚠️ High';
            severityClass = 'high';
        } else if (counts.medium > 0) {
            statusBadge = '📊 Issues';
            severityClass = 'medium';
        } else {
            statusBadge = '📊 Issues';
            severityClass = 'low';
        }

        // Prepare host IDs and names for data attributes
        const hostIds = (scan.host_ids || []).join(',');
        const hostNames = (scan.host_names || []).join(',');

        // Format host names for display (shown below image name)
        const hostNamesDisplay = (scan.host_names && scan.host_names.length > 0)
            ? `<div style="font-size: 0.85em; color: var(--text-secondary, #666); margin-top: 4px;">
                 ${scan.host_names.join(', ')}
               </div>`
            : '';

        return `
            <tr data-image-id="${scan.image_id}"
                data-host-ids="${hostIds}"
                data-host-names="${hostNames}"
                data-severity="${severityClass}"
                data-image-name="${(scan.image_name || scan.image_id).toLowerCase()}"
                style="cursor: pointer;"
                onclick="window.viewScanDetails('${scan.image_id}')">
                <td title="${scan.image_name || scan.image_id}">
                    <div>${scan.image_name || scan.image_id}</div>
                    ${hostNamesDisplay}
                </td>
                <td>${statusBadge}</td>
                <td>${total}</td>
                <td>${counts.critical || 0}</td>
                <td>${counts.high || 0}</td>
                <td>${counts.medium || 0}</td>
                <td>${counts.low || 0}</td>
                <td>${scannedAt}</td>
                <td>
                    <button class="btn btn-secondary" onclick="event.stopPropagation(); window.viewScanDetails('${scan.image_id}')">View</button>
                    <button class="btn btn-primary" onclick="event.stopPropagation(); rescanImage('${scan.image_id}', '${scan.image_name || scan.image_id}')">Rescan</button>
                </td>
            </tr>
        `;
    }).join('');

    // Setup filter event listeners (only once)
    setupFilterListeners();
}

// Populate host filter dropdown
function populateHostFilter(scans) {
    const hostFilter = document.getElementById('securityHostFilter');
    if (!hostFilter) return;

    // Collect all unique hosts
    const hostsMap = new Map();
    scans.forEach(scan => {
        if (scan.host_ids && scan.host_names) {
            scan.host_ids.forEach((hostId, index) => {
                const hostName = scan.host_names[index] || `Host ${hostId}`;
                hostsMap.set(hostId, hostName);
            });
        }
    });

    // Sort hosts by name
    const hosts = Array.from(hostsMap.entries()).sort((a, b) => a[1].localeCompare(b[1]));

    // Preserve current selection
    const currentValue = hostFilter.value;

    // Rebuild dropdown
    hostFilter.innerHTML = '<option value="">All Hosts</option>' +
        hosts.map(([id, name]) => `<option value="${id}">${name}</option>`).join('');

    // Restore selection if it still exists
    if (currentValue && Array.from(hostFilter.options).some(opt => opt.value === currentValue)) {
        hostFilter.value = currentValue;
    }
}

// Setup filter event listeners
let filtersSetup = false;
function setupFilterListeners() {
    if (filtersSetup) return;
    filtersSetup = true;

    const hostFilter = document.getElementById('securityHostFilter');
    const severityFilter = document.getElementById('securitySeverityFilter');
    const statusFilter = document.getElementById('securityStatusFilter');
    const searchInput = document.getElementById('securitySearchInput');

    if (hostFilter) hostFilter.addEventListener('change', applyFilters);
    if (severityFilter) severityFilter.addEventListener('change', applyFilters);
    if (statusFilter) statusFilter.addEventListener('change', applyFilters);
    if (searchInput) searchInput.addEventListener('input', applyFilters);
}

// Apply all filters
function applyFilters() {
    const hostFilter = document.getElementById('securityHostFilter')?.value || '';
    const severityFilter = document.getElementById('securitySeverityFilter')?.value || '';
    const statusFilter = document.getElementById('securityStatusFilter')?.value || '';
    const searchText = document.getElementById('securitySearchInput')?.value.toLowerCase() || '';

    const tbody = document.getElementById('securityScansBody');
    if (!tbody) return;

    const rows = tbody.querySelectorAll('tr[data-image-id]');
    let visibleCount = 0;

    rows.forEach(row => {
        let show = true;

        // Host filter
        if (hostFilter) {
            const hostIds = row.getAttribute('data-host-ids') || '';
            if (!hostIds.split(',').includes(hostFilter)) {
                show = false;
            }
        }

        // Severity filter
        if (severityFilter && show) {
            const severity = row.getAttribute('data-severity') || '';
            if (severity !== severityFilter) {
                show = false;
            }
        }

        // Status filter
        if (statusFilter && show) {
            const severity = row.getAttribute('data-severity') || '';
            if (statusFilter === 'scanned' && (severity === 'failed' || severity === '')) {
                show = false;
            } else if (statusFilter === 'failed' && severity !== 'failed') {
                show = false;
            } else if (statusFilter === 'remote') {
                // This filter doesn't apply to the table (remote scans are already filtered out)
                show = false;
            }
        }

        // Search filter
        if (searchText && show) {
            const imageName = row.getAttribute('data-image-name') || '';
            if (!imageName.includes(searchText)) {
                show = false;
            }
        }

        row.style.display = show ? '' : 'none';
        if (show) visibleCount++;
    });

    // Update count badge
    const scanCountBadge = document.getElementById('scanCountBadge');
    if (scanCountBadge) {
        scanCountBadge.textContent = `${visibleCount} images`;
    }
}

// View detailed scan results
window.viewScanDetails = async function(imageID) {
    try {
        const response = await fetch(`/api/p/security/image/${imageID}`);
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);

        const data = await response.json();
        showVulnerabilityModal(data);
    } catch (error) {
        console.error('Error loading scan details:', error);
        alert('Error loading vulnerability details');
    }
};

// Show vulnerability details modal
function showVulnerabilityModal(data) {
    const { scan, vulnerabilities } = data;

    const modal = document.createElement('div');
    modal.className = 'modal-overlay';
    modal.innerHTML = `
        <div class="modal-content vulnerability-modal">
            <div class="modal-header">
                <h3>Vulnerability Scan: ${scan.image_name}</h3>
                <button onclick="this.closest('.modal-overlay').remove()" class="modal-close">×</button>
            </div>
            <div class="modal-body">
                <div class="scan-metadata">
                    <p><strong>Image ID:</strong> ${scan.image_id}</p>
                    <p><strong>Scanned:</strong> ${new Date(scan.scanned_at).toLocaleString()}</p>
                    <p><strong>Total Vulnerabilities:</strong> ${scan.total_vulnerabilities}</p>
                    <p><strong>Severity Breakdown:</strong>
                        ${scan.severity_counts.critical || 0} Critical,
                        ${scan.severity_counts.high || 0} High,
                        ${scan.severity_counts.medium || 0} Medium,
                        ${scan.severity_counts.low || 0} Low
                    </p>
                </div>

                <div class="vulnerabilities-list">
                    <h4>Vulnerabilities</h4>
                    ${vulnerabilities && vulnerabilities.length > 0 ? `
                        <div class="vulnerability-filter">
                            <input type="text" id="vulnSearch" placeholder="Filter by CVE, package, or description..."
                                   oninput="filterVulnerabilities(this.value)">
                            <select id="vulnSeverityFilter" onchange="filterVulnerabilities()">
                                <option value="">All Severities</option>
                                <option value="CRITICAL">Critical</option>
                                <option value="HIGH">High</option>
                                <option value="MEDIUM">Medium</option>
                                <option value="LOW">Low</option>
                            </select>
                        </div>
                        <table class="vulnerability-table" id="vulnerabilityDetailsTable">
                            <thead>
                                <tr>
                                    <th>CVE</th>
                                    <th>Package</th>
                                    <th>Severity</th>
                                    <th>Installed</th>
                                    <th>Fixed</th>
                                    <th>Description</th>
                                </tr>
                            </thead>
                            <tbody>
                                ${vulnerabilities.map(v => `
                                    <tr data-severity="${v.severity}" data-cve="${v.vulnerability_id}" data-pkg="${v.pkg_name}" data-desc="${v.description || ''}">
                                        <td><a href="https://nvd.nist.gov/vuln/detail/${v.vulnerability_id}" target="_blank">${v.vulnerability_id}</a></td>
                                        <td>${v.pkg_name}</td>
                                        <td><span class="severity-badge ${v.severity.toLowerCase()}">${v.severity}</span></td>
                                        <td>${v.installed_version}</td>
                                        <td>${v.fixed_version || 'N/A'}</td>
                                        <td class="vuln-description">${v.description || v.title || 'No description'}</td>
                                    </tr>
                                `).join('')}
                            </tbody>
                        </table>
                    ` : '<p class="no-data">No vulnerabilities found</p>'}
                </div>
            </div>
            <div class="modal-footer">
                <button onclick="rescanImage('${scan.image_id}')" class="btn">Rescan</button>
                <button onclick="this.closest('.modal-overlay').remove()" class="btn btn-secondary">Close</button>
            </div>
        </div>
    `;

    document.body.appendChild(modal);
}

// Filter vulnerabilities in modal
window.filterVulnerabilities = function() {
    const searchValue = document.getElementById('vulnSearch')?.value.toLowerCase() || '';
    const severityFilter = document.getElementById('vulnSeverityFilter')?.value || '';
    const rows = document.querySelectorAll('#vulnerabilityDetailsTable tbody tr');

    rows.forEach(row => {
        const cve = row.getAttribute('data-cve').toLowerCase();
        const pkg = row.getAttribute('data-pkg').toLowerCase();
        const desc = row.getAttribute('data-desc').toLowerCase();
        const severity = row.getAttribute('data-severity');

        const matchesSearch = !searchValue || cve.includes(searchValue) || pkg.includes(searchValue) || desc.includes(searchValue);
        const matchesSeverity = !severityFilter || severity === severityFilter;

        row.style.display = (matchesSearch && matchesSeverity) ? '' : 'none';
    });
};

// Trigger image rescan
window.rescanImage = async function(imageID) {
    try {
        const response = await fetch(`/api/p/security/scan/${imageID}`, { method: 'POST' });
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);

        alert('Scan queued successfully. Results will be available in about 30 seconds.');

        // Close modal and refresh
        document.querySelector('.modal-overlay')?.remove();
        setTimeout(() => {
            loadVulnerabilitySummary();
        }, 2000);
    } catch (error) {
        console.error('Error triggering scan:', error);
        alert('Error queueing scan');
    }
};

// Trigger scan all images
window.scanAllImages = async function() {
    if (!confirm('This will queue all known images for rescanning. Continue?')) return;

    try {
        const response = await fetch('/api/p/security/scan-all', { method: 'POST' });
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);

        const result = await response.json();

        // Build detailed message with per-host breakdown
        let message = `${result.total_queued} images queued for scanning`;
        if (result.queued_by_host && Object.keys(result.queued_by_host).length > 0) {
            const breakdown = Object.entries(result.queued_by_host)
                .map(([host, count]) => `${host}: ${count}`)
                .join(', ');
            message += `\n\n${breakdown}`;
        }
        alert(message);

        setTimeout(() => {
            loadVulnerabilitySummary();
        }, 2000);
    } catch (error) {
        console.error('Error triggering scan all:', error);
        alert('Error queueing scans');
    }
};

// Update Trivy database
window.updateTrivyDB = async function() {
    if (!confirm('This will update the Trivy vulnerability database. This may take a few minutes. Continue?')) return;

    const btn = event.target;
    btn.disabled = true;
    btn.textContent = 'Updating...';

    try {
        const response = await fetch('/api/p/security/update-db', { method: 'POST' });
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);

        alert('Trivy database updated successfully');
    } catch (error) {
        console.error('Error updating Trivy DB:', error);
        alert('Error updating Trivy database');
    } finally {
        btn.disabled = false;
        btn.textContent = 'Update DB';
    }
};

// Export vulnerability data
window.exportVulnerabilityData = function() {
    const scans = Array.from(document.querySelectorAll('#securityScansTable tbody tr'))
        .filter(row => !row.querySelector('.no-data'))
        .map(row => {
            const cells = row.querySelectorAll('td');
            return {
                image: cells[0].textContent,
                status: cells[1].textContent,
                total: cells[2].textContent,
                breakdown: cells[3].textContent,
                scanned_at: cells[4].textContent
            };
        });

    const csv = [
        ['Image', 'Status', 'Total Vulnerabilities', 'Severity Breakdown', 'Scanned At'].join(','),
        ...scans.map(s => [s.image, s.status, s.total, s.breakdown, s.scanned_at].join(','))
    ].join('\n');

    const blob = new Blob([csv], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'vulnerability-scans.csv';
    a.click();
};

// Show settings modal
window.showSecuritySettings = async function() {
    try {
        const response = await fetch('/api/p/security/settings');
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);

        const settings = await response.json();

        const modal = document.createElement('div');
        modal.className = 'modal-overlay';
        modal.innerHTML = `
            <div class="modal-content settings-modal">
                <div class="modal-header">
                    <h3>Security Scanner Settings</h3>
                    <button onclick="this.closest('.modal-overlay').remove()" class="modal-close">×</button>
                </div>
                <div class="modal-body">
                    <form id="securitySettingsForm">
                        <div class="settings-group">
                            <h4>General</h4>
                            <label>
                                <input type="checkbox" id="enabled" ${settings.enabled ? 'checked' : ''}>
                                Enable vulnerability scanning
                            </label>
                            <label>
                                <input type="checkbox" id="auto_scan_new_images" ${settings.auto_scan_new_images ? 'checked' : ''}>
                                Auto-scan new images
                            </label>
                        </div>

                        <div class="settings-group">
                            <h4>Performance</h4>
                            <label>
                                Worker Pool Size (1-10):
                                <input type="number" id="worker_pool_size" value="${settings.worker_pool_size}" min="1" max="10">
                            </label>
                            <label>
                                Scan Timeout (minutes):
                                <input type="number" id="scan_timeout_minutes" value="${settings.scan_timeout_minutes}" min="1" max="60">
                            </label>
                            <label>
                                Max Queue Size:
                                <input type="number" id="max_queue_size" value="${settings.max_queue_size}" min="10" max="1000">
                            </label>
                        </div>

                        <div class="settings-group">
                            <h4>Cache & Rescanning</h4>
                            <label>
                                Cache TTL (hours):
                                <input type="number" id="cache_ttl_hours" value="${settings.cache_ttl_hours}" min="1" max="168">
                            </label>
                            <label>
                                Rescan Interval (hours):
                                <input type="number" id="rescan_interval_hours" value="${settings.rescan_interval_hours}" min="1" max="720">
                            </label>
                        </div>

                        <div class="settings-group">
                            <h4>Retention</h4>
                            <label>
                                Retention Days (metadata):
                                <input type="number" id="retention_days" value="${settings.retention_days}" min="1" max="365">
                            </label>
                            <label>
                                Detailed Retention Days (CVEs):
                                <input type="number" id="detailed_retention_days" value="${settings.detailed_retention_days}" min="1" max="90">
                            </label>
                        </div>

                        <div class="settings-group">
                            <h4>Notifications</h4>
                            <label>
                                <input type="checkbox" id="alert_on_critical" ${settings.alert_on_critical ? 'checked' : ''}>
                                Alert on Critical vulnerabilities
                            </label>
                            <label>
                                <input type="checkbox" id="alert_on_high" ${settings.alert_on_high ? 'checked' : ''}>
                                Alert on High vulnerabilities
                            </label>
                        </div>

                        <div class="settings-group">
                            <h4>Storage</h4>
                            <label>
                                Cache Directory (read-only):
                                <input type="text" value="${settings.cache_dir}" readonly disabled>
                            </label>
                            <p class="settings-note">Database Update Interval: ${settings.db_update_interval_hours} hours</p>
                        </div>
                    </form>
                </div>
                <div class="modal-footer">
                    <button onclick="saveSecuritySettings()" class="btn">Save Settings</button>
                    <button onclick="this.closest('.modal-overlay').remove()" class="btn btn-secondary">Cancel</button>
                </div>
            </div>
        `;

        document.body.appendChild(modal);
    } catch (error) {
        console.error('Error loading settings:', error);
        alert('Error loading settings');
    }
};

// Save security settings
window.saveSecuritySettings = async function() {
    const form = document.getElementById('securitySettingsForm');
    const settings = {
        enabled: form.querySelector('#enabled').checked,
        auto_scan_new_images: form.querySelector('#auto_scan_new_images').checked,
        worker_pool_size: parseInt(form.querySelector('#worker_pool_size').value),
        scan_timeout_minutes: parseInt(form.querySelector('#scan_timeout_minutes').value),
        max_queue_size: parseInt(form.querySelector('#max_queue_size').value),
        cache_ttl_hours: parseInt(form.querySelector('#cache_ttl_hours').value),
        rescan_interval_hours: parseInt(form.querySelector('#rescan_interval_hours').value),
        retention_days: parseInt(form.querySelector('#retention_days').value),
        detailed_retention_days: parseInt(form.querySelector('#detailed_retention_days').value),
        alert_on_critical: form.querySelector('#alert_on_critical').checked,
        alert_on_high: form.querySelector('#alert_on_high').checked
    };

    try {
        const response = await fetch('/api/p/security/settings', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(settings)
        });

        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);

        alert('Settings saved successfully');
        document.querySelector('.modal-overlay')?.remove();
    } catch (error) {
        console.error('Error saving settings:', error);
        alert('Error saving settings');
    }
};

// Search/filter scans table
window.filterSecurityScans = function() {
    const searchValue = document.getElementById('securitySearch')?.value.toLowerCase() || '';
    const rows = document.querySelectorAll('#securityScansTable tbody tr');

    rows.forEach(row => {
        if (row.querySelector('.no-data')) return;

        const imageId = row.getAttribute('data-image-id');
        const imageName = row.querySelector('td:first-child').textContent.toLowerCase();

        const matches = !searchValue || imageName.includes(searchValue) || imageId.includes(searchValue);
        row.style.display = matches ? '' : 'none';
    });
};

// Initialize security tab
async function initSecurityTab() {
    // Attach event listeners to buttons
    const scanAllBtn = document.getElementById('scanAllImagesBtn');
    const updateDbBtn = document.getElementById('updateTrivyDBBtn');
    const settingsBtn = document.getElementById('vulnerabilitySettingsBtn');

    if (scanAllBtn) {
        scanAllBtn.addEventListener('click', window.scanAllImages);
    }
    if (updateDbBtn) {
        updateDbBtn.addEventListener('click', window.updateTrivyDB);
    }
    if (settingsBtn) {
        settingsBtn.addEventListener('click', window.showSecuritySettings);
    }

    // Pre-load vulnerability scans
    await preloadVulnerabilityScans();

    // Load summary and scans
    await loadVulnerabilitySummary();

    // Setup auto-refresh when tab is active
    const observer = new MutationObserver(() => {
        const securityTab = document.getElementById('securityTab');
        if (securityTab && !securityTab.classList.contains('hidden')) {
            // Tab is active, start refresh
            if (!securityRefreshInterval) {
                securityRefreshInterval = setInterval(loadVulnerabilitySummary, 30000);
            }
        } else {
            // Tab is hidden, stop refresh
            if (securityRefreshInterval) {
                clearInterval(securityRefreshInterval);
                securityRefreshInterval = null;
            }
        }
    });

    observer.observe(document.getElementById('securityTab'), {
        attributes: true,
        attributeFilter: ['class']
    });
}

// Global init function called by plugin system
window.initSecurityPlugin = function(container, sdk) {
    console.log('[SecurityPlugin] Initializing with container:', container);

    if (!container) {
        console.error('[SecurityPlugin] No container element provided');
        return;
    }

    // Inject the security tab HTML into the container
    container.innerHTML = `
        <div id="securityTab" class="security-section-modern">
            <div class="security-header-modern">
                <div class="security-title-group">
                    <h2>🛡️ Vulnerability Scanner</h2>
                    <p class="security-subtitle">Monitor and track security vulnerabilities across all container images</p>
                </div>
                <div class="security-actions">
                    <button id="scanAllImagesBtn" class="btn btn-primary">
                        🔄 Scan All Images
                    </button>
                    <button id="updateTrivyDBBtn" class="btn btn-secondary">
                        📥 Update Database
                    </button>
                    <button id="vulnerabilitySettingsBtn" class="btn btn-secondary">
                        ⚙️ Settings
                    </button>
                </div>
            </div>

            <div class="security-summary-grid">
                <div class="security-summary-card-modern">
                    <div class="card-header">
                        <span class="card-icon">📊</span>
                        <span class="card-label">Total Scanned</span>
                    </div>
                    <div class="card-value" id="totalScannedImages">-</div>
                    <div class="card-footer">images analyzed</div>
                </div>
                <div class="security-summary-card-modern critical-card">
                    <div class="card-header">
                        <span class="card-icon">🚨</span>
                        <span class="card-label">Critical</span>
                    </div>
                    <div class="card-value" id="totalCriticalVulns">-</div>
                    <div class="card-footer">vulnerabilities</div>
                </div>
                <div class="security-summary-card-modern high-card">
                    <div class="card-header">
                        <span class="card-icon">⚠️</span>
                        <span class="card-label">High</span>
                    </div>
                    <div class="card-value" id="totalHighVulns">-</div>
                    <div class="card-footer">vulnerabilities</div>
                </div>
                <div class="security-summary-card-modern">
                    <div class="card-header">
                        <span class="card-icon">🛡️</span>
                        <span class="card-label">At Risk</span>
                    </div>
                    <div class="card-value" id="atRiskImages">-</div>
                    <div class="card-footer">images affected</div>
                </div>
            </div>

            <div class="security-queue-status" id="securityQueueStatus" style="display: none;">
                <div class="queue-status-icon">⏳</div>
                <div class="queue-status-content">
                    <strong>Scanning in progress:</strong>
                    <span id="queueStatusText">-</span>
                </div>
            </div>

            <div class="security-queue-status" style="display: flex; background-color: var(--info-bg, #e3f2fd); border-color: var(--info-border, #2196f3);">
                <div class="queue-status-icon">ℹ️</div>
                <div class="queue-status-content">
                    <strong>Note:</strong>
                    Only images from hosts with vulnerability scanning enabled are shown here. Agents do not have the ability to scan for vulnerabilities locally.
                </div>
            </div>

            <div class="security-charts-grid">
                <div class="security-chart-card">
                    <div class="chart-card-header">
                        <h3>Severity Distribution</h3>
                        <p class="chart-subtitle">Current vulnerability breakdown</p>
                    </div>
                    <div class="security-chart-container">
                        <canvas id="vulnerabilitySeverityChart"></canvas>
                    </div>
                </div>

                <div class="security-chart-card">
                    <div class="chart-card-header">
                        <h3>Vulnerability Trends</h3>
                        <p class="chart-subtitle">Last 30 days</p>
                    </div>
                    <div class="security-chart-container">
                        <canvas id="vulnerabilityTrendsChart"></canvas>
                    </div>
                </div>
            </div>

            <div class="security-table-card">
                <div class="security-table-header-modern">
                    <div class="table-title-group">
                        <h3>Vulnerability Scans</h3>
                        <span class="scan-count" id="scanCountBadge">0 scans</span>
                    </div>
                    <div class="security-filters-modern">
                        <select id="securityHostFilter" class="filter-select">
                            <option value="">All Hosts</option>
                        </select>
                        <select id="securitySeverityFilter" class="filter-select">
                            <option value="">All Severities</option>
                            <option value="critical">Critical</option>
                            <option value="high">High</option>
                            <option value="medium">Medium</option>
                            <option value="low">Low</option>
                            <option value="clean">Clean</option>
                        </select>
                        <select id="securityStatusFilter" class="filter-select">
                            <option value="">All Status</option>
                            <option value="scanned">Scanned Only</option>
                            <option value="remote">Remote Only</option>
                            <option value="failed">Failed Only</option>
                        </select>
                        <input type="text" id="securitySearchInput" class="search-input" placeholder="🔍 Search images...">
                    </div>
                </div>
                <div class="table-container">
                    <table class="security-table-modern">
                        <thead>
                            <tr>
                                <th>Image Name</th>
                                <th>Status</th>
                                <th>Total</th>
                                <th>Critical</th>
                                <th>High</th>
                                <th>Medium</th>
                                <th>Low</th>
                                <th>Scanned</th>
                                <th>Actions</th>
                            </tr>
                        </thead>
                        <tbody id="securityScansBody">
                            <tr>
                                <td colspan="9" class="loading">Loading...</td>
                            </tr>
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    `;

    // Initialize the security tab
    initSecurityTab();
};

// Export vulnerability badge function for use by other components
window.getVulnerabilityBadgeHTML = getVulnerabilityBadgeHTML;
window.getVulnerabilityScan = getVulnerabilityScan;
window.preloadVulnerabilityScans = preloadVulnerabilityScans;
