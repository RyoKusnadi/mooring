-- Accounts: one login of an agent, as a settings folder. No login token is ever here: the agent
-- keeps its own. The built-in login of an agent is the row "default:<agent>" with an empty dir,
-- created when something is first learned about it. login is unknown, signedIn or needed;
-- usage is the JSON the agent's usage reader built (percentages and reset times only).
CREATE TABLE accounts (
	id       TEXT PRIMARY KEY,
	agent    TEXT NOT NULL,
	name     TEXT NOT NULL,
	dir      TEXT NOT NULL DEFAULT '',
	managed  INTEGER NOT NULL DEFAULT 0,
	login    TEXT NOT NULL DEFAULT 'unknown',
	usage    TEXT NOT NULL DEFAULT '',
	position INTEGER NOT NULL DEFAULT 0,
	UNIQUE (agent, name)
);

-- The account new projects use for an agent. No row: the agent's built-in login.
CREATE TABLE account_defaults (
	agent      TEXT PRIMARY KEY,
	account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE
);

-- The account a project uses for an agent. No row: the default above.
CREATE TABLE project_accounts (
	project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	agent      TEXT NOT NULL,
	account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
	PRIMARY KEY (project_id, agent)
);
