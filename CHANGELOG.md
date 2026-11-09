# Changelog

All notable changes to Imperium are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [1.0.0] - 2026-11-10

### Added

- Nine languages: every catalog key exists in `en`, `zh-CN`, `de`, `id`, `ja`, `pt-BR`, `es`, `fr` and `ru`, and `IMPERIUM_LANG` selects any of them. Headings and field names that were written inside the templates (When, Actor, Owner, Email, Joined, Name, Status, and so on) now come from the catalog. A test checks that every language has every key.
- The user page and the role filter list `admin:<app>` for every app in the registry, plus any role the person already holds, so saving never drops a role the form does not know. Only an owner or a global admin can submit the form.
- An optional Clean interface style (flat neutral surfaces, solid bordered cards, quiet type and tabs), chosen on the account page. The HOP look stays the default.

### Changed

- Controls follow the HOP control standard: one radius and one height for buttons, fields and search.
- Times are printed in UTC with the label "UTC" and a `datetime` attribute; a small script shows them in the reader's zone and names it. Without JavaScript the UTC text stays.
- The Settings table has a fixed layout (a label column, a field column capped at 30rem, a narrow column for numbers and switches) and stacks under 720 px. Descriptions are no longer printed in capitals. Without the right to save, the page says so and shows the values read-only.
- The bulk panel on the user, content and report lists starts on an empty action, and Apply stays disabled until a row is ticked and then names the count. The handlers count rows before judging the confirmation, so an empty submit is told "No rows selected."
- nginx sends each security header once.

### Fixed

- An `admin:<app>` no longer sees the whole hub's numbers (accounts, sign-ups, suspended accounts); HOP ID's `/stats` is fetched only for the hub-wide roles. Sync, Search and Settings are limited to the right roles.
- An alias stands in for the owner's username as well as the name and email: in the user list, on the user page and its title, and as the owner of content rows.
