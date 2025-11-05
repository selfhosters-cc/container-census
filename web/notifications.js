// Notification System JavaScript
// Handles all notification-related functionality

// State
let notifications = [];
let channels = [];
let rules = [];
let silences = [];
let unreadCount = 0;
let currentNotifTab = 'inbox';
let showUnreadOnly = false;

// Initialize notification system
function initNotifications() {
    setupNotificationEventListeners();
    loadNotificationData();

    // Refresh notifications every 30 seconds
    setInterval(() => {
        if (currentTab === 'notifications' || document.getElementById('notificationDropdown').classList.contains('show')) {
            loadNotifications();
        }
    }, 30000);
}

// Setup event listeners
function setupNotificationEventListeners() {
    // Modern toggle switches
    const ruleEnabledToggle = document.getElementById('ruleEnabled');
    const ruleEnabledLabel = document.getElementById('ruleEnabledLabel');
    if (ruleEnabledToggle && ruleEnabledLabel) {
        ruleEnabledToggle.addEventListener('change', (e) => {
            ruleEnabledLabel.textContent = e.target.checked ? 'Enabled' : 'Disabled';
            ruleEnabledLabel.classList.toggle('active', e.target.checked);
        });
    }

    const channelEnabledToggle = document.getElementById('channelEnabled');
    const channelEnabledLabel = document.getElementById('channelEnabledLabel');
    if (channelEnabledToggle && channelEnabledLabel) {
        channelEnabledToggle.addEventListener('change', (e) => {
            channelEnabledLabel.textContent = e.target.checked ? 'Enabled' : 'Disabled';
            channelEnabledLabel.classList.toggle('active', e.target.checked);
        });
    }

    // Notification bell toggle
    document.getElementById('notificationBell').addEventListener('click', (e) => {
        e.stopPropagation();
        toggleNotificationDropdown();
    });

    // Close dropdown when clicking outside
    document.addEventListener('click', (e) => {
        const dropdown = document.getElementById('notificationDropdown');
        const bell = document.getElementById('notificationBell');
        if (!dropdown.contains(e.target) && !bell.contains(e.target)) {
            dropdown.classList.remove('show');
        }
    });

    // Dropdown actions
    document.getElementById('markAllRead').addEventListener('click', markAllNotificationsRead);
    document.getElementById('viewAllNotifications').addEventListener('click', () => {
        switchTab('notifications');
        document.getElementById('notificationDropdown').classList.remove('show');
    });
    document.getElementById('clearNotifications').addEventListener('click', clearAllNotifications);

    // Notification tab switching
    document.querySelectorAll('.notification-tab-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            const tab = btn.getAttribute('data-notif-tab');
            switchNotificationTab(tab);
        });
    });

    // Filter buttons
    document.getElementById('filterAll').addEventListener('click', () => {
        showUnreadOnly = false;
        document.getElementById('filterAll').classList.add('active');
        document.getElementById('filterUnread').classList.remove('active');
        renderNotificationInbox();
    });

    document.getElementById('filterUnread').addEventListener('click', () => {
        showUnreadOnly = true;
        document.getElementById('filterUnread').classList.add('active');
        document.getElementById('filterAll').classList.remove('active');
        renderNotificationInbox();
    });

    // Inbox actions
    document.getElementById('markAllReadBtn').addEventListener('click', markAllNotificationsRead);
    document.getElementById('clearAllNotificationsBtn').addEventListener('click', clearAllNotifications);

    // Channel actions
    document.getElementById('addChannelBtn').addEventListener('click', openAddChannelModal);
    document.getElementById('addChannelForm').addEventListener('submit', handleChannelSubmit);
    document.getElementById('channelType').addEventListener('change', updateChannelConfigFields);
    document.getElementById('testChannelBtn').addEventListener('click', testChannel);

    // Rule actions
    document.getElementById('addRuleBtn').addEventListener('click', openAddRuleModal);
    document.getElementById('addRuleForm').addEventListener('submit', handleRuleSubmit);

    // Silence actions
    document.getElementById('addSilenceBtn').addEventListener('click', openAddSilenceModal);
    document.getElementById('addSilenceForm').addEventListener('submit', handleAddSilence);
}

// Toggle notification dropdown
function toggleNotificationDropdown() {
    const dropdown = document.getElementById('notificationDropdown');
    dropdown.classList.toggle('show');

    if (dropdown.classList.contains('show')) {
        loadNotifications(10); // Load recent 10 for dropdown
        renderNotificationDropdown();
    }
}

// Load all notification data
async function loadNotificationData() {
    await Promise.all([
        loadNotifications(),
        loadChannels(),
        loadRules(),
        loadSilences()
    ]);

    updateNotificationBadge();
    if (currentTab === 'notifications') {
        renderCurrentNotificationTab();
    }
}

