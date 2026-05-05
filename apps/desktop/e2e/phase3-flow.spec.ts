import { expect, test, type Page } from '@playwright/test'

const now = '2026-05-03T21:00:00Z'

const projectAlpha = {
  id: 'project-alpha',
  name: 'Alpha',
  description: '',
  created_at: now,
  updated_at: now,
}

const issueAlpha = {
  id: 'issue-alpha',
  project_id: projectAlpha.id,
  identifier: 'ISSUE-1',
  position: 1,
  title: 'Alpha issue',
  description: 'Regression QA target',
  status: 'todo',
  priority: 'medium',
  assignee_id: '',
  creator_id: '',
  created_at: now,
  updated_at: now,
}

const commentOne = {
  id: 'comment-one',
  issue_id: issueAlpha.id,
  author_id: 'local-user',
  author_name: 'You',
  body: 'Please take this from comment context.',
  created_at: now,
  updated_at: now,
}

const codexRuntime = {
  id: 'runtime-codex',
  kind: 'codex',
  display_name: 'Codex CLI',
  binary_path: '/opt/homebrew/bin/codex',
  version_raw: 'codex-cli 0.125.0',
  health_status: 'healthy',
  health_reason: '',
  last_checked_at: now,
  created_at: now,
  updated_at: now,
}

function json(data: unknown, status = 200) {
  return {
    status,
    contentType: 'application/json',
    body: JSON.stringify(data),
  }
}

async function installMockDaemon(page: Page) {
  let runtimes: typeof codexRuntime[] = []
  let agents: Array<Record<string, unknown>> = []
  let assignments: Array<Record<string, unknown>> = []
  let activity = [
    {
      id: 'activity-created',
      issue_id: issueAlpha.id,
      actor_id: '',
      type: 'issue.created',
      summary: 'Created work item',
      metadata: {},
      created_at: now,
    },
  ]

  await page.route('**/api/**', async (route) => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    const method = request.method()

    if (path === '/api/projects') return route.fulfill(json([projectAlpha]))
    if (path === `/api/projects/${projectAlpha.id}/issues`) return route.fulfill(json([issueAlpha]))
    if (path === `/api/projects/${projectAlpha.id}/tags`) return route.fulfill(json([]))
    if (path === `/api/issues/${issueAlpha.id}`) return route.fulfill(json(issueAlpha))
    if (path === `/api/issues/${issueAlpha.id}/tags`) return route.fulfill(json([]))
    if (path === `/api/issues/${issueAlpha.id}/comments`) return route.fulfill(json([commentOne]))
    if (path === `/api/issues/${issueAlpha.id}/activity`) return route.fulfill(json(activity))
    if (path === '/api/runtimes') return route.fulfill(json({ runtimes }))

    if (path === '/api/runtimes/discover' && method === 'POST') {
      runtimes = [codexRuntime]
      return route.fulfill(json({ runtimes, summary: { healthy: 1, degraded: 0, missing: 3 } }))
    }

    if (path === '/api/agents' && method === 'POST') {
      const body = request.postDataJSON() as Record<string, string>
      const agent = {
        id: 'agent-trinity',
        name: body.name,
        role: body.role,
        status: 'offline',
        runtime_id: body.runtime_id,
        runtime_name: codexRuntime.display_name,
        runtime_status: codexRuntime.health_status,
        model: body.model,
        instructions: body.instructions,
        assignable: true,
        assignable_reason: '',
        created_at: now,
        updated_at: now,
      }
      agents = [agent]
      return route.fulfill(json(agent))
    }

    if (path === '/api/agents') return route.fulfill(json(agents))

    if (path === `/api/issues/${issueAlpha.id}/assignments`) {
      if (method === 'POST') {
        const body = request.postDataJSON() as Record<string, string>
        let assignment = assignments.find((item) => item.source_type === body.source_type && item.source_id === body.source_id)
        if (!assignment) {
          assignment = {
            id: `assignment-${assignments.length + 1}`,
            issue_id: issueAlpha.id,
            agent_id: body.agent_id,
            agent_name: 'Trinity',
            runtime_id: codexRuntime.id,
            runtime_name: codexRuntime.display_name,
            runtime_status: codexRuntime.health_status,
            requested_by: 'local-user',
            source_type: body.source_type,
            source_id: body.source_id,
            client_request_id: body.client_request_id,
            status: 'queued',
            dedupe_key: `${issueAlpha.id}:${body.source_type}:${body.source_id}:${body.agent_id}`,
            error_code: '',
            error_message: '',
            requested_at: now,
            accepted_at: null,
            completed_at: null,
            failed_at: null,
            cancelled_at: null,
            created_at: now,
            updated_at: now,
          }
          assignments = [...assignments, assignment]
          activity = [
            ...activity,
            {
              id: `activity-assignment-${assignments.length}`,
              issue_id: issueAlpha.id,
              actor_id: 'local-user',
              type: 'assignment.requested',
              summary: 'Queued assignment for Trinity',
              metadata: { assignment_id: assignment.id },
              created_at: now,
            },
          ]
        }
        return route.fulfill(json({ assignment }))
      }
      return route.fulfill(json({ assignments }))
    }

    if (path === '/api/assignments/assignment-1/cancel' && method === 'POST') {
      assignments = assignments.map((assignment) =>
        assignment.id === 'assignment-1'
          ? { ...assignment, status: 'cancelled', cancelled_at: now, updated_at: now }
          : assignment,
      )
      activity = [
        ...activity,
        {
          id: 'activity-assignment-cancelled',
          issue_id: issueAlpha.id,
          actor_id: 'local-user',
          type: 'assignment.cancelled',
          summary: 'Cancelled assignment for Trinity',
          metadata: { assignment_id: 'assignment-1' },
          created_at: now,
        },
      ]
      return route.fulfill(json({ assignment: assignments[0] }))
    }

    return route.fulfill(json({ error: { message: `unhandled mock route: ${method} ${path}` } }, 500))
  })
}

