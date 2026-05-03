import type { ResourceState, LoadStatus } from '../App'
import type { Issue, Project } from '../daemon'

interface SidebarProps {
  daemonStatus: LoadStatus
  daemonError: string | null
  projects: ResourceState<Project[]>
  selectedProjectId: string | null
  issues: ResourceState<Issue[]>
  selectedIssueId: string | null
  onRetryDaemon: () => void
  onRetryProjects: () => void
  onRetryIssues: () => void
  onSelectProject: (projectId: string) => void
  onSelectIssue: (issueId: string) => void
}

const priorityColor = (priority: string) => {
  switch (priority) {
    case 'high':
      return 'text-amber-500'
    case 'low':
      return 'text-zinc-500'
    default:
      return 'text-blue-400'
  }
}

function RetryButton({ onRetry }: { onRetry: () => void }) {
  return (
    <button
      type="button"
      onClick={onRetry}
      className="mt-2 text-xs font-medium text-zinc-200 hover:text-white"
    >
      Retry
    </button>
  )
}

export default function Sidebar({
  daemonStatus,
  daemonError,
  projects,
  selectedProjectId,
  issues,
  selectedIssueId,
  onRetryDaemon,
  onRetryProjects,
  onRetryIssues,
  onSelectProject,
  onSelectIssue,
}: SidebarProps) {
  return (
    <aside className="w-60 bg-zinc-900 border-r border-zinc-800 flex flex-col overflow-hidden">
      <div className="px-4 py-3 border-b border-zinc-800">
        <h1 className="text-sm font-semibold text-zinc-100 tracking-wide">Zin</h1>
        <p className="text-xs text-zinc-500 mt-0.5">Agent Workspace</p>
      </div>

      <div className="flex-1 overflow-y-auto">
        <div className="px-3 py-2">
          <h2 className="text-xs font-medium text-zinc-500 uppercase tracking-wider px-2 mb-1">
            Projects
          </h2>
          {daemonStatus === 'loading' && (
            <p className="px-2 py-1.5 text-sm text-zinc-500">Connecting to daemon...</p>
          )}
          {daemonStatus === 'error' && (
            <div className="px-2 py-1.5">
              <p className="text-sm text-red-300">Daemon unavailable</p>
              {daemonError && <p className="mt-1 text-xs text-zinc-500">{daemonError}</p>}
              <RetryButton onRetry={onRetryDaemon} />
            </div>
          )}
          {daemonStatus !== 'error' && projects.status === 'loading' && (
            <p className="px-2 py-1.5 text-sm text-zinc-500">Loading projects...</p>
          )}
          {projects.status === 'error' && (
            <div className="px-2 py-1.5">
              <p className="text-sm text-red-300">Projects failed to load</p>
              {projects.error && <p className="mt-1 text-xs text-zinc-500">{projects.error}</p>}
              <RetryButton onRetry={onRetryProjects} />
            </div>
          )}
          {projects.status === 'success' && projects.data.length === 0 && (
            <p className="px-2 py-1.5 text-sm text-zinc-500">No projects yet</p>
          )}
          {projects.status === 'success' &&
            projects.data.map((project) => (
              <button
                key={project.id}
                type="button"
                onClick={() => onSelectProject(project.id)}
                className={`w-full text-left px-2 py-1.5 text-sm rounded transition-colors ${
                  selectedProjectId === project.id
                    ? 'bg-zinc-800 text-zinc-100'
                    : 'text-zinc-300 hover:bg-zinc-800'
                }`}
              >
                <span className="truncate">{project.name}</span>
              </button>
            ))}
        </div>

        <div className="px-3 py-2">
          <h2 className="text-xs font-medium text-zinc-500 uppercase tracking-wider px-2 mb-1">
            Issues
          </h2>
          {projects.status === 'success' && projects.data.length === 0 && (
            <p className="px-2 py-1.5 text-sm text-zinc-500">Create a project to track issues</p>
          )}
          {projects.status === 'success' && projects.data.length > 0 && issues.status === 'loading' && (
            <p className="px-2 py-1.5 text-sm text-zinc-500">Loading issues...</p>
          )}
          {issues.status === 'error' && (
            <div className="px-2 py-1.5">
              <p className="text-sm text-red-300">Issues failed to load</p>
              {issues.error && <p className="mt-1 text-xs text-zinc-500">{issues.error}</p>}
              <RetryButton onRetry={onRetryIssues} />
            </div>
          )}
          {issues.status === 'success' && selectedProjectId && issues.data.length === 0 && (
            <p className="px-2 py-1.5 text-sm text-zinc-500">No issues in this project</p>
          )}
          {issues.status === 'success' &&
            issues.data.map((issue) => (
              <button
                key={issue.id}
                type="button"
                onClick={() => onSelectIssue(issue.id)}
                className={`w-full text-left px-2 py-1.5 text-sm rounded transition-colors flex items-center gap-2 ${
                  selectedIssueId === issue.id
                    ? 'bg-zinc-800 text-zinc-100'
                    : 'text-zinc-300 hover:bg-zinc-800'
                }`}
              >
                <span className={`text-[10px] font-semibold ${priorityColor(issue.priority)}`}>
                  ●
                </span>
                <span className="text-zinc-500 text-xs">{issue.identifier}</span>
                <span className="truncate">{issue.title}</span>
              </button>
            ))}
        </div>
      </div>

      <div className="px-4 py-3 border-t border-zinc-800">
        <div className="text-xs text-zinc-500">
          <span className="text-zinc-400 font-medium">Daemon</span> ·{' '}
          {daemonStatus === 'success' ? 'online' : 'offline'}
        </div>
      </div>
    </aside>
  )
}
