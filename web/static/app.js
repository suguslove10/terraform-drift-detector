// ══════════════════════════════════════════════════════════
// Terraform Drift Detector — Dashboard Controller
// ══════════════════════════════════════════════════════════

let currentReport = null;
let currentFilter = 'all';
let allDrifts = [];

// ── Initialization ─────────────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
    loadStateFiles();
    loadReports();
    loadSchedules();
});

// ── API Helpers ────────────────────────────────────────────
async function api(url, options = {}) {
    try {
        const resp = await fetch(url, {
            headers: { 'Content-Type': 'application/json' },
            ...options,
        });
        if (!resp.ok) {
            const err = await resp.json().catch(() => ({ error: resp.statusText }));
            throw new Error(err.error || resp.statusText);
        }
        return resp.json();
    } catch (err) {
        console.error(`API error: ${url}`, err);
        throw err;
    }
}

// ── Load State Files ───────────────────────────────────────
async function loadStateFiles() {
    try {
        const files = await api('/api/state-files');
        const list = document.getElementById('stateFileList');
        list.innerHTML = '';
        
        if (!files || files.length === 0) {
            return;
        }
        
        files.forEach(f => {
            const opt = document.createElement('option');
            opt.value = f;
            list.appendChild(opt);
        });

        // Autofill input with first option if empty
        const input = document.getElementById('stateFileSelect');
        if (input && !input.value && files.length > 0) {
            input.value = files[0];
        }
    } catch (err) {
        showToast('Failed to load state files', 'error');
    }
}

// ── Run Scan ───────────────────────────────────────────────
async function runScan() {
    const stateFile = document.getElementById('stateFileSelect').value;
    const provider = document.getElementById('providerSelect').value;
    const awsProfile = document.getElementById('profileInput').value;
    
    if (!stateFile) {
        showToast('Please specify a state file (local path or s3://...)', 'error');
        return;
    }
    
    const btn = document.getElementById('runScanBtn');
    btn.classList.add('loading');
    btn.innerHTML = '<div class="spinner"></div> Scanning...';
    
    try {
        const report = await api('/api/scans', {
            method: 'POST',
            body: JSON.stringify({ 
                state_file: stateFile, 
                provider: provider,
                aws_profile: awsProfile 
            }),
        });
        
        currentReport = report;
        allDrifts = report.drifts || [];
        updateStats(report);
        renderDrifts(allDrifts);
        updateLastScan(report.timestamp);
        loadReports();
        
        const driftCount = report.drifted_count + report.deleted_count;
        if (driftCount > 0) {
            showToast(`Scan complete — ${driftCount} resource(s) with drift detected!`, 'error');
        } else {
            showToast('Scan complete — all resources in sync!', 'success');
        }
    } catch (err) {
        showToast(`Scan failed: ${err.message}`, 'error');
    } finally {
        btn.classList.remove('loading');
        btn.innerHTML = `
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polygon points="5 3 19 12 5 21 5 3"/>
            </svg>
            Run Scan
        `;
    }
}

// ── Update Stats ───────────────────────────────────────────
function updateStats(report) {
    animateCounter('statTotal', report.total_resources);
    animateCounter('statSync', report.in_sync_count);
    animateCounter('statDrifted', report.drifted_count);
    animateCounter('statDeleted', report.deleted_count);
}

function animateCounter(id, target) {
    const el = document.getElementById(id);
    const current = parseInt(el.textContent) || 0;
    const diff = target - current;
    const steps = 20;
    const stepTime = 30;
    let step = 0;
    
    const timer = setInterval(() => {
        step++;
        const progress = step / steps;
        const eased = 1 - Math.pow(1 - progress, 3); // ease-out cubic
        el.textContent = Math.round(current + diff * eased);
        
        if (step >= steps) {
            el.textContent = target;
            clearInterval(timer);
        }
    }, stepTime);
}

