import { describe, it, expect, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import App from '../App'
import { DaemonContext, type DaemonConnectionContext } from '../daemon/useDaemon'
import type { Issue, Project } from '../daemon'

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
      throw new Error(`unexpected path: ${path}`)
    })

    renderApp(fetchApi)

    const detail = await screen.findByRole('heading', { name: 'Alpha issue' })
    expect(detail).toBeVisible()
    expect(screen.getByText('Persisted issue description')).toBeVisible()

    fireEvent.click(screen.getByRole('button', { name: 'Beta' }))

    expect(await screen.findByRole('heading', { name: 'Beta issue' })).toBeVisible()
    expect(screen.getByText('In Progress')).toBeVisible()
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
      throw new Error(`unexpected path: ${path}`)
    })

    renderApp(fetchApi)

    expect(await screen.findByRole('heading', { name: 'Alpha issue' })).toBeVisible()

    fireEvent.click(screen.getByRole('button', { name: 'Alpha' }))

    expect(screen.queryByText('Loading issues...')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /ISSUE-1.*Alpha issue/ })).toBeVisible()
  })

  it('retries project loading after an error', async () => {
    const fetchApi = vi
      .fn()
      .mockRejectedValueOnce(new Error('database unavailable'))
      .mockResolvedValueOnce([projectAlpha])
      .mockResolvedValueOnce([])

    renderApp(fetchApi)

    expect(await screen.findByText('Projects failed to load')).toBeVisible()
    expect(screen.getByText('database unavailable')).toBeVisible()

    const projectsPanel = screen.getByText('Projects').closest('div')
    if (!projectsPanel) throw new Error('projects panel missing')

    fireEvent.click(within(projectsPanel).getByRole('button', { name: 'Retry' }))

    expect(await screen.findByRole('button', { name: 'Alpha' })).toBeVisible()
    expect(await screen.findByText('No issues in this project')).toBeVisible()
  })
})
