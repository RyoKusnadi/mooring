-- Language servers. The command is what the user allowed: the full path a server
-- resolved to plus its arguments, as a JSON array. A changed command asks again.
CREATE TABLE lsp_consent (
	server_id  TEXT PRIMARY KEY,
	command    TEXT NOT NULL,
	allowed_at INTEGER NOT NULL
);

-- Servers the user added or overrode: endings and command are JSON arrays.
CREATE TABLE lsp_custom (
	id          TEXT PRIMARY KEY,
	name        TEXT NOT NULL,
	endings     TEXT NOT NULL,
	language_id TEXT NOT NULL,
	command     TEXT NOT NULL
);
