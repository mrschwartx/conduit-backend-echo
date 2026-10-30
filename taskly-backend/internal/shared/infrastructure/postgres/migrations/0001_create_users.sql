CREATE TABLE users (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  username      VARCHAR(50)  NOT NULL,
  email         VARCHAR(254) NOT NULL,
  password_hash TEXT NOT NULL,
  avatar_file   TEXT NOT NULL DEFAULT '',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_unique    ON users (lower(email));
CREATE UNIQUE INDEX users_username_unique ON users (lower(username));