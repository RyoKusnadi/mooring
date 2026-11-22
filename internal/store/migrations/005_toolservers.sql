-- Tool servers (MCP). Secret values are never here: env and header values, tokens and the
-- registered sign-in client are in the secret store under mcp:<id>:...; `set` flags inside the
-- env and headers JSON only say that a value was saved. allowed_sig is the hash of the command
-- line or address the user allowed; a changed one no longer matches.
CREATE TABLE tool_servers (
	id            TEXT PRIMARY KEY,
	name          TEXT NOT NULL UNIQUE,
	kind          TEXT NOT NULL,
	command       TEXT NOT NULL DEFAULT '',
	args          TEXT NOT NULL DEFAULT '[]',
	env           TEXT NOT NULL DEFAULT '[]',
	headers       TEXT NOT NULL DEFAULT '[]',
	url           TEXT NOT NULL DEFAULT '',
	allow_private INTEGER NOT NULL DEFAULT 0,
	enabled       INTEGER NOT NULL DEFAULT 0,
	allowed_sig   TEXT NOT NULL DEFAULT '',
	source        TEXT NOT NULL DEFAULT '{}',
	sign_in       TEXT NOT NULL DEFAULT '',
	expires       INTEGER NOT NULL DEFAULT 0,
	checked_at    INTEGER NOT NULL DEFAULT 0,
	check_ok      INTEGER NOT NULL DEFAULT 0,
	tools         TEXT NOT NULL DEFAULT '[]',
	position      INTEGER NOT NULL
);
