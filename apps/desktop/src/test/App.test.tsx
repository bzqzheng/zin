import { describe, it, expect, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import App from '../App'
import { DaemonContext, type DaemonConnectionContext } from '../daemon/useDaemon'
import type {
  Agent,
  Issue,
  IssueActivity,
  IssueAssignment,
  IssueComment,
  Project,
  RuntimeRecord,
  Tag,
} from '../daemon'

const projectAlpha: Project = {
  id: 'project-alpha',
  name: 'Alpha',
  description: '',
  created_at: '2026-05-01T12:00:00Z',
  updated_at: '2026-05-01T12:00:00Z',
}

const projectBeta: Project = {
  id: 'project-beta',
  name: 'Beta',
  description: '',
  created_at: '2026-05-02T12:00:00Z',
  updated_at: '2026-05-02T12:00:00Z',
}

function makeIssue(overrides: Partial<Issue>): Issue {
  return {
    id: 'issue-alpha',
    project_id: 'project-alpha',
    identifier: 'ISSUE-1',
    position: 1,
    title: 'Alpha issue',
    description: 'Persisted issue description',
    status: 'todo',
    priority: 'medium',
    assignee_id: '',
    creator_id: '',
    created_at: '2026-05-02T14:00:00Z',
    updated_at: '2026-05-02T15:00:00Z',
    ...overrides,
  }
}

const tagBackend: Tag = {
  id: 'tag-backend',
  project_id: 'project-alpha',
  name: 'Backend',
  color: '#3b82f6',
  created_at: '2026-05-02T14:05:00Z',
  updated_at: '2026-05-02T14:05:00Z',
}

const commentOne: IssueComment = {
  id: 'comment-one',
  issue_id: 'issue-alpha',
  author_id: 'local-user',
  author_name: 'You',
  body: 'Existing comment',
  created_at: '2026-05-02T14:10:00Z',
  updated_at: '2026-05-02T14:10:00Z',
}

const activityCreated: IssueActivity = {
  id: 'activity-one',
  issue_id: 'issue-alpha',
  actor_id: '',
  type: 'issue.created',
  summary: 'Created work item',
  metadata: {},
  created_at: '2026-05-02T14:00:00Z',
}

const codexRuntime: RuntimeRecord = {
  id: 'runtime-codex',
  kind: 'codex',
  display_name: 'Codex CLI',
  binary_path: '/usr/local/bin/codex',
  version_raw: 'codex 0.42.0',
  health_status: 'healthy',
  health_reason: '',
  last_checked_at: '2026-05-03T20:00:00Z',
  created_at: '2026-05-03T19:58:00Z',
  updated_at: '2026-05-03T20:00:00Z',
}

const agentBuilder: Agent = {
  id: 'agent-builder',
  name: 'Builder',
  role: 'Implementation',
  status: 'idle',
  runtime_id: 'runtime-codex',
  model_hint: 'gpt-5',
  instructions: 'Build the issue.',
  is_assignable: true,
  created_at: '2026-05-03T20:10:00Z',
  updated_at: '2026-05-03T20:10:00Z',
}

function makeAssignment(overrides: Partial<IssueAssignment>): IssueAssignment {
  return {
    id: 'assignment-one',
    issue_id: 'issue-alpha',
    agent_id: 'agent-builder',
    requested_by: 'local-user',
    status: 'queued',
    source_type: 'issue_detail',
    source_id: 'issue-alpha',
    dedupe_key: 'dedupe-one',
    requested_at: '2026-05-03T20:20:00Z',
    created_at: '2026-05-03T20:20:00Z',
    updated_at: '2026-05-03T20:20:00Z',
    ...overrides,
  }
}

function interactionResponse(path: string) {
  if (path.endsWith('/tags')) return []
  if (path.endsWith('/comments')) return []
  if (path.endsWith('/activity')) return []
  if (path.endsWith('/assignments')) return { assignments: [] }
  if (path === '/api/agents') return []
  if (path === '/api/runtimes') return { runtimes: [] }
  return undefined
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((promiseResolve, promiseReject) => {
    resolve = promiseResolve
    reject = promiseReject
  })
  return { promise, resolve, reject }
}

function renderApp(
  fetchApi: DaemonConnectionContext['fetchApi'],
  overrides: Partial<DaemonConnectionContext> = {},
) {
  const context: DaemonConnectionContext = {
    port: 9876,
    connected: true,
    error: null,
    baseURL: 'http://127.0.0.1:9876',
    startDaemon: vi.fn().mockResolvedValue(9876),
    checkHealth: vi.fn().mockResolvedValue(undefined),
    shutdown: vi.fn().mockResolvedValue(undefined),
    fetchApi,
    ...overrides,
  }

  render(
    <DaemonContext.Provider value={context}>
      <App />
    </DaemonContext.Provider>,
  )

  return context
}

describe('App', () => {
  it('renders an empty project state from the daemon', async () => {
    const fetchApi = vi.fn().mockResolvedValue([])

    renderApp(fetchApi)

    expect(await screen.findByText('No projects yet')).toBeVisible()
    expect(screen.getByText('Create a project to track issues')).toBeVisible()
    expect(screen.getByText('No issue selected')).toBeVisible()
    expect(fetchApi).toHaveBeenCalledWith('/api/projects')
  })

  it('renders an empty issue state for the selected project', async () => {
    const fetchApi = vi.fn((path: string) => {
      if (path === '/api/projects') return Promise.resolve([projectAlpha])
      if (path === '/api/projects/project-alpha/issues') return Promise.resolve([])
      const interactions = interactionResponse(path)
      if (interactions !== undefined) return Promise.resolve(interactions)
      throw new Error(`unexpected path: ${path}`)
    })

    renderApp(fetchApi)

    expect(await screen.findByRole('button', { name: 'Alpha' })).toBeVisible()
    expect(await screen.findByText('No issues in this project')).toBeVisible()
    expect(screen.getByText('No issue selected')).toBeVisible()
  })

  it('loads issue detail from the daemon as selection changes', async () => {
    const alphaIssue = makeIssue({ id: 'issue-alpha', title: 'Alpha issue' })
    const betaIssue = makeIssue({
      id: 'issue-beta',
      project_id: 'project-beta',
      identifier: 'ISSUE-2',
      title: 'Beta issue',
      priority: 'high',
      status: 'in_progress',
    })
    const fetchApi = vi.fn((path: string) => {
      if (path === '/api/projects') return Promise.resolve([projectAlpha, projectBeta])
      if (path === '/api/projects/project-alpha/issues') return Promise.resolve([alphaIssue])
      if (path === '/api/projects/project-beta/issues') return Promise.resolve([betaIssue])
      if (path === '/api/issues/issue-alpha') return Promise.resolve(alphaIssue)
      if (path === '/api/issues/issue-beta') return Promise.resolve(betaIssue)
      const interactions = interactionResponse(path)
      if (interactions !== undefined) return Promise.resolve(interactions)
      throw new Error(`unexpected path: ${path}`)
    })

    renderApp(fetchApi)

    const detail = await screen.findByRole('heading', { name: 'Alpha issue' })
    expect(detail).toBeVisible()
    expect(await screen.findByDisplayValue('Persisted issue description')).toBeVisible()

    fireEvent.click(screen.getByRole('button', { name: 'Beta' }))

    expect(await screen.findByRole('heading', { name: 'Beta issue' })).toBeVisible()
    expect(screen.getAllByText('In Progress')[0]).toBeVisible()
    await waitFor(() => {
      expect(fetchApi).toHaveBeenCalledWith('/api/issues/issue-beta')
    })
  })

  it('keeps the current issue list stable when the selected project is clicked again', async () => {
    const alphaIssue = makeIssue({ id: 'issue-alpha', title: 'Alpha issue' })
    const fetchApi = vi.fn((path: string) => {
      if (path === '/api/projects') return Promise.resolve([projectAlpha])
      if (path === '/api/projects/project-alpha/issues') return Promise.resolve([alphaIssue])
      if (path === '/api/issues/issue-alpha') return Promise.resolve(alphaIssue)
      const interactions = interactionResponse(path)
      if (interactions !== undefined) return Promise.resolve(interactions)
      throw new Error(`unexpected path: ${path}`)
    })

    renderApp(fetchApi)

    expect(await screen.findByRole('heading', { name: 'Alpha issue' })).toBeVisible()

    fireEvent.click(screen.getByRole('button', { name: 'Alpha' }))

    expect(screen.queryByText('Loading issues...')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /ISSUE-1.*Alpha issue/ })).toBeVisible()
  })

  it('retries project loading after an error', async () => {
    let projectAttempts = 0
    const fetchApi = vi.fn((path: string) => {
      if (path === '/api/projects') {
        projectAttempts += 1
        if (projectAttempts === 1) return Promise.reject(new Error('database unavailable'))
        return Promise.resolve([projectAlpha])
      }
      if (path === '/api/projects/project-alpha/issues') return Promise.resolve([])
      const interactions = interactionResponse(path)
      if (interactions !== undefined) return Promise.resolve(interactions)
      throw new Error(`unexpected path: ${path}`)
    })

    renderApp(fetchApi)

    expect(await screen.findByText('Projects failed to load')).toBeVisible()
    expect(screen.getByText('database unavailable')).toBeVisible()

    const projectsPanel = screen.getByText('Projects').closest('div')
    if (!projectsPanel) throw new Error('projects panel missing')

    fireEvent.click(within(projectsPanel).getByRole('button', { name: 'Retry' }))

    expect(await screen.findByRole('button', { name: 'Alpha' })).toBeVisible()
    expect(await screen.findByText('No issues in this project')).toBeVisible()
  })

  it('saves issue fields through the daemon and reloads server truth after retry', async () => {
    let issueState = makeIssue({ id: 'issue-alpha', title: 'Alpha issue' })
    let activityState = [activityCreated]
    let updateAttempts = 0
    const fetchApi = vi.fn((path: string, init?: RequestInit) => {
      if (path === '/api/projects') return Promise.resolve([projectAlpha])
      if (path === '/api/projects/project-alpha/issues') return Promise.resolve([issueState])
      if (path === '/api/projects/project-alpha/tags') return Promise.resolve([])
      if (path === '/api/issues/issue-alpha') {
        if (init?.method === 'PUT') {
          updateAttempts += 1
          if (updateAttempts === 1) return Promise.reject(new Error('write conflict'))
          const body = JSON.parse(String(init.body))
          issueState = {
            ...issueState,
            ...body,
            updated_at: '2026-05-02T16:00:00Z',
          }
          activityState = [
            ...activityState,
            {
              id: 'activity-two',
              issue_id: issueState.id,
              actor_id: '',
              type: 'issue.updated',
              summary: 'Updated work item',
              metadata: {},
              created_at: '2026-05-02T16:00:00Z',
            },
          ]
          return Promise.resolve(issueState)
        }
        return Promise.resolve(issueState)
      }
      if (path === '/api/issues/issue-alpha/tags') return Promise.resolve([])
      if (path === '/api/issues/issue-alpha/comments') return Promise.resolve([])
      if (path === '/api/issues/issue-alpha/activity') return Promise.resolve(activityState)
      const interactions = interactionResponse(path)
      if (interactions !== undefined) return Promise.resolve(interactions)
      throw new Error(`unexpected path: ${path}`)
    })

    renderApp(fetchApi)

    expect(await screen.findByRole('heading', { name: 'Alpha issue' })).toBeVisible()

    fireEvent.change(screen.getByLabelText('Title'), {
      target: { value: 'Server title' },
    })
    fireEvent.change(screen.getByLabelText('Status'), {
      target: { value: 'done' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save Fields' }))

    expect(await screen.findByText('Update failed')).toBeVisible()
    expect(screen.getByText('write conflict')).toBeVisible()
    expect(screen.getByRole('heading', { name: 'Alpha issue' })).toBeVisible()

    fireEvent.click(screen.getByRole('button', { name: 'Save Fields' }))

    expect(await screen.findByRole('heading', { name: 'Server title' })).toBeVisible()
    expect(await screen.findByText('Updated work item')).toBeVisible()
    await waitFor(() => {
      expect(fetchApi).toHaveBeenCalledWith(
        '/api/issues/issue-alpha',
        expect.objectContaining({
          method: 'PUT',
          body: expect.stringContaining('"title":"Server title"'),
        }),
      )
    })
  })

  it('keeps reloaded issue detail visible when the issue list reload is slow after save', async () => {
    let issueState = makeIssue({ id: 'issue-alpha', title: 'Alpha issue' })
    let issueListCalls = 0
    const postUpdateIssues = deferred<Issue[]>()
    const fetchApi = vi.fn((path: string, init?: RequestInit) => {
      if (path === '/api/projects') return Promise.resolve([projectAlpha])
      if (path === '/api/projects/project-alpha/issues') {
        issueListCalls += 1
        if (issueListCalls === 1) return Promise.resolve([issueState])
        return postUpdateIssues.promise
      }
      if (path === '/api/projects/project-alpha/tags') return Promise.resolve([])
      if (path === '/api/issues/issue-alpha') {
        if (init?.method === 'PUT') {
          const body = JSON.parse(String(init.body))
          issueState = {
            ...issueState,
            ...body,
            updated_at: '2026-05-02T16:00:00Z',
          }
          return Promise.resolve(issueState)
        }
        return Promise.resolve(issueState)
      }
      if (path === '/api/issues/issue-alpha/tags') return Promise.resolve([])
      if (path === '/api/issues/issue-alpha/comments') return Promise.resolve([])
      if (path === '/api/issues/issue-alpha/activity') return Promise.resolve([])
      const interactions = interactionResponse(path)
      if (interactions !== undefined) return Promise.resolve(interactions)
      throw new Error(`unexpected path: ${path}`)
    })

    renderApp(fetchApi)

    expect(await screen.findByRole('heading', { name: 'Alpha issue' })).toBeVisible()

    fireEvent.change(screen.getByLabelText('Title'), {
      target: { value: 'Saved title' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save Fields' }))

    await waitFor(() => {
      expect(fetchApi).toHaveBeenCalledWith(
        '/api/issues/issue-alpha',
        expect.objectContaining({ method: 'PUT' }),
      )
    })

    postUpdateIssues.resolve([issueState])

    expect(await screen.findByRole('heading', { name: 'Saved title' })).toBeVisible()
    expect(screen.queryByText('Loading issue...')).not.toBeInTheDocument()
  })

  it('ignores stale mutation reloads after selecting another issue', async () => {
    let alphaIssue = makeIssue({ id: 'issue-alpha', title: 'Alpha issue' })
    const betaIssue = makeIssue({
      id: 'issue-beta',
      identifier: 'ISSUE-2',
      title: 'Beta issue',
      status: 'in_progress',
    })
    const update = deferred<Issue>()
    const fetchApi = vi.fn((path: string, init?: RequestInit) => {
      if (path === '/api/projects') return Promise.resolve([projectAlpha])
      if (path === '/api/projects/project-alpha/issues')
        return Promise.resolve([alphaIssue, betaIssue])
      if (path === '/api/projects/project-alpha/tags') return Promise.resolve([])
      if (path === '/api/issues/issue-alpha') {
        if (init?.method === 'PUT') return update.promise
        return Promise.resolve(alphaIssue)
      }
      if (path === '/api/issues/issue-beta') return Promise.resolve(betaIssue)
      const interactions = interactionResponse(path)
      if (interactions !== undefined) return Promise.resolve(interactions)
      throw new Error(`unexpected path: ${path}`)
    })

    renderApp(fetchApi)

    expect(await screen.findByRole('heading', { name: 'Alpha issue' })).toBeVisible()

    fireEvent.change(screen.getByLabelText('Title'), {
      target: { value: 'Alpha updated' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save Fields' }))
    fireEvent.click(screen.getByRole('button', { name: /ISSUE-2.*Beta issue/ }))

    expect(await screen.findByRole('heading', { name: 'Beta issue' })).toBeVisible()

    alphaIssue = {
      ...alphaIssue,
      title: 'Alpha updated',
      updated_at: '2026-05-02T16:00:00Z',
    }
    update.resolve(alphaIssue)

    await waitFor(() => {
      expect(fetchApi).toHaveBeenCalledWith(
        '/api/issues/issue-alpha',
        expect.objectContaining({ method: 'PUT' }),
      )
    })
    expect(screen.getByRole('heading', { name: 'Beta issue' })).toBeVisible()
    expect(screen.queryByRole('heading', { name: 'Alpha updated' })).not.toBeInTheDocument()
  })

  it('creates, attaches, and detaches tags with server reloads', async () => {
    const alphaIssue = makeIssue({ id: 'issue-alpha', title: 'Alpha issue' })
    let projectTags = [tagBackend]
    let issueTags: Tag[] = []
    const fetchApi = vi.fn((path: string, init?: RequestInit) => {
      if (path === '/api/projects') return Promise.resolve([projectAlpha])
      if (path === '/api/projects/project-alpha/issues') return Promise.resolve([alphaIssue])
      if (path === '/api/projects/project-alpha/tags') return Promise.resolve(projectTags)
      if (path === '/api/issues/issue-alpha') return Promise.resolve(alphaIssue)
      if (path === '/api/issues/issue-alpha/tags') {
        if (init?.method === 'POST') {
          const body = JSON.parse(String(init.body))
          if (body.tag_id) {
            issueTags = projectTags.filter((tag) => tag.id === body.tag_id)
          } else {
            const tag = {
              ...tagBackend,
              id: 'tag-ui',
              name: body.name,
              color: body.color,
            }
            projectTags = [...projectTags, tag]
            issueTags = [tag]
          }
        }
        return Promise.resolve(issueTags)
      }
      if (path === '/api/issues/issue-alpha/tags/tag-backend' && init?.method === 'DELETE') {
        issueTags = []
        return Promise.resolve(undefined)
      }
      if (path === '/api/issues/issue-alpha/tags/tag-ui' && init?.method === 'DELETE') {
        issueTags = []
        return Promise.resolve(undefined)
      }
      if (path === '/api/issues/issue-alpha/comments') return Promise.resolve([])
      if (path === '/api/issues/issue-alpha/activity') return Promise.resolve([])
      const interactions = interactionResponse(path)
      if (interactions !== undefined) return Promise.resolve(interactions)
      throw new Error(`unexpected path: ${path}`)
    })

    renderApp(fetchApi)

    expect(await screen.findByRole('heading', { name: 'Alpha issue' })).toBeVisible()

    fireEvent.click(screen.getByRole('button', { name: 'Attach' }))
    expect(await screen.findByLabelText('Detach Backend')).toBeVisible()

    fireEvent.click(screen.getByLabelText('Detach Backend'))
    expect(await screen.findByText('No tags attached')).toBeVisible()

    fireEvent.change(screen.getByPlaceholderText('New tag'), {
      target: { value: 'UI' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    expect(await screen.findByText('UI')).toBeVisible()
  })

  it('adds, edits, and deletes comments chronologically from the daemon', async () => {
    const alphaIssue = makeIssue({ id: 'issue-alpha', title: 'Alpha issue' })
    let comments = [commentOne]
    const fetchApi = vi.fn((path: string, init?: RequestInit) => {
      if (path === '/api/projects') return Promise.resolve([projectAlpha])
      if (path === '/api/projects/project-alpha/issues') return Promise.resolve([alphaIssue])
      if (path === '/api/projects/project-alpha/tags') return Promise.resolve([])
      if (path === '/api/issues/issue-alpha') return Promise.resolve(alphaIssue)
      if (path === '/api/issues/issue-alpha/tags') return Promise.resolve([])
      if (path === '/api/issues/issue-alpha/activity') return Promise.resolve([])
      if (path === '/api/issues/issue-alpha/comments') {
        if (init?.method === 'POST') {
          const body = JSON.parse(String(init.body))
          comments = [
            ...comments,
            {
              ...commentOne,
              id: 'comment-two',
              body: body.body,
              created_at: '2026-05-02T14:20:00Z',
              updated_at: '2026-05-02T14:20:00Z',
            },
          ]
        }
        return Promise.resolve(comments)
      }
      if (path === '/api/comments/comment-one' && init?.method === 'PUT') {
        const body = JSON.parse(String(init.body))
        comments = comments.map((comment) =>
          comment.id === 'comment-one' ? { ...comment, body: body.body } : comment,
        )
        return Promise.resolve(comments[0])
      }
      if (path === '/api/comments/comment-two' && init?.method === 'DELETE') {
        comments = comments.filter((comment) => comment.id !== 'comment-two')
        return Promise.resolve(undefined)
      }
      const interactions = interactionResponse(path)
      if (interactions !== undefined) return Promise.resolve(interactions)
      throw new Error(`unexpected path: ${path}`)
    })

    renderApp(fetchApi)

    expect(await screen.findByText('Existing comment')).toBeVisible()

    fireEvent.change(screen.getByPlaceholderText('Add a comment'), {
      target: { value: 'Second comment' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Add Comment' }))
    expect(await screen.findByText('Second comment')).toBeVisible()

    const firstComment = screen.getByText('Existing comment').closest('div')
    if (!firstComment) throw new Error('first comment missing')
    fireEvent.click(within(firstComment).getByRole('button', { name: 'Edit' }))
    fireEvent.change(screen.getByDisplayValue('Existing comment'), {
      target: { value: 'Edited comment' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(screen.queryByDisplayValue('Edited comment')).not.toBeInTheDocument(),
    )
    expect(screen.getByText('Edited comment')).toBeVisible()

    const secondComment = screen.getByText('Second comment').closest('div')
    if (!secondComment) throw new Error('second comment missing')
    fireEvent.click(within(secondComment).getByRole('button', { name: 'Delete' }))
    await waitFor(() => expect(screen.queryByText('Second comment')).not.toBeInTheDocument())
  })

  it('creates an assignable agent, assigns work, cancels it, and reloads activity', async () => {
    const alphaIssue = makeIssue({ id: 'issue-alpha', title: 'Alpha issue' })
    let agents: Agent[] = []
    let assignments: IssueAssignment[] = []
    let activityState = [activityCreated]
    const fetchApi = vi.fn((path: string, init?: RequestInit) => {
      if (path === '/api/projects') return Promise.resolve([projectAlpha])
      if (path === '/api/projects/project-alpha/issues') return Promise.resolve([alphaIssue])
      if (path === '/api/projects/project-alpha/tags') return Promise.resolve([])
      if (path === '/api/issues/issue-alpha') return Promise.resolve(alphaIssue)
      if (path === '/api/issues/issue-alpha/tags') return Promise.resolve([])
      if (path === '/api/issues/issue-alpha/comments') return Promise.resolve([])
      if (path === '/api/issues/issue-alpha/activity') return Promise.resolve(activityState)
      if (path === '/api/runtimes') return Promise.resolve({ runtimes: [codexRuntime] })
      if (path === '/api/agents') {
        if (init?.method === 'POST') {
          const body = JSON.parse(String(init.body))
          agents = [
            {
              ...agentBuilder,
              name: body.name,
              role: body.role,
              runtime_id: body.runtime_id,
              model_hint: body.model_hint,
              instructions: body.instructions,
              is_assignable: body.is_assignable,
            },
          ]
          return Promise.resolve(agents[0])
        }
        return Promise.resolve(agents)
      }
      if (path === '/api/issues/issue-alpha/assignments') {
        if (init?.method === 'POST') {
          const assignment = makeAssignment({ id: 'assignment-one' })
          assignments = [assignment]
          activityState = [
            ...activityState,
            {
              id: 'activity-assignment-requested',
              issue_id: alphaIssue.id,
              actor_id: 'local-user',
              type: 'assignment.requested',
              summary: 'Requested assignment for Builder',
              metadata: {},
              created_at: '2026-05-03T20:20:00Z',
            },
          ]
          return Promise.resolve({ assignment, idempotent_replay: false })
        }
        return Promise.resolve({ assignments })
      }
      if (path === '/api/assignments/assignment-one/cancel' && init?.method === 'POST') {
        assignments = assignments.map((assignment) =>
          assignment.id === 'assignment-one'
            ? {
                ...assignment,
                status: 'cancelled',
                cancelled_at: '2026-05-03T20:25:00Z',
              }
            : assignment,
        )
        activityState = [
          ...activityState,
          {
            id: 'activity-assignment-cancelled',
            issue_id: alphaIssue.id,
            actor_id: 'local-user',
            type: 'assignment.cancelled',
            summary: 'Cancelled assignment for Builder',
            metadata: {},
            created_at: '2026-05-03T20:25:00Z',
          },
        ]
        return Promise.resolve({ assignment: assignments[0] })
      }
      throw new Error(`unexpected path: ${path}`)
    })

    renderApp(fetchApi)

    expect(await screen.findByRole('heading', { name: 'Alpha issue' })).toBeVisible()
    expect(await screen.findByText('No agents configured yet.')).toBeVisible()
    await waitFor(() => expect(screen.getByLabelText('Runtime')).toHaveValue('runtime-codex'))

    fireEvent.change(screen.getByLabelText('Agent'), { target: { value: '' } })
    fireEvent.change(screen.getByLabelText('Name'), {
      target: { value: 'Builder' },
    })
    fireEvent.change(screen.getByLabelText('Role'), {
      target: { value: 'Implementation' },
    })
    fireEvent.change(screen.getByLabelText('Model'), {
      target: { value: 'gpt-5' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Create Agent' }))

    await waitFor(() => expect(screen.getByRole('button', { name: 'Assign Agent' })).toBeEnabled())
    fireEvent.click(screen.getByRole('button', { name: 'Assign Agent' }))

    expect(await screen.findByText('Requested assignment for Builder')).toBeVisible()
    expect(screen.getAllByText('Queued')[0]).toBeVisible()

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(await screen.findByText('Cancelled assignment for Builder')).toBeVisible()
    expect(screen.getByText('Cancelled')).toBeVisible()
    await waitFor(() => {
      expect(fetchApi).toHaveBeenCalledWith(
        '/api/issues/issue-alpha/assignments',
        expect.objectContaining({
          method: 'POST',
          body: expect.stringContaining('"agent_id":"agent-builder"'),
        }),
      )
    })
  })

  it('manages runtime discovery, path overrides, revalidation, and diagnostics from settings', async () => {
    let runtimes = [codexRuntime]
    const fetchApi = vi.fn((path: string, init?: RequestInit) => {
      if (path === '/api/projects') return Promise.resolve([])
      if (path === '/api/runtimes') return Promise.resolve({ runtimes })
      if (path === '/api/runtimes/runtime-codex' && init?.method === 'PUT') {
        const body = JSON.parse(String(init.body))
        runtimes = [
          {
            ...runtimes[0],
            display_name: body.display_name,
            binary_path: body.binary_path,
            health_status: 'degraded',
            health_reason: 'probe_failed',
            updated_at: '2026-05-03T20:03:00Z',
          },
        ]
        return Promise.resolve({ runtime: runtimes[0] })
      }
      if (path === '/api/runtimes/validate' && init?.method === 'POST') {
        return Promise.resolve({
          runtime: {
            id: 'runtime-codex',
            health_status: 'healthy',
            health_reason: '',
            last_checked_at: '2026-05-03T20:05:00Z',
            updated_at: '2026-05-03T20:05:00Z',
          },
        })
      }
      if (path === '/api/runtimes/discover' && init?.method === 'POST') {
        runtimes = [
          {
            ...runtimes[0],
            health_status: 'healthy',
            health_reason: '',
            last_checked_at: '2026-05-03T20:06:00Z',
          },
        ]
        return Promise.resolve({
          runtimes,
          summary: { healthy: 1, degraded: 0, missing: 3 },
        })
      }
      throw new Error(`unexpected path: ${path}`)
    })

    renderApp(fetchApi)

    fireEvent.click(screen.getByRole('button', { name: 'Settings' }))

    expect(await screen.findByRole('heading', { name: 'Agent Runtimes' })).toBeVisible()
    expect(await screen.findByText('Codex CLI')).toBeVisible()
    expect(screen.getByText('Healthy')).toBeVisible()
    expect(screen.getByText('Probe passed')).toBeVisible()

    fireEvent.change(screen.getByLabelText('Display name'), {
      target: { value: 'Unsaved Codex' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Revalidate' }))
    expect(await screen.findByDisplayValue('Unsaved Codex')).toBeVisible()

    fireEvent.change(screen.getByLabelText('Display name'), {
      target: { value: 'Codex CLI' },
    })
    fireEvent.change(screen.getByLabelText('Binary path'), {
      target: { value: '/opt/homebrew/bin/codex' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save Path' }))

    expect(await screen.findByText('Degraded')).toBeVisible()
    expect(screen.getByText('probe_failed')).toBeVisible()
    await waitFor(() => {
      expect(fetchApi).toHaveBeenCalledWith(
        '/api/runtimes/runtime-codex',
        expect.objectContaining({
          method: 'PUT',
          body: expect.stringContaining('/opt/homebrew/bin/codex'),
        }),
      )
    })

    fireEvent.click(screen.getByRole('button', { name: 'Revalidate' }))
    expect(await screen.findByText('Healthy')).toBeVisible()

    fireEvent.click(screen.getByRole('button', { name: 'Discover' }))
    expect(await screen.findByText('Missing 3')).toBeVisible()
    await waitFor(() => {
      expect(fetchApi).toHaveBeenCalledWith(
        '/api/runtimes/discover',
        expect.objectContaining({ method: 'POST' }),
      )
    })
  })
})
