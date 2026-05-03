export interface Project {
  id: string
  name: string
  description: string
  created_at: string
  updated_at: string
}

export interface Agent {
  id: string
  name: string
  role: string
  status: string
  runtime_id: string
  runtime_name: string
  runtime_status: string
  model: string
  instructions: string
  assignable: boolean
  assignable_reason: string
  created_at: string
  updated_at: string
}

export interface Issue {
  id: string
  project_id: string
  identifier: string
  position: number
  title: string
  description: string
  status: string
  priority: string
  assignee_id: string
  creator_id: string
  created_at: string
  updated_at: string
}

export interface Tag {
  id: string
  project_id: string
  name: string
  color: string
  created_at: string
  updated_at: string
}

export interface IssueComment {
  id: string
  issue_id: string
  author_id: string
  author_name: string
  body: string
  created_at: string
  updated_at: string
}

export interface IssueAssignment {
  id: string
  issue_id: string
  agent_id: string
  agent_name: string
  runtime_id: string
  runtime_name: string
  runtime_status: string
  requested_by: string
  source_type: 'issue_detail' | 'comment'
  source_id: string
  client_request_id: string
  status: 'queued' | 'accepted' | 'completed' | 'failed' | 'cancelled'
  dedupe_key: string
  requested_at: string
  accepted_at: string | null
  completed_at: string | null
  failed_at: string | null
  cancelled_at: string | null
  created_at: string
  updated_at: string
}

export interface IssueActivity {
  id: string
  issue_id: string
  actor_id: string
  type: string
  summary: string
  metadata: Record<string, unknown>
  created_at: string
}

export type RuntimeKind = 'codex' | 'claude' | 'gemini' | 'opencode'
export type RuntimeHealthStatus = 'healthy' | 'degraded' | 'missing'

export interface RuntimeRecord {
  id: string
  kind: RuntimeKind
  display_name: string
  binary_path: string
  version_raw: string
  health_status: RuntimeHealthStatus
  health_reason: string
  last_checked_at: string
  created_at: string
  updated_at: string
}

export type Runtime = RuntimeRecord

export interface RuntimeListResponse {
  runtimes: RuntimeRecord[]
}

export interface RuntimeDiscoverySummary {
  healthy: number
  degraded: number
  missing: number
}

export interface RuntimeDiscoveryResponse {
  runtimes: RuntimeRecord[]
  summary: RuntimeDiscoverySummary
}

export interface RuntimeMutationResponse {
  runtime: Partial<RuntimeRecord> & { id: string }
}
