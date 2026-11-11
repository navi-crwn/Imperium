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
    const bar = document.querySelector('[data-bulk-bar]');
    const reason = document.getElementById('bulk-reason');
    const confirmBox = document.querySelector('[data-bulk-confirm]');
    const count = document.querySelector('[data-bulk-count]');
    if (!form) return;
    const picks = () => [...document.querySelectorAll('input.bulk-pick')];
    if (apply && !apply.dataset.label) apply.dataset.label = apply.textContent.trim();
    const sync = () => {
        const boxes = picks();
        const on = boxes.filter(b => b.checked);
        boxes.forEach(b => { b.closest('tr').classList.toggle('picked', b.checked); });
        if (all) {
            all.checked = boxes.length > 0 && on.length === boxes.length;
            all.indeterminate = on.length > 0 && on.length < boxes.length;
        }
        if (bar) bar.classList.toggle('is-idle', on.length === 0);
        if (count) count.textContent = String(on.length);
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
    document.addEventListener('change', event => {
        if (event.target.classList && event.target.classList.contains('bulk-pick')) sync();
        if (event.target === action) sync();
    });
    // A form submitted with the keyboard (Enter) never clicks the button: keep the
    // server-side refusal as the second line, this one as the first.
    let approved = false;
    form.addEventListener('submit', event => {
        if (picks().every(b => !b.checked)) { event.preventDefault(); return; }
        if (approved || !bar || !action || action.value === '') return; // no action chosen: the server says so
        event.preventDefault();
        const text = (bar.dataset.confirmTitle || '').replace('{action}', action.options[action.selectedIndex].text).replace('{count}', String(picks().filter(b => b.checked).length));
        const dlg = document.createElement('dialog');
        dlg.className = 'confirm';
        const f = document.createElement('form'); f.method = 'dialog';
        const h = document.createElement('h2'); h.textContent = text; f.append(h);
        const lab = document.createElement('label'); lab.textContent = bar.dataset.reason || ''; lab.htmlFor = 'bulk-dlg-reason'; f.append(lab);
        const inp = document.createElement('input'); inp.type = 'text'; inp.id = 'bulk-dlg-reason'; inp.required = true; inp.maxLength = 255; inp.value = reason ? reason.value : ''; f.append(inp);
        const row = document.createElement('div'); row.className = 'dlg-actions';
        const no = document.createElement('button'); no.type = 'button'; no.className = 'btn ghost'; no.textContent = bar.dataset.cancel || 'Cancel'; no.addEventListener('click', () => dlg.close());
        const ok = document.createElement('button'); ok.type = 'submit'; ok.className = 'btn destructive'; ok.textContent = bar.dataset.ok || 'OK'; row.append(no, ok); f.append(row); dlg.append(f);
        f.addEventListener('submit', e => { e.preventDefault(); if (!inp.value.trim()) { inp.focus(); return; } if (reason) reason.value = inp.value; if (confirmBox) confirmBox.checked = true; approved = true; dlg.close(); form.requestSubmit(apply); });
        dlg.addEventListener('close', () => dlg.remove());
        document.body.append(dlg); dlg.showModal(); inp.focus();
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

// Language and account menus are <details> (they work without JavaScript). This adds
// Escape, click-outside and arrow-key navigation.
(() => {
    const open = () => document.querySelectorAll('details.pop[open]');
    document.addEventListener('click', (e) => {
        open().forEach((d) => { if (!d.contains(e.target)) d.removeAttribute('open'); });
    });
    document.addEventListener('keydown', (e) => {
        const d = e.target.closest ? e.target.closest('details.pop') : null;
        if (e.key === 'Escape') {
            const o = d && d.open ? d : open()[0];
            if (o) { o.removeAttribute('open'); const s = o.querySelector('summary'); if (s) s.focus(); }
            return;
        }
        if (!d || !d.open || ['ArrowDown', 'ArrowUp', 'Home', 'End'].indexOf(e.key) < 0) return;
        const items = Array.from(d.querySelectorAll('.pop-item, .acct-menu a, .acct-menu button'));
        if (!items.length) return;
        e.preventDefault();
        let i = items.indexOf(document.activeElement);
        if (e.key === 'ArrowDown') i = i < 0 ? 0 : (i + 1) % items.length;
        else if (e.key === 'ArrowUp') i = i < 0 ? items.length - 1 : (i - 1 + items.length) % items.length;
        else i = e.key === 'Home' ? 0 : items.length - 1;
        items[i].focus();
    });
    document.querySelectorAll('details.pop').forEach((d) => d.addEventListener('toggle', () => {
        if (!d.open) return;
        open().forEach((o) => { if (o !== d) o.removeAttribute('open'); });
        const f = d.querySelector('.pop-item[aria-current="true"], .pop-item');
        if (f && d.classList.contains('lang-menu')) f.focus();
    }));
})();
// Marks the page as scripted (CSS hides the no-JavaScript fallbacks), and opens the confirmation dialogs
// that the Actions menus link to. Without JavaScript the same links show the dialog through :target.
(() => {
    document.documentElement.classList.add('js');
    document.addEventListener('click', (e) => {
        const t = e.target.closest ? e.target.closest('[data-dialog], [data-dialog-close]') : null;
        if (!t) return;
        e.preventDefault();
        if (t.hasAttribute('data-dialog-close')) { const d = t.closest('dialog'); if (d) d.close(); if (location.hash) history.replaceState(null, '', location.pathname + location.search); return; }
        const d = document.getElementById(t.getAttribute('data-dialog'));
        if (!d || typeof d.showModal !== 'function') return;
        const menu = t.closest('details'); if (menu) menu.removeAttribute('open');
        d.showModal();
        const first = d.querySelector('input[type=text]'); if (first) first.focus();
    });
})();
