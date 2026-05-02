import Sidebar from './components/Sidebar'
import IssueDetail from './components/IssueDetail'
import Timeline from './components/Timeline'

export default function App() {
  return (
    <div className="h-screen w-screen bg-zinc-950 text-zinc-100 flex overflow-hidden">
      <Sidebar />
      <IssueDetail />
      <Timeline />
    </div>
  )
}