// Load notifications
async function loadNotifications(limit = 100) {
    try {
        const url = showUnreadOnly
            ? `/api/notifications/logs?limit=${limit}&unread=true`
            : `/api/notifications/logs?limit=${limit}`;

        const response = await fetch(url);
        if (!response.ok) {
            const errorText = await response.text();
            console.error('Failed to load notifications:', response.status, errorText);
            throw new Error('Failed to load notifications');
        }

        const data = await response.json();
        notifications = Array.isArray(data) ? data : [];
        console.log('Loaded notifications:', notifications.length, 'notifications');
        unreadCount = notifications.filter(n => !n.read).length;
        updateNotificationBadge();

        if (currentNotifTab === 'inbox') {
            renderNotificationInbox();
        }
    } catch (error) {
        console.error('Error loading notifications:', error);
        notifications = [];
    }
}

// Load channels
async function loadChannels() {
    try {
        const response = await fetch('/api/notifications/channels');
        if (!response.ok) throw new Error('Failed to load channels');

        channels = await response.json();
        if (currentNotifTab === 'channels') {
            renderChannelsList();
        }

        // Update rule modal channel selector
        updateRuleChannelSelector();
    } catch (error) {
        console.error('Error loading channels:', error);
        channels = [];
    }
}

// Load rules
async function loadRules() {
    try {
        const response = await fetch('/api/notifications/rules');
        if (!response.ok) throw new Error('Failed to load rules');

        rules = await response.json();
        if (currentNotifTab === 'rules') {
            renderRulesList();
        }
    } catch (error) {
        console.error('Error loading rules:', error);
        rules = [];
    }
}

// Load silences
async function loadSilences() {
    try {
        const response = await fetch('/api/notifications/silences');
        if (!response.ok) throw new Error('Failed to load silences');

        silences = await response.json();
        if (currentNotifTab === 'silences') {
            renderSilencesList();
        }
    } catch (error) {
        console.error('Error loading silences:', error);
        silences = [];
    }
}

// Update notification badge
function updateNotificationBadge() {
    const badge = document.getElementById('notificationBadge');
    const sidebarBadge = document.getElementById('notificationsSidebarBadge');

    if (unreadCount > 0) {
        badge.textContent = unreadCount > 99 ? '99+' : unreadCount;
        if (sidebarBadge) sidebarBadge.textContent = unreadCount;
    } else {
        badge.textContent = '';
        if (sidebarBadge) sidebarBadge.textContent = '';
    }
}

// Render notification dropdown
function renderNotificationDropdown() {
    const list = document.getElementById('notificationList');

    if (notifications.length === 0) {
        list.innerHTML = '<div class="notification-empty">No notifications</div>';
        return;
    }

    list.innerHTML = notifications.slice(0, 10).map(notif => `
        <div class="notification-item ${!notif.read ? 'unread' : ''}" onclick="markNotificationRead(${notif.id})">
            <div class="notification-item-content">
                <div class="notification-item-title">${getEventTypeIcon(notif.event_type)} ${notif.rule_name || 'Notification'}</div>
                <div class="notification-item-message">${notif.message}</div>
                <div class="notification-item-time">${formatTimeAgo(notif.sent_at)}</div>
            </div>
        </div>
    `).join('');
}

// Render notification inbox
function renderNotificationInbox() {
    const list = document.getElementById('notificationInboxList');

    const filteredNotifs = showUnreadOnly
        ? notifications.filter(n => !n.read)
        : notifications;

    if (filteredNotifs.length === 0) {
        list.innerHTML = '<div class="notification-empty">No notifications</div>';
        return;
    }

    list.innerHTML = filteredNotifs.map(notif => `
        <div class="notification-inbox-item ${!notif.read ? 'unread' : ''}" onclick="markNotificationRead(${notif.id})">
            <div class="notification-inbox-header-row">
                <span class="notification-inbox-type ${notif.event_type}">${getEventTypeName(notif.event_type)}</span>
                <span class="notification-inbox-time">${formatTimestamp(notif.sent_at)}</span>
            </div>
            <div class="notification-inbox-message">${notif.message}</div>
            <div class="notification-inbox-details">
                ${notif.container_name ? `<div class="notification-inbox-detail">📦 ${notif.container_name}</div>` : ''}
                ${notif.host_name ? `<div class="notification-inbox-detail">🖥️ ${notif.host_name}</div>` : ''}
                ${notif.image ? `<div class="notification-inbox-detail">🖼️ ${notif.image}</div>` : ''}
            </div>
        </div>
    `).join('');
}

