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
  model_hint: string
  instructions: string
  is_assignable: boolean
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

export type AssignmentStatus = 'queued' | 'running' | 'completed' | 'failed' | 'cancelled'

export interface IssueAssignment {
  id: string
  issue_id: string
  agent_id: string | null
  requested_by: string
  status: AssignmentStatus
  source_type: string
  source_id: string
  dedupe_key: string
  requested_at: string
  accepted_at?: string
  completed_at?: string
  failed_at?: string
  cancelled_at?: string
  created_at: string
  updated_at: string
}

export interface AssignmentListResponse {
  assignments: IssueAssignment[]
}

export interface CreateAssignmentResponse {
  assignment: IssueAssignment
  idempotent_replay: boolean
}

export interface AssignmentMutationResponse {
  assignment: IssueAssignment
}
