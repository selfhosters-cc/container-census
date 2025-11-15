// Onboarding tour for Container Census using Shepherd.js
// This module creates an interactive walkthrough for first-time users

class OnboardingTour {
    constructor() {
        this.tour = null;
        this.currentVersion = null;
    }

    // Initialize the tour with Shepherd
    async init() {
        // Check if Shepherd.js is loaded
        if (typeof Shepherd === 'undefined') {
            console.error('Shepherd.js library not loaded');
            return false;
        }

        // Get current version
        try {
            const response = await fetch('/api/health');
            const health = await response.json();
            this.currentVersion = health.version;
        } catch (err) {
            console.error('Failed to get version:', err);
            this.currentVersion = '1.0.0';
        }

        this.tour = new Shepherd.Tour({
            useModalOverlay: true,
            defaultStepOptions: {
                classes: 'shepherd-theme-custom',
                scrollTo: { behavior: 'smooth', block: 'center' },
                cancelIcon: {
                    enabled: true
                }
            }
        });

        this.addSteps();
        return true;
    }

    // Add all tour steps
    addSteps() {
        // Step 1: Welcome
        this.tour.addStep({
            id: 'welcome',
            title: `Welcome to Container Census v${this.currentVersion}`,
            text: `
                <div class="onboarding-content">
                    <p>Container Census is a powerful multi-host Docker monitoring system.</p>
                    <p><strong>What's new in this version:</strong></p>
                    <ul>
                        <li>Image update management for :latest containers</li>
                        <li>Vulnerability scanning with Trivy</li>
                        <li>CPU & memory monitoring with historical trends</li>
                        <li>Advanced notification system</li>
                        <li>Network graph visualization</li>
                    </ul>
                    <p>Let's take a quick tour to get you started!</p>
                </div>
            `,
            buttons: [
                {
                    text: 'Skip Tour',
                    action: this.tour.cancel,
                    classes: 'shepherd-button-secondary'
                },
                {
                    text: 'Start Tour',
                    action: this.tour.next
                }
            ]
        });

        // Step 2: Local Containers
        this.tour.addStep({
            id: 'containers',
            title: 'Your Containers',
            text: `
                <div class="onboarding-content">
                    <p>Here you can see all containers discovered on your local Docker socket.</p>
                    <p><strong>Managing Multiple Hosts:</strong></p>
                    <p>If you have containers on other machines, you can add them by deploying the lightweight Census Agent and connecting through the Hosts tab.</p>
                    <p>The agent supports remote monitoring without exposing the Docker socket.</p>
                </div>
            `,
            attachTo: {
                element: '[data-tab="containers"]',
                on: 'bottom'
            },
            buttons: [
                {
                    text: 'Back',
                    action: this.tour.back,
                    classes: 'shepherd-button-secondary'
                },
                {
                    text: 'Next',
                    action: this.tour.next
                }
            ],
            when: {
                show: () => {
                    // Switch to containers tab
                    if (typeof switchTab === 'function') {
                        switchTab('containers', false);
                    }
                }
            }
        });

        // Step 3: Vulnerability Scanning
        this.tour.addStep({
            id: 'security',
            title: 'Security & Vulnerability Scanning',
            text: `
                <div class="onboarding-content">
                    <p>Container Census includes integrated Trivy scanning to detect vulnerabilities in your container images.</p>
                    <p><strong>Features:</strong></p>
                    <ul>
                        <li>Automatic scanning of new images</li>
                        <li>CVE tracking with severity levels</li>
                        <li>Alert notifications for critical vulnerabilities</li>
                    </ul>
                    <div id="vuln-enable-container" style="margin-top: 15px;">
                        <label style="display: flex; align-items: center; gap: 8px; cursor: pointer;">
                            <input type="checkbox" id="enableVulnScanning" checked>
                            <span>Enable vulnerability scanning</span>
                        </label>
                    </div>
                </div>
            `,
            attachTo: {
                element: '[data-tab="security"]',
                on: 'bottom'
            },
            buttons: [
                {
                    text: 'Back',
                    action: this.tour.back,
                    classes: 'shepherd-button-secondary'
                },
                {
                    text: 'Next',
                    action: async () => {
                        const enabled = document.getElementById('enableVulnScanning')?.checked;
                        if (enabled !== undefined) {
                            await this.updateVulnerabilitySettings(enabled);
                        }
                        this.tour.next();
                    }
                }
            ],
            when: {
                show: () => {
                    if (typeof switchTab === 'function') {
                        switchTab('security', false);
                    }
                }
            }
        });

        // Step 4: Graph Visualization
        this.tour.addStep({
            id: 'graph',
            title: 'Network Graph',
            text: `
                <div class="onboarding-content">
                    <p>The Graph tab provides a visual representation of your container network topology.</p>
                    <p><strong>What you'll see:</strong></p>
                    <ul>
                        <li>Container relationships and dependencies</li>
                        <li>Network connections between containers</li>
                        <li>Visual identification of isolated containers</li>
                    </ul>
                    <p>This helps you understand how your services communicate.</p>
                </div>
            `,
            attachTo: {
                element: '[data-tab="graph"]',
                on: 'bottom'
            },
            buttons: [
                {
                    text: 'Back',
                    action: this.tour.back,
                    classes: 'shepherd-button-secondary'
                },
                {
                    text: 'Next',
                    action: this.tour.next
                }
            ],
            when: {
                show: () => {
                    if (typeof switchTab === 'function') {
                        switchTab('graph', false);
                    }
                }
            }
        });

        // Step 5: History Tracking
        this.tour.addStep({
            id: 'history',
            title: 'History & Timeline',
            text: `
                <div class="onboarding-content">
                    <p>The History tab shows your container lifecycle over time.</p>
                    <p><strong>Track:</strong></p>
                    <ul>
                        <li>When containers were created and removed</li>
                        <li>State changes (start, stop, restart)</li>
                        <li>Image updates and rollbacks</li>
                    </ul>
                    <p>Perfect for troubleshooting and auditing changes.</p>
                </div>
            `,
            attachTo: {
                element: '[data-tab="history"]',
                on: 'bottom'
            },
            buttons: [
                {
                    text: 'Back',
                    action: this.tour.back,
                    classes: 'shepherd-button-secondary'
                },
                {
                    text: 'Next',
                    action: this.tour.next
                }
            ],
            when: {
                show: () => {
                    if (typeof switchTab === 'function') {
                        switchTab('history', false);
                    }
                }
            }
        });

        // Step 6: Notifications
        this.tour.addStep({
            id: 'notifications',
            title: 'Smart Notifications',
            text: `
                <div class="onboarding-content">
                    <p>Get alerted about important events in your container infrastructure.</p>
                    <p><strong>Alert Types:</strong></p>
                    <ul>
                        <li>Container state changes (stopped, started)</li>
                        <li>New image updates detected</li>
                        <li>High CPU/memory usage</li>
                        <li>Critical vulnerabilities found</li>
                    </ul>
                    <p><strong>Delivery Channels:</strong> Webhooks, Ntfy, or in-app notifications</p>
                </div>
            `,
            attachTo: {
                element: '[data-tab="notifications"]',
                on: 'bottom'
            },
            buttons: [
                {
                    text: 'Back',
                    action: this.tour.back,
                    classes: 'shepherd-button-secondary'
                },
                {
                    text: 'Next',
                    action: this.tour.next
                }
            ],
            when: {
                show: () => {
                    if (typeof switchTab === 'function') {
                        switchTab('notifications', false);
                    }
                }
            }
        });

        // Step 7: Image Updates
        this.tour.addStep({
            id: 'image-updates',
            title: 'Image Update Management',
            text: `
                <div class="onboarding-content">
                    <p>Keep your containers up-to-date with automated image update checking.</p>
                    <p><strong>Features:</strong></p>
                    <ul>
                        <li>Check for newer versions of :latest tagged images</li>
                        <li>Pull updated images and recreate containers</li>
                        <li>Preserve all container configuration during updates</li>
                        <li>Automatic background checking at configurable intervals</li>
                    </ul>
                    <p>Look for the blue "Update" badge on container cards, or use the "Check Updates" button in the dashboard.</p>
                    <div id="update-enable-container" style="margin-top: 15px;">
                        <label style="display: flex; align-items: center; gap: 8px; cursor: pointer;">
                            <input type="checkbox" id="enableImageUpdates" checked>
                            <span>Enable automatic update checking (every 24 hours)</span>
                        </label>
                    </div>
                </div>
            `,
            attachTo: {
                element: '[data-tab="containers"]',
                on: 'bottom'
            },
            buttons: [
                {
                    text: 'Back',
                    action: this.tour.back,
                    classes: 'shepherd-button-secondary'
                },
                {
                    text: 'Next',
                    action: async () => {
                        const enabled = document.getElementById('enableImageUpdates')?.checked;
                        if (enabled !== undefined) {
                            await this.updateImageUpdateSettings(enabled);
                        }
                        this.tour.next();
                    }
                }
            ],
            when: {
                show: () => {
                    if (typeof switchTab === 'function') {
                        switchTab('containers', false);
                    }
                }
            }
        });

        // Step 8: Activity Log
        this.tour.addStep({
            id: 'activity',
            title: 'Activity Log',
            text: `
                <div class="onboarding-content">
                    <p>The Activity Log provides a complete audit trail of all system actions.</p>
                    <p><strong>Recorded Events:</strong></p>
                    <ul>
                        <li>Scanner execution results</li>
                        <li>Telemetry submissions</li>
                        <li>API operations</li>
                        <li>Configuration changes</li>
                    </ul>
                    <p>Essential for compliance and troubleshooting.</p>
                </div>
            `,
            attachTo: {
                element: '[data-tab="activity"]',
                on: 'bottom'
            },
            buttons: [
                {
                    text: 'Back',
                    action: this.tour.back,
                    classes: 'shepherd-button-secondary'
                },
                {
                    text: 'Next',
                    action: this.tour.next
                }
            ],
            when: {
                show: () => {
                    if (typeof switchTab === 'function') {
                        switchTab('activity', false);
                    }
                }
            }
        });

        // Step 9: Telemetry Opt-in (final step)
        this.tour.addStep({
            id: 'telemetry',
            title: 'Join the Selfhosting Community',
            text: `
                <div class="onboarding-content">
                    <p><strong>Help us understand what the selfhosting community is running!</strong></p>
                    <p>By sharing anonymous statistics, you contribute to a collective view of popular images and emerging trends across selfhosters worldwide.</p>
                    <p><strong>What's collected:</strong></p>
                    <ul>
                        <li>Container image names and versions</li>
                        <li>Operating systems and counts</li>
                        <li>Installation ID (random UUID)</li>
                        <li>Timezone (for geographic distribution)</li>
                    </ul>
                    <p><strong>NOT collected:</strong> IP addresses, container content, logs, environment variables, or any personal data</p>
                    <p>See what's popular, what's growing, and how your setup compares to the community!</p>
                    <div id="telemetry-choice" style="margin-top: 15px;">
                        <label style="display: flex; align-items: center; gap: 8px; cursor: pointer;">
                            <input type="checkbox" id="enableTelemetry" checked>
                            <span>Yes, contribute to community insights</span>
                        </label>
                        <p style="font-size: 12px; margin-top: 10px; color: #666;">
                            <a href="https://selfhosters.cc/stats" target="_blank">View public dashboard</a> • Data submitted weekly
                        </p>
                    </div>
                </div>
            `,
            buttons: [
                {
                    text: 'Back',
                    action: this.tour.back,
                    classes: 'shepherd-button-secondary'
                },
                {
                    text: 'Finish Setup',
                    action: async () => {
                        const enabled = document.getElementById('enableTelemetry')?.checked;
                        if (enabled !== undefined) {
                            await this.updateTelemetrySettings(enabled);
                        }
                        await this.completeTour();
                        this.tour.complete();
                    }
                }
            ]
        });

        // Handle tour cancellation
        this.tour.on('cancel', async () => {
            await this.saveProgress('cancelled');
        });

        // Handle tour completion
        this.tour.on('complete', () => {
            if (typeof switchTab === 'function') {
                switchTab('dashboard', false);
            }
            showToast('Welcome!', 'Setup complete. You can replay this tour anytime from the help menu.', 'success');
        });
    }