// Switch notification tab
function switchNotificationTab(tab) {
    currentNotifTab = tab;

    // Update tab buttons
    document.querySelectorAll('.notification-tab-btn').forEach(btn => {
        btn.classList.toggle('active', btn.getAttribute('data-notif-tab') === tab);
    });

    // Update tab content
    document.querySelectorAll('.notif-tab-content').forEach(content => {
        content.classList.remove('active');
    });
    document.getElementById(`${tab}NotifTab`).classList.add('active');

    // Render appropriate content
    renderCurrentNotificationTab();
}

// Render current notification tab
function renderCurrentNotificationTab() {
    switch(currentNotifTab) {
        case 'inbox':
            renderNotificationInbox();
            break;
        case 'channels':
            renderChannelsList();
            break;
        case 'rules':
            renderRulesList();
            break;
        case 'silences':
            renderSilencesList();
            break;
    }
}

// Render channels list
function renderChannelsList() {
    const list = document.getElementById('channelsList');

    if (channels.length === 0) {
        list.innerHTML = '<div class="notification-empty">No channels configured</div>';
        return;
    }

    list.innerHTML = channels.map(ch => `
        <div class="channel-item">
            <div class="channel-item-header">
                <div class="channel-item-title">
                    ${ch.name}
                    <span class="channel-type-badge ${ch.type}">${ch.type}</span>
                    <span class="status-badge ${ch.enabled ? 'enabled' : 'disabled'}">${ch.enabled ? 'Enabled' : 'Disabled'}</span>
                </div>
                <div class="channel-item-actions">
                    <button class="btn btn-sm btn-secondary" onclick="testChannelById(${ch.id})">Test</button>
                    <button class="btn btn-sm btn-secondary" onclick="editChannel(${ch.id})">Edit</button>
                    <button class="btn btn-sm btn-danger" onclick="deleteChannel(${ch.id})">Delete</button>
                </div>
            </div>
            <div class="channel-item-body">
                ${renderChannelDetails(ch)}
            </div>
        </div>
    `).join('');
}

// Render channel details based on type
function renderChannelDetails(channel) {
    const config = channel.config || {};

    switch(channel.type) {
        case 'webhook':
            return `
                <div class="channel-detail"><span class="detail-label">URL:</span> <span class="detail-value">${config.url || 'N/A'}</span></div>
                ${config.headers ? `<div class="channel-detail"><span class="detail-label">Headers:</span> <span class="detail-value">Configured</span></div>` : ''}
            `;
        case 'ntfy':
            return `
                <div class="channel-detail"><span class="detail-label">Server:</span> <span class="detail-value">${config.server_url || 'https://ntfy.sh'}</span></div>
                <div class="channel-detail"><span class="detail-label">Topic:</span> <span class="detail-value">${config.topic || 'N/A'}</span></div>
                ${config.token ? `<div class="channel-detail"><span class="detail-label">Auth:</span> <span class="detail-value">Configured</span></div>` : ''}
            `;
        case 'in_app':
            return '<div class="channel-detail"><span class="detail-value">In-app notifications only</span></div>';
        default:
            return '';
    }
}

// Render rules list
function renderRulesList() {
    const list = document.getElementById('rulesList');

    if (rules.length === 0) {
        list.innerHTML = '<div class="notification-empty">No rules configured</div>';
        return;
    }

    list.innerHTML = rules.map(rule => {
        // Get channel names and types for this rule
        const ruleChannels = channels.filter(ch => rule.channel_ids.includes(ch.id));
        const channelBadges = ruleChannels.map(ch =>
            `<span class="rule-channel-badge ${ch.type}">${ch.name}</span>`
        ).join('');

        return `
        <div class="rule-item ${rule.enabled ? 'enabled' : 'disabled'}">
            <div class="rule-item-header">
                <div class="rule-item-title">
                    ${rule.enabled ? '✅' : '❌'} ${rule.name}
                </div>
                <div class="rule-item-actions">
                    <button class="btn btn-sm btn-secondary" onclick="toggleRule(${rule.id}, ${!rule.enabled})">${rule.enabled ? 'Disable' : 'Enable'}</button>
                    <button class="btn btn-sm btn-secondary" onclick="editRule(${rule.id})">Edit</button>
                    <button class="btn btn-sm btn-danger" onclick="deleteRule(${rule.id})">Delete</button>
                </div>
            </div>
            <div class="rule-item-body">
                <div class="rule-detail">
                    <span class="detail-label">📬 Channels:</span>
                    <div class="rule-channels-list">
                        ${channelBadges || '<span class="detail-value">None</span>'}
                    </div>
                </div>
                <div class="rule-detail">
                    <span class="detail-label">📋 Events:</span>
                    <div class="event-types-list">
                        ${rule.event_types.map(et => `<span class="event-type-tag">${getEventTypeIcon(et)} ${getEventTypeName(et)}</span>`).join('')}
                    </div>
                </div>
                ${rule.container_pattern ? `<div class="rule-detail"><span class="detail-label">📦 Container Pattern:</span> <span class="detail-value">${rule.container_pattern}</span></div>` : ''}
                ${rule.image_pattern ? `<div class="rule-detail"><span class="detail-label">🖼️ Image Pattern:</span> <span class="detail-value">${rule.image_pattern}</span></div>` : ''}
                ${rule.cpu_threshold || rule.memory_threshold ? `<div class="rule-detail"><span class="detail-label">📊 Thresholds:</span> <span class="detail-value">${rule.cpu_threshold ? 'CPU: ' + rule.cpu_threshold + '%' : ''}${rule.cpu_threshold && rule.memory_threshold ? ', ' : ''}${rule.memory_threshold ? 'Memory: ' + rule.memory_threshold + '%' : ''}</span></div>` : ''}
                <div class="rule-detail"><span class="detail-label">⏱️ Cooldown:</span> <span class="detail-value">${rule.cooldown_seconds}s</span></div>
            </div>
        </div>
        `;
    }).join('');
}

