import { describe, it, expect, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { DaemonConnectionProvider } from '../daemon/DaemonConnection'
import { useDaemon } from '../daemon/useDaemon'

const mockInvoke = vi.fn()

vi.mock('@tauri-apps/api/core', () => ({
  invoke: (...args: unknown[]) => mockInvoke(...args),
}))

describe('DaemonConnection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('provides initial disconnected state', () => {
    const { result } = renderHook(() => useDaemon(), {
      wrapper: DaemonConnectionProvider,
    })

    expect(result.current.connected).toBe(false)
    expect(result.current.port).toBeNull()
    expect(result.current.error).toBeNull()
    expect(result.current.baseURL).toBeNull()
  })

  it('updates state on successful daemon start', async () => {
    mockInvoke.mockResolvedValueOnce({ port: 9876 })

    const { result } = renderHook(() => useDaemon(), {
      wrapper: DaemonConnectionProvider,
    })

    await act(async () => {
      const port = await result.current.startDaemon()
      expect(port).toBe(9876)
    })

    expect(result.current.connected).toBe(true)
    expect(result.current.port).toBe(9876)
    expect(result.current.baseURL).toBe('http://127.0.0.1:9876')
  })

  it('sets error on failed daemon start', async () => {
    mockInvoke.mockRejectedValueOnce(new Error('spawn failed'))

    const { result } = renderHook(() => useDaemon(), {
      wrapper: DaemonConnectionProvider,
    })

    await act(async () => {
      try {
        await result.current.startDaemon()
      } catch {
        // expected
      }
    })

    expect(result.current.connected).toBe(false)
    expect(result.current.error).toBe('spawn failed')
  })

  it('clears state on shutdown', async () => {
    mockInvoke.mockResolvedValueOnce({ port: 9876 })
    mockInvoke.mockResolvedValueOnce(undefined)

    const { result } = renderHook(() => useDaemon(), {
      wrapper: DaemonConnectionProvider,
    })

    await act(async () => {
      await result.current.startDaemon()
    })
    expect(result.current.connected).toBe(true)

    await act(async () => {
      await result.current.shutdown()
    })
    expect(result.current.connected).toBe(false)
    expect(result.current.port).toBeNull()
  })
})
