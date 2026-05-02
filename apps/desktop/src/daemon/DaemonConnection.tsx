import { useState, useCallback, type ReactNode } from 'react'
import { invoke } from '@tauri-apps/api/core'
import { DaemonContext, type DaemonConnectionState } from './useDaemon'

export function DaemonConnectionProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<DaemonConnectionState>({
    port: null,
    connected: false,
    error: null,
  })

  const baseURL = state.port ? `http://127.0.0.1:${state.port}` : null

  const fetchApi = useCallback(
    async <T,>(path: string, init?: RequestInit): Promise<T> => {
      if (!baseURL) throw new Error('daemon not connected')
      const resp = await fetch(`${baseURL}${path}`, {
        ...init,
        headers: { 'Content-Type': 'application/json', ...init?.headers },
      })
      if (!resp.ok) {
        const err = await resp.json().catch(() => ({ message: resp.statusText }))
        throw new Error(err?.error?.message || err?.message || `HTTP ${resp.status}`)
      }
      if (resp.status === 204) return undefined as T
      return resp.json()
    },
    [baseURL],
  )

  const startDaemon = useCallback(async (): Promise<number> => {
    try {
      const info: { port: number } = await invoke('start_daemon')
      setState({ port: info.port, connected: true, error: null })
      return info.port
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      setState({ port: null, connected: false, error: message })
      throw err
    }
  }, [])

  const checkHealth = useCallback(async () => {
    try {
      const health: { status: string } = await invoke('check_health')
      if (health.status === 'ok') {
        setState((prev) => ({ ...prev, connected: true, error: null }))
      } else {
        setState((prev) => ({ ...prev, connected: false, error: `status: ${health.status}` }))
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      setState((prev) => ({ ...prev, connected: false, error: message }))
    }
  }, [])

  const shutdown = useCallback(async () => {
    try {
      await invoke('shutdown_daemon')
    } catch {
      // ignore shutdown errors
    } finally {
      setState({ port: null, connected: false, error: null })
    }
  }, [])

  return (
    <DaemonContext.Provider
      value={{
        ...state,
        startDaemon,
        checkHealth,
        shutdown,
        fetchApi,
        baseURL,
      }}
    >
      {children}
    </DaemonContext.Provider>
  )
}
