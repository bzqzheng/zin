import { createContext, useContext } from 'react'

export interface DaemonConnectionState {
  port: number | null
  connected: boolean
  error: string | null
}

export interface DaemonConnectionContext extends DaemonConnectionState {
  startDaemon: () => Promise<number>
  checkHealth: () => Promise<void>
  shutdown: () => Promise<void>
  fetchApi: <T>(path: string, init?: RequestInit) => Promise<T>
  baseURL: string | null
}

export const DaemonContext = createContext<DaemonConnectionContext | null>(null)

export function useDaemon() {
  const ctx = useContext(DaemonContext)
  if (!ctx) {
    throw new Error('useDaemon must be used within DaemonConnectionProvider')
  }
  return ctx
}
