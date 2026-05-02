# Phase 2 EPD: Interactive Work Item Loop

## Summary

Phase 2 turns the current static issue shell into a real local-first work item loop backed by the daemon and SQLite. Users should be able to open the desktop app, select a project, create an issue, tag it, comment on it, see the resulting activity stream, restart the daemon/app, and find the same work item state intact.

This phase deliberately stops short of agent runtime execution, Kanban workflows, realtime collaboration, or document editing. The goal is a trustworthy CRUD loop that makes the workspace feel alive without expanding beyond the foundation surface.

## Current State

- The daemon already persists projects, agents, and issues in SQLite.
- Issue API supports project-scoped list/create plus get/update/delete/status update.
- Issues have generated `identifier` and `position` fields per project.
- Desktop UI has a three-panel layout, but sidebar/detail/timeline data is hard-coded.
- There is no tag, comment, activity, or decision model yet.

## Goals

1. Add issue tags, issue comments, and issue activity events to the daemon data model.
2. Expose the create/read/update flow needed by the desktop app with stable JSON contracts.
3. Replace static desktop issue data with real daemon data in sidebar, detail, and activity panels.
4. Guarantee restart/reload persistence for created issues, tags, comments, and activity.
5. Provide polished empty, loading, error, and retry states for the work item loop.
6. Add focused daemon and desktop tests for the full loop.

## Non-Goals

- No agent runtime adapters, task execution, terminal process orchestration, streaming output, or per-task workdirs.
- No Kanban board, drag/drop columns, sprint planning, estimates, dependencies, or workflow automation.
- No realtime multi-user sync, sockets, CRDTs, or collaboration cursors.
- No markdown document editor, Tiptap integration, or revision history.
- No cloud sync, auth, permissions, billing, or workspace membership model.
- No broad redesign of the existing three-panel app shell.

## Data Model

Add a migration after the current issue contract migration.

### `tags`

Workspace/project-scoped reusable labels.

```sql
CREATE TABLE tags (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    name TEXT NOT NULL,
    color TEXT NOT NULL DEFAULT '#71717a',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    UNIQUE (project_id, name)
);

CREATE INDEX idx_tags_project_id ON tags(project_id);
```

Validation:

- `name`: trim whitespace, required, max 40 chars.
- `color`: required hex color in `#rrggbb`; daemon defaults to neutral gray when omitted.
- Names are unique case-insensitively at the service layer by normalizing comparisons with `LOWER(name)`.

### `issue_tags`

Many-to-many join between issues and tags.

```sql
CREATE TABLE issue_tags (
    issue_id TEXT NOT NULL,
    tag_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (issue_id, tag_id),
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE INDEX idx_issue_tags_tag_id ON issue_tags(tag_id);
```

### `issue_comments`

Human-authored work item discussion entries. Agent-authored comments are represented as plain author metadata for now; no runtime integration is implied.

```sql
CREATE TABLE issue_comments (
    id TEXT PRIMARY KEY,
    issue_id TEXT NOT NULL,
    author_id TEXT NOT NULL DEFAULT '',
    author_name TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE
);

CREATE INDEX idx_issue_comments_issue_id_created_at
    ON issue_comments(issue_id, created_at);
```

Validation:

- `body`: trim surrounding whitespace, required, max 20,000 chars.
- `author_id` and `author_name`: optional strings; preserve as display metadata only.
- Edits update `updated_at`; deletion is hard-delete in Phase 2.

### `issue_activity`

Append-only timeline events for work item actions.

```sql
CREATE TABLE issue_activity (
    id TEXT PRIMARY KEY,
    issue_id TEXT NOT NULL,
    actor_id TEXT NOT NULL DEFAULT '',
    actor_name TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL,
    summary TEXT NOT NULL,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE
);

CREATE INDEX idx_issue_activity_issue_id_created_at
    ON issue_activity(issue_id, created_at);
```

Allowed `type` values for Phase 2:

- `issue.created`
- `issue.updated`
- `issue.status_changed`
- `issue.assignee_changed`
- `tag.added`
- `tag.removed`
- `comment.created`
- `comment.updated`
- `comment.deleted`

Activity is written by daemon repository methods in the same transaction as the user-visible mutation. If activity insertion fails, the whole mutation fails; the UI should never see a tag/comment mutation without the corresponding timeline event.

### Optional Decision Stub

