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
                    <p><strong>What's new in v1.6.0:</strong></p>
                    <ul>
                        <li>🔄 Image update management for :latest containers</li>
                        <li>📊 Configurable card view on Containers tab</li>
                        <li>🎨 Improved dashboard layout and UI</li>
                    </ul>
                    <p>Let's take a quick tour of these new features!</p>
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

        // Step 2: Image Update Management
        this.tour.addStep({
            id: 'image-updates',
            title: '🔄 Image Update Management',
            text: `
                <div class="onboarding-content">
                    <p>Keep your containers up-to-date with automated image update checking.</p>
                    <p><strong>Features:</strong></p>
                    <ul>
                        <li>Check for newer versions of :latest tagged images</li>
                        <li>One-click updates: pull image + recreate container</li>
                        <li>Preserve all container configuration during updates</li>
                        <li>Automatic background checking at configurable intervals</li>
                        <li>Bulk update operations for multiple containers</li>
                    </ul>
                    <p>Look for the blue "Update" badge on container cards, or use the "Check Updates" button in the dashboard.</p>
                    <div id="update-enable-container" style="margin-top: 15px;">
                        <label style="display: flex; align-items: center; gap: 8px; cursor: pointer;">
                            <input type="checkbox" id="enableImageUpdates">
                            <span>Enable automatic update checking (every 24 hours)</span>
                        </label>
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
                    text: 'Next',
                    action: async () => {
                        const enabled = document.getElementById('enableImageUpdates')?.checked;
                        if (enabled) {
                            await this.updateImageUpdateSettings(enabled);
                        }
                        this.tour.next();
                    }
                }
            ]
        });

        // Step 3: Configurable Card View
        this.tour.addStep({
            id: 'card-view',
            title: '📊 Configurable Card View',
            text: `
                <div class="onboarding-content">
                    <p>Customize how you view your containers with flexible card display options.</p>
                    <p><strong>On the Containers tab:</strong></p>
                    <ul>
                        <li>Click the ⚙️ icon to access view settings</li>
                        <li>Toggle visibility of different card sections</li>
                        <li>Show/hide: Networks, Volumes, Environment, Ports, Labels</li>
                        <li>Settings saved per browser for consistent experience</li>
                    </ul>
                    <p>Perfect for focusing on the information that matters most to you!</p>
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
                    if (typeof switchTab === 'function') {
                        switchTab('containers', false);
                    }
                }
            }
        });

        // Step 4: Finish
        this.tour.addStep({
            id: 'finish',
            title: 'All Set!',
            text: `
                <div class="onboarding-content">
                    <p><strong>You're ready to use Container Census v1.6.0!</strong></p>
                    <p>Explore these additional features:</p>
                    <ul>
                        <li>📈 <strong>Monitoring:</strong> Real-time CPU & memory stats with historical trends</li>
                        <li>🛡️ <strong>Security:</strong> Vulnerability scanning with Trivy integration</li>
                        <li>🕸️ <strong>Graph:</strong> Visual network topology and container relationships</li>
                        <li>📅 <strong>History:</strong> Timeline of container lifecycle events</li>
                        <li>🔔 <strong>Notifications:</strong> Alerts via webhooks, Ntfy, or in-app</li>
                    </ul>
                    <p style="margin-top: 15px;">Need help? Check the <a href="https://github.com/selfhosters-cc/container-census" target="_blank">documentation</a> or replay this tour from the help menu.</p>
                </div>
            `,
            buttons: [
                {
                    text: 'Back',
                    action: this.tour.back,
                    classes: 'shepherd-button-secondary'
                },
                {
                    text: 'Get Started',
                    action: async () => {
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
