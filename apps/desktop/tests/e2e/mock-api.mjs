import http from 'node:http'

const portArgIndex = process.argv.indexOf('--port')
const port = portArgIndex >= 0 ? Number(process.argv[portArgIndex + 1]) : 4174
const now = '2026-05-03T20:00:00Z'

const project = {
  id: 'project-alpha',
  name: 'Alpha',
  description: '',
  created_at: now,
  updated_at: now,
}

let issue = {
  id: 'issue-alpha',
  project_id: project.id,
  identifier: 'ISSUE-1',
  position: 1,
  title: 'Runtime QA work',
  description: 'Exercise Phase 3 assignment flow',
  status: 'todo',
  priority: 'high',
  assignee_id: '',
  creator_id: '',
  created_at: now,
  updated_at: now,
}

let runtimes = []
let agents = []
let assignments = []
let activity = [
  {
    id: 'activity-created',
    issue_id: issue.id,
    actor_id: '',
    type: 'issue.created',
    summary: 'Created work item',
    metadata: {},
    created_at: now,
  },
]

function json(res, status, body) {
  res.writeHead(status, {
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
    'Access-Control-Allow-Headers': 'Content-Type',
    'Content-Type': 'application/json',
  })
  res.end(JSON.stringify(body))
}

function noContent(res) {
  res.writeHead(204, {
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
    'Access-Control-Allow-Headers': 'Content-Type',
  })
  res.end()
}

async function readBody(req) {
  let raw = ''
  for await (const chunk of req) raw += chunk
  return raw ? JSON.parse(raw) : {}
}

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url ?? '/', `http://${req.headers.host}`)

  if (req.method === 'OPTIONS') {
    noContent(res)
    return
  }

  if (req.method === 'GET' && url.pathname === '/health') {
    json(res, 200, { status: 'ok' })
    return
  }

  if (req.method === 'GET' && url.pathname === '/api/projects') {
    json(res, 200, [project])
    return
  }

  if (req.method === 'GET' && url.pathname === `/api/projects/${project.id}/issues`) {
    json(res, 200, [issue])
    return
  }

  if (req.method === 'GET' && url.pathname === `/api/issues/${issue.id}`) {
    json(res, 200, issue)
    return
  }

  if (req.method === 'PUT' && url.pathname === `/api/issues/${issue.id}`) {
    const body = await readBody(req)
    issue = { ...issue, ...body, updated_at: '2026-05-03T20:30:00Z' }
    activity.push({
      id: `activity-updated-${activity.length}`,
      issue_id: issue.id,
      actor_id: '',
      type: 'issue.updated',
      summary: 'Updated work item',
      metadata: {},
      created_at: issue.updated_at,
    })
    json(res, 200, issue)
    return
  }

  if (req.method === 'GET' && url.pathname === `/api/projects/${project.id}/tags`) {
    json(res, 200, [])
    return
  }

  if (req.method === 'GET' && url.pathname === `/api/issues/${issue.id}/tags`) {
    json(res, 200, [])
    return
  }

  if (req.method === 'GET' && url.pathname === `/api/issues/${issue.id}/comments`) {
    json(res, 200, [])
    return
  }

  if (req.method === 'GET' && url.pathname === `/api/issues/${issue.id}/activity`) {
    json(res, 200, activity)
    return
  }

  if (req.method === 'GET' && url.pathname === '/api/runtimes') {
    json(res, 200, { runtimes })
    return
  }

  if (req.method === 'POST' && url.pathname === '/api/runtimes/discover') {
    runtimes = [
      {
        id: 'runtime-codex',
        kind: 'codex',
        display_name: 'Codex CLI',
        binary_path: '/usr/local/bin/codex',
        version_raw: 'codex 0.42.0',
        health_status: 'healthy',
        health_reason: '',
        last_checked_at: '2026-05-03T20:05:00Z',
        created_at: now,
        updated_at: '2026-05-03T20:05:00Z',
      },
    ]
    json(res, 200, { runtimes, summary: { healthy: 1, degraded: 0, missing: 3 } })
    return
  }

  if (req.method === 'GET' && url.pathname === '/api/agents') {
    json(res, 200, agents)
    return
  }

  if (req.method === 'POST' && url.pathname === '/api/agents') {
    const body = await readBody(req)
    const agent = {
      id: 'agent-builder',
      name: body.name,
      role: body.role ?? '',
      status: 'idle',
      runtime_id: body.runtime_id ?? '',
      model_hint: body.model_hint ?? '',
      instructions: body.instructions ?? '',
      is_assignable: Boolean(body.runtime_id && body.is_assignable),
      created_at: '2026-05-03T20:10:00Z',
      updated_at: '2026-05-03T20:10:00Z',
    }
    agents = [agent]
    json(res, 201, agent)
    return
  }

  if (req.method === 'POST' && url.pathname === `/api/issues/${issue.id}/assignments`) {
    const body = await readBody(req)
    const assignment = {
      id: 'assignment-one',
      issue_id: issue.id,
      agent_id: body.agent_id,
      requested_by: body.requested_by,
      status: 'queued',
      source_type: body.source_type,
      source_id: body.source_id,
      dedupe_key: body.client_request_id,
      requested_at: '2026-05-03T20:20:00Z',
      created_at: '2026-05-03T20:20:00Z',
      updated_at: '2026-05-03T20:20:00Z',
    }
    assignments = [assignment]
    activity.push({
      id: 'activity-assignment-requested',
      issue_id: issue.id,
      actor_id: 'local-user',
      type: 'assignment.requested',
      summary: 'Requested assignment for Builder',
      metadata: {},
      created_at: assignment.created_at,
    })
    json(res, 201, { assignment, idempotent_replay: false })
    return
  }

  if (req.method === 'GET' && url.pathname === `/api/issues/${issue.id}/assignments`) {
    json(res, 200, { assignments })
    return
  }

  if (req.method === 'POST' && url.pathname === '/api/assignments/assignment-one/cancel') {
    assignments = assignments.map((assignment) => ({
      ...assignment,
      status: 'cancelled',
      cancelled_at: '2026-05-03T20:25:00Z',
      updated_at: '2026-05-03T20:25:00Z',
    }))
    activity.push({
      id: 'activity-assignment-cancelled',
      issue_id: issue.id,
      actor_id: 'local-user',
      type: 'assignment.cancelled',
      summary: 'Cancelled assignment for Builder',
      metadata: {},
      created_at: '2026-05-03T20:25:00Z',
    })
    json(res, 200, { assignment: assignments[0] })
    return
  }

  json(res, 404, { error: { code: 'NOT_FOUND', message: `${req.method} ${url.pathname}`, details: '' } })
})

server.listen(port, '127.0.0.1', () => {
  process.stdout.write(`mock api listening on ${port}\n`)
})

process.on('SIGTERM', () => server.close(() => process.exit(0)))
process.on('SIGINT', () => server.close(() => process.exit(0)))
