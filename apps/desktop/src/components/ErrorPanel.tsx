import { useDaemon } from '../daemon/useDaemon'

export default function ErrorPanel() {
  const { error, connected, startDaemon } = useDaemon()

  if (connected && !error) return null

  return (
    <div className="fixed inset-0 bg-zinc-950/90 flex items-center justify-center z-50">
      <div className="bg-zinc-900 border border-red-900/50 rounded-lg p-8 max-w-md w-full mx-4">
        <div className="flex items-center gap-3 mb-4">
          <div className="w-8 h-8 rounded-full bg-red-500/20 flex items-center justify-center">
            <svg className="w-4 h-4 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z" />
            </svg>
          </div>
          <h2 className="text-lg font-semibold text-red-400">Daemon Connection Error</h2>
        </div>

        <p className="text-sm text-zinc-400 mb-2">The Zin daemon could not be started or is not responding.</p>

        {error && (
          <div className="bg-zinc-950 border border-zinc-800 rounded p-3 mb-4">
            <p className="text-xs font-mono text-red-300 break-all">{error}</p>
          </div>
        )}

        <div className="text-xs text-zinc-500 mb-4">
          Check logs at <code className="text-zinc-400">~/.zin/daemon.log</code>
        </div>

        <div className="flex gap-3">
          <button
            onClick={() => startDaemon().catch(() => {})}
            className="flex-1 px-4 py-2 bg-zinc-800 hover:bg-zinc-700 text-zinc-200 text-sm rounded transition-colors"
          >
            Retry
          </button>
          <button
            onClick={() => window.close()}
            className="px-4 py-2 bg-red-900/30 hover:bg-red-900/50 text-red-300 text-sm rounded transition-colors"
          >
            Quit
          </button>
        </div>
      </div>
    </div>
  )
}
