# Phase 2 EPD: Interactive Work Item Loop

Status: draft for technical and resilience review
Date: 2026-05-02
Owner: Engineering

## Goal

Deliver the first complete interactive local workspace loop: a user can create a work item, tag it, comment on it, see the resulting activity, and restart the app without losing state.

This phase should make the existing shell feel like a real local workspace, not a static prototype. The target user is a solo builder or small team member who needs a lightweight place to track active work and preserve context while agents remain out of scope.

## Current Baseline

The public repo already has:

- Go daemon with SQLite-backed project, issue, and agent CRUD.
- Issue list/create/get/update/delete/status HTTP endpoints.
- Generated issue `identifier` and `position` fields per project.
- Migration runner and daemon lifecycle tests.
- React desktop shell with sidebar, detail pane, and timeline components.
- Frontend API helpers for project, issue, agent, and health endpoints.

The desktop UI still renders static issue/detail/timeline data, and the daemon does not yet model tags, comments, or activity.

## User Loop

1. User opens the desktop app and sees projects plus persisted work items from the daemon.
2. User creates a work item from the sidebar or list surface.
3. The new item appears immediately, is selected, and remains after reload.
4. User adds or removes tags on the selected item.
5. User adds a comment.
6. The detail pane and activity rail update from daemon data.
7. User restarts the daemon/app and sees the same item, tags, comments, and activity in stable order.

## Scope

### Data Model

Add a migration after the current issue contract migration.

`issue_tags`

| Column | Type | Notes |
| --- | --- | --- |
| `id` | TEXT PRIMARY KEY | UUIDv7 |
| `issue_id` | TEXT NOT NULL | FK to `issues(id)` with `ON DELETE CASCADE` |
| `name` | TEXT NOT NULL | Trimmed display label |
| `color` | TEXT NOT NULL DEFAULT '' | UI color token or hex string; empty means default |
| `created_at` | TEXT NOT NULL | RFC3339 UTC |

Constraints and indexes:

- `UNIQUE(issue_id, name COLLATE NOCASE)` so one item cannot carry duplicate tag labels.
- `idx_issue_tags_issue_id` on `issue_id`.
- Reject empty names after trimming.

`issue_comments`

| Column | Type | Notes |
| --- | --- | --- |
| `id` | TEXT PRIMARY KEY | UUIDv7 |
| `issue_id` | TEXT NOT NULL | FK to `issues(id)` with `ON DELETE CASCADE` |
| `author_id` | TEXT NOT NULL DEFAULT '' | Human/agent identity when available; empty allowed for local-only v0 |
| `author_name` | TEXT NOT NULL DEFAULT 'You' | Display name snapshot |
| `body` | TEXT NOT NULL | Markdown/plain text body for this phase |
| `created_at` | TEXT NOT NULL | RFC3339 UTC |
| `updated_at` | TEXT NOT NULL | RFC3339 UTC |

Constraints and indexes:

- Reject empty body after trimming.
- `idx_issue_comments_issue_created` on `(issue_id, created_at, id)`.

`issue_activity`

| Column | Type | Notes |
| --- | --- | --- |
| `id` | TEXT PRIMARY KEY | UUIDv7 |
| `issue_id` | TEXT NOT NULL | FK to `issues(id)` with `ON DELETE CASCADE` |
| `type` | TEXT NOT NULL | `issue.created`, `issue.updated`, `status.changed`, `tag.added`, `tag.removed`, `comment.added` |
| `actor_id` | TEXT NOT NULL DEFAULT '' | Empty allowed in local v0 |
| `actor_name` | TEXT NOT NULL DEFAULT 'You' | Display snapshot |
| `summary` | TEXT NOT NULL | Short UI-ready sentence |
| `metadata_json` | TEXT NOT NULL DEFAULT '{}' | Event-specific fields |
| `created_at` | TEXT NOT NULL | RFC3339 UTC |

Constraints and indexes:

- Validate known `type` values in repository code.
- `idx_issue_activity_issue_created` on `(issue_id, created_at, id)`.

`issue_decisions` is optional in this phase and should only be added as a hidden stub if it prevents a near-term migration conflict. Do not expose decision UI in this slice.

### Store Layer

Add repositories for tags, comments, and activity. Keep activity writes in the same transaction as the user-visible mutation that caused them:

