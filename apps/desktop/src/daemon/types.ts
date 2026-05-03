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

export interface Runtime {
  id: string
  name: string
  kind: string
  command: string
  path: string
  version: string
  status: string
  status_message: string
  last_checked_at: string
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
