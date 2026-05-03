ALTER TABLE agents ADD COLUMN runtime_id TEXT NOT NULL DEFAULT '';
ALTER TABLE agents ADD COLUMN model TEXT NOT NULL DEFAULT '';
ALTER TABLE agents ADD COLUMN instructions TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_agents_runtime_id ON agents(runtime_id);