// Render silences list
function renderSilencesList() {
    const list = document.getElementById('silencesList');

    if (silences.length === 0) {
        list.innerHTML = '<div class="notification-empty">No active silences</div>';
        return;
    }

    list.innerHTML = silences.map(silence => `
        <div class="silence-item">
            <div class="silence-item-header">
                <div class="silence-item-title">
                    ${silence.reason || 'Silence'}
                    ${silence.silenced_until ? `<span class="detail-value">(Expires: ${formatTimestamp(silence.silenced_until)})</span>` : ''}
                </div>
                <div class="silence-item-actions">
                    <button class="btn btn-sm btn-danger" onclick="deleteSilence(${silence.id})">Remove</button>
                </div>
            </div>
            <div class="silence-item-body">
                ${silence.host_id ? `<div class="silence-detail"><span class="detail-label">Host ID:</span> <span class="detail-value">${silence.host_id}</span></div>` : ''}
                ${silence.container_id ? `<div class="silence-detail"><span class="detail-label">Container ID:</span> <span class="detail-value">${silence.container_id}</span></div>` : ''}
                ${silence.host_pattern ? `<div class="silence-detail"><span class="detail-label">Host Pattern:</span> <span class="detail-value">${silence.host_pattern}</span></div>` : ''}
                ${silence.container_pattern ? `<div class="silence-detail"><span class="detail-label">Container Pattern:</span> <span class="detail-value">${silence.container_pattern}</span></div>` : ''}
            </div>
        </div>
    `).join('');
}

// Mark notification as read
async function markNotificationRead(id) {
    try {
        const response = await fetch(`/api/notifications/logs/${id}/read`, {
            method: 'PUT'
        });

        if (response.ok) {
            const notif = notifications.find(n => n.id === id);
            if (notif) notif.read = true;
            unreadCount = notifications.filter(n => !n.read).length;
            updateNotificationBadge();
            renderNotificationDropdown();
            renderNotificationInbox();
        }
    } catch (error) {
        console.error('Error marking notification as read:', error);
    }
}

// Mark all notifications as read
async function markAllNotificationsRead() {
    try {
        const response = await fetch('/api/notifications/logs/read-all', {
            method: 'PUT'
        });

        if (response.ok) {
            notifications.forEach(n => n.read = true);
            unreadCount = 0;
            updateNotificationBadge();
            renderNotificationDropdown();
            renderNotificationInbox();
            showToast('Success', 'All notifications marked as read', 'success');
        }
    } catch (error) {
        console.error('Error marking all notifications as read:', error);
        showToast('Error', 'Failed to mark notifications as read', 'error');
    }
}

// Clear all notifications
async function clearAllNotifications() {
    if (!confirm('Are you sure you want to clear all notifications?')) return;

    try {
        const response = await fetch('/api/notifications/logs/clear', {
            method: 'DELETE'
        });

        if (response.ok) {
            notifications = [];
            unreadCount = 0;
            updateNotificationBadge();
            renderNotificationDropdown();
            renderNotificationInbox();
            showToast('Success', 'All notifications cleared', 'success');
        }
    } catch (error) {
        console.error('Error clearing notifications:', error);
        showToast('Error', 'Failed to clear notifications', 'error');
    }
}

// Channel management
let currentChannelMode = 'add';
let currentChannelId = null;