- Create issue inserts `issues` row and `issue_activity(issue.created)`.
- Update issue inserts activity for meaningful field changes only.
- Update status inserts `status.changed` with old/new status metadata.
- Add tag inserts tag and `tag.added`.
- Remove tag deletes tag and inserts `tag.removed`.
- Add comment inserts comment and `comment.added`.

Use repository methods that return the persisted row as read back from SQLite. Do not assemble response objects from unsaved input.

### API Contract

All JSON responses use the existing direct resource style: successful responses return the resource or array directly; errors use the existing `response.Error` shape.

#### Issue Summary

Existing issue endpoints should return an expanded issue shape:

```json
{
  "id": "uuid",
  "project_id": "uuid",
  "identifier": "ISSUE-1",
  "position": 1,
  "title": "Draft launch checklist",
  "description": "Markdown text",
  "status": "todo",
  "priority": "medium",
  "assignee_id": "",
  "creator_id": "",
  "tags": [
    { "id": "uuid", "issue_id": "uuid", "name": "Launch", "color": "", "created_at": "2026-05-02T00:00:00Z" }
  ],
  "comment_count": 2,
  "last_activity_at": "2026-05-02T00:00:00Z",
  "created_at": "2026-05-02T00:00:00Z",
  "updated_at": "2026-05-02T00:00:00Z"
}
```

List endpoints may omit full `comments` and `activity` arrays but must include `tags`, `comment_count`, and `last_activity_at` so the sidebar can render without N+1 calls.

#### Issue Detail

`GET /api/issues/{id}` returns the issue summary plus:

```json
{
  "comments": [
    {
      "id": "uuid",
      "issue_id": "uuid",
      "author_id": "",
      "author_name": "You",
      "body": "Comment body",
      "created_at": "2026-05-02T00:00:00Z",
      "updated_at": "2026-05-02T00:00:00Z"
    }
  ],
  "activity": [
    {
      "id": "uuid",
      "issue_id": "uuid",
      "type": "comment.added",
      "actor_id": "",
      "actor_name": "You",
      "summary": "You commented",
      "metadata": { "comment_id": "uuid" },
      "created_at": "2026-05-02T00:00:00Z"
    }
  ]
}
```

#### New Endpoints

`POST /api/issues/{id}/tags`

Request:

```json
{ "name": "Launch", "color": "" }
```

Responses:

- `201` with tag resource.
- `400 BAD_REQUEST` for invalid JSON or empty name.
- `404 NOT_FOUND` when issue does not exist.
- `409 CONFLICT` when the tag already exists on the issue.

`DELETE /api/issues/{id}/tags/{tag_id}`

Responses:

- `204` on success.
- `404 NOT_FOUND` when issue or tag does not exist.

`GET /api/issues/{id}/comments`

Returns comments ordered by `(created_at, id)` ascending.

`POST /api/issues/{id}/comments`

Request:

```json
{ "body": "Comment body", "author_id": "", "author_name": "You" }
```

Responses:

- `201` with comment resource.
- `400 BAD_REQUEST` for invalid JSON or empty body.
- `404 NOT_FOUND` when issue does not exist.

`GET /api/issues/{id}/activity`

Returns activity ordered by `(created_at, id)` ascending.

#### Validation

Keep status and priority validation intentionally small for this loop:

- Status values accepted by create/update/status endpoints: `todo`, `in_progress`, `done`, `blocked`.
- Priority values accepted by create/update endpoints: `low`, `medium`, `high`.
- Empty status still defaults to `todo`; empty priority still defaults to `medium`.
- Invalid values return `400 BAD_REQUEST` before any SQLite write or activity insert.

### Frontend Integration

Replace static desktop data with daemon-backed state.

- Introduce a workspace data hook responsible for projects, selected project, issues, selected issue detail, loading state, retry, and mutations.
- Sidebar:
  - Render daemon projects and issues.
  - Provide create work item action with title, optional description, priority, and status defaults.
  - Keep selected issue stable after refetch.
- Detail pane:
  - Render selected issue fields from daemon data.
  - Support status and priority updates with optimistic or fast refetch behavior.
  - Render tag chips with add/remove controls.
  - Render comments with a composer.
  - Show polished empty states when no project, no issue, no comments, or no tags exist.
