export default function IssueDetail() {
  return (
    <main className="flex-1 bg-zinc-950 overflow-y-auto">
      <div className="max-w-3xl mx-auto px-8 py-6">
        <div className="mb-6">
          <div className="flex items-center gap-3 mb-2">
            <span className="text-xs font-mono text-zinc-500 bg-zinc-900 px-2 py-0.5 rounded">
              ZIN-8
            </span>
            <span className="text-xs font-medium text-amber-500 bg-amber-500/10 px-2 py-0.5 rounded">
              High
            </span>
            <span className="text-xs text-zinc-500 bg-zinc-900 px-2 py-0.5 rounded">
              In Progress
            </span>
          </div>
          <h2 className="text-xl font-semibold text-zinc-100">
            Tauri Frontend Scaffold
          </h2>
          <p className="text-sm text-zinc-500 mt-1">
            Created by Trinity · May 2, 2026
          </p>
        </div>

        <section className="mb-8">
          <h3 className="text-sm font-medium text-zinc-300 mb-3">Description</h3>
          <div className="text-sm text-zinc-400 leading-relaxed space-y-2">
            <p>
              Tauri v2 desktop app with React + TypeScript + Tailwind CSS,
              three-panel layout. This is the frontend foundation for the Zin
              Agent workspace.
            </p>
            <ul className="list-disc list-inside space-y-1 text-zinc-500">
              <li>Tauri v2 project initialized at apps/desktop/</li>
              <li>React 19 + TypeScript 5.x setup</li>
              <li>Tailwind CSS configured (utility-first)</li>
              <li>Three-panel layout: Sidebar, Issue Detail, Timeline</li>
              <li>Static layout only — no data fetching in Phase 1</li>
            </ul>
          </div>
        </section>

        <section>
          <h3 className="text-sm font-medium text-zinc-300 mb-3">Activity</h3>
          <div className="space-y-3">
            <div className="flex gap-3">
              <div className="w-6 h-6 rounded-full bg-zinc-800 flex items-center justify-center text-[10px] text-zinc-400 shrink-0">
                T
              </div>
              <div>
                <p className="text-sm">
                  <span className="text-zinc-300 font-medium">Trinity</span>{' '}
                  <span className="text-zinc-500">created this issue</span>
                </p>
                <p className="text-xs text-zinc-600">2 hours ago</p>
              </div>
            </div>
            <div className="flex gap-3">
              <div className="w-6 h-6 rounded-full bg-zinc-800 flex items-center justify-center text-[10px] text-zinc-400 shrink-0">
                M
              </div>
              <div>
                <p className="text-sm">
                  <span className="text-zinc-300 font-medium">Morpheus</span>{' '}
                  <span className="text-zinc-500">assigned to Trinity</span>
                </p>
                <p className="text-xs text-zinc-600">1 hour ago</p>
              </div>
            </div>
          </div>
        </section>
      </div>
    </main>
  )
}
