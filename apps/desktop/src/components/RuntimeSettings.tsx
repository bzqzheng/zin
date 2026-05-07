import { useMemo, useState } from 'react'
import type { ResourceState } from '../App'
import type { RuntimeDiscoverySummary, RuntimeKind, RuntimeRecord } from '../daemon'

interface RuntimeSettingsProps {
  runtimes: ResourceState<RuntimeRecord[]>
  discoverySummary: RuntimeDiscoverySummary | null
  mutationPending: boolean
  mutationError: string | null
  onRetry: () => void
  onDiscover: () => Promise<void>
  onUpdateRuntime: (runtimeId: string, input: { display_name?: string; binary_path?: string }) => Promise<void>
  onValidateRuntime: (runtimeId: string) => Promise<void>
}

const runtimeOrder: RuntimeKind[] = ['codex', 'claude', 'gemini', 'opencode']

const kindLabels: Record<RuntimeKind, string> = {
  codex: 'Codex CLI',
  claude: 'Claude CLI',
  gemini: 'Gemini CLI',
  opencode: 'OpenCode CLI',
}
const supportedRuntimeLabels = runtimeOrder.map((kind) => kindLabels[kind])

const inputStyle =
  'w-full rounded border border-zinc-800 bg-zinc-950 px-2.5 py-2 text-sm text-zinc-100 outline-none focus:border-zinc-600'
const buttonStyle =
  'rounded border border-zinc-700 px-3 py-1.5 text-sm font-medium text-zinc-200 hover:border-zinc-500 hover:text-white disabled:cursor-not-allowed disabled:opacity-50'

function displayStatus(status: string) {
  return status.charAt(0).toUpperCase() + status.slice(1)
}

function statusClass(status: string) {
  switch (status) {
    case 'healthy':
      return 'border-emerald-900/60 bg-emerald-950/20 text-emerald-300'
    case 'degraded':
      return 'border-amber-900/60 bg-amber-950/20 text-amber-300'
    default:
      return 'border-zinc-800 bg-zinc-900 text-zinc-400'
  }
}

function formatDateTime(value: string) {
  if (!value) return 'Never'
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  }).format(new Date(value))
}

function RuntimeCard({
  runtime,
  disabled,
  onUpdateRuntime,
  onValidateRuntime,
}: {
  runtime: RuntimeRecord
  disabled: boolean
  onUpdateRuntime: RuntimeSettingsProps['onUpdateRuntime']
  onValidateRuntime: RuntimeSettingsProps['onValidateRuntime']
}) {
  const [draft, setDraft] = useState({
    sourceDisplayName: runtime.display_name,
    sourceBinaryPath: runtime.binary_path,
    displayName: runtime.display_name,
    binaryPath: runtime.binary_path,
  })

  if (draft.sourceDisplayName !== runtime.display_name || draft.sourceBinaryPath !== runtime.binary_path) {
    setDraft({
      sourceDisplayName: runtime.display_name,
      sourceBinaryPath: runtime.binary_path,
      displayName: runtime.display_name,
      binaryPath: runtime.binary_path,
    })
  }

  const displayName = draft.displayName
  const binaryPath = draft.binaryPath
  const hasDraftChanges = displayName !== runtime.display_name || binaryPath !== runtime.binary_path

  return (
    <section className="rounded border border-zinc-800 bg-zinc-900/40 p-4">
      <div className="mb-3 flex items-start justify-between gap-4">
        <div>
          <h3 className="text-sm font-medium text-zinc-100">{displayName || kindLabels[runtime.kind]}</h3>
          <p className="mt-1 font-mono text-xs text-zinc-500">{runtime.kind}</p>
        </div>
        <span className={`rounded border px-2 py-1 text-xs font-medium ${statusClass(runtime.health_status)}`}>
          {displayStatus(runtime.health_status)}
        </span>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <label className="block">
          <span className="mb-1 block text-xs font-medium text-zinc-500">Display name</span>
          <input
            value={displayName}
            onChange={(event) => setDraft((current) => ({ ...current, displayName: event.target.value }))}
            className={inputStyle}
          />
        </label>
        <label className="block">
          <span className="mb-1 block text-xs font-medium text-zinc-500">Binary path</span>
          <input
            value={binaryPath}
            onChange={(event) => setDraft((current) => ({ ...current, binaryPath: event.target.value }))}
            placeholder="Missing from PATH"
            className={inputStyle}
          />
        </label>
      </div>

      <dl className="mt-3 grid grid-cols-3 gap-3 text-xs">
        <div>
          <dt className="text-zinc-500">Version</dt>
          <dd className="mt-1 truncate text-zinc-300">{runtime.version_raw || 'Unknown'}</dd>
        </div>
        <div>
          <dt className="text-zinc-500">Last checked</dt>
          <dd className="mt-1 text-zinc-300">{formatDateTime(runtime.last_checked_at)}</dd>
        </div>
        <div>
          <dt className="text-zinc-500">Diagnostics</dt>
          <dd className="mt-1 truncate text-zinc-300">{runtime.health_reason || 'Probe passed'}</dd>
        </div>
      </dl>

      <div className="mt-4 flex gap-2">
        <button
          type="button"
          disabled={disabled || !hasDraftChanges || displayName.trim() === ''}
          onClick={() => onUpdateRuntime(runtime.id, { display_name: displayName, binary_path: binaryPath })}
          className={buttonStyle}
        >
          Save Path
        </button>
        <button
          type="button"
          disabled={disabled}
          onClick={() => onValidateRuntime(runtime.id)}
          className={buttonStyle}
        >
          Revalidate
        </button>
      </div>
    </section>
  )
}

