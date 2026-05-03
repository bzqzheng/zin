import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '../index.css'
import App from '../App'
import { DaemonContext, type DaemonConnectionContext } from '../daemon/useDaemon'

const baseURL = window.location.origin

const daemon: DaemonConnectionContext = {
  port: window.location.port ? Number(window.location.port) : 80,
  connected: true,
  error: null,
  baseURL,
  startDaemon: async () => daemon.port ?? 0,
  checkHealth: async () => undefined,
  shutdown: async () => undefined,
  fetchApi: async <T,>(path: string, init?: RequestInit): Promise<T> => {
    const response = await fetch(`${baseURL}${path}`, {
      ...init,
      headers: { 'Content-Type': 'application/json', ...init?.headers },
    })
    if (!response.ok) {
      const err = await response.json().catch(() => ({ message: response.statusText }))
      throw new Error(err?.error?.message || err?.message || `HTTP ${response.status}`)
    }
    if (response.status === 204) return undefined as T
    return response.json()
  },
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <DaemonContext.Provider value={daemon}>
      <App />
    </DaemonContext.Provider>
  </StrictMode>,
)
