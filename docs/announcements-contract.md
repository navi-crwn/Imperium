# Hub admin API for announcements: contract

Status: Imperium implements the client side; `scripts/fakehop` and `internal/fakehub` implement this contract so the
pages are tested without HOP ID. HOP ID (`navi-crwn/HEL.ink`) has to answer exactly like `internal/fakehub`.

It follows HOP ID's existing admin API for users (`app/Http/Controllers/Api/HopAdminController.php`): same prefix,
same signature, same middleware, same envelope, same paging, same audit.

## Common rules

- Prefix `/api/hop`, inside the existing group `Route::middleware('hop.signed', 'throttle:600,1,hop')` and
  `Route::middleware('hop.admin_client')`: signed with the HOP App Protocol, only for an app with the
  `admin_client` capability (Imperium). Not reachable from a browser session.
- Request bodies are JSON. The acting person is the `X-HOP-Actor` header (`<sub>:<username>`, or `system`), as for users.
- Success: `{"ok": true, "data": ...}`; lists add `"page": {"next": "<cursor>"|null, "total": <int>}`.
- Error: `{"ok": false, "error": "<code>", "message": "<text>"}` with the status below.
- Audit: every write is recorded in `hop_audits` like a user action: `actor`, `app_slug` (the calling app),
  `action` (`announcement.create`, `announcement.update`, `announcement.publish`, `announcement.unpublish`,
  `announcement.delete`), `target_type` = `announcement`, `target_id`, `reason` (first 500 characters), `params`
  (create/update: the validated fields; delete: `title`). They show in `GET /api/hop/audit` like the rest.
- The announcement model is unchanged: `title` (≤160), `body` (≤5000), `level` (`info`, `warning`, `critical`),
  `app_slug` (null = HOP as a whole, else a slug of `HopApps::all()`), `published_at`, `expires_at`, `author_id`.
  No new column or table.
- Times go in and out as ISO 8601 / RFC 3339 and are stored as UTC (`2026-12-01T09:30:00Z`).

### The announcement object (`row`)

```json
{
  "id": "12",
  "title": "Planned maintenance on Saturday",
  "body": "PixelHop is read-only for about 20 minutes from 02:00 UTC.",
  "level": "info",
  "app_slug": "pixelhop",
  "state": "scheduled",
  "published_at": "2026-11-14T02:00:00Z",
  "expires_at": "2026-11-15T02:00:00Z",
  "author": "1:superadmin",
  "created_at": "2026-11-11T10:00:00Z",
  "updated_at": "2026-11-11T10:00:00Z"
}
```

