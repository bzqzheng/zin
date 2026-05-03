import { useDaemon } from './useDaemon'
import type { Project, Issue, Agent, Runtime } from './types'

export function useApi() {
  const { fetchApi, baseURL } = useDaemon()

  return {
    projects: {
      list: () => fetchApi<Project[]>('/api/projects'),
      get: (id: string) => fetchApi<Project>(`/api/projects/${id}`),
      create: (name: string, description?: string) =>
        fetchApi<Project>('/api/projects', {
          method: 'POST',
          body: JSON.stringify({ name, description: description || '' }),
        }),
      update: (id: string, data: { name?: string; description?: string }) =>
        fetchApi<Project>(`/api/projects/${id}`, {
          method: 'PUT',
          body: JSON.stringify(data),
        }),
      delete: (id: string) =>
        fetchApi<void>(`/api/projects/${id}`, { method: 'DELETE' }),
    },

    issues: {
      list: (projectId: string) =>
        fetchApi<Issue[]>(`/api/projects/${projectId}/issues`),
      get: (id: string) => fetchApi<Issue>(`/api/issues/${id}`),
      create: (projectId: string, title: string, data?: Partial<Issue>) =>
        fetchApi<Issue>(`/api/projects/${projectId}/issues`, {
          method: 'POST',
          body: JSON.stringify({ title, ...data }),
        }),
      update: (id: string, data: Partial<Issue>) =>
        fetchApi<Issue>(`/api/issues/${id}`, {
          method: 'PUT',
          body: JSON.stringify(data),
        }),
      updateStatus: (id: string, status: string) =>
        fetchApi<Issue>(`/api/issues/${id}/status`, {
          method: 'PUT',
          body: JSON.stringify({ status }),
        }),
      delete: (id: string) =>
        fetchApi<void>(`/api/issues/${id}`, { method: 'DELETE' }),
    },

    agents: {
      list: () => fetchApi<Agent[]>('/api/agents'),
      listAssignable: () => fetchApi<Agent[]>('/api/agents?assignable=true'),
      get: (id: string) => fetchApi<Agent>(`/api/agents/${id}`),
      create: (data: { name: string; role?: string; runtime_id?: string; model?: string; instructions?: string }) =>
        fetchApi<Agent>('/api/agents', {
          method: 'POST',
          body: JSON.stringify(data),
        }),
      update: (
        id: string,
        data: { name?: string; role?: string; status?: string; runtime_id?: string; model?: string; instructions?: string },
      ) =>
        fetchApi<Agent>(`/api/agents/${id}`, {
          method: 'PUT',
          body: JSON.stringify(data),
        }),
      delete: (id: string) =>
        fetchApi<void>(`/api/agents/${id}`, { method: 'DELETE' }),
    },

    runtimes: {
      list: () => fetchApi<Runtime[]>('/api/runtimes'),
      create: (data: {
        name: string
        kind: string
        command?: string
        path?: string
        version?: string
        status?: string
        status_message?: string
      }) =>
        fetchApi<Runtime>('/api/runtimes', {
          method: 'POST',
          body: JSON.stringify(data),
        }),
    },

    health: {
      check: () => fetchApi<{ status: string; uptime_seconds: number; db_status: string; version: string; port: number }>('/health'),
    },

    baseURL,
  }
}