// ── Render Drifts ──────────────────────────────────────────
function renderDrifts(drifts) {
    const tbody = document.getElementById('resultsBody');
    
    if (!drifts || drifts.length === 0) {
        tbody.innerHTML = `
            <tr class="empty-row">
                <td colspan="6">
                    <div class="empty-state">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                            <path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/>
                        </svg>
                        <p>No scan results yet</p>
                        <span>Run a scan to detect drift</span>
                    </div>
                </td>
            </tr>
        `;
        return;
    }
    
    // Sort: DELETED first, then DRIFTED, then IN_SYNC
    const statusOrder = { 'DELETED': 0, 'DRIFTED': 1, 'IN_SYNC': 2 };
    const sorted = [...drifts].sort((a, b) => 
        (statusOrder[a.status] ?? 3) - (statusOrder[b.status] ?? 3)
    );
    
    tbody.innerHTML = sorted.map((drift, index) => {
        const statusClass = drift.status === 'IN_SYNC' ? 'sync' : 
                           drift.status === 'DRIFTED' ? 'drifted' : 'deleted';
        
        const attrCount = (drift.attribute_diffs || []).length;
        const tagCount = Object.keys(drift.tag_diffs || {}).length;
        const totalChanges = attrCount + tagCount;
        
        const changesHTML = totalChanges > 0 
            ? `<span class="changes-count has-changes">${totalChanges} change${totalChanges > 1 ? 's' : ''}</span>`
            : drift.status === 'DELETED' 
                ? `<span class="changes-count has-changes">Resource gone</span>`
                : `<span class="changes-count no-changes">—</span>`;
        
        const actionHTML = drift.status !== 'IN_SYNC'
            ? `<button class="btn btn-ghost btn-sm" onclick="showDetail(${index})">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width:14px;height:14px">
                        <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/>
                        <circle cx="12" cy="12" r="3"/>
                    </svg>
                    View
               </button>`
            : '';
        
        return `
            <tr data-status="${drift.status}" data-name="${drift.name}" data-type="${drift.type}" 
                style="animation: fadeIn 0.3s ease ${index * 0.05}s both">
                <td>
                    <span class="status-badge ${statusClass}">
                        <span class="badge-dot"></span>
                        ${drift.status.replace('_', ' ')}
                    </span>
                </td>
                <td><span class="resource-name">${escapeHtml(drift.name)}</span></td>
                <td><span class="resource-type">${escapeHtml(drift.type)}</span></td>
                <td><span class="resource-id">${escapeHtml(drift.resource_id)}</span></td>
                <td>${changesHTML}</td>
                <td>${actionHTML}</td>
            </tr>
        `;
    }).join('');
}

