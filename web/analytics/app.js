let topImagesChart = null;
let growthChart = null;
let registriesChart = null;
let versionsChart = null;
let scanIntervalsChart = null;
let activityHeatmapChart = null;
let geographyChart = null;
let composeAdoptionChart = null;
let connectivityChart = null;
let sharedVolumesChart = null;
let customNetworksChart = null;

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
            maintainAspectRatio: false,
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
                        title: function(context) {
                            // Show full image name in tooltip title
                            return context[0].label;
                        },
                        label: function(context) {
                            // Access the stored data for this index
                            const imageData = window.topImagesData ? window.topImagesData[context.dataIndex] : null;
                            if (imageData) {
                                return [
                                    ' ' + context.parsed.x.toLocaleString() + ' containers',
                                    ' ' + imageData.installation_count.toLocaleString() + ' installations (' + imageData.adoption_percentage + '%)'
                                ];
                            }
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
                    },
                    ticks: {
                        autoSkip: false,
                        font: {
                            size: 11
                        },
                        callback: function(value, index) {
                            const label = this.getLabelForValue(value);
                            // Truncate labels to prevent overflow, but show full in tooltip
                            return label.length > 35 ? label.substring(0, 32) + '...' : label;
                        }
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

    // Compose Adoption Chart (Pie)
    const composeAdoptionCtx = document.getElementById('composeAdoptionChart').getContext('2d');
    composeAdoptionChart = new Chart(composeAdoptionCtx, {
        type: 'pie',
        data: {
            labels: [],
            datasets: [{
                data: [],
                backgroundColor: ['#3498db', '#95a5a6']
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            plugins: {
                legend: {
                    position: 'bottom'
                }
            }
        }
    });

    // Container Connectivity Chart (Bar)
    const connectivityCtx = document.getElementById('connectivityChart').getContext('2d');
    connectivityChart = new Chart(connectivityCtx, {
        type: 'bar',
        data: {
            labels: [],
            datasets: [{
                label: 'Metrics',
                data: [],
                backgroundColor: ['#3498db', '#e74c3c', '#f39c12']
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            scales: {
                y: {
                    beginAtZero: true
                }
            },
            plugins: {
                legend: {
                    display: false
                }
            }
        }
    });

    // Shared Volumes Chart (Doughnut)
    const sharedVolumesCtx = document.getElementById('sharedVolumesChart').getContext('2d');
    sharedVolumesChart = new Chart(sharedVolumesCtx, {
        type: 'doughnut',
        data: {
            labels: [],
            datasets: [{
                data: [],
                backgroundColor: ['#9b59b6', '#bdc3c7']
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            plugins: {
                legend: {
                    position: 'bottom'
                }
            }
        }
    });

    // Custom Networks Chart (Bar)
    const customNetworksCtx = document.getElementById('customNetworksChart').getContext('2d');
    customNetworksChart = new Chart(customNetworksCtx, {
        type: 'bar',
        data: {
            labels: [],
            datasets: [{
                label: 'Network Count',
                data: [],
                backgroundColor: ['#3498db', '#2ecc71', '#95a5a6']
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            scales: {
                y: {
                    beginAtZero: true
                }
            },
            plugins: {
                legend: {
                    display: false
                }
            }
        }
    });
}

async function loadVersion() {
    try {
        const response = await fetch('/health');
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
        await loadConnectionMetrics(days);
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
        document.getElementById('avgContainers').textContent = data.avg_containers_per_install ? data.avg_containers_per_install.toFixed(1) : '-';
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

        // Store data globally for tooltip access
        window.topImagesData = data;

        // Update chart with full image names (truncation handled by y-axis callback)
        topImagesChart.data.labels = data.map(item => item.image);
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

async function loadConnectionMetrics(days) {
    try {
        const response = await fetch(`/api/stats/connection-metrics?days=${days}`);
        if (!response.ok) throw new Error('Failed to fetch connection metrics');

        const data = await response.json();

        // Update Compose Adoption Chart
        composeAdoptionChart.data.labels = [
            `Using Compose (${data.compose_percentage}%)`,
            `Not Using Compose (${100 - data.compose_percentage}%)`
        ];
        composeAdoptionChart.data.datasets[0].data = [
            data.containers_in_compose,
            data.total_containers - data.containers_in_compose
        ];
        composeAdoptionChart.update();

        // Update Container Connectivity Chart
        connectivityChart.data.labels = ['Avg Connections', 'With Dependencies', 'Total Projects'];
        connectivityChart.data.datasets[0].data = [
            parseFloat(data.avg_connections_per_container.toFixed(2)),
            data.containers_with_deps,
            data.compose_project_count
        ];
        connectivityChart.update();

        // Update Shared Volumes Chart
        sharedVolumesChart.data.labels = ['Shared Volumes', 'Other Volumes'];
        sharedVolumesChart.data.datasets[0].data = [
            data.shared_volume_count,
            data.total_volumes - data.shared_volume_count
        ];
        sharedVolumesChart.update();

        // Update Custom Networks Chart
        customNetworksChart.data.labels = ['Total Networks', 'Custom Networks', 'Default Networks'];
        customNetworksChart.data.datasets[0].data = [
            data.network_count,
            data.custom_network_count,
            data.network_count - data.custom_network_count
        ];
        customNetworksChart.update();

    } catch (error) {
        console.error('Failed to load connection metrics:', error);
    }
}

async function loadGeography(days) {
    try {
        const response = await fetch(`/api/stats/geography?days=${days}`);
        if (!response.ok) throw new Error('Failed to fetch geography');

        const data = await response.json();

        // Aggregate by region (multiple timezones can map to the same region)
        const regionMap = new Map();
        data.forEach(item => {
            const region = item.region || 'Unknown';
            const current = regionMap.get(region) || 0;
            regionMap.set(region, current + item.installations);
        });

        // Convert to array and sort by installations (descending)
        const aggregated = Array.from(regionMap.entries())
            .map(([region, installations]) => ({ region, installations }))
            .sort((a, b) => b.installations - a.installations);

        // Update chart
        geographyChart.data.labels = aggregated.map(item => item.region);
        geographyChart.data.datasets[0].data = aggregated.map(item => item.installations);
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

// ========== Container Images Table Functions ==========

let imageDetailsData = [];
let currentPage = 0;
let pageSize = 50;
let currentSort = { column: 'count', order: 'desc' };
let totalContainers = 0;

// Tab switching
function showTab(tabName, clickedButton) {
    // Update tab buttons
    document.querySelectorAll('.tab-button').forEach(btn => {
        btn.classList.remove('active');
    });

    // Add active class to the clicked button
    if (clickedButton) {
        clickedButton.classList.add('active');
    } else {
        // Fallback: find button by tab name
        const targetButton = tabName === 'charts' ?
            document.querySelector('.tab-button[onclick*="charts"]') :
            tabName === 'images' ?
            document.querySelector('.tab-button[onclick*="images"]') :
            document.querySelector('.tab-button[onclick*="database"]');
        if (targetButton) {
            targetButton.classList.add('active');
        }
    }

    // Update tab content
    document.querySelectorAll('.tab-content').forEach(content => {
        content.classList.remove('active');
    });

    if (tabName === 'charts') {
        document.getElementById('chartsTab').classList.add('active');
    } else if (tabName === 'images') {
        document.getElementById('imagesTab').classList.add('active');
        // Load image data if not already loaded
        if (imageDetailsData.length === 0) {
            loadImageDetails();
        }
    } else if (tabName === 'database') {
        document.getElementById('databaseTab').classList.add('active');
        // Load database view if not already loaded
        if (!dbDataLoaded) {
            loadDatabaseView();
        }
    }
}

// Load image details from API
async function loadImageDetails() {
    const days = document.getElementById('timeRange').value;
    const search = document.getElementById('imageSearch').value;

    const params = new URLSearchParams({
        days: days,
        limit: 1000, // Load more for client-side pagination
        offset: 0,
        sort_by: currentSort.column,
        sort_order: currentSort.order
    });

    if (search) {
        params.append('search', search);
    }

    try {
        const response = await fetch(`/api/stats/image-details?${params}`);
        const data = await response.json();

        imageDetailsData = data.images || [];
        currentPage = 0;

        renderImageTable();
    } catch (error) {
        console.error('Failed to load image details:', error);
        document.getElementById('imagesTableBody').innerHTML =
            '<tr><td colspan="5" class="error-cell">Failed to load data</td></tr>';
    }
}

// Render the table with current page data
function renderImageTable() {
    const tbody = document.getElementById('imagesTableBody');

    if (imageDetailsData.length === 0) {
        tbody.innerHTML = '<tr><td colspan="5" class="empty-cell">No images found</td></tr>';
        document.getElementById('resultsCount').textContent = 'No images found';
        updatePaginationButtons();
        return;
    }

    // Calculate total containers for percentage
    totalContainers = imageDetailsData.reduce((sum, img) => sum + img.count, 0);

    // Get current page data
    const start = currentPage * pageSize;
    const end = start + pageSize;
    const pageData = imageDetailsData.slice(start, end);

    // Render rows
    tbody.innerHTML = pageData.map(img => {
        const percentage = totalContainers > 0 ? ((img.count / totalContainers) * 100).toFixed(2) : '0';
        const registryBadge = getRegistryBadge(img.registry);

        return `
            <tr>
                <td class="image-name">${escapeHtml(img.image)}</td>
                <td class="number">${img.count.toLocaleString()}</td>
                <td>${registryBadge}</td>
                <td class="number">${img.installation_count}</td>
                <td class="number">${percentage}%</td>
            </tr>
        `;
    }).join('');

    // Update results count
    const showing = `Showing ${start + 1}-${Math.min(end, imageDetailsData.length)} of ${imageDetailsData.length} images`;
    document.getElementById('resultsCount').textContent = showing;

    updatePaginationButtons();
}

// Format bytes to human-readable size
function formatBytes(bytes) {
    if (bytes === 0 || !bytes) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// Get registry badge HTML
function getRegistryBadge(registry) {
    const badges = {
        'Docker Hub': '<span class="registry-badge docker-hub">Docker Hub</span>',
        'ghcr.io': '<span class="registry-badge ghcr">GHCR</span>',
        'quay.io': '<span class="registry-badge quay">Quay</span>',
        'gcr.io': '<span class="registry-badge gcr">GCR</span>',
        'mcr.microsoft.com': '<span class="registry-badge mcr">MCR</span>',
    };
    return badges[registry] || `<span class="registry-badge other">${escapeHtml(registry)}</span>`;
}

// Escape HTML to prevent XSS
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Sort table by column
function sortTable(column, headerElement) {
    // Toggle sort order if clicking same column
    if (currentSort.column === column) {
        currentSort.order = currentSort.order === 'asc' ? 'desc' : 'asc';
    } else {
        currentSort.column = column;
        currentSort.order = 'desc'; // Default to descending for new column
    }

    // Update sort indicators
    document.querySelectorAll('.sort-indicator').forEach(el => {
        el.textContent = '';
    });

    const header = headerElement.closest('th');
    const indicator = header.querySelector('.sort-indicator');
    indicator.textContent = currentSort.order === 'asc' ? '▲' : '▼';

    // Sort data
    imageDetailsData.sort((a, b) => {
        let aVal, bVal;

        switch(column) {
            case 'name':
                aVal = a.image.toLowerCase();
                bVal = b.image.toLowerCase();
                break;
            case 'count':
                aVal = a.count;
                bVal = b.count;
                break;
            case 'registry':
                aVal = a.registry.toLowerCase();
                bVal = b.registry.toLowerCase();
                break;
            case 'installations':
                aVal = a.installation_count;
                bVal = b.installation_count;
                break;
            case 'percentage':
                // Calculate percentage for sorting
                aVal = totalContainers > 0 ? (a.count / totalContainers) * 100 : 0;
                bVal = totalContainers > 0 ? (b.count / totalContainers) * 100 : 0;
                break;
            default:
                return 0;
        }

        if (aVal < bVal) return currentSort.order === 'asc' ? -1 : 1;
        if (aVal > bVal) return currentSort.order === 'asc' ? 1 : -1;
        return 0;
    });

    currentPage = 0;
    renderImageTable();
}

// Filter images by search
function filterImages() {
    const search = document.getElementById('imageSearch').value.toLowerCase();

    if (search === '') {
        // Reload all data if search is cleared
        loadImageDetails();
        return;
    }

    // Filter client-side for better UX
    const allImages = [...imageDetailsData];
    imageDetailsData = allImages.filter(img =>
        img.image.toLowerCase().includes(search)
    );

    currentPage = 0;
    renderImageTable();
}

// Change page
function changePage(delta) {
    const maxPage = Math.ceil(imageDetailsData.length / pageSize) - 1;
    currentPage = Math.max(0, Math.min(maxPage, currentPage + delta));
    renderImageTable();
}

// Update pagination button states
function updatePaginationButtons() {
    const maxPage = Math.ceil(imageDetailsData.length / pageSize) - 1;

    document.getElementById('prevPage').disabled = currentPage === 0;
    document.getElementById('nextPage').disabled = currentPage >= maxPage;

    const pageNum = currentPage + 1;
    const totalPages = Math.max(1, maxPage + 1);
    document.getElementById('pageInfo').textContent = `Page ${pageNum} of ${totalPages}`;
}

// ========== Database Viewer Functions ==========

let dbData = [];
let dbCurrentPage = 0;
let dbPageSize = 50;
let dbDataLoaded = false;
let dbAutoRefreshInterval = null;

async function loadDatabaseView() {
    const table = document.getElementById('dbTableSelect').value;
    const installationFilter = document.getElementById('dbInstallationFilter').value;

    // For telemetry_reports, sort by timestamp to show recently updated records first
    const sortBy = (table === 'telemetry_reports') ? 'timestamp' :
                   (table === 'submission_events') ? 'id' : 'timestamp';

    const params = new URLSearchParams({
        table: table,
        limit: dbPageSize,
        offset: dbCurrentPage * dbPageSize,
        sort_by: sortBy,
        sort_order: 'DESC'
    });

    if (installationFilter) {
        params.append('installation_id', installationFilter);
    }

    try {
        const response = await fetch(`/api/stats/database-view?${params}`);
        if (!response.ok) throw new Error('Failed to fetch database view');

        const data = await response.json();
        dbData = data;
        dbDataLoaded = true;

        renderDatabaseView(data);
    } catch (error) {
        console.error('Failed to load database view:', error);
        document.getElementById('databaseContent').innerHTML =
            '<p class="error-message">Failed to load data: ' + error.message + '</p>';
    }
}

function renderDatabaseView(data) {
    const container = document.getElementById('databaseContent');
    const records = data.records || [];

    if (records.length === 0) {
        container.innerHTML = '<p class="empty-message">No records found</p>';
        updateDbRecordCount(0, 0);
        updateDbPaginationButtons();
        return;
    }

    // Update record count
    const total = data.pagination.total;
    const showing = records.length;
    updateDbRecordCount(showing, total);

    // Render records as expandable cards
    container.innerHTML = records.map((record, index) => {
        const recordId = `db-record-${index}`;
        return renderDatabaseRecord(record, recordId, data.table);
    }).join('');

    updateDbPaginationButtons();
}

function renderDatabaseRecord(record, recordId, tableName) {
    // Get key fields for summary
    let summary = '';
    let eventClass = '';

    switch (tableName) {
        case 'telemetry_reports':
            summary = `
                <div class="db-record-summary">
                    <span class="db-field"><strong>ID:</strong> ${record.id}</span>
                    <span class="db-field"><strong>Installation:</strong> ${truncateId(record.installation_id)}</span>
                    <span class="db-field"><strong>Version:</strong> ${record.version || 'N/A'}</span>
                    <span class="db-field"><strong>Containers:</strong> ${record.total_containers}</span>
                    <span class="db-field"><strong>Hosts:</strong> ${record.host_count}</span>
                    <span class="db-field"><strong>Last Updated:</strong> ${formatTimestamp(record.timestamp)}</span>
                </div>
            `;
            break;

        case 'submission_events':
            eventClass = record.event_type === 'new' ? 'event-new' : 'event-update';
            summary = `
                <div class="db-record-summary ${eventClass}">
                    <span class="db-field"><strong>ID:</strong> ${record.id}</span>
                    <span class="db-field"><strong>Type:</strong> <span class="event-badge ${eventClass}">${record.event_type.toUpperCase()}</span></span>
                    <span class="db-field"><strong>Installation:</strong> ${truncateId(record.installation_id)}</span>
                    <span class="db-field"><strong>Containers:</strong> ${record.containers}</span>
                    <span class="db-field"><strong>Hosts:</strong> ${record.hosts}</span>
                    <span class="db-field"><strong>Timestamp:</strong> ${formatTimestamp(record.timestamp)}</span>
                </div>
            `;
            break;

        case 'image_stats':
            summary = `
                <div class="db-record-summary">
                    <span class="db-field"><strong>ID:</strong> ${record.id}</span>
                    <span class="db-field"><strong>Image:</strong> ${record.image}</span>
                    <span class="db-field"><strong>Count:</strong> ${record.count}</span>
                    <span class="db-field"><strong>Installation:</strong> ${truncateId(record.installation_id)}</span>
                    <span class="db-field"><strong>Timestamp:</strong> ${formatTimestamp(record.timestamp)}</span>
                </div>
            `;
            break;
    }

    return `
        <div class="db-record ${eventClass}">
            <div class="db-record-header" onclick="toggleDbRecord('${recordId}')">
                ${summary}
                <span class="expand-icon" id="${recordId}-icon">▼</span>
            </div>
            <div class="db-record-details" id="${recordId}" style="display: none;">
                <pre class="json-view">${formatJson(record)}</pre>
            </div>
        </div>
    `;
}

function toggleDbRecord(recordId) {
    const details = document.getElementById(recordId);
    const icon = document.getElementById(`${recordId}-icon`);

    if (details.style.display === 'none') {
        details.style.display = 'block';
        icon.textContent = '▲';
    } else {
        details.style.display = 'none';
        icon.textContent = '▼';
    }
}

function formatJson(obj) {
    return JSON.stringify(obj, null, 2);
}

function formatTimestamp(timestamp) {
    if (!timestamp) return 'N/A';

    const date = new Date(timestamp);
    const now = new Date();
    const diff = now - date;

    // Show relative time if recent
    if (diff < 60000) { // < 1 minute
        return 'Just now';
    } else if (diff < 3600000) { // < 1 hour
        const mins = Math.floor(diff / 60000);
        return `${mins}m ago`;
    } else if (diff < 86400000) { // < 1 day
        const hours = Math.floor(diff / 3600000);
        return `${hours}h ago`;
    }

    // Otherwise show formatted date
    return date.toLocaleString();
}

function truncateId(id) {
    if (!id) return 'N/A';
    return id.length > 12 ? id.substring(0, 12) + '...' : id;
}

function updateDbRecordCount(showing, total) {
    const start = dbCurrentPage * dbPageSize + 1;
    const end = dbCurrentPage * dbPageSize + showing;
    document.getElementById('dbRecordCount').textContent =
        `Showing ${start}-${end} of ${total} records`;
}

function updateDbPaginationButtons() {
    const hasPrevious = dbCurrentPage > 0;
    const hasNext = dbData.pagination && (dbCurrentPage + 1) * dbPageSize < dbData.pagination.total;

    document.getElementById('dbPrevPage').disabled = !hasPrevious;
    document.getElementById('dbNextPage').disabled = !hasNext;

    const pageNum = dbCurrentPage + 1;
    const totalPages = dbData.pagination ? Math.ceil(dbData.pagination.total / dbPageSize) : 1;
    document.getElementById('dbPageInfo').textContent = `Page ${pageNum} of ${totalPages}`;
}

function changeDbPage(delta) {
    dbCurrentPage = Math.max(0, dbCurrentPage + delta);
    loadDatabaseView();
}

function toggleAutoRefresh() {
    const checkbox = document.getElementById('dbAutoRefresh');

    if (checkbox.checked) {
        // Start auto-refresh
        dbAutoRefreshInterval = setInterval(() => {
            loadDatabaseView();
        }, 5000);
        console.log('Database auto-refresh enabled (5s interval)');
    } else {
        // Stop auto-refresh
        if (dbAutoRefreshInterval) {
            clearInterval(dbAutoRefreshInterval);
            dbAutoRefreshInterval = null;
        }
        console.log('Database auto-refresh disabled');
    }
}

function exportDatabaseView() {
    if (!dbData || !dbData.records || dbData.records.length === 0) {
        alert('No data to export');
        return;
    }

    const exportData = {
        table: dbData.table,
        exported_at: new Date().toISOString(),
        total_records: dbData.pagination.total,
        records: dbData.records
    };

    const blob = new Blob([JSON.stringify(exportData, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `census-${dbData.table}-${new Date().toISOString().split('T')[0]}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);

    console.log('Exported database view to JSON');
}

// Clean up auto-refresh on page unload
window.addEventListener('beforeunload', () => {
    if (dbAutoRefreshInterval) {
        clearInterval(dbAutoRefreshInterval);
    }
});
