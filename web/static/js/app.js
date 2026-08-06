/* OpenVPN Manager — shared front-end helpers.
 *
 * Everything the console pages need in common lives here: the API client, the
 * toast/notification layer, HTML escaping, formatters and the badge/pagination
 * renderers. Pages should not re-declare these.
 */

/* -------------------------------------------------------------------------
 * API client
 * ----------------------------------------------------------------------- */

/**
 * Pull the most useful message out of an error response.
 *
 * The API is not fully consistent: most handlers return dto.ErrorResponse
 * ({error, message, code}) while the audit endpoints return {error} only.
 * Prefer `message`, fall back to `error`, then to the status text.
 */
async function apiErrorMessage(response, fallback = 'Request failed') {
    try {
        const data = await response.json();
        if (data && typeof data === 'object') {
            if (typeof data.message === 'string' && data.message) return data.message;
            if (typeof data.error === 'string' && data.error) return data.error;
        }
    } catch {
        /* body was empty or not JSON */
    }
    return response.statusText ? `${fallback} (${response.status} ${response.statusText})` : fallback;
}

async function apiRequest(method, url, body) {
    const init = { method, headers: {} };
    if (body !== undefined) {
        init.headers['Content-Type'] = 'application/json';
        init.body = JSON.stringify(body);
    }

    let response;
    try {
        response = await fetch(url, init);
    } catch {
        throw new Error('Connection error. Please check your network and try again.');
    }

    if (response.status === 401) {
        globalThis.location.href = '/login';
        throw new Error('Your session has expired. Please sign in again.');
    }

    if (!response.ok) {
        throw new Error(await apiErrorMessage(response));
    }

    if (response.status === 204) return null;

    const text = await response.text();
    if (!text) return null;
    try {
        return JSON.parse(text);
    } catch {
        return null;
    }
}

const api = {
    get: (url) => apiRequest('GET', url),
    post: (url, data) => apiRequest('POST', url, data ?? {}),
    put: (url, data) => apiRequest('PUT', url, data ?? {}),
    delete: (url) => apiRequest('DELETE', url),
    errorMessage: apiErrorMessage,
};

/**
 * Walk every page of a paginated list endpoint.
 *
 * Only for compact pickers that genuinely need the full set (assigning a
 * network to a group, for example) — never for rendering a table.
 */
async function getAllPaginatedItems(url, property) {
    const items = [];
    let page = 1;
    let totalPages = 1;

    while (page <= totalPages) {
        const separator = url.includes('?') ? '&' : '?';
        const data = await api.get(`${url}${separator}page=${page}&page_size=100`);
        items.push(...((data && data[property]) || []));
        totalPages = (data && data.total_pages) || 1;
        page += 1;
    }

    return items;
}

/**
 * Run async tasks with a bounded number in flight.
 *
 * Row-level lookups (a user's groups, a group's networks) are one request per
 * row; issuing them sequentially makes a 30-row page feel broken, and issuing
 * all of them at once hammers the API.
 */
async function mapWithConcurrency(items, limit, task) {
    const results = new Array(items.length);
    let cursor = 0;

    async function worker() {
        while (cursor < items.length) {
            const index = cursor++;
            results[index] = await task(items[index], index);
        }
    }

    await Promise.all(
        Array.from({ length: Math.min(limit, items.length) }, () => worker())
    );
    return results;
}

/* -------------------------------------------------------------------------
 * Escaping
 * ----------------------------------------------------------------------- */

/** Escape a value for interpolation into innerHTML. */
function escapeHtml(value) {
    if (value === null || value === undefined) return '';
    return String(value)
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#39;');
}

/** Escape a value for use as a quoted argument inside an inline handler. */
function escapeAttr(value) {
    return escapeHtml(JSON.stringify(String(value ?? '')));
}

/* -------------------------------------------------------------------------
 * Toasts
 * ----------------------------------------------------------------------- */

const TOAST_ICONS = {
    success: 'bi-check-circle-fill',
    danger: 'bi-exclamation-octagon-fill',
    warning: 'bi-exclamation-triangle-fill',
    info: 'bi-info-circle-fill',
};

function toastRegion() {
    let region = document.getElementById('toastRegion');
    if (!region) {
        region = document.createElement('div');
        region.id = 'toastRegion';
        region.className = 'toast-region';
        region.setAttribute('aria-live', 'polite');
        document.body.appendChild(region);
    }
    return region;
}

/** Show a transient notification. Type: success | danger | warning | info. */
function showToast(message, type = 'success', timeout = 5000) {
    const region = toastRegion();
    const toast = document.createElement('div');
    toast.className = `app-toast is-${type}`;
    toast.setAttribute('role', type === 'danger' ? 'alert' : 'status');
    toast.innerHTML = `
        <i class="bi ${TOAST_ICONS[type] || TOAST_ICONS.info}"></i>
        <div class="app-toast-body">${escapeHtml(message)}</div>
        <button type="button" class="app-toast-close" aria-label="Dismiss"><i class="bi bi-x-lg"></i></button>
    `;

    const dismiss = () => {
        if (!toast.isConnected) return;
        toast.classList.add('is-leaving');
        setTimeout(() => toast.remove(), 180);
    };

    toast.querySelector('.app-toast-close').addEventListener('click', dismiss);
    region.appendChild(toast);
    if (timeout > 0) setTimeout(dismiss, timeout);
    return toast;
}

