import { useCallback, useEffect, useRef, useState } from 'react'
import Sidebar from './components/Sidebar'
import IssueDetail, { type AgentConfigInput, type IssueUpdateInput } from './components/IssueDetail'
import RuntimeSettings from './components/RuntimeSettings'
import Timeline from './components/Timeline'
import { useDaemon } from './daemon'
import type {
  Agent,
  Issue,
  IssueActivity,
  IssueComment,
  Project,
  RuntimeDiscoveryResponse,
  RuntimeDiscoverySummary,
  RuntimeListResponse,
  RuntimeMutationResponse,
  RuntimeRecord,
  Tag,
} from './daemon'

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

const emptyProjectTags: ResourceState<Tag[]> = {
  status: 'idle',
  data: [],
  error: null,
}

const emptyLoadedProjectTags: ResourceState<Tag[]> = {
  status: 'success',
  data: [],
  error: null,
}

const emptyIssueTags: ResourceState<Tag[]> = {
  status: 'idle',
  data: [],
  error: null,
}

const emptyLoadedIssueTags: ResourceState<Tag[]> = {
  status: 'success',
  data: [],
  error: null,
}

const emptyComments: ResourceState<IssueComment[]> = {
  status: 'idle',
  data: [],
  error: null,
}

const emptyLoadedComments: ResourceState<IssueComment[]> = {
  status: 'success',
  data: [],
  error: null,
}

const emptyActivity: ResourceState<IssueActivity[]> = {
  status: 'idle',
  data: [],
  error: null,
}

const emptyLoadedActivity: ResourceState<IssueActivity[]> = {
  status: 'success',
  data: [],
  error: null,
}

const emptyAgents: ResourceState<Agent[]> = {
  status: 'idle',
  data: [],
  error: null,
}

const emptyLoadedAgents: ResourceState<Agent[]> = {
  status: 'success',
  data: [],
  error: null,
}

const emptyRuntimes: ResourceState<RuntimeRecord[]> = {
  status: 'idle',
  data: [],
  error: null,
}

const emptyLoadedRuntimes: ResourceState<RuntimeRecord[]> = {
  status: 'success',
  data: [],
  error: null,
}

function errorMessage(err: unknown) {
  return err instanceof Error ? err.message : String(err)
}

function jsonRequest(method: string, body?: unknown): RequestInit {
  return {
    method,
    body: body === undefined ? undefined : JSON.stringify(body),
  }
}

function runtimeList(data: RuntimeListResponse | RuntimeRecord[]): RuntimeRecord[] {
  return Array.isArray(data) ? data : data.runtimes
}

function mergeRuntime(
  runtimes: RuntimeRecord[],
  runtimePatch: RuntimeMutationResponse['runtime'],
): RuntimeRecord[] {
  return runtimes.map((runtime) =>
    runtime.id === runtimePatch.id ? { ...runtime, ...runtimePatch } : runtime,
  )
}

