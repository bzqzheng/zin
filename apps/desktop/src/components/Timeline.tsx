import type { ResourceState } from '../App'
import type { Issue } from '../daemon'

interface TimelineProps {
  issue: ResourceState<Issue | null>
}

function formatDateTime(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  }).format(new Date(value))
}

export default function Timeline({ issue }: TimelineProps) {
  const events = issue.data
    ? [
        {
          id: 'created',
          action: 'Issue created',
          time: formatDateTime(issue.data.created_at),
          dot: 'bg-emerald-500',
        },
        {
          id: 'updated',
          action: `Status: ${issue.data.status}`,
          time: formatDateTime(issue.data.updated_at),
          dot: 'bg-amber-500',
        },
      ]
    : []

  return (
    <aside className="w-72 bg-zinc-900 border-l border-zinc-800 flex flex-col overflow-hidden">
      <div className="px-4 py-3 border-b border-zinc-800">
        <h2 className="text-sm font-semibold text-zinc-100">Timeline</h2>
      </div>

      <div className="flex-1 overflow-y-auto px-3 py-2">
        {issue.status === 'loading' && (
          <p className="px-2 py-2 text-sm text-zinc-500">Loading timeline...</p>
        )}
        {issue.status !== 'loading' && events.length === 0 && (
          <p className="px-2 py-2 text-sm text-zinc-500">No timeline events</p>
        )}
        {events.length > 0 && (
          <div className="space-y-1">
            {events.map((event) => (
              <div
                key={event.id}
                className="flex gap-3 px-2 py-2 rounded hover:bg-zinc-800/50 transition-colors"
              >
                <div className="relative mt-0.5">
                  <div className={`w-2 h-2 rounded-full ${event.dot}`} />
                </div>
                <div className="min-w-0">
                  <p className="text-sm text-zinc-300 truncate">{event.action}</p>
                  <p className="text-xs text-zinc-500">{event.time}</p>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="px-4 py-3 border-t border-zinc-800">
        <p className="text-xs text-zinc-600">{events.length} events</p>
      </div>
    </aside>
  )
}
