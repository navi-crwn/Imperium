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
// Bulk selection on the reports queue: one checkbox for the page, and a count on
// the submit button so a destructive action names how many rows it will touch.
(function installBulkSelection() {
    const form = document.getElementById('bulk-form');
    const all = document.querySelector('[data-bulk-all]');
    const apply = document.querySelector('[data-bulk-apply]');
    if (!form) return;
    const picks = () => [...form.querySelectorAll('input.bulk-pick')];
    const sync = () => {
        const boxes = picks();
        const on = boxes.filter(b => b.checked);
        picks().forEach(b => { b.closest('tr').classList.toggle('picked', b.checked); });
        if (all) {
            all.checked = boxes.length > 0 && on.length === boxes.length;
            all.indeterminate = on.length > 0 && on.length < boxes.length;
        }
        if (apply) apply.textContent = on.length ? `${apply.dataset.label} (${on.length})` : apply.dataset.label;
    };
    if (apply && !apply.dataset.label) apply.dataset.label = apply.textContent.trim();
    if (all) all.addEventListener('change', () => {
        picks().forEach(b => { b.checked = all.checked; });
        sync();
    });
    form.addEventListener('change', event => {
        if (event.target.classList.contains('bulk-pick')) sync();
    });
    sync();
})();