- `id` is a string, like `sub` for users. `app_slug`, `published_at`, `expires_at`, `author` are `null` when empty.
- `state` is computed on the hub at request time: `draft` (`published_at` null), `scheduled` (`published_at` in the
  future), `expired` (published, `expires_at` not in the future), else `live` (the model's `isLive()`).
- `author` is `author_id`'s `<id>:<username>` when the user still exists, else the actor string that created it, else null.

## Calls

### 1. List: `GET /api/hop/announcements?state=&level=&cursor=&limit=`

- `state` one of `draft`, `scheduled`, `live`, `expired` (anything else is ignored); `level` one of the three levels
  (anything else is ignored).
- Newest first by `id`. `cursor` is the last `id` of the previous page (rows with a lower id). `limit` 1–200, default 50.
- `page.next` is the last `id` of this page when the page is full (`count === limit`), else `null`; `page.total` is the
  number of rows matching the filters, ignoring the cursor. Same as `GET /api/hop/users`.
- 200 `{"ok": true, "data": [row, ...], "page": {"next": "6", "total": 30}}`.

### 2. Show: `GET /api/hop/announcements/{id}`

- 200 `{"ok": true, "data": row}`.
- 404 `{"ok": false, "error": "not_found", "message": "No such announcement."}` (as users do, via `->missing()`).

### 3. Create: `POST /api/hop/announcements`

Request:

```json
{
  "title": "Maintenance",
  "body": "Line one\nLine two",
  "level": "warning",
  "app_slug": "pixelhop",
  "published_at": "2026-12-01T09:30:00Z",
  "expires_at": "2026-12-02T09:30:00Z",
  "publish_now": false,
  "reason": "planned work"
}
```

- `title`, `body`, `level` are required. `app_slug`, `published_at`, `expires_at` may be `null` or missing. `published_at`
  null = draft.
- `publish_now: true` sets `published_at` to the current time and wins over `published_at` (as the hub's form does).
- `reason` is optional on the hub (stored as for users); Imperium always sends it (≤255 characters).
- 201 `{"ok": true, "data": row}`; `author_id` is the actor's id when the actor is an existing user (same rule as
  `set_roles`' `granted_by`).
- 422 `bad_params` with `message` naming the first problem, for: missing/blank or too long `title` or `body`; `level` not in
  `Announcement::LEVELS`; `app_slug` neither empty nor a slug of `HopApps::all()`; a time that is not ISO 8601;
  `expires_at` that is not after `published_at` (an `expires_at` without a `published_at` is refused too).
  Messages: `title is required`, `title may not be longer than 160 characters`, `body is required`,
  `body may not be longer than 5000 characters`, `level must be one of info, warning, critical`,
  `app_slug must be empty or the slug of a registered app`, `published_at must be an ISO 8601 date and time`,
  `expires_at must be an ISO 8601 date and time`, `expires_at must be after published_at`.

### 4. Update: `PUT /api/hop/announcements/{id}`

- Same body, same rules, same 422s as create. It replaces every editable field: a missing or `null` `app_slug`,
  `published_at` or `expires_at` clears it (Imperium's edit form always sends the whole announcement).
- 200 `{"ok": true, "data": row}`; 404 `not_found`.

### 5. Publish state and delete: `POST /api/hop/announcements/{id}/actions`

Like `POST /api/hop/users/{user}/actions`: `{"action": "publish" | "unpublish" | "delete", "reason": "..."}`.

- `publish`: a `draft` or `scheduled` announcement gets `published_at` = now; a `live` one is left as it is (200, no
  change); an `expired` one answers 409 `expired` `This announcement has expired; change its expiry date first.`
- `unpublish`: sets `published_at` to null (back to a draft), whatever the state; a draft stays a draft (200).
- `delete`: removes the row. 200 `{"ok": true, "data": row + {"deleted": true}}` (the row as it was).
- 200 `{"ok": true, "data": row}` for the first two.
- 422 `unknown_action` `action must be one of publish, unpublish, delete`; 404 `not_found`.

### Errors and limits (all calls)

| Status | `error` | When |
|---|---|---|
| 401 | (signature errors, as today) | bad or stale signature, replayed nonce |
| 403 | `forbidden` | the calling app has no `admin_client` capability (middleware, as today) |
| 404 | `not_found` | no such announcement |
| 409 | `expired` | publish of an expired announcement |
| 422 | `bad_params`, `unknown_action` | validation, see above |
| 429 | (throttle, as today) | more than 600 requests a minute per app |

Limits: the list page size is at most 200; `title` 160, `body` 5000, `reason` 500 stored.

## Role parity: what needs a change in HOP ID

Imperium closes every gap its own side can close (owner-only role changes, no ownership grant, no self-suspend or
self-delete, tokens taken on suspend). These remain on the HOP ID side:

1. **`set_roles` accepts `owner`** (`HopAdminController::act`, case `set_roles`), while the hub's own page refuses
   ("Ownership is set with SUPERADMIN_EMAIL or hop:owner"). Imperium refuses it before asking; the API should too:
   reject a role list that adds `owner` to a user who does not hold it (422 `bad_params`).
2. **No check of who acts.** `act()` trusts the app; the hub's page requires `isOwner()` for roles and refuses
   suspending/deleting oneself. The API only knows `X-HOP-Actor`. It should look the actor up: `set_roles` only if the
   actor is an owner; `suspend` and `delete` refused (403) when the actor id equals the target id.
3. **`suspend` does not delete the user's API tokens**, the hub's page does (`$user->apiTokens()->delete()`).
   Imperium sends `revoke_tokens` right after `suspend`; doing it inside `suspend` makes it atomic.
4. **The user detail has less than the hub's page**: `GET /api/hop/users/{user}` returns the number of API tokens, not
   the list (name, last used) the hub's show page lists, and 5 logins instead of 10. Imperium shows what the API gives.
5. **Activity log**: the hub's page writes `ActivityLog` rows (`suspended`, `activated`); the API writes `hop_audits`
   only. Not a gap for Imperium (its audit page reads `hop_audits`), but retiring `/admin` drops the `ActivityLog` entries.