export default function RuntimeSettings({
  runtimes,
  discoverySummary,
  mutationPending,
  mutationError,
  onRetry,
  onDiscover,
  onUpdateRuntime,
  onValidateRuntime,
}: RuntimeSettingsProps) {
  const sortedRuntimes = useMemo(
    () => [...runtimes.data].sort((left, right) => runtimeOrder.indexOf(left.kind) - runtimeOrder.indexOf(right.kind)),
    [runtimes.data],
  )

  return (
    <main className="flex-1 overflow-y-auto bg-zinc-950">
      <div className="mx-auto max-w-5xl px-8 py-6">
        <div className="mb-6 flex flex-wrap items-start justify-between gap-4">
          <div>
            <h2 className="text-xl font-semibold text-zinc-100">Agent Runtimes</h2>
            <p className="mt-1 text-sm text-zinc-500">Local CLI health and path overrides</p>
          </div>
          <button type="button" disabled={mutationPending} onClick={() => void onDiscover()} className={buttonStyle}>
            Discover
          </button>
        </div>

        {discoverySummary && (
          <div className="mb-5 flex flex-wrap gap-2 text-xs text-zinc-400">
            <span className="rounded border border-emerald-900/60 bg-emerald-950/20 px-2 py-1">
              Healthy {discoverySummary.healthy}
            </span>
            <span className="rounded border border-amber-900/60 bg-amber-950/20 px-2 py-1">
              Degraded {discoverySummary.degraded}
            </span>
            <span className="rounded border border-zinc-800 bg-zinc-900 px-2 py-1">
              Missing {discoverySummary.missing}
            </span>
          </div>
        )}

        {mutationError && (
          <div className="mb-5 rounded border border-red-900/50 bg-red-950/20 px-3 py-2">
            <p className="text-sm text-red-300">Runtime update failed</p>
            <p className="mt-1 text-sm text-zinc-500">{mutationError}</p>
          </div>
        )}

        {runtimes.status === 'idle' || runtimes.status === 'loading' ? (
          <p className="text-sm text-zinc-500">Loading runtimes...</p>
        ) : null}

        {runtimes.status === 'error' && (
          <div className="rounded border border-red-900/50 bg-red-950/20 px-3 py-2">
            <p className="text-sm text-red-300">Runtimes failed to load</p>
            {runtimes.error && <p className="mt-1 text-sm text-zinc-500">{runtimes.error}</p>}
            <button type="button" onClick={onRetry} className="mt-2 text-sm font-medium text-zinc-200 hover:text-white">
              Retry
            </button>
          </div>
        )}

        {runtimes.status === 'success' && sortedRuntimes.length === 0 && (
          <div className="rounded border border-zinc-800 bg-zinc-900/40 px-4 py-5">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <p className="text-sm font-medium text-zinc-200">No runtimes discovered</p>
                <p className="mt-1 text-sm text-zinc-500">
                  Supported CLIs: {supportedRuntimeLabels.join(', ')}.
                </p>
              </div>
              <button
                type="button"
                disabled={mutationPending}
                onClick={() => void onDiscover()}
                className={buttonStyle}
              >
                Find Runtimes
              </button>
            </div>
            <div className="mt-4 border-t border-zinc-800 pt-3">
              <p className="text-xs text-zinc-500">
                Install a supported CLI, then discover again or enter its binary path after discovery creates the runtime row.
              </p>
              <a
                href="https://github.com/bzqzheng/zin/blob/main/docs/architecture.md#agent-runtime-adapters"
                className="mt-2 inline-block text-xs font-medium text-zinc-200 hover:text-white"
              >
                Runtime setup docs
              </a>
            </div>
          </div>
        )}

        {runtimes.status === 'success' && sortedRuntimes.length > 0 && (
          <div className="grid gap-3">
            {sortedRuntimes.map((runtime) => (
              <RuntimeCard
                key={runtime.id}
                runtime={runtime}
                disabled={mutationPending}
                onUpdateRuntime={onUpdateRuntime}
                onValidateRuntime={onValidateRuntime}
              />
            ))}
          </div>
        )}
      </div>
    </main>
  )
}
