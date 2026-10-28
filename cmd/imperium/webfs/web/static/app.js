// The explorer sidebar: a hamburger slides it in on narrow screens and folds it
// away on wide ones (remembered per browser). The app folders are <details>
// elements, so expanding and collapsing them needs no script.
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