function openAddChannelModal() {
    currentChannelMode = 'add';
    currentChannelId = null;

    document.getElementById('addChannelForm').reset();
    document.getElementById('channelType').value = '';
    updateChannelConfigFields();

    // Update modal title for "Add" mode
    document.querySelector('#addChannelModal .modal-header h3').textContent = 'Add Notification Channel';
    document.querySelector('#addChannelModal .btn-primary').textContent = 'Add Channel';

    document.getElementById('addChannelModal').classList.add('show');
}

function closeAddChannelModal() {
    document.getElementById('addChannelModal').classList.remove('show');
}

function updateChannelConfigFields() {
    const type = document.getElementById('channelType').value;
    document.getElementById('webhookConfig').style.display = type === 'webhook' ? 'block' : 'none';
    document.getElementById('ntfyConfig').style.display = type === 'ntfy' ? 'block' : 'none';
}

async function handleChannelSubmit(e) {
    e.preventDefault();

    if (currentChannelMode === 'edit' && currentChannelId) {
        await handleUpdateChannel(currentChannelId);
    } else {
        await handleAddChannel();
    }
}

async function handleAddChannel() {
    const type = document.getElementById('channelType').value;
    const config = {};

    if (type === 'webhook') {
        config.url = document.getElementById('webhookURL').value;
        const headersText = document.getElementById('webhookHeaders').value;
        if (headersText) {
            try {
                config.headers = JSON.parse(headersText);
            } catch (error) {
                showToast('Error', 'Invalid JSON in headers field', 'error');
                return;
            }
        }
    } else if (type === 'ntfy') {
        config.server_url = document.getElementById('ntfyServerURL').value || 'https://ntfy.sh';
        config.topic = document.getElementById('ntfyTopic').value;
        config.token = document.getElementById('ntfyToken').value || '';
    }

    const channel = {
        name: document.getElementById('channelName').value,
        type: type,
        config: config,
        enabled: document.getElementById('channelEnabled').checked
    };

    try {
        const response = await fetch('/api/notifications/channels', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(channel)
        });

        if (response.ok) {
            await loadChannels();
            closeAddChannelModal();
            showToast('Success', 'Channel created successfully', 'success');
        } else {
            const error = await response.json();
            showToast('Error', error.error || 'Failed to create channel', 'error');
        }
    } catch (error) {
        console.error('Error creating channel:', error);
        showToast('Error', 'Failed to create channel', 'error');
    }
}

async function testChannelById(id) {
    try {
        const response = await fetch(`/api/notifications/channels/${id}/test`, {
            method: 'POST'
        });

        if (response.ok) {
            showToast('Success', 'Test notification sent', 'success');

            // Reload notifications after a short delay to see the test
            setTimeout(() => {
                loadNotifications();
            }, 500);
        } else {
            const error = await response.json();
            showToast('Error', error.error || 'Failed to send test notification', 'error');
        }
    } catch (error) {
        console.error('Error testing channel:', error);
        showToast('Error', 'Failed to send test notification', 'error');
    }
}

async function deleteChannel(id) {
    if (!confirm('Are you sure you want to delete this channel?')) return;

    try {
        const response = await fetch(`/api/notifications/channels/${id}`, {
            method: 'DELETE'
        });

        if (response.ok) {
            await loadChannels();
            showToast('Success', 'Channel deleted successfully', 'success');
        } else {
            showToast('Error', 'Failed to delete channel', 'error');
        }
    } catch (error) {
        console.error('Error deleting channel:', error);
        showToast('Error', 'Failed to delete channel', 'error');
    }
}

// Rule management
let currentRuleMode = 'add';
let currentRuleId = null;

function openAddRuleModal() {
    currentRuleMode = 'add';
    currentRuleId = null;

    document.getElementById('addRuleForm').reset();
    populateRuleHostSelector();
    updateRuleChannelSelector();

    // Update modal title for "Add" mode
    document.querySelector('#addRuleModal .modal-header h3').textContent = 'Add Notification Rule';
    document.querySelector('#addRuleModal .btn-primary').textContent = 'Add Rule';

    document.getElementById('addRuleModal').classList.add('show');
}

function closeAddRuleModal() {
    document.getElementById('addRuleModal').classList.remove('show');
}

function populateRuleHostSelector() {
    const selectors = [document.getElementById('ruleHost'), document.getElementById('silenceHost')];
    selectors.forEach(select => {
        if (select) {
            select.innerHTML = '<option value="">All Hosts</option>' +
                hosts.map(h => `<option value="${h.id}">${h.name}</option>`).join('');
        }
    });
}

function updateRuleChannelSelector() {
    const select = document.getElementById('ruleChannels');
    if (select) {
        select.innerHTML = channels.map(ch =>
            `<option value="${ch.id}">${ch.name} (${ch.type})</option>`
        ).join('');
    }
}

