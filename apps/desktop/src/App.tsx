import { useCallback, useEffect, useRef, useState } from 'react'
import Sidebar from './components/Sidebar'
import IssueDetail, { type IssueUpdateInput } from './components/IssueDetail'
import Timeline from './components/Timeline'
import { useDaemon } from './daemon'
import type { Issue, IssueActivity, IssueComment, Project, Tag } from './daemon'

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

function errorMessage(err: unknown) {
  return err instanceof Error ? err.message : String(err)
}

function jsonRequest(method: string, body?: unknown): RequestInit {
  return {
    method,
    body: body === undefined ? undefined : JSON.stringify(body),
  }
}

export default function App() {
  const { baseURL, error: daemonError, fetchApi, startDaemon } = useDaemon()
  const daemonStartRef = useRef<Promise<unknown> | null>(null)
  const [daemonStatus, setDaemonStatus] = useState<LoadStatus>('loading')
  const [projects, setProjects] = useState<ResourceState<Project[]>>(emptyProjects)
  const [issues, setIssues] = useState<ResourceState<Issue[]>>(emptyIssues)
  const [issueDetail, setIssueDetail] = useState<ResourceState<Issue | null>>(emptyIssueDetail)
  const [projectTags, setProjectTags] = useState<ResourceState<Tag[]>>(emptyProjectTags)
  const [issueTags, setIssueTags] = useState<ResourceState<Tag[]>>(emptyIssueTags)
  const [comments, setComments] = useState<ResourceState<IssueComment[]>>(emptyComments)
  const [activity, setActivity] = useState<ResourceState<IssueActivity[]>>(emptyActivity)
  const [preferredProjectId, setPreferredProjectId] = useState<string | null>(null)
  const [preferredIssueId, setPreferredIssueId] = useState<string | null>(null)
  const [mutationPending, setMutationPending] = useState(false)
  const [mutationError, setMutationError] = useState<string | null>(null)

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
    setPreferredProjectId(null)
    setPreferredIssueId(null)
    setMutationError(null)
  }, [resetIssueInteractions])

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
    async (issueId: string | null = selectedIssueId) => {
      if (!baseURL || !issueId) {
        setIssueDetail(emptyLoadedIssueDetail)
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

  const loadIssueTags = useCallback(
    async (issueId: string | null = selectedIssueId) => {
      if (!baseURL || !issueId) {
        setIssueTags(emptyLoadedIssueTags)
        return
      }

      setIssueTags((prev) => ({ ...prev, status: 'loading', error: null }))
      try {
        const data = await fetchIssueTags(issueId)
        setIssueTags({ status: 'success', data, error: null })
      } catch (err) {
        setIssueTags({ status: 'error', data: [], error: errorMessage(err) })
      }
    },
    [baseURL, fetchIssueTags, selectedIssueId],
  )

  const loadComments = useCallback(
    async (issueId: string | null = selectedIssueId) => {
      if (!baseURL || !issueId) {
        setComments(emptyLoadedComments)
        return
      }

      setComments((prev) => ({ ...prev, status: 'loading', error: null }))
      try {
        const data = await fetchComments(issueId)
        setComments({ status: 'success', data, error: null })
      } catch (err) {
        setComments({ status: 'error', data: [], error: errorMessage(err) })
      }
    },
    [baseURL, fetchComments, selectedIssueId],
  )

  const loadActivity = useCallback(
    async (issueId: string | null = selectedIssueId) => {
      if (!baseURL || !issueId) {
        setActivity(emptyLoadedActivity)
        return
      }

      setActivity((prev) => ({ ...prev, status: 'loading', error: null }))
      try {
        const data = await fetchActivity(issueId)
        setActivity({ status: 'success', data, error: null })
      } catch (err) {
        setActivity({ status: 'error', data: [], error: errorMessage(err) })
      }
    },
    [baseURL, fetchActivity, selectedIssueId],
  )

  const reloadIssueContext = useCallback(
    async (issueId: string | null = selectedIssueId, projectId: string | null = selectedProjectId) => {
      if (!issueId) {
        setIssueDetail(emptyLoadedIssueDetail)
        resetIssueInteractions(true)
        return
      }

      await Promise.all([
        loadIssueDetail(issueId),
        loadIssueTags(issueId),
        loadComments(issueId),
        loadActivity(issueId),
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

  const runMutation = useCallback(async (operation: () => Promise<void>) => {
    setMutationPending(true)
    setMutationError(null)
    try {
      await operation()
    } catch (err) {
      setMutationError(errorMessage(err))
      throw err
    } finally {
      setMutationPending(false)
    }
  }, [])

  const updateIssue = useCallback(
    async (input: IssueUpdateInput) => {
      if (!selectedIssueId || !selectedProjectId) return

      await runMutation(async () => {
        await fetchApi<Issue>(`/api/issues/${selectedIssueId}`, jsonRequest('PUT', input))
        await Promise.all([loadIssues(selectedProjectId), reloadIssueContext(selectedIssueId, selectedProjectId)])
      })
    },
    [fetchApi, loadIssues, reloadIssueContext, runMutation, selectedIssueId, selectedProjectId],
  )

  const createTag = useCallback(
    async (name: string, color: string) => {
      if (!selectedIssueId || !selectedProjectId) return

      await runMutation(async () => {
        await fetchApi<Tag[]>(`/api/issues/${selectedIssueId}/tags`, jsonRequest('POST', { name, color }))
        await reloadIssueContext(selectedIssueId, selectedProjectId)
      })
    },
    [fetchApi, reloadIssueContext, runMutation, selectedIssueId, selectedProjectId],
  )

  const attachTag = useCallback(
    async (tagId: string) => {
      if (!selectedIssueId || !selectedProjectId) return

      await runMutation(async () => {
        await fetchApi<Tag[]>(`/api/issues/${selectedIssueId}/tags`, jsonRequest('POST', { tag_id: tagId }))
        await reloadIssueContext(selectedIssueId, selectedProjectId)
      })
    },
    [fetchApi, reloadIssueContext, runMutation, selectedIssueId, selectedProjectId],
  )

  const detachTag = useCallback(
    async (tagId: string) => {
      if (!selectedIssueId || !selectedProjectId) return

      await runMutation(async () => {
        await fetchApi<void>(`/api/issues/${selectedIssueId}/tags/${tagId}`, jsonRequest('DELETE'))
        await reloadIssueContext(selectedIssueId, selectedProjectId)
      })
    },
    [fetchApi, reloadIssueContext, runMutation, selectedIssueId, selectedProjectId],
  )

  const createComment = useCallback(
    async (body: string) => {
      if (!selectedIssueId || !selectedProjectId) return

      await runMutation(async () => {
        await fetchApi<IssueComment>(
          `/api/issues/${selectedIssueId}/comments`,
          jsonRequest('POST', { body, author_id: 'local-user' }),
        )
        await reloadIssueContext(selectedIssueId, selectedProjectId)
      })
    },
    [fetchApi, reloadIssueContext, runMutation, selectedIssueId, selectedProjectId],
  )

  const updateComment = useCallback(
    async (commentId: string, body: string) => {
      if (!selectedIssueId || !selectedProjectId) return

      await runMutation(async () => {
        await fetchApi<IssueComment>(`/api/comments/${commentId}`, jsonRequest('PUT', { body }))
        await reloadIssueContext(selectedIssueId, selectedProjectId)
      })
    },
    [fetchApi, reloadIssueContext, runMutation, selectedIssueId, selectedProjectId],
  )

  const deleteComment = useCallback(
    async (commentId: string) => {
      if (!selectedIssueId || !selectedProjectId) return

      await runMutation(async () => {
        await fetchApi<void>(`/api/comments/${commentId}`, jsonRequest('DELETE'))
        await reloadIssueContext(selectedIssueId, selectedProjectId)
      })
    },
    [fetchApi, reloadIssueContext, runMutation, selectedIssueId, selectedProjectId],
  )

  const selectProject = useCallback(
    (projectId: string) => {
      if (projectId === selectedProjectId) return

      setPreferredProjectId(projectId)
      setPreferredIssueId(null)
      setIssues({ status: 'loading', data: [], error: null })
      setIssueDetail({ status: 'loading', data: null, error: null })
      setProjectTags({ status: 'loading', data: [], error: null })
      resetIssueInteractions()
      setMutationError(null)
    },
    [resetIssueInteractions, selectedProjectId],
  )

  const selectIssue = useCallback((issueId: string) => {
    setPreferredIssueId(issueId)
    setIssueDetail({ status: 'loading', data: null, error: null })
    setIssueTags({ status: 'loading', data: [], error: null })
    setComments({ status: 'loading', data: [], error: null })
    setActivity({ status: 'loading', data: [], error: null })
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

    async function load() {
      try {
        const issueData = await fetchIssueDetail(issueId)
        if (!cancelled) {
          setIssueDetail({ status: 'success', data: issueData, error: null })
        }
      } catch (err) {
        if (!cancelled) {
          setIssueDetail({ status: 'error', data: null, error: errorMessage(err) })
        }
      }

      await Promise.all([
        Promise.resolve()
          .then(() => fetchIssueTags(issueId))
          .then((data) => {
            if (!cancelled) setIssueTags({ status: 'success', data: data ?? [], error: null })
          })
          .catch((err) => {
            if (!cancelled) setIssueTags({ status: 'error', data: [], error: errorMessage(err) })
          }),
        Promise.resolve()
          .then(() => fetchComments(issueId))
          .then((data) => {
            if (!cancelled) setComments({ status: 'success', data: data ?? [], error: null })
          })
          .catch((err) => {
            if (!cancelled) setComments({ status: 'error', data: [], error: errorMessage(err) })
          }),
        Promise.resolve()
          .then(() => fetchActivity(issueId))
          .then((data) => {
            if (!cancelled) setActivity({ status: 'success', data: data ?? [], error: null })
          })
          .catch((err) => {
            if (!cancelled) setActivity({ status: 'error', data: [], error: errorMessage(err) })
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
        onSelectIssue={selectIssue}
      />
      <IssueDetail
        issue={visibleIssueDetail}
        projectTags={visibleProjectTags}
        issueTags={visibleIssueTags}
        comments={visibleComments}
        mutationPending={mutationPending}
        mutationError={mutationError}
        onRetryIssue={() => loadIssueDetail()}
        onRetryTags={() => {
          void Promise.all([loadProjectTags(), loadIssueTags()])
        }}
        onRetryComments={() => loadComments()}
        onUpdateIssue={updateIssue}
        onCreateTag={createTag}
        onAttachTag={attachTag}
        onDetachTag={detachTag}
        onCreateComment={createComment}
        onUpdateComment={updateComment}
        onDeleteComment={deleteComment}
      />
      <Timeline
        activity={visibleActivity}
        issueSelected={Boolean(visibleIssueDetail.data)}
        onRetry={() => loadActivity()}
      />
    </div>
  )
}
