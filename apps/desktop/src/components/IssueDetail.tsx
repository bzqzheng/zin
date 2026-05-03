import { useEffect, useMemo, useState } from 'react'
import type { ResourceState } from '../App'
import type { Agent, Issue, IssueAssignment, IssueComment, RuntimeRecord, Tag } from '../daemon'

export interface IssueUpdateInput {
  title: string
  description: string
  status: string
  priority: string
  assignee_id: string
}

export interface AgentConfigInput {
  name: string
  role: string
  runtime_id: string
  model: string
  instructions: string
}

interface IssueDetailProps {
  issue: ResourceState<Issue | null>
  projectTags: ResourceState<Tag[]>
  issueTags: ResourceState<Tag[]>
  comments: ResourceState<IssueComment[]>
  assignments: ResourceState<IssueAssignment[]>
  agents: ResourceState<Agent[]>
  runtimes: ResourceState<RuntimeRecord[]>
  mutationPending: boolean
  mutationError: string | null
  onRetryIssue: () => void
  onRetryTags: () => void
  onRetryComments: () => void
  onRetryAssignments: () => void
  onRetryAgents: () => void
  onUpdateIssue: (input: IssueUpdateInput) => Promise<void>
  onCreateTag: (name: string, color: string) => Promise<void>
  onAttachTag: (tagId: string) => Promise<void>
  onDetachTag: (tagId: string) => Promise<void>
  onCreateComment: (body: string) => Promise<void>
  onUpdateComment: (commentId: string, body: string) => Promise<void>
  onDeleteComment: (commentId: string) => Promise<void>
  onCreateAssignment: (agentId: string, source: { type: 'issue_detail' | 'comment'; id?: string }) => Promise<void>
  onCancelAssignment: (assignmentId: string) => Promise<void>
  onCreateAgent: (input: AgentConfigInput) => Promise<void>
  onUpdateAgent: (agentId: string, input: AgentConfigInput) => Promise<void>
}

const labelStyle = 'text-xs bg-zinc-900 px-2 py-0.5 rounded'
const inputStyle =
  'w-full rounded border border-zinc-800 bg-zinc-950 px-2.5 py-2 text-sm text-zinc-100 outline-none focus:border-zinc-600'
const buttonStyle =
  'rounded border border-zinc-700 px-3 py-1.5 text-sm font-medium text-zinc-200 hover:border-zinc-500 hover:text-white disabled:cursor-not-allowed disabled:opacity-50'

function formatDate(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  }).format(new Date(value))
}

function formatDateTime(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  }).format(new Date(value))
}

