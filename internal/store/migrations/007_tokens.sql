-- Token saving (phase 12). Settings are JSON values by key; no secrets are kept here.
CREATE TABLE settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

-- Raw command output of a chat, kept so the user can read what the agent was not shown.
-- At most 50 per chat; the oldest go first. The id is made by Mooring, per chat.
CREATE TABLE chat_outputs (
	chat_id    TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
	id         TEXT NOT NULL,
	tool_id    TEXT NOT NULL DEFAULT '',
	raw        BLOB NOT NULL,
	lines      INTEGER NOT NULL,
	bytes      INTEGER NOT NULL,
	kept_lines INTEGER NOT NULL,
	exit_code  INTEGER,
	created_at INTEGER NOT NULL,
	PRIMARY KEY (chat_id, id)
);

-- The chat a fresh chat was started from with a summary.
ALTER TABLE chats ADD COLUMN from_chat TEXT NOT NULL DEFAULT '';

-- Tokens spent by small-model jobs (titles, commit messages), per day.
CREATE TABLE job_usage (
	day    TEXT NOT NULL,
	agent  TEXT NOT NULL,
	model  TEXT NOT NULL,
	kind   TEXT NOT NULL,
	runs   INTEGER NOT NULL DEFAULT 0,
	tokens INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (day, agent, model, kind)
);
