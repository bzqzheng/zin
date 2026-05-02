export default function Sidebar() {
  const projects = [
    { id: '1', name: 'Zin Agent', issueCount: 5 },
    { id: '2', name: 'Design System', issueCount: 3 },
    { id: '3', name: 'Documentation', issueCount: 2 },
  ]

  const issues = [
    { id: 'ISSUE-1', title: 'Add real-time collaboration to docs', priority: 'high' },
    { id: 'ISSUE-2', title: 'Improve agent memory retrieval', priority: 'high' },
    { id: 'ISSUE-3', title: 'Design channel sidebar', priority: 'high' },
    { id: 'ISSUE-4', title: 'Add dark mode toggle', priority: 'medium' },
    { id: 'ISSUE-5', title: 'Write onboarding guide', priority: 'medium' },
    { id: 'ISSUE-6', title: 'Optimize daemon startup time', priority: 'medium' },
    { id: 'ISSUE-7', title: 'Add keyboard shortcuts', priority: 'low' },
  ]

  const priorityColor = (p: string) =>
    p === 'high' ? 'text-amber-500' : 'text-slate-400'

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
          {projects.map((p) => (
            <button
              key={p.id}
              className="w-full text-left px-2 py-1.5 text-sm text-zinc-300 hover:bg-zinc-800 rounded transition-colors"
            >
              <span>{p.name}</span>
              <span className="text-xs text-zinc-500 ml-2">{p.issueCount}</span>
            </button>
          ))}
        </div>

        <div className="px-3 py-2">
          <h2 className="text-xs font-medium text-zinc-500 uppercase tracking-wider px-2 mb-1">
            Issues
          </h2>
          {issues.map((issue) => (
            <button
              key={issue.id}
              className="w-full text-left px-2 py-1.5 text-sm text-zinc-300 hover:bg-zinc-800 rounded transition-colors flex items-center gap-2"
            >
              <span className={`text-[10px] font-semibold ${priorityColor(issue.priority)}`}>
                ●
              </span>
              <span className="text-zinc-500 text-xs">{issue.id}</span>
              <span className="truncate">{issue.title}</span>
            </button>
          ))}
        </div>
      </div>

      <div className="px-4 py-3 border-t border-zinc-800">
        <div className="text-xs text-zinc-500">
          <span className="text-zinc-400 font-medium">Trinity</span> · online
        </div>
      </div>
    </aside>
  )
}