async function handleRuleSubmit(e) {
    e.preventDefault();

    if (currentRuleMode === 'edit' && currentRuleId) {
        await handleUpdateRule(currentRuleId);
    } else {
        await handleAddRule();
    }
}

async function handleAddRule() {
    const eventTypes = Array.from(document.querySelectorAll('input[name="eventTypes"]:checked'))
        .map(cb => cb.value);

    if (eventTypes.length === 0) {
        showToast('Error', 'Please select at least one event type', 'error');
        return;
    }

    const channelIds = Array.from(document.getElementById('ruleChannels').selectedOptions)
        .map(opt => parseInt(opt.value));

    if (channelIds.length === 0) {
        showToast('Error', 'Please select at least one channel', 'error');
        return;
    }

    const rule = {
        name: document.getElementById('ruleName').value,
        enabled: document.getElementById('ruleEnabled').checked,
        event_types: eventTypes,
        container_pattern: document.getElementById('ruleContainerPattern').value || '',
        image_pattern: document.getElementById('ruleImagePattern').value || '',
        threshold_duration_seconds: parseInt(document.getElementById('ruleThresholdDuration').value) || 120,
        cooldown_seconds: parseInt(document.getElementById('ruleCooldown').value) || 300,
        channel_ids: channelIds
    };

    const hostId = document.getElementById('ruleHost').value;
    if (hostId) rule.host_id = parseInt(hostId);

    const cpuThreshold = document.getElementById('ruleCPUThreshold').value;
    if (cpuThreshold) rule.cpu_threshold = parseFloat(cpuThreshold);

    const memThreshold = document.getElementById('ruleMemoryThreshold').value;
    if (memThreshold) rule.memory_threshold = parseFloat(memThreshold);

    try {
        const response = await fetch('/api/notifications/rules', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(rule)
        });

        if (response.ok) {
            await loadRules();
            closeAddRuleModal();
            showToast('Success', 'Rule created successfully', 'success');
        } else {
            const error = await response.json();
            showToast('Error', error.error || 'Failed to create rule', 'error');
        }
    } catch (error) {
        console.error('Error creating rule:', error);
        showToast('Error', 'Failed to create rule', 'error');
    }
}

