let topImagesChart = null;
let growthChart = null;

// Initialize on page load
document.addEventListener('DOMContentLoaded', () => {
    initCharts();
    loadVersion();
    loadData();

    // Set up time range change handler
    document.getElementById('timeRange').addEventListener('change', loadData);
});

function initCharts() {
    // Top Images Chart
    const topImagesCtx = document.getElementById('topImagesChart').getContext('2d');
    topImagesChart = new Chart(topImagesCtx, {
        type: 'bar',
        data: {
            labels: [],
            datasets: [{
                label: 'Container Count',
                data: [],
                backgroundColor: 'rgba(102, 126, 234, 0.8)',
                borderColor: 'rgba(102, 126, 234, 1)',
                borderWidth: 1
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            indexAxis: 'y',
            plugins: {
                legend: {
                    display: false
                },
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            return context.parsed.x + ' containers';
                        }
                    }
                }
            },
            scales: {
                x: {
                    beginAtZero: true,
                    title: {
                        display: true,
                        text: 'Total Container Count'
                    }
                }
            }
        }
    });

    // Growth Chart
    const growthCtx = document.getElementById('growthChart').getContext('2d');
    growthChart = new Chart(growthCtx, {
        type: 'line',
        data: {
            labels: [],
            datasets: [{
                label: 'Active Installations',
                data: [],
                borderColor: 'rgba(102, 126, 234, 1)',
                backgroundColor: 'rgba(102, 126, 234, 0.1)',
                tension: 0.4,
                fill: true,
                pointRadius: 5,
                pointHoverRadius: 7,
                pointBackgroundColor: 'rgba(102, 126, 234, 1)'
            }, {
                label: 'Avg Containers per Installation',
                data: [],
                borderColor: 'rgba(118, 75, 162, 1)',
                backgroundColor: 'rgba(118, 75, 162, 0.1)',
                tension: 0.4,
                fill: true,
                yAxisID: 'y1',
                pointRadius: 5,
                pointHoverRadius: 7,
                pointBackgroundColor: 'rgba(118, 75, 162, 1)'
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            plugins: {
                legend: {
                    display: true,
                    position: 'top'
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    position: 'left',
                    title: {
                        display: true,
                        text: 'Installations'
                    },
                    ticks: {
                        stepSize: 1
                    }
                },
                y1: {
                    beginAtZero: true,
                    position: 'right',
                    title: {
                        display: true,
                        text: 'Avg Containers'
                    },
                    grid: {
                        drawOnChartArea: false
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
        document.getElementById('totalContainers').textContent = formatNumber(data.total_containers);
        document.getElementById('totalHosts').textContent = formatNumber(data.total_hosts);
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
