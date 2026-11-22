CREATE TABLE projects (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	path       TEXT NOT NULL UNIQUE,
	position   INTEGER NOT NULL,
	created_at INTEGER NOT NULL
);

CREATE TABLE layouts (
	project_id TEXT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
	body       TEXT NOT NULL,
	updated_at INTEGER NOT NULL
);

-- status: running, done, or unknown (outcome no longer kept, or lost in a restart).
-- args_hash is a hash, never the arguments themselves.
CREATE TABLE commands (
	id        TEXT PRIMARY KEY,
	name      TEXT NOT NULL,
	args_hash TEXT NOT NULL,
	status    TEXT NOT NULL,
	outcome   TEXT,
	done_at   INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX commands_status_done ON commands(status, done_at);
