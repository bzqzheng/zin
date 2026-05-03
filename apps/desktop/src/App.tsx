import { useCallback, useEffect, useRef, useState } from 'react'
import Sidebar from './components/Sidebar'
import IssueDetail from './components/IssueDetail'
import Timeline from './components/Timeline'
import { useDaemon } from './daemon'
import type { Issue, Project } from './daemon'

export type LoadStatus = 'idle' | 'loading' | 'success' | 'error'

export interface ResourceState<T> {
  status: LoadStatus
  data: T
  error: string | null
}

const emptyProjects: ResourceState<Project[]> = {
  status: 'idle',
  data: [],
  error: null,
}

const emptyIssues: ResourceState<Issue[]> = {
  status: 'idle',
  data: [],
  error: null,
}

const emptyLoadedIssues: ResourceState<Issue[]> = {
  status: 'success',
  data: [],
  error: null,
}

const emptyIssueDetail: ResourceState<Issue | null> = {
  status: 'idle',
  data: null,
  error: null,
}

const emptyLoadedIssueDetail: ResourceState<Issue | null> = {
  status: 'success',
  data: null,
  error: null,
}

function errorMessage(err: unknown) {
  return err instanceof Error ? err.message : String(err)
}

export default function App() {
  const { baseURL, error: daemonError, fetchApi, startDaemon } = useDaemon()
  const daemonStartRef = useRef<Promise<unknown> | null>(null)
  const [daemonStatus, setDaemonStatus] = useState<LoadStatus>('loading')
  const [projects, setProjects] = useState<ResourceState<Project[]>>(emptyProjects)
  const [issues, setIssues] = useState<ResourceState<Issue[]>>(emptyIssues)
  const [issueDetail, setIssueDetail] = useState<ResourceState<Issue | null>>(emptyIssueDetail)
  const [preferredProjectId, setPreferredProjectId] = useState<string | null>(null)
  const [preferredIssueId, setPreferredIssueId] = useState<string | null>(null)

  const selectedProjectId =
    projects.status === 'success'
      ? (projects.data.find((project) => project.id === preferredProjectId)?.id ??
        projects.data[0]?.id ??
        null)
      : preferredProjectId

  const selectedIssueId =
    issues.status === 'success'
      ? (issues.data.find((issue) => issue.id === preferredIssueId)?.id ?? issues.data[0]?.id ?? null)
      : preferredIssueId

  const visibleIssues =
    projects.status === 'success' && !selectedProjectId
      ? emptyLoadedIssues
      : issues

  const visibleIssueDetail =
    visibleIssues.status === 'success' && !selectedIssueId
      ? emptyLoadedIssueDetail
      : issueDetail

  const fetchProjects = useCallback(() => fetchApi<Project[]>('/api/projects'), [fetchApi])

  const fetchIssues = useCallback(
    (projectId: string) => fetchApi<Issue[]>(`/api/projects/${projectId}/issues`),
    [fetchApi],
  )

  const fetchIssueDetail = useCallback(
    (issueId: string) => fetchApi<Issue>(`/api/issues/${issueId}`),
    [fetchApi],
  )

  const resetWorkspaceCache = useCallback(() => {
    setProjects(emptyProjects)
    setIssues(emptyIssues)
    setIssueDetail(emptyIssueDetail)
    setPreferredProjectId(null)
    setPreferredIssueId(null)
  }, [])

  const startDaemonOnce = useCallback(() => {
    if (!daemonStartRef.current) {
      daemonStartRef.current = startDaemon().catch((err) => {
        daemonStartRef.current = null
        throw err
      })
    }

    return daemonStartRef.current
  }, [startDaemon])

  const connectDaemon = useCallback(async () => {
    setDaemonStatus('loading')
    try {
      await startDaemonOnce()
      setDaemonStatus('success')
      if (!baseURL) {
        setProjects((prev) => ({ ...prev, status: 'loading', error: null }))
      }
    } catch {
      setDaemonStatus('error')
      resetWorkspaceCache()
    }
  }, [baseURL, resetWorkspaceCache, startDaemonOnce])

  const loadProjects = useCallback(async () => {
    if (!baseURL) return
    setProjects((prev) => ({ ...prev, status: 'loading', error: null }))
    try {
      const data = await fetchProjects()
      setProjects({ status: 'success', data, error: null })
      setIssues(data.length > 0 ? { status: 'loading', data: [], error: null } : emptyLoadedIssues)
      setIssueDetail(emptyLoadedIssueDetail)
    } catch (err) {
      setProjects({ status: 'error', data: [], error: errorMessage(err) })
      setIssues(emptyIssues)
      setIssueDetail(emptyIssueDetail)
    }
  }, [baseURL, fetchProjects])

  const loadIssues = useCallback(
    async (projectId: string | null = selectedProjectId) => {
      if (!baseURL || !projectId) {
        setIssues({ status: 'success', data: [], error: null })
        setPreferredIssueId(null)
        return
      }

      setIssues((prev) => ({ ...prev, status: 'loading', error: null }))
      try {
        const data = await fetchIssues(projectId)
        setIssues({ status: 'success', data, error: null })
        setIssueDetail(
          data.length > 0 ? { status: 'loading', data: null, error: null } : emptyLoadedIssueDetail,
        )
      } catch (err) {
        setIssues({ status: 'error', data: [], error: errorMessage(err) })
        setIssueDetail(emptyLoadedIssueDetail)
      }
    },
    [baseURL, fetchIssues, selectedProjectId],
  )

  const loadIssueDetail = useCallback(
    async (issueId: string | null = selectedIssueId) => {
      if (!baseURL || !issueId) {
        setIssueDetail({ status: 'success', data: null, error: null })
        return
      }

      setIssueDetail((prev) => ({ ...prev, status: 'loading', error: null }))
      try {
        const data = await fetchIssueDetail(issueId)
        setIssueDetail({ status: 'success', data, error: null })
      } catch (err) {
        setIssueDetail({ status: 'error', data: null, error: errorMessage(err) })
      }
    },
    [baseURL, fetchIssueDetail, selectedIssueId],
  )

  const selectProject = useCallback(
    (projectId: string) => {
      if (projectId === selectedProjectId) return

      setPreferredProjectId(projectId)
      setPreferredIssueId(null)
      setIssues({ status: 'loading', data: [], error: null })
      setIssueDetail({ status: 'loading', data: null, error: null })
    },
    [selectedProjectId],
  )

  useEffect(() => {
    let cancelled = false

    async function start() {
      try {
        await startDaemonOnce()
        if (!cancelled) {
          setDaemonStatus('success')
          if (!baseURL) {
            setProjects((prev) => ({ ...prev, status: 'loading', error: null }))
          }
        }
      } catch {
        if (!cancelled) {
          setDaemonStatus('error')
          resetWorkspaceCache()
        }
      }
    }

    void start()

    return () => {
      cancelled = true
    }
  }, [baseURL, resetWorkspaceCache, startDaemonOnce])

  useEffect(() => {
    if (!baseURL) return

    let cancelled = false

    async function load() {
      try {
        const data = await fetchProjects()
        if (!cancelled) {
          setProjects({ status: 'success', data, error: null })
          setIssues(data.length > 0 ? { status: 'loading', data: [], error: null } : emptyLoadedIssues)
          setIssueDetail(emptyLoadedIssueDetail)
        }
      } catch (err) {
        if (!cancelled) {
          setProjects({ status: 'error', data: [], error: errorMessage(err) })
          setIssues(emptyIssues)
          setIssueDetail(emptyIssueDetail)
        }
      }
    }

    void load()

    return () => {
      cancelled = true
    }
  }, [baseURL, fetchProjects])

  useEffect(() => {
    if (!baseURL || projects.status !== 'success') return

    if (!selectedProjectId) return

    let cancelled = false
    const projectId = selectedProjectId

    async function load() {
      try {
        const data = await fetchIssues(projectId)
        if (!cancelled) {
          setIssues({ status: 'success', data, error: null })
          setIssueDetail(
            data.length > 0
              ? { status: 'loading', data: null, error: null }
              : emptyLoadedIssueDetail,
          )
        }
      } catch (err) {
        if (!cancelled) {
          setIssues({ status: 'error', data: [], error: errorMessage(err) })
          setIssueDetail(emptyLoadedIssueDetail)
        }
      }
    }

    void load()

    return () => {
      cancelled = true
    }
  }, [baseURL, fetchIssues, projects.status, selectedProjectId])

  useEffect(() => {
    if (!baseURL || issues.status !== 'success') return

    if (!selectedIssueId) return

    let cancelled = false
    const issueId = selectedIssueId

    async function load() {
      try {
        const data = await fetchIssueDetail(issueId)
        if (!cancelled) {
          setIssueDetail({ status: 'success', data, error: null })
        }
      } catch (err) {
        if (!cancelled) {
          setIssueDetail({ status: 'error', data: null, error: errorMessage(err) })
        }
      }
    }

    void load()

    return () => {
      cancelled = true
    }
  }, [baseURL, fetchIssueDetail, issues.status, selectedIssueId])

  return (
    <div className="h-screen w-screen bg-zinc-950 text-zinc-100 flex overflow-hidden">
      <Sidebar
        daemonError={daemonError}
        daemonStatus={daemonStatus}
        projects={projects}
        selectedProjectId={selectedProjectId}
        issues={visibleIssues}
        selectedIssueId={selectedIssueId}
        onRetryDaemon={connectDaemon}
        onRetryProjects={loadProjects}
        onRetryIssues={() => loadIssues()}
        onSelectProject={selectProject}
        onSelectIssue={setPreferredIssueId}
      />
      <IssueDetail issue={visibleIssueDetail} onRetry={() => loadIssueDetail()} />
      <Timeline issue={visibleIssueDetail} />
    </div>
  )
}
