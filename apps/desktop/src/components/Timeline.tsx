import type { ResourceState } from '../App'
import type { IssueActivity } from '../daemon'

interface TimelineProps {
  activity: ResourceState<IssueActivity[]>
  issueSelected: boolean
  onRetry: () => void
}

function formatDateTime(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  }).format(new Date(value))
}

function typeDot(type: string) {
  if (type.startsWith('issue.')) return 'bg-amber-500'
  if (type.startsWith('tag.')) return 'bg-blue-500'
  if (type.startsWith('comment.')) return 'bg-emerald-500'
  return 'bg-zinc-600'
}

export default function Timeline({ activity, issueSelected, onRetry }: TimelineProps) {
  return (
    <aside className="w-72 bg-zinc-900 border-l border-zinc-800 flex flex-col overflow-hidden">
      <div className="px-4 py-3 border-b border-zinc-800">
        <h2 className="text-sm font-semibold text-zinc-100">Timeline</h2>
      </div>

      <div className="flex-1 overflow-y-auto px-3 py-2">
        {!issueSelected && activity.status !== 'loading' && (
          <p className="px-2 py-2 text-sm text-zinc-500">No timeline events</p>
        )}
        {activity.status === 'loading' && (
          <p className="px-2 py-2 text-sm text-zinc-500">Loading timeline...</p>
        )}
        {activity.status === 'error' && (
          <div className="px-2 py-2">
            <p className="text-sm text-red-300">Timeline failed to load</p>
            {activity.error && <p className="mt-1 text-xs text-zinc-500">{activity.error}</p>}
            <button
              type="button"
              onClick={onRetry}
              className="mt-2 text-xs font-medium text-zinc-200 hover:text-white"
            >
              Retry
            </button>
          </div>
        )}
        {issueSelected && activity.status === 'success' && activity.data.length === 0 && (
          <p className="px-2 py-2 text-sm text-zinc-500">No timeline events</p>
        )}
        {activity.status === 'success' && activity.data.length > 0 && (
          <div className="space-y-1">
            {activity.data.map((event) => (
              <div
                key={event.id}
                className="flex gap-3 px-2 py-2 rounded hover:bg-zinc-800/50 transition-colors"
              >
                <div className="relative mt-0.5">
                  <div className={`w-2 h-2 rounded-full ${typeDot(event.type)}`} />
                </div>
                <div className="min-w-0">
                  <p className="truncate text-sm text-zinc-300">{event.summary}</p>
                  <p className="text-xs text-zinc-500">{formatDateTime(event.created_at)}</p>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="px-4 py-3 border-t border-zinc-800">
        <p className="text-xs text-zinc-600">{activity.data.length} events</p>
      </div>
    </aside>
  )
}