// ── Show Detail Modal ──────────────────────────────────────
function showDetail(index) {
    const statusOrder = { 'DELETED': 0, 'DRIFTED': 1, 'IN_SYNC': 2 };
    const sorted = [...allDrifts].sort((a, b) => 
        (statusOrder[a.status] ?? 3) - (statusOrder[b.status] ?? 3)
    );
    const drift = sorted[index];
    if (!drift) return;
    
    const modal = document.getElementById('detailModal');
    const title = document.getElementById('modalTitle');
    const body = document.getElementById('modalBody');
    
    title.textContent = `${drift.name} (${drift.type})`;
    
    let html = '';
    
    // Status banner
    const statusClass = drift.status === 'IN_SYNC' ? 'sync' : 
                        drift.status === 'DRIFTED' ? 'drifted' : 'deleted';
    html += `
        <div style="margin-bottom: 1.5rem;">
            <span class="status-badge ${statusClass}" style="font-size: 0.85rem; padding: 0.4rem 1rem;">
                <span class="badge-dot"></span>
                ${drift.status.replace('_', ' ')}
            </span>
            <span style="margin-left: 0.75rem; color: var(--text-secondary); font-size: 0.9rem;">
                ID: <code style="font-family: var(--font-mono); color: var(--text-primary);">${escapeHtml(drift.resource_id)}</code>
            </span>
        </div>
    `;
    
    if (drift.status === 'DELETED') {
        html += `
            <div class="diff-section">
                <div style="text-align: center; padding: 2rem; color: var(--color-deleted);">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" style="width: 48px; height: 48px; margin-bottom: 1rem;">
                        <circle cx="12" cy="12" r="10"/>
                        <line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/>
                    </svg>
                    <p style="font-size: 1.1rem; font-weight: 600;">Resource Not Found</p>
                    <p style="font-size: 0.85rem; color: var(--text-secondary); margin-top: 0.5rem;">
                        This resource exists in the Terraform state but was not found in the cloud provider.
                        It may have been manually deleted.
                    </p>
                </div>
            </div>
        `;
    }
    
    // Attribute diffs
    if (drift.attribute_diffs && drift.attribute_diffs.length > 0) {
        html += `
            <div class="diff-section">
                <div class="diff-section-title">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/>
                    </svg>
                    Attribute Changes (${drift.attribute_diffs.length})
                </div>
        `;
        
        drift.attribute_diffs.forEach(ad => {
            html += `
                <div class="diff-item">
                    <div class="diff-item-header">${escapeHtml(ad.name)}</div>
                    <div class="diff-row">
                        <div class="diff-value expected">
                            <span class="diff-label">Expected (State)</span>
                            ${formatValue(ad.expected)}
                        </div>
                        <div class="diff-value actual">
                            <span class="diff-label">Actual (Cloud)</span>
                            ${formatValue(ad.actual)}
                        </div>
                    </div>
                </div>
            `;
        });
        
        html += '</div>';
    }
    
    // Tag diffs
    if (drift.tag_diffs && Object.keys(drift.tag_diffs).length > 0) {
        html += `
            <div class="diff-section">
                <div class="diff-section-title">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/>
                        <line x1="7" y1="7" x2="7.01" y2="7"/>
                    </svg>
                    Tag Changes (${Object.keys(drift.tag_diffs).length})
                </div>
        `;
        
        Object.entries(drift.tag_diffs).forEach(([key, td]) => {
            html += `
                <div class="diff-item">
                    <div class="diff-item-header">
                        ${escapeHtml(key)}
                        <span class="tag-status ${td.status}">${td.status}</span>
                    </div>
                    <div class="diff-row">
                        <div class="diff-value expected">
                            <span class="diff-label">Expected</span>
                            ${td.expected || '<em style="color: var(--text-tertiary);">not set</em>'}
                        </div>
                        <div class="diff-value actual">
                            <span class="diff-label">Actual</span>
                            ${td.actual || '<em style="color: var(--text-tertiary);">not set</em>'}
                        </div>
                    </div>
                </div>
            `;
        });
        
        html += '</div>';
    }
    
    body.innerHTML = html;
    modal.classList.add('active');
}

function closeModal(event) {
    if (event && event.target !== event.currentTarget) return;
    document.getElementById('detailModal').classList.remove('active');
}

// Close modal on Escape
document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
        document.getElementById('detailModal').classList.remove('active');
    }
});

// ── Filters ────────────────────────────────────────────────
function setFilter(filter) {
    currentFilter = filter;
    
    // Update chip active state
    document.querySelectorAll('.chip').forEach(chip => {
        chip.classList.toggle('active', chip.dataset.filter === filter);
    });
    
    filterResources();
}

function filterResources() {
    const searchTerm = document.getElementById('searchInput').value.toLowerCase();
    const rows = document.querySelectorAll('#resultsBody tr:not(.empty-row)');
    
    rows.forEach(row => {
        const status = row.dataset.status;
        const name = (row.dataset.name || '').toLowerCase();
        const type = (row.dataset.type || '').toLowerCase();
        
        const matchesFilter = currentFilter === 'all' || status === currentFilter;
        const matchesSearch = !searchTerm || name.includes(searchTerm) || type.includes(searchTerm);
        
        row.style.display = matchesFilter && matchesSearch ? '' : 'none';
    });
}

// ── Schedules ──────────────────────────────────────────────
async function loadSchedules() {
    try {
        const schedules = await api('/api/schedules');
        renderSchedules(schedules);
    } catch (err) {
        console.error('Failed to load schedules:', err);
    }
}

async function createSchedule() {
    const stateFile = document.getElementById('stateFileSelect').value;
    const provider = document.getElementById('providerSelect').value;
    const interval = document.getElementById('schedInterval').value;
    
    if (!stateFile) {
        showToast('Please select a state file first', 'error');
        return;
    }
    
    try {
        await api('/api/schedules', {
            method: 'POST',
            body: JSON.stringify({
                state_file: stateFile,
                provider: provider,
                interval: interval,
                enabled: true,
            }),
        });
        
        showToast(`Schedule created: every ${interval}`, 'success');
        loadSchedules();
    } catch (err) {
        showToast(`Failed to create schedule: ${err.message}`, 'error');
    }
}