async function toggleRule(id, enabled) {
    const rule = rules.find(r => r.id === id);
    if (!rule) return;

    rule.enabled = enabled;

    try {
        const response = await fetch(`/api/notifications/rules/${id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(rule)
        });

        if (response.ok) {
            await loadRules();
            showToast('Success', `Rule ${enabled ? 'enabled' : 'disabled'}`, 'success');
        } else {
            showToast('Error', 'Failed to update rule', 'error');
        }
    } catch (error) {
        console.error('Error toggling rule:', error);
        showToast('Error', 'Failed to update rule', 'error');
    }
}

async function deleteRule(id) {
    if (!confirm('Are you sure you want to delete this rule?')) return;

    try {
        const response = await fetch(`/api/notifications/rules/${id}`, {
            method: 'DELETE'
        });

        if (response.ok) {
            await loadRules();
            showToast('Success', 'Rule deleted successfully', 'success');
        } else {
            showToast('Error', 'Failed to delete rule', 'error');
        }
    } catch (error) {
        console.error('Error deleting rule:', error);
        showToast('Error', 'Failed to delete rule', 'error');
    }
}

// Silence management
function openAddSilenceModal() {
    document.getElementById('addSilenceForm').reset();
    populateRuleHostSelector();

    // Set default expiry to 1 hour from now
    const now = new Date();
    now.setHours(now.getHours() + 1);
    document.getElementById('silenceEndsAt').value = now.toISOString().slice(0, 16);

    document.getElementById('addSilenceModal').classList.add('show');
}

function closeAddSilenceModal() {
    document.getElementById('addSilenceModal').classList.remove('show');
}

async function handleAddSilence(e) {
    e.preventDefault();

    const silence = {
        reason: document.getElementById('silenceReason').value || '',
        container_id: document.getElementById('silenceContainer').value || '',
        host_pattern: document.getElementById('silenceHostPattern').value || '',
        container_pattern: document.getElementById('silenceContainerPattern').value || '',
        silenced_until: document.getElementById('silenceEndsAt').value || null
    };

    const hostId = document.getElementById('silenceHost').value;
    if (hostId) silence.host_id = parseInt(hostId);

    try {
        const response = await fetch('/api/notifications/silences', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(silence)
        });

        if (response.ok) {
            await loadSilences();
            closeAddSilenceModal();
            showToast('Success', 'Silence created successfully', 'success');
        } else {
            const error = await response.json();
            showToast('Error', error.error || 'Failed to create silence', 'error');
        }
    } catch (error) {
        console.error('Error creating silence:', error);
        showToast('Error', 'Failed to create silence', 'error');
    }
}

async function deleteSilence(id) {
    if (!confirm('Are you sure you want to remove this silence?')) return;

    try {
        const response = await fetch(`/api/notifications/silences/${id}`, {
            method: 'DELETE'
        });

        if (response.ok) {
            await loadSilences();
            showToast('Success', 'Silence removed successfully', 'success');
        } else {
            showToast('Error', 'Failed to remove silence', 'error');
        }
    } catch (error) {
        console.error('Error removing silence:', error);
        showToast('Error', 'Failed to remove silence', 'error');
    }
}

// Edit Channel
function editChannel(id) {
    const channel = channels.find(c => c.id === id);
    if (!channel) return;

    currentChannelMode = 'edit';
    currentChannelId = id;

    // Populate form with existing data
    document.getElementById('channelName').value = channel.name;
    document.getElementById('channelType').value = channel.type;
    document.getElementById('channelEnabled').checked = channel.enabled;

    // Update config fields based on type
    updateChannelConfigFields();

    if (channel.type === 'webhook') {
        document.getElementById('webhookURL').value = channel.config.url || '';
        if (channel.config.headers) {
            document.getElementById('webhookHeaders').value = JSON.stringify(channel.config.headers, null, 2);
        }
    } else if (channel.type === 'ntfy') {
        document.getElementById('ntfyServerURL').value = channel.config.server_url || 'https://ntfy.sh';
        document.getElementById('ntfyTopic').value = channel.config.topic || '';
        document.getElementById('ntfyToken').value = channel.config.token || '';
    }

    // Change modal title
    document.querySelector('#addChannelModal .modal-header h3').textContent = 'Edit Channel';
    document.querySelector('#addChannelModal .btn-primary').textContent = 'Update Channel';

    document.getElementById('addChannelModal').classList.add('show');
}

async function handleUpdateChannel(id) {
    const type = document.getElementById('channelType').value;
    const config = {};

    if (type === 'webhook') {
        config.url = document.getElementById('webhookURL').value;
        const headersText = document.getElementById('webhookHeaders').value;
        if (headersText) {
            try {
                config.headers = JSON.parse(headersText);
            } catch (error) {
                showToast('Error', 'Invalid JSON in headers field', 'error');
                return;
            }
        }
    } else if (type === 'ntfy') {
        config.server_url = document.getElementById('ntfyServerURL').value || 'https://ntfy.sh';
        config.topic = document.getElementById('ntfyTopic').value;
        config.token = document.getElementById('ntfyToken').value || '';
    }

    const channel = {
        id: id,
        name: document.getElementById('channelName').value,
        type: type,
        config: config,
        enabled: document.getElementById('channelEnabled').checked
    };

    try {
        const response = await fetch(`/api/notifications/channels/${id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(channel)
        });

        if (response.ok) {
            await loadChannels();
            closeAddChannelModal();
            showToast('Success', 'Channel updated successfully', 'success');
        } else {
            const error = await response.json();
            showToast('Error', error.error || 'Failed to update channel', 'error');
        }
    } catch (error) {
        console.error('Error updating channel:', error);
        showToast('Error', 'Failed to update channel', 'error');
    }
}

// Edit Rule
function editRule(id) {
    const rule = rules.find(r => r.id === id);
    if (!rule) return;

    currentRuleMode = 'edit';
    currentRuleId = id;

    // Populate form with existing data
    document.getElementById('ruleName').value = rule.name;
    document.getElementById('ruleEnabled').checked = rule.enabled;

    // Check event types
    document.querySelectorAll('input[name="eventTypes"]').forEach(cb => {
        cb.checked = rule.event_types.includes(cb.value);
    });

    // Set patterns and thresholds
    document.getElementById('ruleHost').value = rule.host_id || '';
    document.getElementById('ruleContainerPattern').value = rule.container_pattern || '';
    document.getElementById('ruleImagePattern').value = rule.image_pattern || '';
    document.getElementById('ruleCPUThreshold').value = rule.cpu_threshold || '';
    document.getElementById('ruleMemoryThreshold').value = rule.memory_threshold || '';
    document.getElementById('ruleThresholdDuration').value = rule.threshold_duration_seconds || 120;
    document.getElementById('ruleCooldown').value = rule.cooldown_seconds || 300;

    // Select channels
    const channelSelect = document.getElementById('ruleChannels');
    Array.from(channelSelect.options).forEach(opt => {
        opt.selected = rule.channel_ids.includes(parseInt(opt.value));
    });

    // Change modal title
    document.querySelector('#addRuleModal .modal-header h3').textContent = 'Edit Rule';
    document.querySelector('#addRuleModal .btn-primary').textContent = 'Update Rule';

    document.getElementById('addRuleModal').classList.add('show');
}

