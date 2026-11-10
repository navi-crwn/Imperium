// The times in the audit, user and report tables are written by the server in UTC.
// Each one carries its instant in a datetime attribute, so the browser can show it in
// the reader's own zone and say which zone it is. Without JavaScript the UTC text and
// its "UTC" label stay as they are, which is honest either way.
(function localiseTimes() {
    const times = document.querySelectorAll('time[datetime]');
    if (!times.length) return;
    let zone = '';
    try {
        zone = (Intl.DateTimeFormat().resolvedOptions().timeZone || '').replace(/_/g, ' ');
    } catch (e) { /* an old browser: the UTC label stays */ }
    const fmt = (iso) => {
        const d = new Date(iso);
        if (isNaN(d.getTime())) return null;
        const p = (n) => String(n).padStart(2, '0');
        return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
    };
    const utc = (iso) => {
        const d = new Date(iso);
        if (isNaN(d.getTime())) return null;
        const p = (n) => String(n).padStart(2, '0');
        return `${d.getUTCFullYear()}-${p(d.getUTCMonth() + 1)}-${p(d.getUTCDate())} ${p(d.getUTCHours())}:${p(d.getUTCMinutes())}`;
    };
    times.forEach((el) => {
        const iso = el.getAttribute('datetime');
        const local = fmt(iso);
        const z = el.parentElement && el.parentElement.querySelector('.time-zone');
        if (!local) return;
        el.textContent = local;
        el.title = `${utc(iso)} ${el.dataset.utcLabel || 'UTC'}`;
        if (z && zone) z.textContent = zone;
    });
})();
// The flat sidebar slides in on narrow screens and folds away on wide ones.
// The wide-screen preference is remembered per browser.
(function installSidebar() {
    const body = document.body;
    const side = document.getElementById('side');
    const toggle = document.querySelector('[data-side-toggle]');
    if (!side || !toggle) return;
    const narrow = window.matchMedia('(max-width: 900px)');
    const KEY = 'imperium.side';

    const sync = () => {
        const open = narrow.matches ? body.classList.contains('side-open') : !body.classList.contains('side-collapsed');
        toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
    };
    try {
        if (!narrow.matches && localStorage.getItem(KEY) === 'collapsed') body.classList.add('side-collapsed');
    } catch (e) { /* storage may be unavailable */ }
    sync();

    toggle.addEventListener('click', () => {
        if (narrow.matches) {
            body.classList.toggle('side-open');
        } else {
            const collapsed = body.classList.toggle('side-collapsed');
            try { localStorage.setItem(KEY, collapsed ? 'collapsed' : 'open'); } catch (e) { /* ignore */ }
        }
        sync();
    });
    document.querySelectorAll('[data-side-close]').forEach((el) => el.addEventListener('click', () => {
        body.classList.remove('side-open');
        sync();
    }));
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape' && body.classList.contains('side-open')) {
            body.classList.remove('side-open');
            sync();
        }
    });
    narrow.addEventListener?.('change', () => {
        body.classList.remove('side-open');
        sync();
    });
    // Keep the active item in view when the tree is long.
    const on = side.querySelector('.leaf.on, .node.on');
    if (on && typeof on.scrollIntoView === 'function') on.scrollIntoView({ block: 'nearest' });
})();
// These are page links, not ARIA tab panels. Keep native Tab/Enter navigation
// and add arrow/Home/End shortcuts within the single horizontal app bar.
document.querySelectorAll('.dash-tabs').forEach(nav => {
    nav.addEventListener('keydown', event => {
        if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
        const links = [...nav.querySelectorAll('a')];
        const current = links.indexOf(document.activeElement);
        if (current < 0) return;
        event.preventDefault();
        const index = event.key === 'Home' ? 0 : event.key === 'End' ? links.length - 1
            : (current + (event.key === 'ArrowRight' ? 1 : -1) + links.length) % links.length;
        links[index].focus();
        links[index].scrollIntoView({block: 'nearest', inline: 'nearest'});
    });
});
// Confirmation happens before submitting the existing, authorized action form.
(function installActionConfirmation() {
    const dialog = document.getElementById('action-confirm');
    if (!dialog) return;
    let pending = null;
    let approved = null;
    document.addEventListener('submit', event => {
        const button = event.submitter;
        if (!button || !button.classList.contains('danger') || dialog.contains(button)) return;
        if (approved === button) { approved = null; return; }
        event.preventDefault();
        pending = {form: event.target, button};
        dialog.querySelector('[data-confirm-label]').textContent = button.textContent.trim();
        dialog.returnValue = 'cancel';
        dialog.showModal();
    });
    dialog.addEventListener('close', () => {
        const action = pending;
        pending = null;
        if (!action) return;
        if (dialog.returnValue === 'confirm') {
            approved = action.button;
            action.form.requestSubmit(action.button);
        } else action.button.focus();
    });
})();
// Bulk selection on the reports queue, the content lists and the user list: one
// checkbox for the page, and a count on the submit button so a destructive action
// names how many rows it will touch. The button stays disabled until at least one
// row is picked, so an empty submit cannot reach the server at all (dashboard test
// 2026-11-08: "Apply" was live with 0 rows and the action defaulted to Suspend).
(function installBulkSelection() {
    const form = document.getElementById('bulk-form');
    const all = document.querySelector('[data-bulk-all]');
    const apply = document.querySelector('[data-bulk-apply]');
    const action = document.querySelector('[data-bulk-action]');
    if (!form) return;
    const picks = () => [...form.querySelectorAll('input.bulk-pick')];
    if (apply && !apply.dataset.label) apply.dataset.label = apply.textContent.trim();
    const sync = () => {
        const boxes = picks();
        const on = boxes.filter(b => b.checked);
        boxes.forEach(b => { b.closest('tr').classList.toggle('picked', b.checked); });
        if (all) {
            all.checked = boxes.length > 0 && on.length === boxes.length;
            all.indeterminate = on.length > 0 && on.length < boxes.length;
        }
        const chosen = !action || action.value !== '';
        if (action) action.classList.toggle('need-choice', !chosen);
        if (apply) {
            apply.disabled = on.length === 0;
            apply.textContent = on.length ? `${apply.dataset.label} (${on.length})` : apply.dataset.label;
            apply.title = on.length === 0 ? apply.dataset.empty : '';
        }
    };
    if (apply && !apply.dataset.empty) apply.dataset.empty = apply.getAttribute('title') || '';
    if (all) all.addEventListener('change', () => {
        picks().forEach(b => { b.checked = all.checked; });
        sync();
    });
    form.addEventListener('change', event => {
        if (event.target.classList.contains('bulk-pick')) sync();
        if (event.target === action) sync();
    });
    // A form submitted with the keyboard (Enter) never clicks the button: keep the
    // server-side refusal as the second line, this one as the first.
    form.addEventListener('submit', event => {
        if (picks().every(b => !b.checked)) event.preventDefault();
    });
    sync();
})();

// Interface style: a host-only cookie the server reads for the next page; applied now without a reload.
(() => {
    const pick = document.querySelector('[data-ui-select]');
    if (!pick) return;
    pick.addEventListener('change', () => {
        const clean = pick.value === 'clean';
        document.cookie = 'hop_ui=' + (clean ? 'clean' : 'hop') + '; path=/; max-age=31536000; SameSite=Lax' + (location.protocol === 'https:' ? '; Secure' : '');
        if (clean) document.documentElement.dataset.ui = 'clean'; else document.documentElement.removeAttribute('data-ui');
    });
})();

// Language and theme pickers save on change; without JavaScript the Apply button does the same.
(() => {
    document.documentElement.classList.add('js');
    document.querySelectorAll('select[data-pref-auto]').forEach((sel) => {
        sel.addEventListener('change', () => sel.form.submit());
    });
    document.addEventListener('click', (e) => {
        document.querySelectorAll('details.acct[open]').forEach((d) => { if (!d.contains(e.target)) d.removeAttribute('open'); });
    });
})();
