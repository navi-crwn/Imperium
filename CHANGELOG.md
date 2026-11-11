# Changelog

All notable changes to Imperium are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [1.2.0] - 2026-11-12

### Added
- Announcements, in the HOP ID section: a list (state, level, app, paging, filters), a page for each, create and edit forms (title, text, level, app, publish and expiry time in UTC, "publish now"), publish, unpublish and delete (destructive) from the Actions menu. Every change asks for a reason, is refused on the server without one, and is written to the audit trail. Owners and admins may change announcements; moderators and support read them. Nine languages. This is what HOP ID's old `/admin/announcements` page did, so that page can be retired once HOP ID serves the new API.
- `docs/announcements-contract.md`: the admin API HOP ID has to offer for announcements (list, show, create, update, publish/unpublish/delete), in the style of its user API, and the role-parity gaps that need a change in HOP ID. `scripts/fakehop` serves it (from `internal/fakehub`, which the tests use too).

### Changed
- Announcement forms follow HOP ID's order: "publish now" is applied first, so "publish now, ends next week" no longer needs a publish date. The acting person is sent to HOP ID in the shape it requires (a number without leading zeros, `:<username>` only when the name has no space or hyphen).
- Aligned with what HOP ID really does (announcements API, role rules): the API's time formats, the expiry check against the submitted publish time, 409 on a draft whose end is past. The user's Sessions tab lists the account's API tokens (name, last use, created). The extra token revocation after a suspension is no longer shown as a failure if HOP ID refuses it. The "not yourself" checks compare ids as numbers (`026` is `26`), one at a time and in bulk. The state and level filters of the announcements list show translated labels.
- Changing a person's roles is for owners only, as on HOP ID's own admin page (it was any admin). The roles form asks for a reason; ownership is shown but cannot be given here (set `SUPERADMIN_EMAIL` on HOP ID).
- Suspending a person (one at a time or in bulk) also revokes their API tokens, as HOP ID's own page does; it is a second, audited step.
- Suspending or deleting the account you are signed in with is refused.

## [1.1.0] - 2026-11-11

### Added
- PayHop joins the family: its mark, dashboard numbers (accounts and sessions; saved payment methods and bills are never shown), the PayHop card on a user, `fakehop` and the protocol note. The backup and restore scripts now cover the FileHop and PayHop databases and env files, and `tahap3-registry.sh` knows `payhop`.
- One admin frame for every app and for Users: a header (name, status, link), the same five tabs (Dashboard, Content, Reports, Settings, More), and three shared pages for lists, details and settings. The dashboard has fixed slots (up to five numbers, one chart, latest, open reports).
- Components: four button variants (primary, secondary, ghost, destructive), an Actions menu, a confirmation dialog that also works without JavaScript, a themed checkbox with a select-all that shows a partial state and a bar that counts the selection, a switch for on/off settings, and tabs.
- A user has tabs (Profile, Content, Sessions, Activity) and an Actions menu; paired actions (Suspend and Activate, Hide and Show) are one entry that fits the current state; each role has a one-line description.
- Long lists are paged (25 rows, Next at the top), Audit has one tab per source, and a list of an extension page can be searched and extended.
- `fakehop -bulk` serves long generated lists for layout checks.
- The sync script reads the hop-assets version from the manifest.
- Topbar: a compact language menu (globe + code, native names, Escape, click-outside and arrow keys), a one-button theme cycle (light, dark, system) and an account menu (profile, sign out), following the family chrome. Standard controls (8 px radius, 36 px high, 44 px on touch). They work without JavaScript through plain forms posting to `/prefs`.
- The language is now per reader: the `hop_lang` cookie, set by `POST /prefs`, picks one of the nine catalogue languages; `IMPERIUM_LANG` is only the default when there is no cookie (Telegram digests keep using it). The theme (`hop_theme`: light, dark, or system) is applied as `data-theme` on the page.
- Test that `appMark`, `kpiOrder` and the mark list in `scripts/sync-hop-assets.sh` match the embedded hop-assets manifest.

