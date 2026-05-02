import { useDaemon } from './useDaemon'
import type { Project, Issue, Agent } from './types'

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
      get: (id: string) => fetchApi<Agent>(`/api/agents/${id}`),
      create: (name: string, role?: string) =>
        fetchApi<Agent>('/api/agents', {
          method: 'POST',
          body: JSON.stringify({ name, role: role || '' }),
        }),
      update: (id: string, data: { name?: string; role?: string; status?: string }) =>
        fetchApi<Agent>(`/api/agents/${id}`, {
          method: 'PUT',
          body: JSON.stringify(data),
        }),
      delete: (id: string) =>
        fetchApi<void>(`/api/agents/${id}`, { method: 'DELETE' }),
    },

    health: {
      check: () => fetchApi<{ status: string; uptime_seconds: number; db_status: string; version: string; port: number }>('/health'),
    },

    baseURL,
  }
}