Do not build decision workflows in Phase 2. Reserve an optional nullable `decision_id` in `issue_activity.metadata_json` only when a future action explicitly references a decision. Do not create a `decisions` table yet.

## Daemon Repository Plan

Add focused repositories rather than expanding `IssueRepository` into a catch-all:

- `TagRepository`
  - `ListByProject(projectID string) ([]*Tag, error)`
  - `Create(projectID, name, color string) (*Tag, error)`
  - `Update(tag *Tag) error`
  - `Delete(id string) error`
- `IssueTagRepository`
  - `ListForIssue(issueID string) ([]*Tag, error)`
  - `Add(issueID, tagID, actorID, actorName string) error`
  - `Remove(issueID, tagID, actorID, actorName string) error`
- `CommentRepository`
  - `ListByIssue(issueID string) ([]*IssueComment, error)`
  - `Create(issueID, authorID, authorName, body string) (*IssueComment, error)`
  - `Update(commentID, body string) (*IssueComment, error)`
  - `Delete(commentID string) error`
- `ActivityRepository`
  - `ListByIssue(issueID string) ([]*IssueActivity, error)`
  - `Append(tx txLike, event IssueActivityCreate) (*IssueActivity, error)`

Use transaction helpers for composite mutations:

- Issue create writes `issue.created`.
- Issue update writes one `issue.updated` event summarizing changed fields; status and assignee changes use their specific activity types.
- Tag add/remove writes `tag.added` or `tag.removed`.
- Comment create/update/delete writes comment activity.

Keep the initial implementation synchronous and local. No event bus is required for Phase 2.

## API Contract

All responses use the existing daemon JSON shape convention: successful responses return the object/array directly; errors use the existing `{ code, message, detail }` response helper.

### Issue Shapes

`Issue` extends the current shape with embedded tags only where the UI needs list/detail rendering:

```ts
interface Issue {
  id: string
  project_id: string
  identifier: string
  position: number
  title: string
  description: string
  status: string
  priority: string
  assignee_id: string
  creator_id: string
  created_at: string
  updated_at: string
  tags?: Tag[]
}
```

`GET /api/projects/{project_id}/issues`

- Query params: existing `status`, `priority`; add optional `tag_id`.
- Returns: `Issue[]` ordered by `position ASC`, each with `tags: Tag[]`.
- Empty project returns `[]`.

`POST /api/projects/{project_id}/issues`

Request:

```json
{
  "title": "Write local persistence test",
  "description": "Cover restart/reload behavior.",
  "status": "todo",
  "priority": "high",
  "tag_ids": ["tag-id"]
}
```

Response: `201 Issue` with generated identifier, position, timestamps, and `tags`.

Validation:

- Project must exist or return `404 NOT_FOUND`.
- `title` required after trimming.
- Unknown tag IDs return `400 BAD_REQUEST`.
- Issue insert and initial tag associations happen in one transaction.

`GET /api/issues/{issue_id}`

Response:

```ts
interface IssueDetailResponse {
  issue: Issue
  comments: IssueComment[]
  activity: IssueActivity[]
}
```

`PUT /api/issues/{issue_id}`

Request fields remain optional:

```json
{
  "title": "Updated title",
  "description": "Updated description",
  "status": "in_progress",
  "priority": "medium",
  "assignee_id": "agent-or-human-id",
  "creator_id": "creator-id"
}
```

Response: `200 IssueDetailResponse`.

Rules:

- Missing issue returns `404 NOT_FOUND`.
- Empty title is rejected if provided.
- Activity metadata records old/new values for changed fields.
- `updated_at` changes exactly once per mutation.

### Tag Shapes

```ts
interface Tag {
  id: string
  project_id: string
  name: string
  color: string
  created_at: string
  updated_at: string
}
```

`GET /api/projects/{project_id}/tags`

- Returns `Tag[]` ordered by `name COLLATE NOCASE ASC`.
- Missing project returns `404 NOT_FOUND`.

`POST /api/projects/{project_id}/tags`

Request:

```json
{ "name": "bug", "color": "#ef4444" }
```

Response: `201 Tag`.

`PUT /api/tags/{tag_id}`

Request:

```json
{ "name": "backend", "color": "#3b82f6" }
```

Response: `200 Tag`.

`DELETE /api/tags/{tag_id}`

- Response: `204`.
- Cascades issue associations through foreign keys.