    // Start the tour
    async start() {
        if (!this.tour) {
            const initialized = await this.init();
            if (!initialized) return;
        }
        this.tour.start();
    }

    // Save tour progress
    async saveProgress(step) {
        try {
            await fetch('/api/preferences', {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': 'Basic ' + btoa(localStorage.getItem('auth') || ':')
                },
                body: JSON.stringify({
                    'onboarding_step': step
                })
            });
        } catch (err) {
            console.error('Failed to save tour progress:', err);
        }
    }

    // Mark tour as completed
    async completeTour() {
        try {
            await fetch('/api/preferences', {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': 'Basic ' + btoa(localStorage.getItem('auth') || ':')
                },
                body: JSON.stringify({
                    'onboarding_completed': 'true',
                    'onboarding_version': this.currentVersion
                })
            });
        } catch (err) {
            console.error('Failed to mark tour complete:', err);
        }
    }

    // Update vulnerability scanning settings
    async updateVulnerabilitySettings(enabled) {
        try {
            const response = await fetch('/api/vulnerabilities/settings', {
                method: 'GET',
                headers: {
                    'Authorization': 'Basic ' + btoa(localStorage.getItem('auth') || ':')
                }
            });
            const settings = await response.json();

            settings.enabled = enabled;
            settings.auto_scan_new_images = enabled;

            await fetch('/api/vulnerabilities/settings', {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': 'Basic ' + btoa(localStorage.getItem('auth') || ':')
                },
                body: JSON.stringify(settings)
            });

            if (enabled) {
                showToast('Security', 'Vulnerability scanning enabled', 'success');
            }
        } catch (err) {
            console.error('Failed to update vulnerability settings:', err);
        }
    }

    // Update image update settings
    async updateImageUpdateSettings(enabled) {
        try {
            const settings = {
                auto_check_enabled: enabled,
                check_interval_hours: 24,
                only_check_latest_tags: true
            };

            await fetch('/api/image-updates/settings', {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': 'Basic ' + btoa(localStorage.getItem('auth') || ':')
                },
                body: JSON.stringify(settings)
            });

            if (enabled) {
                showToast('Updates', 'Automatic update checking enabled', 'success');
            }
        } catch (err) {
            console.error('Failed to update image update settings:', err);
        }
    }

    // Update telemetry settings
    async updateTelemetrySettings(enabled) {
        try {
            const response = await fetch('/api/telemetry/endpoints', {
                method: 'GET',
                headers: {
                    'Authorization': 'Basic ' + btoa(localStorage.getItem('auth') || ':')
                }
            });

            if (!response.ok) {
                console.error('Failed to fetch telemetry endpoints:', response.status);
                return;
            }

            const endpoints = await response.json();

            // Find community endpoint
            const communityEndpoint = endpoints.find(e => e.name === 'community');
            if (communityEndpoint) {
                const updateResponse = await fetch(`/api/telemetry/endpoints/${communityEndpoint.name}`, {
                    method: 'PUT',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': 'Basic ' + btoa(localStorage.getItem('auth') || ':')
                    },
                    body: JSON.stringify({
                        enabled: enabled
                    })
                });

                if (!updateResponse.ok) {
                    const error = await updateResponse.json();
                    console.error('Failed to update telemetry endpoint:', error);
                    showToast('Error', 'Failed to update telemetry settings', 'error');
                    return;
                }

                if (enabled) {
                    showToast('Thank You!', 'Anonymous telemetry enabled', 'success');
                }

                console.log('Telemetry endpoint updated successfully:', enabled);
            } else {
                console.error('Community telemetry endpoint not found');
                showToast('Error', 'Community endpoint not found', 'error');
            }
        } catch (err) {
            console.error('Failed to update telemetry settings:', err);
            showToast('Error', 'Failed to update telemetry settings', 'error');
        }
    }

    // Check if tour should be shown
    static async shouldShow() {
        try {
            const response = await fetch('/api/preferences', {
                method: 'GET',
                headers: {
                    'Authorization': 'Basic ' + btoa(localStorage.getItem('auth') || ':')
                }
            });
            const prefs = await response.json();

            // Show tour if not completed
            return prefs.onboarding_completed !== 'true';
        } catch (err) {
            console.error('Failed to check onboarding status:', err);
            return false; // Don't show on error
        }
    }
}

// Export for use in main app
window.OnboardingTour = OnboardingTour;