test('runtime discovery to agent assignment and cancellation', async ({ page }) => {
  await installMockDaemon(page)
  await page.goto('/e2e/index.html')

  await expect(page.getByRole('heading', { name: 'Alpha issue' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Assign Agent' })).toBeDisabled()

  await page.getByRole('button', { name: 'Settings' }).click()
  await page.getByRole('button', { name: 'Discover' }).click()
  await expect(page.getByText('Codex CLI')).toBeVisible()
  await expect(page.getByText('Missing 3')).toBeVisible()

  await page.getByRole('button', { name: 'Issues' }).click()
  await page.getByLabel('Name').fill('Trinity')
  await page.getByLabel('Runtime').selectOption(codexRuntime.id)
  await page.getByLabel('Model').fill('gpt-5')
  await page.getByLabel('Instructions').fill('Complete the assigned work.')
  await page.getByRole('button', { name: 'Create Agent' }).click()

  await expect(page.getByRole('button', { name: 'Assign Agent' })).toBeEnabled()
  await page.getByRole('button', { name: 'Assign Agent' }).click()
  await page.getByRole('button', { name: 'Assign Agent' }).click()

  const assignmentsSection = page.locator('section').filter({ has: page.getByRole('heading', { name: 'Assignments' }) })
  await expect(page.getByText('Queued assignment for Trinity')).toBeVisible()
  await expect(assignmentsSection.getByText('Queued', { exact: true })).toHaveCount(1)

  await page.getByRole('button', { name: 'Assign', exact: true }).click()
  await expect(assignmentsSection.getByText('Comment', { exact: true })).toBeVisible()

  await page.getByRole('button', { name: 'Cancel' }).first().click()
  await expect(page.getByText('Cancelled assignment for Trinity')).toBeVisible()
  await expect(assignmentsSection.getByText('Cancelled', { exact: true })).toBeVisible()
})
