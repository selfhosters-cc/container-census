let topImagesChart = null;
let growthChart = null;
let registriesChart = null;
let versionsChart = null;
let scanIntervalsChart = null;
let activityHeatmapChart = null;
let geographyChart = null;

// Vibrant color palette for charts
const colorPalette = [
    '#FF6B6B', '#4ECDC4', '#45B7D1', '#FFA07A', '#98D8C8',
    '#F7DC6F', '#BB8FCE', '#85C1E2', '#F8B739', '#52B788',
    '#FF8FAB', '#6C5CE7', '#00D2D3', '#FDA7DF', '#74B9FF',
    '#A29BFE', '#FD79A8', '#FDCB6E', '#6C5CE7', '#00B894'
];

// Gradient colors for different chart types
const gradientColors = {
    blue: { start: 'rgba(102, 126, 234, 0.8)', end: 'rgba(102, 126, 234, 0.1)', solid: '#667eea' },
    purple: { start: 'rgba(118, 75, 162, 0.8)', end: 'rgba(118, 75, 162, 0.1)', solid: '#764ba2' },
    teal: { start: 'rgba(78, 205, 196, 0.8)', end: 'rgba(78, 205, 196, 0.1)', solid: '#4ECDC4' },
    coral: { start: 'rgba(255, 107, 107, 0.8)', end: 'rgba(255, 107, 107, 0.1)', solid: '#FF6B6B' }
};

// Initialize on page load
document.addEventListener('DOMContentLoaded', () => {
    initCharts();
    loadVersion();
    loadData();

    // Set up time range change handler
    document.getElementById('timeRange').addEventListener('change', loadData);
});