`PUT /api/issues/{issue_id}/tags/{tag_id}`

- Adds a tag to an issue.
- Idempotent: adding an existing association returns `200 IssueDetailResponse`, not an error.
- Missing issue or tag returns `404 NOT_FOUND`.
- Cross-project issue/tag mismatch returns `400 BAD_REQUEST`.

`DELETE /api/issues/{issue_id}/tags/{tag_id}`

- Removes a tag from an issue.
- Idempotent: removing a missing association returns `200 IssueDetailResponse`.

### Comment Shapes

```ts
interface IssueComment {
  id: string
  issue_id: string
  author_id: string
  author_name: string
  body: string
  created_at: string
  updated_at: string
}
```

`GET /api/issues/{issue_id}/comments`

- Returns `IssueComment[]` ordered by `created_at ASC`.

`POST /api/issues/{issue_id}/comments`

Request:

```json
{
  "body": "This needs a restart persistence test.",
  "author_id": "local-user",
  "author_name": "You"
}
```

Response: `201 IssueDetailResponse`.

`PUT /api/comments/{comment_id}`

Request:

```json
{ "body": "Edited comment body." }
```

Response: `200 IssueDetailResponse`.

`DELETE /api/comments/{comment_id}`

Response: `200 IssueDetailResponse` for the owning issue after deletion.

### Activity Shapes

```ts
interface IssueActivity {
  id: string
  issue_id: string
  actor_id: string
  actor_name: string
  type: string
  summary: string
  metadata: Record<string, unknown>
  created_at: string
}
```

`GET /api/issues/{issue_id}/activity`

- Returns `IssueActivity[]` ordered by `created_at ASC`.
- `metadata_json` is decoded to `metadata`; invalid stored JSON should be treated as a daemon internal error because it indicates data corruption.

## Frontend Integration Plan

Keep the existing three-panel shell:

- Left sidebar: project list plus issue list for selected project.
- Center detail: selected issue fields, tag chips, editable description, comment composer/list.
- Right timeline: activity events for the selected issue.

### State Ownership

Introduce a small app-level workspace state container in `App.tsx` or a dedicated hook:

- `selectedProjectId`
- `selectedIssueId`
- `projects`
- `issuesByProject`
- `selectedIssueDetail`
- loading/error state per request class

Avoid global stores until the app has more than one workflow. A local reducer or explicit hooks are enough for Phase 2.

### Sidebar

- Load projects from `api.projects.list()`.
- Select the first project by default when no selection exists.
- Load issues with `api.issues.list(projectId)`.
- Show tag chips or color swatches compactly in each issue row.
- Add a "new issue" control that opens an inline title field or compact form.
- On successful create, prepend/select the new issue or preserve server order after refetch.

Empty/error states:

- No projects: show a compact empty state with a create-project affordance.
- No issues: show a compact empty state with a create-issue affordance.
- Failed list: show an error panel with retry.

### Detail Panel

- Load `GET /api/issues/{id}` when selection changes.
- Render title, identifier, priority, status, tags, description, and comments from daemon data.
- Allow title/description/status/priority edits with explicit save interactions.
- Add/remove tags through issue tag endpoints.
- Add comment through a composer; clear composer only after the daemon confirms success.
- Disable mutation controls while a mutation is in flight for the selected issue.

Empty/error states:

- No issue selected: show an unframed neutral empty state.
- Deleted/missing issue: clear selection, refresh list, show a recoverable message.
- Save failure: keep local draft visible and show retry/cancel.

### Timeline Panel

- Render `IssueActivity[]` from selected issue detail.
- Use deterministic labels from `type`; do not expose raw JSON metadata as primary UI.
- Show timestamps using compact relative labels backed by actual `created_at`.
- Empty activity should only happen for legacy data; show "No activity yet" rather than fake events.

### API Client Types

Extend `apps/desktop/src/daemon/types.ts` with:

- `Tag`
- `IssueComment`
- `IssueActivity`
- `IssueDetailResponse`

Extend `useApi()` with:

- `tags.list/create/update/delete`
- `issues.addTag/removeTag`
- `comments.list/create/update/delete`
- `activity.list`
- Change `issues.get/update/create` return types to match the new contracts where applicable.

## Persistence and Reload Guarantees

The implementation must prove:

1. Creating an issue commits issue row, generated identifier/position, requested tags, and `issue.created` activity in one transaction.
2. Adding a tag commits the join row and activity event in one transaction.
3. Creating a comment commits the comment row and activity event in one transaction.
4. Closing and reopening the SQLite connection preserves projects, issues, tags, comments, and activity.
5. Restarting the daemon process exposes the same data through HTTP.
6. Cascade deletion remains intact: deleting a project removes issues, tags, issue tag joins, comments, and activity.
7. No UI path relies on hard-coded sample work item data after Phase 2.

## Test Plan

### Daemon Store Tests

Add or extend store tests for:

- Create project, issue, tag, attach tag, create comment, list issue detail.
- Duplicate tag names rejected per project.
- Same tag name allowed across different projects.
- Cross-project issue/tag association rejected.
- Comment create trims body and rejects empty body.
- Activity rows are created for issue creation, tag attach, comment create, status update.
- Close/reopen DB and verify issue, tag, comment, and activity are still present.
- Project delete cascades related Phase 2 rows.

### Daemon HTTP Contract Tests

Add handler tests for:

- `POST /api/projects/{project_id}/issues` with `tag_ids`.
- `GET /api/issues/{issue_id}` returns issue, comments, and activity.
- `POST /api/projects/{project_id}/tags` and duplicate validation.
- `PUT`/`DELETE /api/issues/{issue_id}/tags/{tag_id}` idempotency.
- `POST /api/issues/{issue_id}/comments` and empty-body rejection.
- Error shape consistency for missing issue, missing tag, bad JSON, and cross-project tag attach.

### Daemon E2E Test

Extend the existing daemon e2e test:

1. Start daemon with a temp data dir.
2. Create project.
3. Create tag.
4. Create issue with tag.
5. Add comment.
6. Fetch issue detail and assert tag/comment/activity are present.
7. Stop daemon.
8. Restart daemon with the same data dir.
9. Fetch issue detail again and assert persisted state is unchanged.

### Desktop Tests

Update Vitest tests to use mocked daemon API responses:

- Initial render loads projects/issues from API, not static arrays.
- Empty issue list shows create affordance.
- Create issue calls API, then selects/render the created issue.
- Tag chip renders from API data.
- Comment composer submits and renders returned comment.
- Detail save failure keeps draft text and exposes retry.
- Timeline renders returned activity events.

### Manual Verification

Before handoff:

- Run Go unit tests.
- Build daemon binary and run e2e.
- Run desktop lint/test.
- Start daemon and desktop dev server.
- In the app, create an issue, create/attach a tag, add a comment, restart daemon, refresh app, and verify the same issue state returns.

## Execution Breakdown

1. Migration and model structs.
2. Store repositories and transaction helpers.
3. HTTP routes/handlers and contract tests.
4. Desktop API type/client updates.
5. Sidebar/detail/timeline data integration.
6. Empty/error/retry states.
7. E2E persistence/restart test.
8. Manual app verification and public artifact hygiene check.

## Implementation Guardrails

- Keep all Phase 2 data local to SQLite.
- Prefer additive migrations; do not rewrite existing issue rows unless required.
- Preserve current issue identifiers and positions.
- Keep API response contracts explicit and test them.
- Do not add a frontend state library unless local React state becomes demonstrably insufficient.
- Do not introduce agent runtime concepts behind comment/activity names.
- Do not add Kanban status columns or drag/drop; status remains an issue field.
- Do not create sample/fake UI data once real daemon loading is wired.

## Open Technical Decisions

1. Whether `IssueDetailResponse` should be the response for every mutation or whether mutations should return only the changed resource. Recommendation: return `IssueDetailResponse` for issue-specific mutations because the UI needs comments/activity/tags refreshed immediately.
2. Whether tag name uniqueness should be enforced by a generated lowercase column or service-level checks. Recommendation: service-level validation now, generated column only if SQLite portability becomes an issue.
3. Whether comment edits/deletes are necessary in the first execution slice. Recommendation: implement them in Phase 2 because their API shape is small and their activity events close the lifecycle cleanly.

## Approval Gate

Execution should not begin until:

- Technical draft confirms the schema, API shape, UI integration, and verification plan are internally consistent.
- Resilience review checks transaction boundaries, restart persistence, error contracts, and scope creep risks.
- Product sign-off confirms this is the right Phase 2 slice before implementation tickets are created.