async function toggleSchedule(id) {
    try {
        await api(`/api/schedules/${id}/toggle`, { method: 'POST' });
        loadSchedules();
    } catch (err) {
        showToast(`Failed to toggle schedule: ${err.message}`, 'error');
    }
}

function renderSchedules(schedules) {
    const container = document.getElementById('activeSchedules');
    
    if (!schedules || schedules.length === 0) {
        container.innerHTML = '';
        return;
    }
    
    container.innerHTML = schedules.map(s => `
        <div class="schedule-item">
            <div class="schedule-info">
                <div class="schedule-status ${s.enabled ? 'enabled' : 'disabled'}"></div>
                <span>${escapeHtml(s.state_file)}</span>
                <span style="color: var(--text-tertiary);">•</span>
                <span style="color: var(--text-secondary);">${s.interval}</span>
                ${s.last_run ? `<span style="color: var(--text-tertiary); font-size: 0.75rem;">Last: ${formatTime(s.last_run)}</span>` : ''}
            </div>
            <div class="schedule-actions">
                <button class="btn btn-ghost btn-sm" onclick="toggleSchedule('${escapeHtml(s.id)}')">
                    ${s.enabled ? 'Pause' : 'Resume'}
                </button>
            </div>
        </div>
    `).join('');
}

// ── Reports / History ──────────────────────────────────────
async function loadReports() {
    try {
        const reports = await api('/api/reports');
        renderHistory(reports);
    } catch (err) {
        console.error('Failed to load reports:', err);
    }
}

function renderHistory(reports) {
    const container = document.getElementById('historyList');
    
    if (!reports || reports.length === 0) {
        container.innerHTML = `
            <div class="empty-state small">
                <p>No scan history</p>
            </div>
        `;
        return;
    }
    
    container.innerHTML = reports.slice(0, 20).map(r => `
        <div class="history-item" onclick="loadReport('${escapeHtml(r.id)}')">
            <div class="history-meta">
                <span class="history-time">${formatTime(r.timestamp)}</span>
                <span class="history-file">${escapeHtml(r.state_file)} • ${r.provider}</span>
            </div>
            <div class="history-stats">
                <span class="history-stat sync">✓ ${r.in_sync_count}</span>
                <span class="history-stat drifted">⚠ ${r.drifted_count}</span>
                <span class="history-stat deleted">✗ ${r.deleted_count}</span>
            </div>
        </div>
    `).join('');
}

async function loadReport(id) {
    try {
        const report = await api(`/api/reports/${id}`);
        currentReport = report;
        allDrifts = report.drifts || [];
        updateStats(report);
        renderDrifts(allDrifts);
        updateLastScan(report.timestamp);
        showToast('Report loaded', 'info');
    } catch (err) {
        showToast(`Failed to load report: ${err.message}`, 'error');
    }
}

// ── UI Helpers ─────────────────────────────────────────────
function updateLastScan(timestamp) {
    const el = document.getElementById('lastScanTime');
    const dot = el.querySelector('.pulse-dot');
    dot.classList.add('active');
    el.innerHTML = '';
    el.appendChild(dot);
    el.appendChild(document.createTextNode(` Last scan: ${formatTime(timestamp)}`));
}

function showToast(message, type = 'info') {
    const container = document.getElementById('toastContainer');
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.innerHTML = `<span class="toast-message">${escapeHtml(message)}</span>`;
    container.appendChild(toast);
    
    setTimeout(() => {
        toast.remove();
    }, 4000);
}

function formatTime(timestamp) {
    if (!timestamp) return 'N/A';
    try {
        const d = new Date(timestamp);
        return d.toLocaleString('en-US', {
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit',
        });
    } catch {
        return timestamp;
    }
}

function formatValue(val) {
    if (val === null || val === undefined) {
        return '<em style="color: var(--text-tertiary);">null</em>';
    }
    if (typeof val === 'object') {
        return `<pre style="margin:0; white-space:pre-wrap; font-family: var(--font-mono); font-size: 0.8rem;">${escapeHtml(JSON.stringify(val, null, 2))}</pre>`;
    }
    return escapeHtml(String(val));
}

function escapeHtml(str) {
    if (typeof str !== 'string') return str;
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}
