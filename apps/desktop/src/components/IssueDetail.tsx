import type { ResourceState } from '../App'
import type { Issue } from '../daemon'

interface IssueDetailProps {
  issue: ResourceState<Issue | null>
  onRetry: () => void
}

const labelStyle = 'text-xs bg-zinc-900 px-2 py-0.5 rounded'

function formatDate(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
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

export default function IssueDetail({ issue, onRetry }: IssueDetailProps) {
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
          <p className="text-sm text-red-300">Issue failed to load</p>
          {issue.error && <p className="mt-1 text-sm text-zinc-500">{issue.error}</p>}
          <button
            type="button"
            onClick={onRetry}
            className="mt-3 text-sm font-medium text-zinc-200 hover:text-white"
          >
            Retry
          </button>
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
          <div className="flex items-center gap-3 mb-2">
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
          <p className="text-sm text-zinc-500 mt-1">
            Created {formatDate(issue.data.created_at)}
          </p>
        </div>

        <section className="mb-8">
          <h3 className="text-sm font-medium text-zinc-300 mb-3">Description</h3>
          {issue.data.description ? (
            <p className="text-sm text-zinc-400 leading-relaxed whitespace-pre-wrap">
              {issue.data.description}
            </p>
          ) : (
            <p className="text-sm text-zinc-500">No description</p>
          )}
        </section>
      </div>
    </main>
  )
}