- Timeline rail:
  - Render issue activity from daemon data.
  - Fall back to an empty activity state for new or failed loads.
- Error/retry:
  - Use existing daemon connection and error panel patterns.
  - Expose retry for project/issue/detail fetch failures.
  - Do not leave the user with a blank three-panel layout.

The first screen should remain the workspace itself. Do not add a marketing page or setup wizard.

### Persistence Guarantee

The acceptance guarantee is daemon-level, not React-local:

- Every create/tag/comment/activity mutation must write to SQLite before the API returns success.
- Related mutations and activity rows must commit atomically.
- On daemon restart with the same data directory, project issues, tags, comments, and activity must reload through the public HTTP API.
- UI tests can mock the API, but at least one Go E2E test must start the daemon twice against the same temp data directory and verify the persisted loop through HTTP.

## Tests

### Go

- Migration test verifies new tables, indexes, foreign keys, uniqueness, and migration ordering.
- Store tests cover create issue activity, add/remove tag, duplicate tag conflict, add comment, cascade delete, and activity ordering.
- Handler contract tests cover success and error responses for tags, comments, and activity.
- E2E test:
  1. Start daemon with temp data dir.
  2. Create project.
  3. Create issue.
  4. Add tag.
  5. Add comment.
  6. Fetch issue detail and verify tags/comments/activity.
  7. Stop daemon.
  8. Restart daemon with same data dir.
  9. Fetch issue detail again and verify state survived restart.

### Frontend

- API helper tests for new tag/comment/activity endpoints.
- App/component tests with mocked daemon responses for:
  - loading state
  - empty project/list/detail states
  - create work item flow
  - tag add/remove flow
  - comment composer flow
  - retry after failed fetch
- Keep layout tests focused on visible user behavior, not implementation-specific component names.

### Manual Verification

- Run daemon locally and desktop dev server against it.
- Create a project if none exists.
- Create a work item.
- Add at least two tags and remove one.
- Add a comment.
- Confirm activity rail records creation, tag, comment, and status changes.
- Restart daemon with the same data dir and verify the same state appears.

## Execution Tickets

Create implementation tickets after EPD approval in this order:

1. Daemon migration and store layer for tags, comments, and activity.
2. HTTP API contract expansion for issue detail, tag, comment, and activity endpoints.
3. Desktop data integration replacing static issue/detail/timeline data.
4. Polished empty/error/retry states and interaction finish.
5. Persistence E2E and manual verification pass.

Keep each ticket narrow enough to review independently. The daemon contract tickets should land before the desktop depends on them.

## Out of Scope

- Agent runtime adapters, task execution, streaming output, provider credentials, or workdir orchestration.
- Kanban board mechanics, drag/drop columns, sprint planning, or backlog workflow depth.
- Rich document editing, Tiptap, ProseMirror collaboration, or markdown revision history.
- Cloud sync, multiplayer, auth, invites, or account billing.
- Decision timeline UI beyond activity events required for this loop.
- Notification systems or external integrations.

## Risks And Mitigations

- Risk: activity can drift from the mutation it describes.
  Mitigation: write activity rows in the same SQLite transaction as the mutation.

- Risk: expanded issue detail creates N+1 query behavior.
  Mitigation: list issues with tags/comment counts/last activity via batched queries or joins; only fetch full comments/activity for the selected issue.

- Risk: UI becomes Kanban-shaped because the model is named `Issue`.
  Mitigation: user-facing copy should prefer "work item" where possible, and the first loop remains list/detail/activity.

- Risk: duplicate tags vary by case or whitespace.
  Mitigation: trim names in handlers/repositories and enforce case-insensitive uniqueness per issue.

- Risk: tests pass with in-memory state while restart persistence is broken.
  Mitigation: require the daemon restart E2E test against a temp data directory backed by SQLite.

## Approval Criteria

The EPD is ready for execution breakdown when reviewers agree that:

- Data model covers tags, comments, and activity without committing to Kanban or agent runtime scope.
- API contracts are explicit enough for daemon and desktop work to proceed independently.
- Persistence/reload guarantee is testable through HTTP and SQLite restart behavior.
- Frontend plan replaces static data with real daemon state and includes empty/error/retry states.
- Execution tickets are narrow and ordered by dependency.