export default function App() {
  const { baseURL, error: daemonError, fetchApi, startDaemon } = useDaemon()
  const daemonStartRef = useRef<Promise<unknown> | null>(null)
  const issueLoadSeqRef = useRef(0)
  const [daemonStatus, setDaemonStatus] = useState<LoadStatus>('loading')
  const [projects, setProjects] = useState<ResourceState<Project[]>>(emptyProjects)
  const [issues, setIssues] = useState<ResourceState<Issue[]>>(emptyIssues)
  const [issueDetail, setIssueDetail] = useState<ResourceState<Issue | null>>(emptyIssueDetail)
  const [projectTags, setProjectTags] = useState<ResourceState<Tag[]>>(emptyProjectTags)
  const [issueTags, setIssueTags] = useState<ResourceState<Tag[]>>(emptyIssueTags)
  const [comments, setComments] = useState<ResourceState<IssueComment[]>>(emptyComments)
  const [activity, setActivity] = useState<ResourceState<IssueActivity[]>>(emptyActivity)
  const [agents, setAgents] = useState<ResourceState<Agent[]>>(emptyAgents)
  const [runtimes, setRuntimes] = useState<ResourceState<RuntimeRecord[]>>(emptyRuntimes)
  const [runtimeDiscoverySummary, setRuntimeDiscoverySummary] = useState<RuntimeDiscoverySummary | null>(null)
  const [preferredProjectId, setPreferredProjectId] = useState<string | null>(null)
  const [preferredIssueId, setPreferredIssueId] = useState<string | null>(null)
  const [activeView, setActiveView] = useState<'issues' | 'settings'>('issues')
  const [mutationPending, setMutationPending] = useState(false)
  const [mutationError, setMutationError] = useState<string | null>(null)
  const [runtimeMutationPending, setRuntimeMutationPending] = useState(false)
  const [runtimeMutationError, setRuntimeMutationError] = useState<string | null>(null)

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

  const visibleProjectTags =
    projects.status === 'success' && !selectedProjectId
      ? emptyLoadedProjectTags
      : projectTags

  const visibleIssueTags =
    visibleIssues.status === 'success' && !selectedIssueId
      ? emptyLoadedIssueTags
      : issueTags

  const visibleComments =
    visibleIssues.status === 'success' && !selectedIssueId
      ? emptyLoadedComments
      : comments

  const visibleActivity =
    visibleIssues.status === 'success' && !selectedIssueId
      ? emptyLoadedActivity
      : activity

  const fetchProjects = useCallback(() => fetchApi<Project[]>('/api/projects'), [fetchApi])

  const fetchIssues = useCallback(
    (projectId: string) => fetchApi<Issue[]>(`/api/projects/${projectId}/issues`),
    [fetchApi],
  )

  const fetchIssueDetail = useCallback(
    (issueId: string) => fetchApi<Issue>(`/api/issues/${issueId}`),
    [fetchApi],
  )

  const fetchProjectTags = useCallback(
    (projectId: string) => fetchApi<Tag[]>(`/api/projects/${projectId}/tags`),
    [fetchApi],
  )

  const fetchIssueTags = useCallback(
    (issueId: string) => fetchApi<Tag[]>(`/api/issues/${issueId}/tags`),
    [fetchApi],
  )

  const fetchComments = useCallback(
    (issueId: string) => fetchApi<IssueComment[]>(`/api/issues/${issueId}/comments`),
    [fetchApi],
  )

  const fetchActivity = useCallback(
    (issueId: string) => fetchApi<IssueActivity[]>(`/api/issues/${issueId}/activity`),
    [fetchApi],
  )

  const fetchAgents = useCallback(() => fetchApi<Agent[]>('/api/agents'), [fetchApi])

  const fetchRuntimes = useCallback(
    () => fetchApi<RuntimeListResponse | RuntimeRecord[]>('/api/runtimes'),
    [fetchApi],
  )

  const resetIssueInteractions = useCallback((loaded = false) => {
    setIssueTags(loaded ? emptyLoadedIssueTags : emptyIssueTags)
    setComments(loaded ? emptyLoadedComments : emptyComments)
    setActivity(loaded ? emptyLoadedActivity : emptyActivity)
  }, [])

  const resetWorkspaceCache = useCallback(() => {
    setProjects(emptyProjects)
    setIssues(emptyIssues)
    setIssueDetail(emptyIssueDetail)
    setProjectTags(emptyProjectTags)
    resetIssueInteractions()
    setAgents(emptyAgents)
    setRuntimes(emptyRuntimes)
    setRuntimeDiscoverySummary(null)
    setPreferredProjectId(null)
    setPreferredIssueId(null)
    setActiveView('issues')
    setMutationError(null)
    setRuntimeMutationError(null)
  }, [resetIssueInteractions])

  const loadAgents = useCallback(async () => {
    if (!baseURL) {
      setAgents(emptyLoadedAgents)
      return
    }
    setAgents((prev) => ({ ...prev, status: 'loading', error: null }))
    try {
      const data = await fetchAgents()
      setAgents({ status: 'success', data, error: null })
    } catch (err) {
      setAgents({ status: 'error', data: [], error: errorMessage(err) })
    }
  }, [baseURL, fetchAgents])

  const loadRuntimes = useCallback(async () => {
    if (!baseURL) {
      setRuntimes(emptyLoadedRuntimes)
      return
    }
    setRuntimes((prev) => ({ ...prev, status: 'loading', error: null }))
    try {
      const data = await fetchRuntimes()
      setRuntimes({ status: 'success', data: runtimeList(data), error: null })
    } catch (err) {
      setRuntimes({ status: 'error', data: [], error: errorMessage(err) })
    }
  }, [baseURL, fetchRuntimes])

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
      setProjectTags(data.length > 0 ? { status: 'loading', data: [], error: null } : emptyLoadedProjectTags)
      resetIssueInteractions(data.length === 0)
    } catch (err) {
      setProjects({ status: 'error', data: [], error: errorMessage(err) })
      setIssues(emptyIssues)
      setIssueDetail(emptyIssueDetail)
      setProjectTags(emptyProjectTags)
      resetIssueInteractions()
    }
  }, [baseURL, fetchProjects, resetIssueInteractions])

  const loadProjectTags = useCallback(
    async (projectId: string | null = selectedProjectId) => {
      if (!baseURL || !projectId) {
        setProjectTags(emptyLoadedProjectTags)
        return
      }

      setProjectTags((prev) => ({ ...prev, status: 'loading', error: null }))
      try {
        const data = await fetchProjectTags(projectId)
        setProjectTags({ status: 'success', data, error: null })
      } catch (err) {
        setProjectTags({ status: 'error', data: [], error: errorMessage(err) })
      }
    },
    [baseURL, fetchProjectTags, selectedProjectId],
  )

  const loadIssues = useCallback(
    async (projectId: string | null = selectedProjectId) => {
      if (!baseURL || !projectId) {
        setIssues(emptyLoadedIssues)
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
        resetIssueInteractions(data.length === 0)
      } catch (err) {
        setIssues({ status: 'error', data: [], error: errorMessage(err) })
        setIssueDetail(emptyLoadedIssueDetail)
        resetIssueInteractions(true)
      }
    },
    [baseURL, fetchIssues, resetIssueInteractions, selectedProjectId],
  )

  const loadIssueDetail = useCallback(
    async (issueId: string | null = selectedIssueId, sequence = issueLoadSeqRef.current) => {
      if (!baseURL || !issueId) {
        if (sequence === issueLoadSeqRef.current) {
          setIssueDetail(emptyLoadedIssueDetail)
        }
        return
      }

      if (sequence === issueLoadSeqRef.current) {
        setIssueDetail((prev) => ({ ...prev, status: 'loading', error: null }))
      }
      try {
        const data = await fetchIssueDetail(issueId)
        if (sequence === issueLoadSeqRef.current) {
          setIssueDetail({ status: 'success', data, error: null })
        }
      } catch (err) {
        if (sequence === issueLoadSeqRef.current) {
          setIssueDetail({ status: 'error', data: null, error: errorMessage(err) })
        }
      }
    },
    [baseURL, fetchIssueDetail, selectedIssueId],
  )

  const loadIssueTags = useCallback(
    async (issueId: string | null = selectedIssueId, sequence = issueLoadSeqRef.current) => {
      if (!baseURL || !issueId) {
        if (sequence === issueLoadSeqRef.current) {
          setIssueTags(emptyLoadedIssueTags)
        }
        return
      }

      if (sequence === issueLoadSeqRef.current) {
        setIssueTags((prev) => ({ ...prev, status: 'loading', error: null }))
      }
      try {
        const data = await fetchIssueTags(issueId)
        if (sequence === issueLoadSeqRef.current) {
          setIssueTags({ status: 'success', data, error: null })
        }
      } catch (err) {
        if (sequence === issueLoadSeqRef.current) {
          setIssueTags({ status: 'error', data: [], error: errorMessage(err) })
        }
      }
    },
    [baseURL, fetchIssueTags, selectedIssueId],
  )

  const loadComments = useCallback(
    async (issueId: string | null = selectedIssueId, sequence = issueLoadSeqRef.current) => {
      if (!baseURL || !issueId) {
        if (sequence === issueLoadSeqRef.current) {
          setComments(emptyLoadedComments)
        }
        return
      }

      if (sequence === issueLoadSeqRef.current) {
        setComments((prev) => ({ ...prev, status: 'loading', error: null }))
      }
      try {
        const data = await fetchComments(issueId)
        if (sequence === issueLoadSeqRef.current) {
          setComments({ status: 'success', data, error: null })
        }
      } catch (err) {
        if (sequence === issueLoadSeqRef.current) {
          setComments({ status: 'error', data: [], error: errorMessage(err) })
        }
      }
    },
    [baseURL, fetchComments, selectedIssueId],
  )

  const loadActivity = useCallback(
    async (issueId: string | null = selectedIssueId, sequence = issueLoadSeqRef.current) => {
      if (!baseURL || !issueId) {
        if (sequence === issueLoadSeqRef.current) {
          setActivity(emptyLoadedActivity)
        }
        return
      }

      if (sequence === issueLoadSeqRef.current) {
        setActivity((prev) => ({ ...prev, status: 'loading', error: null }))
      }
      try {
        const data = await fetchActivity(issueId)
        if (sequence === issueLoadSeqRef.current) {
          setActivity({ status: 'success', data, error: null })
        }
      } catch (err) {
        if (sequence === issueLoadSeqRef.current) {
          setActivity({ status: 'error', data: [], error: errorMessage(err) })
        }
      }
    },
    [baseURL, fetchActivity, selectedIssueId],
  )

  const reloadIssueContext = useCallback(
    async (
      issueId: string | null = selectedIssueId,
      projectId: string | null = selectedProjectId,
      sequence = issueLoadSeqRef.current,
    ) => {
      if (!issueId) {
        if (sequence === issueLoadSeqRef.current) {
          setIssueDetail(emptyLoadedIssueDetail)
          resetIssueInteractions(true)
        }
        return
      }

      await Promise.all([
        loadIssueDetail(issueId, sequence),
        loadIssueTags(issueId, sequence),
        loadComments(issueId, sequence),
        loadActivity(issueId, sequence),
        projectId ? loadProjectTags(projectId) : Promise.resolve(),
      ])
    },
    [
      loadActivity,
      loadComments,
      loadIssueDetail,
      loadIssueTags,
      loadProjectTags,
      resetIssueInteractions,
      selectedIssueId,
      selectedProjectId,
    ],
  )

  const runMutation = useCallback(async (operation: (sequence: number) => Promise<void>) => {
    const sequence = issueLoadSeqRef.current
    setMutationPending(true)
    setMutationError(null)
    try {
      await operation(sequence)
    } catch (err) {
      if (sequence === issueLoadSeqRef.current) {
        setMutationError(errorMessage(err))
      }
      throw err
    } finally {
      if (sequence === issueLoadSeqRef.current) {
        setMutationPending(false)
      }
    }
  }, [])

  const runRuntimeMutation = useCallback(async (operation: () => Promise<void>) => {
    setRuntimeMutationPending(true)
    setRuntimeMutationError(null)
    try {
      await operation()
    } catch (err) {
      setRuntimeMutationError(errorMessage(err))
      throw err
    } finally {
      setRuntimeMutationPending(false)
    }
  }, [])

  const discoverRuntimes = useCallback(async () => {
    if (!baseURL) return

    await runRuntimeMutation(async () => {
      const data = await fetchApi<RuntimeDiscoveryResponse>(
        '/api/runtimes/discover',
        jsonRequest('POST', { path_overrides: {} }),
      )
      setRuntimes({ status: 'success', data: data.runtimes, error: null })
      setRuntimeDiscoverySummary(data.summary)
      await loadAgents()
    })
  }, [baseURL, fetchApi, loadAgents, runRuntimeMutation])

  const updateRuntime = useCallback(
    async (runtimeId: string, input: { display_name?: string; binary_path?: string }) => {
      await runRuntimeMutation(async () => {
        const data = await fetchApi<RuntimeMutationResponse>(
          `/api/runtimes/${runtimeId}`,
          jsonRequest('PUT', input),
        )
        setRuntimes((prev) => ({ ...prev, data: mergeRuntime(prev.data, data.runtime) }))
        await loadAgents()
      })
    },
    [fetchApi, loadAgents, runRuntimeMutation],
  )

  const validateRuntime = useCallback(
    async (runtimeId: string) => {
      await runRuntimeMutation(async () => {
        const data = await fetchApi<RuntimeMutationResponse>(
          '/api/runtimes/validate',
          jsonRequest('POST', { runtime_id: runtimeId }),
        )
        setRuntimes((prev) => ({ ...prev, data: mergeRuntime(prev.data, data.runtime) }))
        await loadAgents()
      })
    },
    [fetchApi, loadAgents, runRuntimeMutation],
  )

  const updateIssue = useCallback(
    async (input: IssueUpdateInput) => {
      if (!selectedIssueId || !selectedProjectId) return

      await runMutation(async (sequence) => {
        await fetchApi<Issue>(`/api/issues/${selectedIssueId}`, jsonRequest('PUT', input))
        if (sequence !== issueLoadSeqRef.current) return
        await loadIssues(selectedProjectId)
        if (sequence !== issueLoadSeqRef.current) return
        await loadAgents()
        if (sequence !== issueLoadSeqRef.current) return
        await reloadIssueContext(selectedIssueId, selectedProjectId, sequence)
      })
    },
    [fetchApi, loadAgents, loadIssues, reloadIssueContext, runMutation, selectedIssueId, selectedProjectId],
  )

  const createTag = useCallback(
    async (name: string, color: string) => {
      if (!selectedIssueId || !selectedProjectId) return

      await runMutation(async (sequence) => {
        await fetchApi<Tag[]>(`/api/issues/${selectedIssueId}/tags`, jsonRequest('POST', { name, color }))
        if (sequence !== issueLoadSeqRef.current) return
        await reloadIssueContext(selectedIssueId, selectedProjectId, sequence)
      })
    },
    [fetchApi, reloadIssueContext, runMutation, selectedIssueId, selectedProjectId],
  )

  const attachTag = useCallback(
    async (tagId: string) => {
      if (!selectedIssueId || !selectedProjectId) return

      await runMutation(async (sequence) => {
        await fetchApi<Tag[]>(`/api/issues/${selectedIssueId}/tags`, jsonRequest('POST', { tag_id: tagId }))
        if (sequence !== issueLoadSeqRef.current) return
        await reloadIssueContext(selectedIssueId, selectedProjectId, sequence)
      })
    },
    [fetchApi, reloadIssueContext, runMutation, selectedIssueId, selectedProjectId],
  )

  const detachTag = useCallback(
    async (tagId: string) => {
      if (!selectedIssueId || !selectedProjectId) return

      await runMutation(async (sequence) => {
        await fetchApi<void>(`/api/issues/${selectedIssueId}/tags/${tagId}`, jsonRequest('DELETE'))
        if (sequence !== issueLoadSeqRef.current) return
        await reloadIssueContext(selectedIssueId, selectedProjectId, sequence)
      })
    },
    [fetchApi, reloadIssueContext, runMutation, selectedIssueId, selectedProjectId],
  )

  const createComment = useCallback(
    async (body: string) => {
      if (!selectedIssueId || !selectedProjectId) return

      await runMutation(async (sequence) => {
        await fetchApi<IssueComment>(
          `/api/issues/${selectedIssueId}/comments`,
          jsonRequest('POST', { body, author_id: 'local-user' }),
        )
        if (sequence !== issueLoadSeqRef.current) return
        await reloadIssueContext(selectedIssueId, selectedProjectId, sequence)
      })
    },
    [fetchApi, reloadIssueContext, runMutation, selectedIssueId, selectedProjectId],
  )

  const updateComment = useCallback(
    async (commentId: string, body: string) => {
      if (!selectedIssueId || !selectedProjectId) return

      await runMutation(async (sequence) => {
        await fetchApi<IssueComment>(`/api/comments/${commentId}`, jsonRequest('PUT', { body }))
        if (sequence !== issueLoadSeqRef.current) return
        await reloadIssueContext(selectedIssueId, selectedProjectId, sequence)
      })
    },
    [fetchApi, reloadIssueContext, runMutation, selectedIssueId, selectedProjectId],
  )

  const deleteComment = useCallback(
    async (commentId: string) => {
      if (!selectedIssueId || !selectedProjectId) return

      await runMutation(async (sequence) => {
        await fetchApi<void>(`/api/comments/${commentId}`, jsonRequest('DELETE'))
        if (sequence !== issueLoadSeqRef.current) return
        await reloadIssueContext(selectedIssueId, selectedProjectId, sequence)
      })
    },
    [fetchApi, reloadIssueContext, runMutation, selectedIssueId, selectedProjectId],
  )

  const createAgent = useCallback(
    async (input: AgentConfigInput) => {
      await runMutation(async () => {
        await fetchApi<Agent>('/api/agents', jsonRequest('POST', input))
        await loadAgents()
      })
    },
    [fetchApi, loadAgents, runMutation],
  )

  const updateAgent = useCallback(
    async (agentId: string, input: AgentConfigInput) => {
      await runMutation(async () => {
        await fetchApi<Agent>(`/api/agents/${agentId}`, jsonRequest('PUT', input))
        await loadAgents()
      })
    },
    [fetchApi, loadAgents, runMutation],
  )

  const openRuntimeSettings = useCallback(() => {
    setActiveView('settings')
    if (runtimes.status === 'idle') {
      void loadRuntimes()
    }
  }, [loadRuntimes, runtimes.status])

  const selectProject = useCallback(
    (projectId: string) => {
      if (projectId === selectedProjectId) return

      issueLoadSeqRef.current += 1
      setPreferredProjectId(projectId)
      setPreferredIssueId(null)
      setIssues({ status: 'loading', data: [], error: null })
      setIssueDetail({ status: 'loading', data: null, error: null })
      setProjectTags({ status: 'loading', data: [], error: null })
      resetIssueInteractions()
      setMutationPending(false)
      setMutationError(null)
    },
    [resetIssueInteractions, selectedProjectId],
  )

  const selectIssue = useCallback((issueId: string) => {
    issueLoadSeqRef.current += 1
    setPreferredIssueId(issueId)
    setIssueDetail({ status: 'loading', data: null, error: null })
    setIssueTags({ status: 'loading', data: [], error: null })
    setComments({ status: 'loading', data: [], error: null })
    setActivity({ status: 'loading', data: [], error: null })
    setMutationPending(false)
    setMutationError(null)
  }, [])

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
          setProjectTags(
            data.length > 0 ? { status: 'loading', data: [], error: null } : emptyLoadedProjectTags,
          )
          resetIssueInteractions(data.length === 0)
        }
      } catch (err) {
        if (!cancelled) {
          setProjects({ status: 'error', data: [], error: errorMessage(err) })
          setIssues(emptyIssues)
          setIssueDetail(emptyIssueDetail)
          setProjectTags(emptyProjectTags)
          resetIssueInteractions()
        }
      }
    }

    void load()

    return () => {
      cancelled = true
    }
  }, [baseURL, fetchProjects, resetIssueInteractions])

  useEffect(() => {
    if (!baseURL) return

    let cancelled = false

    async function load() {
      setAgents((prev) => ({ ...prev, status: 'loading', error: null }))
      setRuntimes((prev) => ({ ...prev, status: 'loading', error: null }))
      const [agentResult, runtimeResult] = await Promise.allSettled([fetchAgents(), fetchRuntimes()])

      if (cancelled) return

      if (agentResult.status === 'fulfilled') {
        setAgents({ status: 'success', data: agentResult.value ?? [], error: null })
      } else {
        setAgents({ status: 'error', data: [], error: errorMessage(agentResult.reason) })
      }

      if (runtimeResult.status === 'fulfilled') {
        setRuntimes({ status: 'success', data: runtimeList(runtimeResult.value ?? []), error: null })
      } else {
        setRuntimes({ status: 'error', data: [], error: errorMessage(runtimeResult.reason) })
      }
    }

    void load()

    return () => {
      cancelled = true
    }
  }, [baseURL, fetchAgents, fetchRuntimes])

  useEffect(() => {
    if (!baseURL || projects.status !== 'success') return

    if (!selectedProjectId) return

    let cancelled = false
    const projectId = selectedProjectId

    async function load() {
      try {
        const issueData = await fetchIssues(projectId)
        if (!cancelled) {
          setIssues({ status: 'success', data: issueData, error: null })
          setIssueDetail(
            issueData.length > 0
              ? { status: 'loading', data: null, error: null }
              : emptyLoadedIssueDetail,
          )
          resetIssueInteractions(issueData.length === 0)
        }
      } catch (err) {
        if (!cancelled) {
          setIssues({ status: 'error', data: [], error: errorMessage(err) })
          setIssueDetail(emptyLoadedIssueDetail)
          resetIssueInteractions(true)
        }
      }

      try {
        const tagData = await fetchProjectTags(projectId)
        if (!cancelled) {
          setProjectTags({ status: 'success', data: tagData ?? [], error: null })
        }
      } catch (err) {
        if (!cancelled) {
          setProjectTags({ status: 'error', data: [], error: errorMessage(err) })
        }
      }
    }

    void load()

    return () => {
      cancelled = true
    }
  }, [baseURL, fetchIssues, fetchProjectTags, projects.status, resetIssueInteractions, selectedProjectId])

  useEffect(() => {
    if (!baseURL || issues.status !== 'success') return

    if (!selectedIssueId) return

    let cancelled = false
    const issueId = selectedIssueId
    const sequence = issueLoadSeqRef.current

    async function load() {
      try {
        const issueData = await fetchIssueDetail(issueId)
        if (!cancelled && sequence === issueLoadSeqRef.current) {
          setIssueDetail({ status: 'success', data: issueData, error: null })
        }
      } catch (err) {
        if (!cancelled && sequence === issueLoadSeqRef.current) {
          setIssueDetail({ status: 'error', data: null, error: errorMessage(err) })
        }
      }

      await Promise.all([
        Promise.resolve()
          .then(() => fetchIssueTags(issueId))
          .then((data) => {
            if (!cancelled && sequence === issueLoadSeqRef.current) {
              setIssueTags({ status: 'success', data: data ?? [], error: null })
            }
          })
          .catch((err) => {
            if (!cancelled && sequence === issueLoadSeqRef.current) {
              setIssueTags({ status: 'error', data: [], error: errorMessage(err) })
            }
          }),
        Promise.resolve()
          .then(() => fetchComments(issueId))
          .then((data) => {
            if (!cancelled && sequence === issueLoadSeqRef.current) {
              setComments({ status: 'success', data: data ?? [], error: null })
            }
          })
          .catch((err) => {
            if (!cancelled && sequence === issueLoadSeqRef.current) {
              setComments({ status: 'error', data: [], error: errorMessage(err) })
            }
          }),
        Promise.resolve()
          .then(() => fetchActivity(issueId))
          .then((data) => {
            if (!cancelled && sequence === issueLoadSeqRef.current) {
              setActivity({ status: 'success', data: data ?? [], error: null })
            }
          })
          .catch((err) => {
            if (!cancelled && sequence === issueLoadSeqRef.current) {
              setActivity({ status: 'error', data: [], error: errorMessage(err) })
            }
          }),
      ])
    }

    void load()

    return () => {
      cancelled = true
    }
  }, [baseURL, fetchActivity, fetchComments, fetchIssueDetail, fetchIssueTags, issues.status, selectedIssueId])

  return (
    <div className="h-screen w-screen bg-zinc-950 text-zinc-100 flex overflow-hidden">
      <Sidebar
        activeView={activeView}
        daemonError={daemonError}
        daemonStatus={daemonStatus}
        projects={projects}
        selectedProjectId={selectedProjectId}
        issues={visibleIssues}
        selectedIssueId={selectedIssueId}
        onRetryDaemon={connectDaemon}
        onRetryProjects={loadProjects}
        onRetryIssues={() => loadIssues()}
        onOpenIssues={() => setActiveView('issues')}
        onOpenSettings={openRuntimeSettings}
        onSelectProject={selectProject}
        onSelectIssue={selectIssue}
      />
      {activeView === 'settings' ? (
        <RuntimeSettings
          runtimes={runtimes}
          discoverySummary={runtimeDiscoverySummary}
          mutationPending={runtimeMutationPending}
          mutationError={runtimeMutationError}
          onRetry={loadRuntimes}
          onDiscover={discoverRuntimes}
          onUpdateRuntime={updateRuntime}
          onValidateRuntime={validateRuntime}
        />
      ) : (
        <>
          <IssueDetail
            issue={visibleIssueDetail}
            projectTags={visibleProjectTags}
            issueTags={visibleIssueTags}
            comments={visibleComments}
            agents={agents}
            runtimes={runtimes}
            mutationPending={mutationPending}
            mutationError={mutationError}
            onRetryIssue={() => loadIssueDetail()}
            onRetryTags={() => {
              void Promise.all([loadProjectTags(), loadIssueTags()])
            }}
            onRetryComments={() => loadComments()}
            onRetryAgents={() => {
              void Promise.all([loadAgents(), loadRuntimes()])
            }}
            onUpdateIssue={updateIssue}
            onCreateTag={createTag}
            onAttachTag={attachTag}
            onDetachTag={detachTag}
            onCreateComment={createComment}
            onUpdateComment={updateComment}
            onDeleteComment={deleteComment}
            onCreateAgent={createAgent}
            onUpdateAgent={updateAgent}
          />
          <Timeline
            activity={visibleActivity}
            issueSelected={Boolean(visibleIssueDetail.data)}
            onRetry={() => loadActivity()}
          />
        </>
      )}
    </div>
  )
}
