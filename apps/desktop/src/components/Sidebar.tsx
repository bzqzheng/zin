import type { ResourceState, LoadStatus } from '../App'
import type { Issue, Project } from '../daemon'

interface SidebarProps {
  activeView: 'issues' | 'settings'
  daemonStatus: LoadStatus
  daemonError: string | null
  projects: ResourceState<Project[]>
  selectedProjectId: string | null
  issues: ResourceState<Issue[]>
  selectedIssueId: string | null
  onRetryDaemon: () => void
  onRetryProjects: () => void
  onRetryIssues: () => void
  onOpenIssues: () => void
  onOpenSettings: () => void
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

function displayLabel(value: string) {
  return value
    .split('_')
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
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
  activeView,
  daemonStatus,
  daemonError,
  projects,
  selectedProjectId,
  issues,
  selectedIssueId,
  onRetryDaemon,
  onRetryProjects,
  onRetryIssues,
  onOpenIssues,
  onOpenSettings,
  onSelectProject,
  onSelectIssue,
}: SidebarProps) {
  const selectedProject = projects.data.find((project) => project.id === selectedProjectId) ?? null

  return (
    <aside className="w-72 shrink-0 bg-zinc-900 border-r border-zinc-800 flex flex-col overflow-hidden">
      <div className="px-4 py-4 border-b border-zinc-800">
        <h1 className="text-base font-semibold text-zinc-100">Zin</h1>
        <p className="text-xs text-zinc-500 mt-0.5">Workspace</p>
      </div>

      <div className="flex-1 overflow-y-auto px-3 py-3">
        <button
          type="button"
          aria-label="Issues"
          onClick={onOpenIssues}
          className={`mb-4 w-full rounded border px-3 py-2 text-left text-sm transition-colors ${
            activeView === 'issues'
              ? 'border-zinc-700 bg-zinc-800 text-zinc-100'
              : 'border-zinc-800 bg-zinc-950 text-zinc-400 hover:border-zinc-700 hover:text-zinc-200'
          }`}
        >
          <span className="block font-medium">Issues</span>
          <span className="mt-0.5 block text-xs text-zinc-500">
            {selectedProject ? selectedProject.name : 'Select a project'}
          </span>
        </button>

        <div className="py-1">
          <h2 className="px-2 mb-1 text-xs font-semibold text-zinc-500 uppercase tracking-wider">
            Projects
          </h2>
          {daemonStatus === 'loading' && (
            <p className="px-2 py-2 text-sm text-zinc-500">Connecting to daemon...</p>
          )}
          {daemonStatus === 'error' && (
            <div className="px-2 py-2">
              <p className="text-sm text-red-300">Daemon unavailable</p>
              {daemonError && <p className="mt-1 text-xs text-zinc-500">{daemonError}</p>}
              <RetryButton onRetry={onRetryDaemon} />
            </div>
          )}
          {daemonStatus !== 'error' && projects.status === 'loading' && (
            <p className="px-2 py-2 text-sm text-zinc-500">Loading projects...</p>
          )}
          {projects.status === 'error' && (
            <div className="px-2 py-2">
              <p className="text-sm text-red-300">Projects failed to load</p>
              {projects.error && <p className="mt-1 text-xs text-zinc-500">{projects.error}</p>}
              <RetryButton onRetry={onRetryProjects} />
            </div>
          )}
          {projects.status === 'success' && projects.data.length === 0 && (
            <p className="px-2 py-2 text-sm text-zinc-500">No projects yet</p>
          )}
          {projects.status === 'success' &&
            projects.data.map((project) => (
              <button
                key={project.id}
                type="button"
                onClick={() => {
                  onOpenIssues()
                  onSelectProject(project.id)
                }}
                className={`w-full text-left px-2 py-1.5 text-sm rounded transition-colors ${
                  selectedProjectId === project.id
                    ? 'bg-zinc-800 text-zinc-100'
                    : 'text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200'
                }`}
              >
                <span className="truncate">{project.name}</span>
              </button>
            ))}
        </div>

        <div className="py-3">
          <div className="px-2 mb-1 flex items-center justify-between gap-2">
            <h2 className="text-xs font-semibold text-zinc-500 uppercase tracking-wider">
              Work Items
            </h2>
            {issues.status === 'success' && (
              <span className="text-xs text-zinc-600">{issues.data.length}</span>
            )}
          </div>
          {selectedProject && (
            <p className="px-2 pb-1 text-xs text-zinc-600 truncate">
              {selectedProject.name}
            </p>
          )}
          {activeView === 'settings' && (
            <p className="px-2 py-2 text-sm text-zinc-500">Workspace settings open</p>
          )}
          {activeView === 'issues' && projects.status === 'success' && projects.data.length === 0 && (
            <p className="px-2 py-2 text-sm text-zinc-500">Create a project to track work</p>
          )}
          {activeView === 'issues' &&
            projects.status === 'success' &&
            projects.data.length > 0 &&
            issues.status === 'loading' && (
              <p className="px-2 py-2 text-sm text-zinc-500">Loading work items...</p>
            )}
          {activeView === 'issues' && issues.status === 'error' && (
            <div className="px-2 py-2">
              <p className="text-sm text-red-300">Work items failed to load</p>
              {issues.error && <p className="mt-1 text-xs text-zinc-500">{issues.error}</p>}
              <RetryButton onRetry={onRetryIssues} />
            </div>
          )}
          {activeView === 'issues' && issues.status === 'success' && selectedProjectId && issues.data.length === 0 && (
            <p className="px-2 py-2 text-sm text-zinc-500">No work items in this project</p>
          )}
          {activeView === 'issues' && issues.status === 'success' && issues.data.length > 0 && (
            <div className="space-y-1">
              {issues.data.map((issue) => (
                <button
                  key={issue.id}
                  type="button"
                  onClick={() => {
                    onOpenIssues()
                    onSelectIssue(issue.id)
                  }}
                  className={`w-full rounded px-2 py-2 text-left transition-colors ${
                    selectedIssueId === issue.id
                      ? 'bg-zinc-800 text-zinc-100'
                      : 'text-zinc-300 hover:bg-zinc-800/80'
                  }`}
                >
                  <span className="mb-1 flex min-w-0 items-center gap-2 text-xs">
                    <span className={`text-[10px] font-semibold ${priorityColor(issue.priority)}`}>
                      ●
                    </span>
                    <span className="font-mono text-zinc-500">{issue.identifier}</span>
                    <span className="text-zinc-600">{displayLabel(issue.status)}</span>
                  </span>
                  <span className="block truncate text-sm">{issue.title}</span>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      <div className="space-y-3 px-4 py-3 border-t border-zinc-800 bg-zinc-900">
        <button
          type="button"
          aria-label="Settings"
          onClick={onOpenSettings}
          className={`w-full rounded border px-3 py-2 text-left text-xs transition-colors ${
            activeView === 'settings'
              ? 'border-zinc-700 bg-zinc-800 text-zinc-100'
              : 'border-zinc-800 bg-zinc-950 text-zinc-500 hover:border-zinc-700 hover:text-zinc-300'
          }`}
        >
          <span className="block font-medium">Settings</span>
          <span className="mt-0.5 block text-zinc-600">Runtimes and paths</span>
        </button>
        <div className="text-xs text-zinc-500">
          <span className="text-zinc-400 font-medium">Daemon</span> ·{' '}
          {daemonStatus === 'success' ? 'online' : 'offline'}
        </div>
      </div>
    </aside>
  )
}
