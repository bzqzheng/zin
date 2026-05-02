export default function Timeline() {
  const events = [
    { id: '1', action: 'Issue created', agent: 'Morpheus', time: '2h ago', type: 'create' },
    { id: '2', action: 'Assigned to Trinity', agent: 'Morpheus', time: '1h ago', type: 'assign' },
    { id: '3', action: 'Status: In Progress', agent: 'Trinity', time: '5m ago', type: 'status' },
    { id: '4', action: 'Scaffold started', agent: 'Trinity', time: 'just now', type: 'update' },
  ]

  const typeDot = (t: string) => {
    switch (t) {
      case 'create':
        return 'bg-emerald-500'
      case 'assign':
        return 'bg-blue-500'
      case 'status':
        return 'bg-amber-500'
      default:
        return 'bg-zinc-600'
    }
  }

  return (
    <aside className="w-72 bg-zinc-900 border-l border-zinc-800 flex flex-col overflow-hidden">
      <div className="px-4 py-3 border-b border-zinc-800">
        <h2 className="text-sm font-semibold text-zinc-100">Timeline</h2>
      </div>

      <div className="flex-1 overflow-y-auto px-3 py-2">
        <div className="space-y-1">
          {events.map((event) => (
            <div
              key={event.id}
              className="flex gap-3 px-2 py-2 rounded hover:bg-zinc-800/50 transition-colors"
            >
              <div className="relative mt-0.5">
                <div className={`w-2 h-2 rounded-full ${typeDot(event.type)}`} />
              </div>
              <div className="min-w-0">
                <p className="text-sm text-zinc-300 truncate">{event.action}</p>
                <p className="text-xs text-zinc-500">
                  {event.agent} · {event.time}
                </p>
              </div>
            </div>
          ))}
        </div>
      </div>

      <div className="px-4 py-3 border-t border-zinc-800">
        <p className="text-xs text-zinc-600">4 events · Phase 1</p>
      </div>
    </aside>
  )
}