/** Alias kept so pages can read as prose: showAlert('Saved', 'success'). */
function showAlert(message, type = 'success') {
    return showToast(message, type);
}

/** Render an inline alert inside a modal body. Pass an empty message to clear. */
function setFormAlert(elementId, message, type = 'danger') {
    const el = document.getElementById(elementId);
    if (!el) return;
    if (!message) {
        el.classList.add('d-none');
        el.textContent = '';
        return;
    }
    el.className = `alert alert-${type}`;
    el.textContent = message;
    el.classList.remove('d-none');
}

function clearFormAlert(elementId) {
    setFormAlert(elementId, '');
}

/* -------------------------------------------------------------------------
 * Formatters
 * ----------------------------------------------------------------------- */

function formatDateTime(value) {
    if (!value) return '';
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleString();
}

function formatDate(value) {
    if (!value) return '';
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleDateString();
}

/** ISO date portion, safe for <input type="date">. */
function toDateInput(value) {
    if (!value) return '';
    return String(value).substring(0, 10);
}

function formatBytes(bytes) {
    const value = Number(bytes) || 0;
    if (value === 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
    const exponent = Math.min(Math.floor(Math.log(Math.abs(value)) / Math.log(1024)), units.length - 1);
    const scaled = value / Math.pow(1024, exponent);
    const digits = exponent === 0 ? 0 : scaled >= 100 ? 0 : scaled >= 10 ? 1 : 2;
    return `${scaled.toFixed(digits)} ${units[exponent]}`;
}

/** Human duration between two timestamps; `end` may be null for live sessions. */
function formatDuration(start, end) {
    const startDate = new Date(start);
    if (Number.isNaN(startDate.getTime())) return '-';
    const endDate = end ? new Date(end) : new Date();
    const seconds = Math.max(0, Math.floor((endDate - startDate) / 1000));

    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = seconds % 60;

    if (days > 0) return `${days}d ${hours}h`;
    if (hours > 0) return `${hours}h ${minutes}m`;
    if (minutes > 0) return `${minutes}m ${secs}s`;
    return `${secs}s`;
}

function initials(name) {
    const parts = String(name || '').trim().split(/\s+/).filter(Boolean);
    if (parts.length === 0) return '?';
    const first = Array.from(parts[0])[0] || '';
    const last = parts.length > 1 ? Array.from(parts[parts.length - 1])[0] || '' : '';
    return (first + last).toUpperCase();
}

/* -------------------------------------------------------------------------
 * Badges
 * ----------------------------------------------------------------------- */

const ROLE_PILLS = { ADMIN: 'pill-red', MANAGER: 'pill-amber', USER: 'pill-neutral' };

function rolePill(role) {
    return `<span class="pill ${ROLE_PILLS[role] || 'pill-neutral'}">${escapeHtml(role)}</span>`;
}

function statusPill(isActive) {
    return isActive
        ? '<span class="pill pill-green"><i class="bi bi-check-circle-fill"></i>Active</span>'
        : '<span class="pill pill-red"><i class="bi bi-slash-circle-fill"></i>Inactive</span>';
}

const ACTION_PILLS = {
    CREATE: 'pill-green',
    UPDATE: 'pill-amber',
    DELETE: 'pill-red',
    LOGIN: 'pill-blue',
    LOGOUT: 'pill-neutral',
    READ: 'pill-cyan',
};

function actionPill(action) {
    return `<span class="pill ${ACTION_PILLS[action] || 'pill-neutral'}">${escapeHtml(action)}</span>`;
}

const DISCONNECT_LABELS = {
    USER_REQUEST: { pill: 'pill-neutral', text: 'User request' },
    TIMEOUT: { pill: 'pill-amber', text: 'Timeout' },
    SERVER_SHUTDOWN: { pill: 'pill-cyan', text: 'Server shutdown' },
    ERROR: { pill: 'pill-red', text: 'Error' },
    ADMIN_ACTION: { pill: 'pill-violet', text: 'Admin action' },
};

/** Session status derived from `disconnected_at` / `disconnect_reason`. */
function sessionStatusPill(session) {
    if (!session.disconnected_at) {
        return '<span class="pill pill-green"><i class="bi bi-broadcast"></i>Connected</span>';
    }
    const meta = DISCONNECT_LABELS[session.disconnect_reason] || {
        pill: 'pill-neutral',
        text: session.disconnect_reason || 'Disconnected',
    };
    return `<span class="pill ${meta.pill}">${escapeHtml(meta.text)}</span>`;
}

/* -------------------------------------------------------------------------
 * Rendering helpers
 * ----------------------------------------------------------------------- */

function emptyStateRow(colspan, title, text, icon = 'bi-inbox') {
    return `
        <tr>
            <td colspan="${colspan}">
                <div class="empty-state">
                    <span class="empty-state-icon"><i class="bi ${icon}"></i></span>
                    <p class="empty-state-title">${escapeHtml(title)}</p>
                    ${text ? `<p class="empty-state-text">${escapeHtml(text)}</p>` : ''}
                </div>
            </td>
        </tr>
    `;
}

function errorStateRow(colspan, message) {
    return `
        <tr>
            <td colspan="${colspan}">
                <div class="empty-state">
                    <span class="empty-state-icon text-danger"><i class="bi bi-exclamation-triangle"></i></span>
                    <p class="empty-state-title">Could not load data</p>
                    <p class="empty-state-text">${escapeHtml(message)}</p>
                </div>
            </td>
        </tr>
    `;
}

function skeletonRows(rows, columns) {
    const cells = Array.from({ length: columns }, () => '<td><span class="skeleton skeleton-text"></span></td>').join('');
    return Array.from({ length: rows }, () => `<tr>${cells}</tr>`).join('');
}

/**
 * Build a pagination control.
 *
 * `onPage` is the name of a global function taking the page number; it is
 * emitted into inline handlers, so pass a plain identifier.
 */
function renderPagination(container, { page, totalPages, total, label = 'items', onPage }) {
    const el = typeof container === 'string' ? document.getElementById(container) : container;
    if (!el) return;

    if (!totalPages || totalPages <= 1) {
        el.innerHTML = total ? `<span>${total} ${escapeHtml(label)}</span>` : '';
        return;
    }

    const pageButton = (n, text = String(n), disabled = false, active = false) => `
        <li class="page-item${disabled ? ' disabled' : ''}${active ? ' active' : ''}">
            <a class="page-link" href="#" onclick="${onPage}(${n}); return false;">${text}</a>
        </li>
    `;

    const start = Math.max(1, page - 2);
    const end = Math.min(totalPages, page + 2);
    let items = pageButton(page - 1, '<i class="bi bi-chevron-left"></i>', page <= 1);

    if (start > 1) {
        items += pageButton(1);
        if (start > 2) items += '<li class="page-item disabled"><span class="page-link">…</span></li>';
    }
    for (let i = start; i <= end; i++) items += pageButton(i, String(i), false, i === page);
    if (end < totalPages) {
        if (end < totalPages - 1) items += '<li class="page-item disabled"><span class="page-link">…</span></li>';
        items += pageButton(totalPages);
    }
    items += pageButton(page + 1, '<i class="bi bi-chevron-right"></i>', page >= totalPages);

    el.innerHTML = `
        <span>Page ${page} of ${totalPages} · ${total} ${escapeHtml(label)}</span>
        <nav aria-label="Pagination"><ul class="pagination pagination-sm">${items}</ul></nav>
    `;
}

/**
 * Filter already-rendered table rows against their `data-searchable` text.
 *
 * The list APIs have no server-side search parameter, so this narrows the
 * current page only — pages must label the control accordingly.
 */
function filterTableRows(tbodyId, query, noticeId) {
    const tbody = document.getElementById(tbodyId);
    if (!tbody) return;
    const needle = query.trim().toLowerCase();
    let visible = 0;

    tbody.querySelectorAll('tr[data-searchable]').forEach((row) => {
        const match = !needle || row.dataset.searchable.includes(needle);
        row.hidden = !match;
        if (match) visible++;
    });

    const notice = noticeId ? document.getElementById(noticeId) : null;
    if (notice) notice.classList.toggle('d-none', visible > 0);
}

/* -------------------------------------------------------------------------
 * Session
 * ----------------------------------------------------------------------- */

async function logout() {
    try {
        await fetch('/api/v1/auth/logout', { method: 'POST' });
    } catch {
        /* sign out locally regardless */
    }
    globalThis.location.href = '/login';
}

/* -------------------------------------------------------------------------
 * Theme
 * ----------------------------------------------------------------------- */

function currentTheme() {
    return document.documentElement.getAttribute('data-bs-theme') === 'dark' ? 'dark' : 'light';
}

function applyTheme(theme) {
    document.documentElement.setAttribute('data-bs-theme', theme);
    try {
        localStorage.setItem('ovpn.theme', theme);
    } catch {
        /* storage unavailable (private mode) — the theme still applies */
    }
    document.querySelectorAll('[data-theme-icon]').forEach((icon) => {
        icon.className = theme === 'dark' ? 'bi bi-sun' : 'bi bi-moon-stars';
    });
    document.dispatchEvent(new CustomEvent('themechange', { detail: { theme } }));
}

/* -------------------------------------------------------------------------
 * Boot
 * ----------------------------------------------------------------------- */

document.addEventListener('DOMContentLoaded', () => {
    applyTheme(currentTheme());

    document.querySelectorAll('[data-theme-toggle]').forEach((button) => {
        button.addEventListener('click', () => applyTheme(currentTheme() === 'dark' ? 'light' : 'dark'));
    });

    document.querySelectorAll('[data-avatar-name]').forEach((el) => {
        el.textContent = initials(el.dataset.avatarName);
    });

    document.querySelectorAll('[data-bs-toggle="tooltip"]').forEach((el) => new bootstrap.Tooltip(el));
});