async function handleUpdateRule(id) {
    const eventTypes = Array.from(document.querySelectorAll('input[name="eventTypes"]:checked'))
        .map(cb => cb.value);

    if (eventTypes.length === 0) {
        showToast('Error', 'Please select at least one event type', 'error');
        return;
    }

    const channelIds = Array.from(document.getElementById('ruleChannels').selectedOptions)
        .map(opt => parseInt(opt.value));

    if (channelIds.length === 0) {
        showToast('Error', 'Please select at least one channel', 'error');
        return;
    }

    const rule = {
        id: id,
        name: document.getElementById('ruleName').value,
        enabled: document.getElementById('ruleEnabled').checked,
        event_types: eventTypes,
        container_pattern: document.getElementById('ruleContainerPattern').value || '',
        image_pattern: document.getElementById('ruleImagePattern').value || '',
        threshold_duration_seconds: parseInt(document.getElementById('ruleThresholdDuration').value) || 120,
        cooldown_seconds: parseInt(document.getElementById('ruleCooldown').value) || 300,
        channel_ids: channelIds
    };

    const hostId = document.getElementById('ruleHost').value;
    if (hostId) rule.host_id = parseInt(hostId);

    const cpuThreshold = document.getElementById('ruleCPUThreshold').value;
    if (cpuThreshold) rule.cpu_threshold = parseFloat(cpuThreshold);

    const memThreshold = document.getElementById('ruleMemoryThreshold').value;
    if (memThreshold) rule.memory_threshold = parseFloat(memThreshold);

    try {
        const response = await fetch(`/api/notifications/rules/${id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(rule)
        });

        if (response.ok) {
            await loadRules();
            closeAddRuleModal();
            showToast('Success', 'Rule updated successfully', 'success');
        } else {
            const error = await response.json();
            showToast('Error', error.error || 'Failed to update rule', 'error');
        }
    } catch (error) {
        console.error('Error updating rule:', error);
        showToast('Error', 'Failed to update rule', 'error');
    }
}

// Utility functions
function getEventTypeIcon(type) {
    const icons = {
        new_image: '🖼️',
        state_change: '🔄',
        container_started: '▶️',
        container_stopped: '⏹️',
        container_paused: '⏸️',
        container_resumed: '▶️',
        high_cpu: '📈',
        high_memory: '💾',
        anomalous_behavior: '⚠️'
    };
    return icons[type] || '📬';
}

function getEventTypeName(type) {
    const names = {
        new_image: 'New Image',
        state_change: 'State Change',
        container_started: 'Started',
        container_stopped: 'Stopped',
        container_paused: 'Paused',
        container_resumed: 'Resumed',
        high_cpu: 'High CPU',
        high_memory: 'High Memory',
        anomalous_behavior: 'Anomaly'
    };
    return names[type] || type;
}

function formatTimeAgo(timestamp) {
    if (!timestamp) return 'Unknown';

    const now = new Date();
    const time = new Date(timestamp);

    if (isNaN(time.getTime())) {
        console.error('Invalid timestamp:', timestamp);
        return 'Invalid date';
    }

    const diff = Math.floor((now - time) / 1000);

    if (isNaN(diff)) {
        console.error('Invalid diff calculation:', { now, time, timestamp });
        return 'Unknown';
    }

    if (diff < 0) return 'Just now';
    if (diff < 60) return `${diff}s ago`;
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;

    const days = Math.floor(diff / 86400);
    if (isNaN(days)) return 'Unknown';
    return `${days}d ago`;
}

function formatTimestamp(timestamp) {
    if (!timestamp) return 'Unknown';

    const date = new Date(timestamp);

    if (isNaN(date.getTime())) return 'Invalid date';

    return date.toLocaleString();
}

// Make functions globally available
window.initNotifications = initNotifications;
window.markNotificationRead = markNotificationRead;
window.markAllNotificationsRead = markAllNotificationsRead;
window.clearAllNotifications = clearAllNotifications;
window.testChannelById = testChannelById;
window.deleteChannel = deleteChannel;
window.editChannel = editChannel;
window.toggleRule = toggleRule;
window.deleteRule = deleteRule;
window.editRule = editRule;
window.deleteSilence = deleteSilence;
window.closeAddChannelModal = closeAddChannelModal;
window.closeAddRuleModal = closeAddRuleModal;
window.closeAddSilenceModal = closeAddSilenceModal;
window.testChannel = async () => {
    // Test current channel in form
    showToast('Info', 'Please save the channel first, then use the Test button', 'info');
};
