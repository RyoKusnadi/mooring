-- Saved agent chats. No tokens, passwords or keys are stored here: spec holds the
-- session spec without tool-server environment values or headers.
CREATE TABLE chats (
	id         TEXT PRIMARY KEY,
	project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	agent      TEXT NOT NULL,
	title      TEXT NOT NULL,
	session_id TEXT NOT NULL DEFAULT '',
	spec       TEXT NOT NULL,
	caps       TEXT NOT NULL DEFAULT '{}',
	mode       TEXT NOT NULL DEFAULT '',
	model      TEXT NOT NULL DEFAULT '',
	usage      TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);
CREATE INDEX chats_project ON chats(project_id, updated_at);

-- One row per message; idx is the message id in the chat's stream.
CREATE TABLE chat_messages (
	chat_id TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
	idx     INTEGER NOT NULL,
	body    TEXT NOT NULL,
	PRIMARY KEY (chat_id, idx)
) WITHOUT ROWID;