function initCharts() {
    // Top Images Chart with vibrant colors
    const topImagesCtx = document.getElementById('topImagesChart').getContext('2d');
    topImagesChart = new Chart(topImagesCtx, {
        type: 'bar',
        data: {
            labels: [],
            datasets: [{
                label: 'Container Count',
                data: [],
                backgroundColor: colorPalette,
                borderColor: colorPalette.map(color => color),
                borderWidth: 2,
                borderRadius: 6,
                barThickness: 'flex',
                maxBarThickness: 30
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            indexAxis: 'y',
            animation: {
                duration: 1500,
                easing: 'easeInOutQuart'
            },
            plugins: {
                legend: {
                    display: false
                },
                tooltip: {
                    backgroundColor: 'rgba(0, 0, 0, 0.8)',
                    padding: 12,
                    titleFont: {
                        size: 14,
                        weight: 'bold'
                    },
                    bodyFont: {
                        size: 13
                    },
                    callbacks: {
                        label: function(context) {
                            return ' ' + context.parsed.x.toLocaleString() + ' containers';
                        }
                    }
                }
            },
            scales: {
                x: {
                    beginAtZero: true,
                    title: {
                        display: true,
                        text: 'Total Container Count',
                        font: {
                            size: 14,
                            weight: 'bold'
                        }
                    },
                    grid: {
                        color: 'rgba(0, 0, 0, 0.05)'
                    }
                },
                y: {
                    grid: {
                        display: false
                    }
                }
            }
        }
    });

    // Growth Chart with enhanced gradients
    const growthCtx = document.getElementById('growthChart').getContext('2d');
    growthChart = new Chart(growthCtx, {
        type: 'line',
        data: {
            labels: [],
            datasets: [{
                label: 'Active Installations',
                data: [],
                borderColor: gradientColors.blue.solid,
                backgroundColor: function(context) {
                    const chart = context.chart;
                    const {ctx, chartArea} = chart;
                    if (!chartArea) return gradientColors.blue.start;
                    const gradient = ctx.createLinearGradient(0, chartArea.bottom, 0, chartArea.top);
                    gradient.addColorStop(0, gradientColors.blue.end);
                    gradient.addColorStop(1, gradientColors.blue.start);
                    return gradient;
                },
                tension: 0.4,
                fill: true,
                pointRadius: 6,
                pointHoverRadius: 9,
                pointBackgroundColor: gradientColors.blue.solid,
                pointBorderColor: '#fff',
                pointBorderWidth: 2,
                pointHoverBorderWidth: 3,
                borderWidth: 3
            }, {
                label: 'Avg Containers per Installation',
                data: [],
                borderColor: gradientColors.purple.solid,
                backgroundColor: function(context) {
                    const chart = context.chart;
                    const {ctx, chartArea} = chart;
                    if (!chartArea) return gradientColors.purple.start;
                    const gradient = ctx.createLinearGradient(0, chartArea.bottom, 0, chartArea.top);
                    gradient.addColorStop(0, gradientColors.purple.end);
                    gradient.addColorStop(1, gradientColors.purple.start);
                    return gradient;
                },
                tension: 0.4,
                fill: true,
                yAxisID: 'y1',
                pointRadius: 6,
                pointHoverRadius: 9,
                pointBackgroundColor: gradientColors.purple.solid,
                pointBorderColor: '#fff',
                pointBorderWidth: 2,
                pointHoverBorderWidth: 3,
                borderWidth: 3
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            interaction: {
                mode: 'index',
                intersect: false
            },
            animation: {
                duration: 2000,
                easing: 'easeInOutQuart'
            },
            plugins: {
                legend: {
                    display: true,
                    position: 'top',
                    labels: {
                        usePointStyle: true,
                        padding: 15,
                        font: {
                            size: 13,
                            weight: '500'
                        }
                    }
                },
                tooltip: {
                    backgroundColor: 'rgba(0, 0, 0, 0.8)',
                    padding: 12,
                    titleFont: {
                        size: 14,
                        weight: 'bold'
                    },
                    bodyFont: {
                        size: 13
                    }
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    position: 'left',
                    title: {
                        display: true,
                        text: 'Installations',
                        font: {
                            size: 14,
                            weight: 'bold'
                        }
                    },
                    ticks: {
                        stepSize: 1
                    },
                    grid: {
                        color: 'rgba(0, 0, 0, 0.05)'
                    }
                },
                y1: {
                    beginAtZero: true,
                    position: 'right',
                    title: {
                        display: true,
                        text: 'Avg Containers',
                        font: {
                            size: 14,
                            weight: 'bold'
                        }
                    },
                    grid: {
                        drawOnChartArea: false
                    }
                }
            }
        }
    });

    // Registries Chart (Doughnut)
    const registriesCtx = document.getElementById('registriesChart').getContext('2d');
    registriesChart = new Chart(registriesCtx, {
        type: 'doughnut',
        data: {
            labels: [],
            datasets: [{
                data: [],
                backgroundColor: [
                    '#FF6B6B', '#4ECDC4', '#45B7D1', '#FFA07A',
                    '#98D8C8', '#F7DC6F', '#BB8FCE', '#85C1E2'
                ],
                borderWidth: 3,
                borderColor: '#fff',
                hoverOffset: 15
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            animation: {
                animateScale: true,
                animateRotate: true,
                duration: 1500,
                easing: 'easeInOutQuart'
            },
            plugins: {
                legend: {
                    display: true,
                    position: 'bottom',
                    labels: {
                        padding: 15,
                        font: {
                            size: 12
                        },
                        usePointStyle: true,
                        generateLabels: function(chart) {
                            const data = chart.data;
                            if (data.labels.length && data.datasets.length) {
                                return data.labels.map((label, i) => {
                                    const value = data.datasets[0].data[i];
                                    const total = data.datasets[0].data.reduce((a, b) => a + b, 0);
                                    const percentage = ((value / total) * 100).toFixed(1);
                                    return {
                                        text: `${label} (${percentage}%)`,
                                        fillStyle: data.datasets[0].backgroundColor[i],
                                        hidden: false,
                                        index: i
                                    };
                                });
                            }
                            return [];
                        }
                    }
                },
                tooltip: {
                    backgroundColor: 'rgba(0, 0, 0, 0.8)',
                    padding: 12,
                    callbacks: {
                        label: function(context) {
                            const label = context.label || '';
                            const value = context.parsed || 0;
                            const total = context.dataset.data.reduce((a, b) => a + b, 0);
                            const percentage = ((value / total) * 100).toFixed(1);
                            return ` ${label}: ${value.toLocaleString()} (${percentage}%)`;
                        }
                    }
                }
            }
        }
    });

    // Versions Chart (Horizontal Bar)
    const versionsCtx = document.getElementById('versionsChart').getContext('2d');
    versionsChart = new Chart(versionsCtx, {
        type: 'bar',
        data: {
            labels: [],
            datasets: [{
                label: 'Installations',
                data: [],
                backgroundColor: [
                    '#667eea', '#764ba2', '#4ECDC4', '#FF6B6B',
                    '#FFA07A', '#52B788', '#F7DC6F', '#BB8FCE',
                    '#74B9FF', '#A29BFE'
                ],
                borderRadius: 6,
                borderWidth: 2,
                borderColor: '#fff'
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            indexAxis: 'y',
            animation: {
                duration: 1500,
                easing: 'easeInOutQuart'
            },
            plugins: {
                legend: {
                    display: false
                },
                tooltip: {
                    backgroundColor: 'rgba(0, 0, 0, 0.8)',
                    padding: 12,
                    callbacks: {
                        label: function(context) {
                            return ' ' + context.parsed.x + ' installations';
                        }
                    }
                }
            },
            scales: {
                x: {
                    beginAtZero: true,
                    ticks: {
                        stepSize: 1
                    },
                    grid: {
                        color: 'rgba(0, 0, 0, 0.05)'
                    }
                },
                y: {
                    grid: {
                        display: false
                    }
                }
            }
        }
    });

    // Scan Intervals Chart (Doughnut)
    const scanIntervalsCtx = document.getElementById('scanIntervalsChart').getContext('2d');
    scanIntervalsChart = new Chart(scanIntervalsCtx, {
        type: 'doughnut',
        data: {
            labels: [],
            datasets: [{
                data: [],
                backgroundColor: [
                    '#667eea', '#4ECDC4', '#FF6B6B', '#F7DC6F',
                    '#52B788', '#BB8FCE', '#FFA07A', '#74B9FF'
                ],
                borderWidth: 3,
                borderColor: '#fff',
                hoverOffset: 15
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            animation: {
                animateScale: true,
                animateRotate: true,
                duration: 1500,
                easing: 'easeInOutQuart'
            },
            plugins: {
                legend: {
                    display: true,
                    position: 'bottom',
                    labels: {
                        padding: 15,
                        font: {
                            size: 12
                        },
                        usePointStyle: true
                    }
                },
                tooltip: {
                    backgroundColor: 'rgba(0, 0, 0, 0.8)',
                    padding: 12,
                    callbacks: {
                        label: function(context) {
                            const label = context.label || '';
                            const value = context.parsed || 0;
                            return ` ${label}: ${value} installations`;
                        }
                    }
                }
            }
        }
    });

    // Activity Heatmap Chart (Matrix/Bubble)
    const activityHeatmapCtx = document.getElementById('activityHeatmapChart').getContext('2d');
    activityHeatmapChart = new Chart(activityHeatmapCtx, {
        type: 'bubble',
        data: {
            datasets: [{
                label: 'Activity',
                data: [],
                backgroundColor: function(context) {
                    const value = context.raw ? context.raw.r : 0;
                    const alpha = Math.min(value / 10, 1);
                    return `rgba(102, 126, 234, ${alpha})`;
                },
                borderColor: 'rgba(102, 126, 234, 0.8)',
                borderWidth: 1
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            animation: {
                duration: 1500,
                easing: 'easeInOutQuart'
            },
            plugins: {
                legend: {
                    display: false
                },
                tooltip: {
                    backgroundColor: 'rgba(0, 0, 0, 0.8)',
                    padding: 12,
                    callbacks: {
                        label: function(context) {
                            const days = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
                            const day = days[context.raw.x];
                            const hour = context.raw.y;
                            const count = context.raw.r;
                            return ` ${day} ${hour}:00 - ${count} reports`;
                        }
                    }
                }
            },
            scales: {
                x: {
                    type: 'linear',
                    position: 'bottom',
                    min: -0.5,
                    max: 6.5,
                    ticks: {
                        stepSize: 1,
                        callback: function(value) {
                            const days = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
                            return days[value] || '';
                        }
                    },
                    grid: {
                        color: 'rgba(0, 0, 0, 0.05)'
                    }
                },
                y: {
                    type: 'linear',
                    min: -0.5,
                    max: 23.5,
                    reverse: false,
                    ticks: {
                        stepSize: 2,
                        callback: function(value) {
                            return value + ':00';
                        }
                    },
                    grid: {
                        color: 'rgba(0, 0, 0, 0.05)'
                    }
                }
            }
        }
    });

    // Geography Chart (Horizontal Bar)
    const geographyCtx = document.getElementById('geographyChart').getContext('2d');
    geographyChart = new Chart(geographyCtx, {
        type: 'bar',
        data: {
            labels: [],
            datasets: [{
                label: 'Installations',
                data: [],
                backgroundColor: [
                    '#4ECDC4', '#FF6B6B', '#45B7D1', '#F7DC6F',
                    '#52B788', '#FFA07A', '#BB8FCE', '#98D8C8',
                    '#85C1E2', '#74B9FF'
                ],
                borderRadius: 6,
                borderWidth: 2,
                borderColor: '#fff'
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            indexAxis: 'y',
            animation: {
                duration: 1500,
                easing: 'easeInOutQuart'
            },
            plugins: {
                legend: {
                    display: false
                },
                tooltip: {
                    backgroundColor: 'rgba(0, 0, 0, 0.8)',
                    padding: 12,
                    callbacks: {
                        label: function(context) {
                            const total = context.dataset.data.reduce((a, b) => a + b, 0);
                            const percentage = ((context.parsed.x / total) * 100).toFixed(1);
                            return ` ${context.parsed.x} installations (${percentage}%)`;
                        }
                    }
                }
            },
            scales: {
                x: {
                    beginAtZero: true,
                    ticks: {
                        stepSize: 1
                    },
                    grid: {
                        color: 'rgba(0, 0, 0, 0.05)'
                    }
                },
                y: {
                    grid: {
                        display: false
                    }
                }
            }
        }
    });
}

async function loadVersion() {
    try {
        const response = await fetch('/health');
        const data = await response.json();
        if (data.version) {
            document.getElementById('versionBadge').textContent = 'v' + data.version;
        }
    } catch (error) {
        console.error('Error loading version:', error);
    }
}

async function loadData() {
    const days = document.getElementById('timeRange').value;

    try {
        // Load summary stats
        await loadSummary();

        // Load top images
        await loadTopImages(days);

        // Load growth data
        await loadGrowth(days);

        // Load new charts
        await loadRegistries(days);
        await loadVersions();
        await loadScanIntervals();
        await loadActivityHeatmap(days);
        await loadGeography(days);
    } catch (error) {
        console.error('Failed to load data:', error);
    }
}

async function loadSummary() {
    try {
        const response = await fetch('/api/stats/summary');
        if (!response.ok) throw new Error('Failed to fetch summary');

        const data = await response.json();

        document.getElementById('totalInstallations').textContent = formatNumber(data.installations);
        document.getElementById('totalSubmissions').textContent = formatNumber(data.total_submissions);
        document.getElementById('totalContainers').textContent = formatNumber(data.total_containers);
        document.getElementById('totalHosts').textContent = formatNumber(data.total_hosts);
        document.getElementById('totalAgents').textContent = formatNumber(data.total_agents);
        document.getElementById('uniqueImages').textContent = formatNumber(data.unique_images);
    } catch (error) {
        console.error('Failed to load summary:', error);
        document.getElementById('totalInstallations').textContent = 'Error';
    }
}

async function loadTopImages(days) {
    try {
        const response = await fetch(`/api/stats/top-images?limit=20&days=${days}`);
        if (!response.ok) throw new Error('Failed to fetch top images');

        const data = await response.json();

        // Update chart
        topImagesChart.data.labels = data.map(item => truncateImageName(item.image));
        topImagesChart.data.datasets[0].data = data.map(item => item.count);
        topImagesChart.update();
    } catch (error) {
        console.error('Failed to load top images:', error);
    }
}

async function loadGrowth(days) {
    try {
        const response = await fetch(`/api/stats/growth?days=${days}`);
        if (!response.ok) throw new Error('Failed to fetch growth');

        const data = await response.json();

        // Update chart
        growthChart.data.labels = data.map(item => formatDate(item.date));
        growthChart.data.datasets[0].data = data.map(item => item.installations);
        growthChart.data.datasets[1].data = data.map(item => Math.round(item.avg_containers));
        growthChart.update();
    } catch (error) {
        console.error('Failed to load growth:', error);
    }
}

async function loadRegistries(days) {
    try {
        const response = await fetch(`/api/stats/registries?days=${days}`);
        if (!response.ok) throw new Error('Failed to fetch registries');

        const data = await response.json();

        // Update chart
        registriesChart.data.labels = data.map(item => item.registry);
        registriesChart.data.datasets[0].data = data.map(item => item.count);
        registriesChart.update();
    } catch (error) {
        console.error('Failed to load registries:', error);
    }
}

async function loadVersions() {
    try {
        const response = await fetch('/api/stats/versions');
        if (!response.ok) throw new Error('Failed to fetch versions');

        const data = await response.json();

        // Update chart
        versionsChart.data.labels = data.map(item => 'v' + item.version);
        versionsChart.data.datasets[0].data = data.map(item => item.installations);
        versionsChart.update();
    } catch (error) {
        console.error('Failed to load versions:', error);
    }
}

async function loadScanIntervals() {
    try {
        const response = await fetch('/api/stats/scan-intervals');
        if (!response.ok) throw new Error('Failed to fetch scan intervals');

        const data = await response.json();

        // Format labels to show time in a readable format
        const labels = data.map(item => formatScanInterval(item.interval));

        // Update chart
        scanIntervalsChart.data.labels = labels;
        scanIntervalsChart.data.datasets[0].data = data.map(item => item.installations);
        scanIntervalsChart.update();
    } catch (error) {
        console.error('Failed to load scan intervals:', error);
    }
}

async function loadActivityHeatmap(days) {
    try {
        const response = await fetch(`/api/stats/activity-heatmap?days=${days}`);
        if (!response.ok) throw new Error('Failed to fetch activity heatmap');

        const data = await response.json();

        // Convert to bubble chart format: {x: day_of_week, y: hour, r: count}
        const bubbleData = data.map(item => ({
            x: item.day_of_week,
            y: item.hour_of_day,
            r: Math.max(3, Math.min(item.report_count * 2, 20)) // Scale bubble size
        }));

        // Update chart
        activityHeatmapChart.data.datasets[0].data = bubbleData;
        activityHeatmapChart.update();
    } catch (error) {
        console.error('Failed to load activity heatmap:', error);
    }
}

async function loadGeography(days) {
    try {
        const response = await fetch(`/api/stats/geography?days=${days}`);
        if (!response.ok) throw new Error('Failed to fetch geography');

        const data = await response.json();

        // Sort by installations (descending)
        data.sort((a, b) => b.installations - a.installations);

        // Update chart
        geographyChart.data.labels = data.map(item => item.region);
        geographyChart.data.datasets[0].data = data.map(item => item.installations);
        geographyChart.update();
    } catch (error) {
        console.error('Failed to load geography:', error);
    }
}

function refreshData() {
    loadData();
}

// Helper functions
function formatNumber(num) {
    if (num >= 1000000) {
        return (num / 1000000).toFixed(1) + 'M';
    } else if (num >= 1000) {
        return (num / 1000).toFixed(1) + 'K';
    }
    return num.toString();
}

function truncateImageName(name) {
    if (name.length > 40) {
        return name.substring(0, 37) + '...';
    }
    return name;
}

function formatDate(dateStr) {
    const date = new Date(dateStr);
    return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}

function formatScanInterval(seconds) {
    if (seconds < 60) {
        return seconds + 's';
    } else if (seconds < 3600) {
        const minutes = Math.floor(seconds / 60);
        return minutes + 'm';
    } else if (seconds < 86400) {
        const hours = Math.floor(seconds / 3600);
        return hours + 'h';
    } else {
        const days = Math.floor(seconds / 86400);
        return days + 'd';
    }
}

// Live Submission Tracking
let lastEventID = 0;
let pollInterval = null;
let sessionEventCount = 0;

async function pollRecentEvents() {
    try {
        const response = await fetch(`/api/stats/recent-events?since=${lastEventID}&limit=10`);
        if (!response.ok) {
            console.error('Failed to fetch recent events');
            return;
        }

        const events = await response.json();

        if (events.length > 0) {
            // Update lastEventID to the highest ID received
            const maxID = Math.max(...events.map(e => e.id));
            if (maxID > lastEventID) {
                lastEventID = maxID;

                // Process the most recent event (they come in DESC order, so first is newest)
                const newestEvent = events[0];
                showSubmissionIndicator(newestEvent);

                // Increment session counter
                sessionEventCount += events.length;
                updateEventCounter();

                // Also refresh summary stats when new submissions arrive
                loadSummary();
            }
        }
    } catch (error) {
        console.error('Error polling events:', error);
    }
}

function updateEventCounter() {
    const counter = document.getElementById('eventCounter');
    const indicator = document.getElementById('liveIndicator');

    if (sessionEventCount > 0) {
        counter.textContent = sessionEventCount;
        counter.style.display = 'inline-block';
        indicator.classList.add('has-events');

        // Highlight briefly
        counter.classList.add('highlight');
        setTimeout(() => counter.classList.remove('highlight'), 500);
    }
}

function showSubmissionIndicator(event) {
    const indicator = document.getElementById('liveIndicator');
    const status = document.getElementById('liveStatus');
    const dot = indicator.querySelector('.pulse-dot');
    
    // Remove existing classes
    dot.classList.remove('new', 'update');
    status.classList.remove('new', 'update');
    
    // Add appropriate class based on event type
    const eventClass = event.event_type; // "new" or "update"
    dot.classList.add(eventClass);
    status.classList.add(eventClass);
    
    // Update status text
    const eventLabel = event.event_type === 'new' ? 'New Install' : 'Update';
    const installID = event.installation_id.substring(0, 8);
    status.textContent = `${eventLabel}: ${installID}... (${event.containers} containers, ${event.hosts} hosts)`;
    
    // Clear the animation class after it completes (1s)
    setTimeout(() => {
        dot.classList.remove('new', 'update');
        status.classList.remove('new', 'update');
        status.textContent = 'Waiting...';
    }, 3000);
}

function startLiveTracking() {
    // Initial load to get the latest event ID
    fetch('/api/stats/recent-events?limit=1')
        .then(res => res.json())
        .then(events => {
            if (events.length > 0) {
                lastEventID = events[0].id;
            }
            // Start polling every 5 seconds
            pollInterval = setInterval(pollRecentEvents, 5000);
        })
        .catch(err => console.error('Failed to initialize live tracking:', err));
}

function stopLiveTracking() {
    if (pollInterval) {
        clearInterval(pollInterval);
        pollInterval = null;
    }
}

// Start live tracking when page loads
document.addEventListener('DOMContentLoaded', () => {
    startLiveTracking();
});

// Stop tracking when page unloads
window.addEventListener('beforeunload', () => {
    stopLiveTracking();
});
