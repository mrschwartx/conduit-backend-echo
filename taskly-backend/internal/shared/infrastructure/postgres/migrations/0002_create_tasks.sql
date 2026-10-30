CREATE TABLE tasks (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name        VARCHAR(120) NOT NULL,
  description VARCHAR(2000) NOT NULL DEFAULT '',
  priority    VARCHAR(50) NOT NULL CHECK (priority IN ('low','medium','high')),
  due         TIMESTAMPTZ NOT NULL,
  status      VARCHAR(50) NOT NULL CHECK (status IN ('todo','in_progress','done')),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX tasks_owner_status ON tasks (owner_id, status);