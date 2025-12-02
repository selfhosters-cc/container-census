// NPM Plugin JavaScript
// This file is loaded by the plugin system when the NPM tab is shown

(function() {
    // State
    var npmInstances = [];
    var npmExposed = [];

    async function npmLoadData() {
        console.log('NPM: Loading data...');
        try {
            // Load instances
            var instResp = await fetchWithAuth('/api/p/npm/instances');
            console.log('NPM: Instances response:', instResp.status);
            if (instResp.ok) {
                npmInstances = await instResp.json();
                console.log('NPM: Loaded instances:', npmInstances);
                npmRenderInstances();
            } else {
                console.error('NPM: Failed to load instances:', instResp.status);
                document.getElementById('npmInstances').innerHTML = '<div class="empty-state">Failed to load instances: ' + instResp.status + '</div>';
            }

            // Load exposed services
            var expResp = await fetchWithAuth('/api/p/npm/exposed');
            console.log('NPM: Exposed response:', expResp.status);
            if (expResp.ok) {
                npmExposed = await expResp.json();
                console.log('NPM: Loaded exposed:', npmExposed);
                npmRenderExposed();
            } else {
                console.error('NPM: Failed to load exposed:', expResp.status);
                document.getElementById('npmExposed').innerHTML = '<div class="empty-state">Failed to load exposed services: ' + expResp.status + '</div>';
            }
        } catch (error) {
            console.error('NPM: Failed to load data:', error);
            document.getElementById('npmInstances').innerHTML = '<div class="empty-state">Error: ' + error.message + '</div>';
        }
    }

    function npmRenderInstances() {
        var container = document.getElementById('npmInstances');
        if (!container) return;

        if (npmInstances.length === 0) {
            container.innerHTML = '<div class="empty-state">No NPM instances configured. Click "Add Instance" to get started.</div>';
            return;
        }

        container.innerHTML = npmInstances.map(function(inst) {
            var statusClass = inst.last_error ? 'error' : 'success';
            var statusText = inst.last_error ? 'Error' : 'Connected';
            var lastSync = inst.last_sync ? new Date(inst.last_sync).toLocaleString() : 'Never';

            return '<div class="npm-instance-card">' +
                '<div class="instance-header">' +
                    '<h4>' + escapeHtml(inst.name) + '</h4>' +
                    '<span class="status-badge ' + statusClass + '">' + statusText + '</span>' +
                '</div>' +
                '<div class="instance-details">' +
                    '<div class="detail"><span class="label">URL:</span> ' + escapeHtml(inst.url) + '</div>' +
                    '<div class="detail"><span class="label">Email:</span> ' + escapeHtml(inst.email) + '</div>' +
                    '<div class="detail"><span class="label">Last Sync:</span> ' + lastSync + '</div>' +
                    (inst.last_error ? '<div class="detail error"><span class="label">Error:</span> ' + escapeHtml(inst.last_error) + '</div>' : '') +
                '</div>' +
                '<div class="instance-actions">' +
                    '<button class="btn btn-sm npm-sync-btn" data-id="' + inst.id + '">Sync</button>' +
                    '<button class="btn btn-sm npm-test-btn" data-id="' + inst.id + '">Test</button>' +
                    '<button class="btn btn-sm npm-edit-btn" data-id="' + inst.id + '">Edit</button>' +
                    '<button class="btn btn-sm btn-danger npm-delete-btn" data-id="' + inst.id + '">Delete</button>' +
                '</div>' +
            '</div>';
        }).join('');
    }

    function npmRenderExposed() {
        var container = document.getElementById('npmExposed');
        if (!container) return;

        if (npmExposed.length === 0) {
            container.innerHTML = '<div class="empty-state">No exposed services found. Make sure your NPM instances are configured and synced.</div>';
            return;
        }

        var html = '<table class="data-table"><thead><tr>' +
            '<th>Domain</th><th>SSL</th><th>Container</th><th>Instance</th><th>Actions</th>' +
            '</tr></thead><tbody>';

        for (var i = 0; i < npmExposed.length; i++) {
            var exp = npmExposed[i];
            for (var j = 0; j < exp.mappings.length; j++) {
                var mapping = exp.mappings[j];
                var domains = mapping.domain_names || [];
                var primaryDomain = domains[0] || 'N/A';
                var scheme = mapping.ssl_enabled ? 'https' : 'http';

                html += '<tr>' +
                    '<td><a href="' + scheme + '://' + escapeHtml(primaryDomain) + '" target="_blank">' + escapeHtml(primaryDomain) + '</a></td>' +
                    '<td>' + (mapping.ssl_enabled ? '<span class="badge success">Yes</span>' : '<span class="badge">No</span>') + '</td>' +
                    '<td>' + escapeHtml(exp.container_key) + '</td>' +
                    '<td>' + escapeHtml(mapping.instance_name) + '</td>' +
                    '<td><a href="' + scheme + '://' + escapeHtml(primaryDomain) + '" target="_blank" class="btn btn-sm">Open</a></td>' +
                    '</tr>';
            }
        }

        html += '</tbody></table>';
        container.innerHTML = html;
    }

    function npmShowAddInstance() {
        document.getElementById('npmModalTitle').textContent = 'Add NPM Instance';
        document.getElementById('npmInstanceId').value = '';
        document.getElementById('npmInstanceName').value = '';
        document.getElementById('npmInstanceUrl').value = '';
        document.getElementById('npmInstanceEmail').value = '';
        document.getElementById('npmInstancePassword').value = '';
        document.getElementById('npmInstanceModal').style.display = 'flex';
    }

    function npmEditInstance(id) {
        var inst = npmInstances.find(function(i) { return i.id === id; });
        if (!inst) return;

        document.getElementById('npmModalTitle').textContent = 'Edit NPM Instance';
        document.getElementById('npmInstanceId').value = inst.id;
        document.getElementById('npmInstanceName').value = inst.name;
        document.getElementById('npmInstanceUrl').value = inst.url;
        document.getElementById('npmInstanceEmail').value = inst.email;
        document.getElementById('npmInstancePassword').value = '';
        document.getElementById('npmInstanceModal').style.display = 'flex';
    }

    function npmCloseModal() {
        document.getElementById('npmInstanceModal').style.display = 'none';
    }

    async function npmSaveInstance(event) {
        event.preventDefault();

        var id = document.getElementById('npmInstanceId').value;
        var data = {
            name: document.getElementById('npmInstanceName').value,
            url: document.getElementById('npmInstanceUrl').value,
            email: document.getElementById('npmInstanceEmail').value,
            password: document.getElementById('npmInstancePassword').value
        };

        try {
            var url = id ? '/api/p/npm/instances/' + id : '/api/p/npm/instances';
            var method = id ? 'PUT' : 'POST';

            var resp = await fetchWithAuth(url, {
                method: method,
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });

            if (resp.ok) {
                npmCloseModal();
                npmLoadData();
                showNotification('Instance saved successfully', 'success');
            } else {
                var error = await resp.text();
                showNotification('Failed to save instance: ' + error, 'error');
            }
        } catch (error) {
            showNotification('Failed to save instance: ' + error.message, 'error');
        }
    }

    async function npmDeleteInstance(id) {
        if (!confirm('Are you sure you want to delete this NPM instance?')) return;

        try {
            var resp = await fetchWithAuth('/api/p/npm/instances/' + id, { method: 'DELETE' });
            if (resp.ok) {
                npmLoadData();
                showNotification('Instance deleted', 'success');
            } else {
                showNotification('Failed to delete instance', 'error');
            }
        } catch (error) {
            showNotification('Failed to delete instance: ' + error.message, 'error');
        }
    }

    async function npmTestInstance(id) {
        try {
            var resp = await fetchWithAuth('/api/p/npm/instances/' + id + '/test', { method: 'POST' });
            var result = await resp.json();

            if (result.success) {
                showNotification('Connection successful!', 'success');
            } else {
                showNotification('Connection failed: ' + result.error, 'error');
            }
        } catch (error) {
            showNotification('Test failed: ' + error.message, 'error');
        }
    }

    async function npmSyncInstance(id) {
        try {
            var resp = await fetchWithAuth('/api/p/npm/instances/' + id + '/sync', { method: 'POST' });
            var result = await resp.json();

            if (result.success) {
                showNotification('Synced ' + result.host_count + ' proxy hosts', 'success');
                npmLoadData();
            } else {
                showNotification('Sync failed: ' + result.error, 'error');
            }
        } catch (error) {
            showNotification('Sync failed: ' + error.message, 'error');
        }
    }

    // Initialize function called by plugin system
    function npmInit() {
        console.log('NPM: Initializing...');

        // Set up event listeners for static elements
        var addBtn = document.getElementById('npmAddInstanceBtn');
        if (addBtn) {
            addBtn.addEventListener('click', npmShowAddInstance);
        }

        var closeBtn = document.getElementById('npmCloseModalBtn');
        if (closeBtn) {
            closeBtn.addEventListener('click', npmCloseModal);
        }

        var cancelBtn = document.getElementById('npmCancelBtn');
        if (cancelBtn) {
            cancelBtn.addEventListener('click', npmCloseModal);
        }

        var form = document.getElementById('npmInstanceForm');
        if (form) {
            form.addEventListener('submit', npmSaveInstance);
        }

        // Event delegation for dynamically created buttons
        var instancesContainer = document.getElementById('npmInstances');
        if (instancesContainer) {
            instancesContainer.addEventListener('click', function(e) {
                var btn = e.target.closest('button');
                if (!btn) return;

                var id = parseInt(btn.dataset.id, 10);
                if (btn.classList.contains('npm-sync-btn')) {
                    npmSyncInstance(id);
                } else if (btn.classList.contains('npm-test-btn')) {
                    npmTestInstance(id);
                } else if (btn.classList.contains('npm-edit-btn')) {
                    npmEditInstance(id);
                } else if (btn.classList.contains('npm-delete-btn')) {
                    npmDeleteInstance(id);
                }
            });
        }

        // Load data
        npmLoadData();
    }

    // Expose init function globally so plugin system can call it
    window.npmPluginInit = npmInit;

    // Auto-initialize if DOM elements are already present
    if (document.getElementById('npmAddInstanceBtn')) {
        npmInit();
    }
})();