### Removed
- `scripts/deploy-notehop.sh`, an old deploy helper for a single app that nothing else called.

### Changed
- Every action on a row runs from its own form in a menu on that row; the bulk form no longer contains the table, so a row action and the bulk action never share fields. Actions that cannot be undone (delete, purge, take down, block an IP, clear custom CSS, release a slug) open a dialog that asks for a reason and, for accounts, the username; the server refuses them without the dialog's confirmation.
- The bulk bar appears over the table once rows are ticked and asks for confirmation and a reason in a dialog (the "I am sure" tick stays only without JavaScript).
- Tables keep one row per line, nothing overflows its box, the content area uses the screen, raw JSON wraps and stays folded, text is at least 12 px, and the sidebar runs the height of the page.
- Keys that apps report are labelled in the nine languages instead of printed raw.
- hop-assets v9 (manifest, floating tiles, marks) and the family table of the README.


### Fixed
- Every action in a row or detail Actions menu now opens the confirmation dialog with a required reason (only the irreversible ones are red), and the server refuses an empty reason for user, content and report actions, single and bulk, before anything is sent or logged; the reason of the bulk bar is required too.
- The extension actions `release`, `ban` and `remove` open a dialog (reason required) and need `ack=yes` on the server, like the other irreversible actions.
- An item URL that is not `http://` or `https://` is shown as plain text, not as a link; an item whose status is not a known one offers both halves of every pair (Hide and Show, ...); dialog ids carry the row index so two rows never share one.
- The action labels exist in all nine languages (they were only en and id).
- `tahap0-sudo.sh` checks the new sudoers file before it replaces the installed one, and the backup reads the service databases as the service user, not as root; if the install itself fails it says so, removes the temporary file and stops.

## [1.0.0] - 2026-11-10

### Added
- The sign-in page footer starts with the HOP ID mark and "Part of Hop Easy Link", linking to hel.ink.
- Nine languages: every catalog key exists in `en`, `zh-CN`, `de`, `id`, `ja`, `pt-BR`, `es`, `fr` and `ru`, and `IMPERIUM_LANG` selects any of them. Headings and field names that were written inside the templates (When, Actor, Owner, Email, Joined, Name, Status, and so on) now come from the catalog. A test checks that every language has every key.
- The user page and the role filter list `admin:<app>` for every app in the registry, plus any role the person already holds, so saving never drops a role the form does not know. Only an owner or a global admin can submit the form.
- An optional Clean interface style (flat neutral surfaces, solid bordered cards, quiet type and tabs), chosen on the account page. The HOP look stays the default.

### Changed
- New LinkHop and BioHop marks from hop-assets v6 (the embedded manifest and, where the app carries them, the home art and floating-tile copies).
- Controls follow the HOP control standard: one radius and one height for buttons, fields and search.
- Times are printed in UTC with the label "UTC" and a `datetime` attribute; a small script shows them in the reader's zone and names it. Without JavaScript the UTC text stays.
- The Settings table has a fixed layout (a label column, a field column capped at 30rem, a narrow column for numbers and switches) and stacks under 720 px. Descriptions are no longer printed in capitals. Without the right to save, the page says so and shows the values read-only.
- The bulk panel on the user, content and report lists starts on an empty action, and Apply stays disabled until a row is ticked and then names the count. The handlers count rows before judging the confirmation, so an empty submit is told "No rows selected."
- nginx sends each security header once.

### Fixed
- Signing out goes through HOP ID with `return=` set to Imperium's sign-in page, so the person ends up back on Imperium's own login.
- An `admin:<app>` no longer sees the whole hub's numbers (accounts, sign-ups, suspended accounts); HOP ID's `/stats` is fetched only for the hub-wide roles. Sync, Search and Settings are limited to the right roles.
- An alias stands in for the owner's username as well as the name and email: in the user list, on the user page and its title, and as the owner of content rows.