function displayLabel(value: string) {
  return value
    .split('_')
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

function EmptyDetail() {
  return (
    <div className="max-w-3xl mx-auto px-8 py-6">
      <p className="text-sm text-zinc-500">No issue selected</p>
    </div>
  )
}

function SectionError({
  title,
  error,
  onRetry,
}: {
  title: string
  error: string | null
  onRetry: () => void
}) {
  return (
    <div className="rounded border border-red-900/50 bg-red-950/20 px-3 py-2">
      <p className="text-sm text-red-300">{title}</p>
      {error && <p className="mt-1 text-sm text-zinc-500">{error}</p>}
      <button type="button" onClick={onRetry} className="mt-2 text-sm font-medium text-zinc-200 hover:text-white">
        Retry
      </button>
    </div>
  )
}

function CommentItem({
  comment,
  disabled,
  assignableAgents,
  onUpdate,
  onDelete,
  onAssign,
}: {
  comment: IssueComment
  disabled: boolean
  assignableAgents: Agent[]
  onUpdate: (commentId: string, body: string) => Promise<void>
  onDelete: (commentId: string) => Promise<void>
  onAssign: (agentId: string, commentId: string) => Promise<void>
}) {
  const [editing, setEditing] = useState(false)
  const [body, setBody] = useState(comment.body)
  const [selectedAgentId, setSelectedAgentId] = useState('')

  useEffect(() => {
    if (!assignableAgents.some((agent) => agent.id === selectedAgentId)) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setSelectedAgentId(assignableAgents[0]?.id ?? '')
    }
  }, [assignableAgents, selectedAgentId])

  if (editing) {
    return (
      <div className="rounded border border-zinc-800 bg-zinc-900/40 p-3">
        <textarea
          value={body}
          onChange={(event) => setBody(event.target.value)}
          rows={3}
          className={inputStyle}
        />
        <div className="mt-2 flex gap-2">
          <button
            type="button"
            disabled={disabled || body.trim() === ''}
            onClick={async () => {
              try {
                await onUpdate(comment.id, body)
                setEditing(false)
              } catch {
                // Parent renders the recoverable mutation error.
              }
            }}
            className={buttonStyle}
          >
            Save
          </button>
          <button
            type="button"
            disabled={disabled}
            onClick={() => {
              setBody(comment.body)
              setEditing(false)
            }}
            className={buttonStyle}
          >
            Cancel
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="rounded border border-zinc-800 bg-zinc-900/40 p-3">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-sm font-medium text-zinc-300">{comment.author_name || 'You'}</p>
          <p className="text-xs text-zinc-500">{formatDateTime(comment.created_at)}</p>
        </div>
        <div className="flex gap-2">
          <button
            type="button"
            disabled={disabled}
            onClick={() => {
              setBody(comment.body)
              setEditing(true)
            }}
            className="text-xs text-zinc-400 hover:text-white"
          >
            Edit
          </button>
          <button
            type="button"
            disabled={disabled}
            onClick={() => {
              void onDelete(comment.id).catch(() => undefined)
            }}
            className="text-xs text-red-300 hover:text-red-100"
          >
            Delete
          </button>
        </div>
      </div>
      <p className="mt-2 whitespace-pre-wrap text-sm leading-relaxed text-zinc-400">{comment.body}</p>
      <div className="mt-3 flex flex-wrap items-center gap-2">
        <select
          value={selectedAgentId}
          onChange={(event) => setSelectedAgentId(event.target.value)}
          disabled={disabled || assignableAgents.length === 0}
          className="min-w-48 rounded border border-zinc-800 bg-zinc-950 px-2 py-1.5 text-xs text-zinc-100 outline-none focus:border-zinc-600"
          aria-label={`Assign comment from ${comment.author_name || 'You'}`}
        >
          {assignableAgents.length === 0 ? (
            <option value="">No assignable agents</option>
          ) : (
            assignableAgents.map((agent) => (
              <option key={agent.id} value={agent.id}>
                {agent.name}
              </option>
            ))
          )}
        </select>
        <button
          type="button"
          disabled={disabled || selectedAgentId === ''}
          onClick={() => {
            void onAssign(selectedAgentId, comment.id).catch(() => undefined)
          }}
          className="rounded border border-zinc-700 px-2.5 py-1.5 text-xs font-medium text-zinc-200 hover:border-zinc-500 hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
        >
          Assign
        </button>
      </div>
    </div>
  )
}

function assignmentNeedsRuntimeWarning(assignment: IssueAssignment) {
  return assignment.status === 'queued' && assignment.runtime_status !== '' && assignment.runtime_status !== 'healthy'
}

function AssignmentItem({
  assignment,
  disabled,
  onCancel,
}: {
  assignment: IssueAssignment
  disabled: boolean
  onCancel: (assignmentId: string) => Promise<void>
}) {
  return (
    <div className="rounded border border-zinc-800 bg-zinc-900/40 p-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm font-medium text-zinc-200">{assignment.agent_name || 'Deleted agent'}</span>
            <span className={`${labelStyle} text-zinc-400`}>{displayLabel(assignment.status)}</span>
            <span className={`${labelStyle} text-zinc-500`}>
              {assignment.source_type === 'comment' ? 'Comment' : 'Issue detail'}
            </span>
          </div>
          <p className="mt-1 text-xs text-zinc-500">
            Queued {formatDateTime(assignment.requested_at)}{assignment.runtime_name ? ` via ${assignment.runtime_name}` : ''}
          </p>
          {assignmentNeedsRuntimeWarning(assignment) && (
            <p className="mt-2 text-xs text-amber-300">
              Runtime is {displayLabel(assignment.runtime_status)}. This assignment remains queued.
            </p>
          )}
        </div>
        {assignment.status !== 'completed' && assignment.status !== 'failed' && assignment.status !== 'cancelled' && (
          <button
            type="button"
            disabled={disabled}
            onClick={() => {
              void onCancel(assignment.id).catch(() => undefined)
            }}
            className="text-xs text-red-300 hover:text-red-100 disabled:cursor-not-allowed disabled:opacity-50"
          >
            Cancel
          </button>
        )}
      </div>
    </div>
  )
}

export default function IssueDetail({
  issue,
  projectTags,
  issueTags,
  comments,
  assignments,
  agents,
  runtimes,
  mutationPending,
  mutationError,
  onRetryIssue,
  onRetryTags,
  onRetryComments,
  onRetryAssignments,
  onRetryAgents,
  onUpdateIssue,
  onCreateTag,
  onAttachTag,
  onDetachTag,
  onCreateComment,
  onUpdateComment,
  onDeleteComment,
  onCreateAssignment,
  onCancelAssignment,
  onCreateAgent,
  onUpdateAgent,
}: IssueDetailProps) {
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [status, setStatus] = useState('todo')
  const [priority, setPriority] = useState('medium')
  const [assigneeId, setAssigneeId] = useState('')
  const [newTagName, setNewTagName] = useState('')
  const [newTagColor, setNewTagColor] = useState('#3b82f6')
  const [selectedTagId, setSelectedTagId] = useState('')
  const [commentBody, setCommentBody] = useState('')
  const [selectedAssignmentAgentId, setSelectedAssignmentAgentId] = useState('')
  const [selectedConfigAgentId, setSelectedConfigAgentId] = useState('')
  const [agentName, setAgentName] = useState('')
  const [agentRole, setAgentRole] = useState('')
  const [agentRuntimeId, setAgentRuntimeId] = useState('')
  const [agentModel, setAgentModel] = useState('')
  const [agentInstructions, setAgentInstructions] = useState('')

  useEffect(() => {
    if (!issue.data) return
    // Server reloads are source of truth after mutations; reset draft fields to the latest issue.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setTitle(issue.data.title)
    setDescription(issue.data.description)
    setStatus(issue.data.status)
    setPriority(issue.data.priority)
    setAssigneeId(issue.data.assignee_id)
  }, [issue.data])

  const availableTags = useMemo(() => {
    const attachedTags = Array.isArray(issueTags.data) ? issueTags.data : []
    const allTags = Array.isArray(projectTags.data) ? projectTags.data : []
    const attached = new Set(attachedTags.map((tag) => tag.id))
    return allTags.filter((tag) => !attached.has(tag.id))
  }, [issueTags.data, projectTags.data])

  const assignableAgents = useMemo(() => agents.data.filter((agent) => agent.assignable), [agents.data])

  const activeAssignments = useMemo(
    () => assignments.data.filter((assignment) => assignment.status !== 'cancelled'),
    [assignments.data],
  )

  const selectedAgent = useMemo(
    () => agents.data.find((agent) => agent.id === assigneeId) ?? null,
    [agents.data, assigneeId],
  )

  const selectedConfigAgent = useMemo(
    () => agents.data.find((agent) => agent.id === selectedConfigAgentId) ?? null,
    [agents.data, selectedConfigAgentId],
  )

  useEffect(() => {
    if (agents.status !== 'success') return
    if (assignableAgents.some((agent) => agent.id === selectedAssignmentAgentId)) return
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setSelectedAssignmentAgentId(assignableAgents[0]?.id ?? '')
  }, [agents.status, assignableAgents, selectedAssignmentAgentId])

  useEffect(() => {
    if (agents.status !== 'success') return
    if (selectedConfigAgentId && agents.data.some((agent) => agent.id === selectedConfigAgentId)) return
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setSelectedConfigAgentId(agents.data[0]?.id ?? '')
  }, [agents.data, agents.status, selectedConfigAgentId])

  useEffect(() => {
    if (!selectedConfigAgent) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setAgentName('')
      setAgentRole('')
      setAgentRuntimeId(runtimes.data[0]?.id ?? '')
      setAgentModel('')
      setAgentInstructions('')
      return
    }
    setAgentName(selectedConfigAgent.name)
    setAgentRole(selectedConfigAgent.role)
    setAgentRuntimeId(selectedConfigAgent.runtime_id)
    setAgentModel(selectedConfigAgent.model)
    setAgentInstructions(selectedConfigAgent.instructions)
  }, [runtimes.data, selectedConfigAgent])

  useEffect(() => {
    if (!availableTags.some((tag) => tag.id === selectedTagId)) {
      // Keep the attach select on a valid server-backed option as tags reload.
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setSelectedTagId(availableTags[0]?.id ?? '')
    }
  }, [availableTags, selectedTagId])

  if (issue.status === 'loading') {
    return (
      <main className="flex-1 bg-zinc-950 overflow-y-auto">
        <div className="max-w-3xl mx-auto px-8 py-6">
          <p className="text-sm text-zinc-500">Loading issue...</p>
        </div>
      </main>
    )
  }

  if (issue.status === 'error') {
    return (
      <main className="flex-1 bg-zinc-950 overflow-y-auto">
        <div className="max-w-3xl mx-auto px-8 py-6">
          <SectionError title="Issue failed to load" error={issue.error} onRetry={onRetryIssue} />
        </div>
      </main>
    )
  }

  if (!issue.data) {
    return (
      <main className="flex-1 bg-zinc-950 overflow-y-auto">
        <EmptyDetail />
      </main>
    )
  }

  return (
    <main className="flex-1 bg-zinc-950 overflow-y-auto">
      <div className="max-w-3xl mx-auto px-8 py-6">
        <div className="mb-6">
          <div className="mb-2 flex flex-wrap items-center gap-3">
            <span className={`${labelStyle} font-mono text-zinc-500`}>
              {issue.data.identifier}
            </span>
            <span className={`${labelStyle} font-medium text-amber-500`}>
              {displayLabel(issue.data.priority)}
            </span>
            <span className={`${labelStyle} text-zinc-500`}>
              {displayLabel(issue.data.status)}
            </span>
          </div>
          <h2 className="text-xl font-semibold text-zinc-100">
            {issue.data.title}
          </h2>
          <p className="mt-1 text-sm text-zinc-500">
            Created {formatDate(issue.data.created_at)}
          </p>
          {activeAssignments.length > 0 && (
            <div className="mt-3 flex flex-wrap gap-2">
              {activeAssignments.map((assignment) => (
                <span
                  key={assignment.id}
                  className="inline-flex items-center gap-2 rounded border border-zinc-800 bg-zinc-900 px-2.5 py-1 text-xs text-zinc-300"
                >
                  {assignment.agent_name || 'Deleted agent'}
                  <span className="text-zinc-500">{displayLabel(assignment.status)}</span>
                  {assignmentNeedsRuntimeWarning(assignment) && <span className="text-amber-300">Runtime degraded</span>}
                </span>
              ))}
            </div>
          )}
        </div>

        {mutationError && (
          <div className="mb-5 rounded border border-red-900/50 bg-red-950/20 px-3 py-2">
            <p className="text-sm text-red-300">Update failed</p>
            <p className="mt-1 text-sm text-zinc-500">{mutationError}</p>
            <p className="mt-1 text-xs text-zinc-500">Adjust the fields or retry the same action.</p>
          </div>
        )}

        <section className="mb-8">
          <h3 className="mb-3 text-sm font-medium text-zinc-300">Fields</h3>
          <div className="space-y-3">
            <label className="block">
              <span className="mb-1 block text-xs font-medium text-zinc-500">Title</span>
              <input value={title} onChange={(event) => setTitle(event.target.value)} className={inputStyle} />
            </label>
            <label className="block">
              <span className="mb-1 block text-xs font-medium text-zinc-500">Description</span>
              <textarea
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                rows={5}
                className={inputStyle}
              />
            </label>
            <div className="grid grid-cols-3 gap-3">
              <label className="block">
                <span className="mb-1 block text-xs font-medium text-zinc-500">Status</span>
                <select value={status} onChange={(event) => setStatus(event.target.value)} className={inputStyle}>
                  <option value="todo">Todo</option>
                  <option value="in_progress">In Progress</option>
                  <option value="blocked">Blocked</option>
                  <option value="done">Done</option>
                </select>
              </label>
              <label className="block">
                <span className="mb-1 block text-xs font-medium text-zinc-500">Priority</span>
                <select value={priority} onChange={(event) => setPriority(event.target.value)} className={inputStyle}>
                  <option value="low">Low</option>
                  <option value="medium">Medium</option>
                  <option value="high">High</option>
                </select>
              </label>
              <label className="block">
                <span className="mb-1 block text-xs font-medium text-zinc-500">Assignee</span>
                <select
                  value={assigneeId}
                  onChange={(event) => setAssigneeId(event.target.value)}
                  disabled={agents.status !== 'success'}
                  className={inputStyle}
                >
                  <option value="">Unassigned</option>
                  {selectedAgent && !selectedAgent.assignable && (
                    <option value={selectedAgent.id}>
                      {selectedAgent.name} ({displayLabel(selectedAgent.runtime_status || 'unavailable')})
                    </option>
                  )}
                  {assignableAgents.map((agent) => (
                    <option key={agent.id} value={agent.id}>
                      {agent.name}
                      {agent.model ? ` · ${agent.model}` : ''}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            {(agents.status === 'loading' || runtimes.status === 'loading') && (
              <p className="text-xs text-zinc-500">Loading agent runtime state...</p>
            )}
            {(agents.status === 'error' || runtimes.status === 'error') && (
              <SectionError
                title="Agents failed to load"
                error={agents.error ?? runtimes.error}
                onRetry={onRetryAgents}
              />
            )}
            {agents.status === 'success' && agents.data.length === 0 && (
              <p className="text-xs text-zinc-500">No agents configured yet.</p>
            )}
            {agents.status === 'success' && agents.data.length > 0 && assignableAgents.length === 0 && (
              <p className="text-xs text-amber-300">
                No assignable agents. Bind an agent to a healthy runtime before assigning work.
              </p>
            )}
            {selectedAgent && !selectedAgent.assignable && (
              <p className="text-xs text-amber-300">{selectedAgent.assignable_reason}</p>
            )}
            <button
              type="button"
              disabled={mutationPending || title.trim() === ''}
              onClick={() => {
                void onUpdateIssue({
                  title,
                  description,
                  status,
                  priority,
                  assignee_id: assigneeId,
                }).catch(() => undefined)
              }}
              className={buttonStyle}
            >
              Save Fields
            </button>
          </div>
        </section>

        <section className="mb-8">
          <div className="mb-3 flex items-center justify-between gap-3">
            <h3 className="text-sm font-medium text-zinc-300">Assignments</h3>
            {assignments.status === 'loading' && <span className="text-xs text-zinc-500">Loading assignments...</span>}
          </div>
          {assignments.status === 'error' ? (
            <SectionError title="Assignments failed to load" error={assignments.error} onRetry={onRetryAssignments} />
          ) : (
            <div className="space-y-3">
              <div className="flex flex-wrap gap-2">
                <select
                  value={selectedAssignmentAgentId}
                  onChange={(event) => setSelectedAssignmentAgentId(event.target.value)}
                  disabled={mutationPending || assignableAgents.length === 0}
                  className={inputStyle}
                >
                  {assignableAgents.length === 0 ? (
                    <option value="">No assignable agents</option>
                  ) : (
                    assignableAgents.map((agent) => (
                      <option key={agent.id} value={agent.id}>
                        {agent.name}
                        {agent.model ? ` · ${agent.model}` : ''}
                      </option>
                    ))
                  )}
                </select>
                <button
                  type="button"
                  disabled={mutationPending || selectedAssignmentAgentId === ''}
                  onClick={() => {
                    void onCreateAssignment(selectedAssignmentAgentId, { type: 'issue_detail' }).catch(() => undefined)
                  }}
                  className={buttonStyle}
                >
                  Assign Agent
                </button>
              </div>
              {assignments.data.length === 0 && <p className="text-sm text-zinc-500">No assignments yet</p>}
              {assignments.data.map((assignment) => (
                <AssignmentItem
                  key={assignment.id}
                  assignment={assignment}
                  disabled={mutationPending}
                  onCancel={onCancelAssignment}
                />
              ))}
            </div>
          )}
        </section>

        <section className="mb-8">
          <div className="mb-3 flex items-center justify-between gap-3">
            <h3 className="text-sm font-medium text-zinc-300">Agents</h3>
            {(agents.status === 'loading' || runtimes.status === 'loading') && (
              <span className="text-xs text-zinc-500">Loading agents...</span>
            )}
          </div>
          {(agents.status === 'error' || runtimes.status === 'error') ? (
            <SectionError
              title="Agent configuration failed to load"
              error={agents.error ?? runtimes.error}
              onRetry={onRetryAgents}
            />
          ) : (
            <div className="space-y-3">
              <div className="grid grid-cols-2 gap-3">
                <label className="block">
                  <span className="mb-1 block text-xs font-medium text-zinc-500">Agent</span>
                  <select
                    value={selectedConfigAgentId}
                    onChange={(event) => setSelectedConfigAgentId(event.target.value)}
                    className={inputStyle}
                  >
                    <option value="">New agent</option>
                    {agents.data.map((agent) => (
                      <option key={agent.id} value={agent.id}>
                        {agent.name}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="block">
                  <span className="mb-1 block text-xs font-medium text-zinc-500">Runtime</span>
                  <select
                    value={agentRuntimeId}
                    onChange={(event) => setAgentRuntimeId(event.target.value)}
                    className={inputStyle}
                  >
                    <option value="">No runtime</option>
                    {runtimes.data.map((runtime) => (
                      <option key={runtime.id} value={runtime.id}>
                        {runtime.display_name} ({displayLabel(runtime.health_status)})
                      </option>
                    ))}
                  </select>
                </label>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <label className="block">
                  <span className="mb-1 block text-xs font-medium text-zinc-500">Name</span>
                  <input value={agentName} onChange={(event) => setAgentName(event.target.value)} className={inputStyle} />
                </label>
                <label className="block">
                  <span className="mb-1 block text-xs font-medium text-zinc-500">Role</span>
                  <input value={agentRole} onChange={(event) => setAgentRole(event.target.value)} className={inputStyle} />
                </label>
              </div>
              <label className="block">
                <span className="mb-1 block text-xs font-medium text-zinc-500">Model</span>
                <input value={agentModel} onChange={(event) => setAgentModel(event.target.value)} className={inputStyle} />
              </label>
              <label className="block">
                <span className="mb-1 block text-xs font-medium text-zinc-500">Instructions</span>
                <textarea
                  value={agentInstructions}
                  onChange={(event) => setAgentInstructions(event.target.value)}
                  rows={3}
                  className={inputStyle}
                />
              </label>
              {selectedConfigAgent && !selectedConfigAgent.assignable && (
                <p className="text-xs text-amber-300">{selectedConfigAgent.assignable_reason}</p>
              )}
              {runtimes.status === 'success' && runtimes.data.length === 0 && (
                <p className="text-xs text-zinc-500">Add or discover a runtime before this agent can become assignable.</p>
              )}
              <button
                type="button"
                disabled={mutationPending || agentName.trim() === ''}
                onClick={async () => {
                  const input = {
                    name: agentName,
                    role: agentRole,
                    runtime_id: agentRuntimeId,
                    model: agentModel,
                    instructions: agentInstructions,
                  }
                  try {
                    if (selectedConfigAgentId) {
                      await onUpdateAgent(selectedConfigAgentId, input)
                    } else {
                      await onCreateAgent(input)
                    }
                  } catch {
                    // Parent renders the recoverable mutation error.
                  }
                }}
                className={buttonStyle}
              >
                {selectedConfigAgentId ? 'Save Agent' : 'Create Agent'}
              </button>
            </div>
          )}
        </section>

        <section className="mb-8">
          <div className="mb-3 flex items-center justify-between gap-3">
            <h3 className="text-sm font-medium text-zinc-300">Tags</h3>
            {(projectTags.status === 'loading' || issueTags.status === 'loading') && (
              <span className="text-xs text-zinc-500">Loading tags...</span>
            )}
          </div>
          {(projectTags.status === 'error' || issueTags.status === 'error') ? (
            <SectionError
              title="Tags failed to load"
              error={projectTags.error ?? issueTags.error}
              onRetry={onRetryTags}
            />
          ) : (
            <div className="space-y-3">
              <div className="flex flex-wrap gap-2">
                {issueTags.data.length === 0 && <p className="text-sm text-zinc-500">No tags attached</p>}
                {issueTags.data.map((tag) => (
                  <span
                    key={tag.id}
                    className="inline-flex items-center gap-2 rounded border border-zinc-800 bg-zinc-900 px-2.5 py-1 text-sm text-zinc-300"
                  >
                    <span className="h-2 w-2 rounded-full" style={{ backgroundColor: tag.color || '#71717a' }} />
                    {tag.name}
                    <button
                      type="button"
                      disabled={mutationPending}
                      onClick={() => {
                        void onDetachTag(tag.id).catch(() => undefined)
                      }}
                      className="text-zinc-500 hover:text-red-200"
                      aria-label={`Detach ${tag.name}`}
                    >
                      x
                    </button>
                  </span>
                ))}
              </div>
              <div className="flex gap-2">
                <input
                  value={newTagName}
                  onChange={(event) => setNewTagName(event.target.value)}
                  placeholder="New tag"
                  className={inputStyle}
                />
                <input
                  type="color"
                  value={newTagColor}
                  onChange={(event) => setNewTagColor(event.target.value)}
                  aria-label="Tag color"
                  className="h-10 w-14 rounded border border-zinc-800 bg-zinc-950 p-1"
                />
                <button
                  type="button"
                  disabled={mutationPending || newTagName.trim() === ''}
                  onClick={async () => {
                    try {
                      await onCreateTag(newTagName, newTagColor)
                      setNewTagName('')
                    } catch {
                      // Parent renders the recoverable mutation error.
                    }
                  }}
                  className={buttonStyle}
                >
                  Create
                </button>
              </div>
              <div className="flex gap-2">
                <select
                  value={selectedTagId}
                  onChange={(event) => setSelectedTagId(event.target.value)}
                  disabled={availableTags.length === 0}
                  className={inputStyle}
                >
                  {availableTags.length === 0 ? (
                    <option value="">No available tags</option>
                  ) : (
                    availableTags.map((tag) => (
                      <option key={tag.id} value={tag.id}>
                        {tag.name}
                      </option>
                    ))
                  )}
                </select>
                <button
                  type="button"
                  disabled={mutationPending || selectedTagId === ''}
                  onClick={() => {
                    void onAttachTag(selectedTagId).catch(() => undefined)
                  }}
                  className={buttonStyle}
                >
                  Attach
                </button>
              </div>
            </div>
          )}
        </section>

        <section>
          <div className="mb-3 flex items-center justify-between gap-3">
            <h3 className="text-sm font-medium text-zinc-300">Comments</h3>
            {comments.status === 'loading' && <span className="text-xs text-zinc-500">Loading comments...</span>}
          </div>
          {comments.status === 'error' ? (
            <SectionError title="Comments failed to load" error={comments.error} onRetry={onRetryComments} />
          ) : (
            <div className="space-y-3">
              {comments.data.length === 0 && <p className="text-sm text-zinc-500">No comments yet</p>}
              {comments.data.map((comment) => (
                <CommentItem
                  key={comment.id}
                  comment={comment}
                  disabled={mutationPending}
                  assignableAgents={assignableAgents}
                  onUpdate={onUpdateComment}
                  onDelete={onDeleteComment}
                  onAssign={(agentId, commentId) => onCreateAssignment(agentId, { type: 'comment', id: commentId })}
                />
              ))}
              <div className="rounded border border-zinc-800 bg-zinc-900/40 p-3">
                <textarea
                  value={commentBody}
                  onChange={(event) => setCommentBody(event.target.value)}
                  rows={3}
                  placeholder="Add a comment"
                  className={inputStyle}
                />
                <button
                  type="button"
                  disabled={mutationPending || commentBody.trim() === ''}
                  onClick={async () => {
                    try {
                      await onCreateComment(commentBody)
                      setCommentBody('')
                    } catch {
                      // Parent renders the recoverable mutation error.
                    }
                  }}
                  className={`mt-2 ${buttonStyle}`}
                >
                  Add Comment
                </button>
              </div>
            </div>
          )}
        </section>
      </div>
    </main>
  )
}
