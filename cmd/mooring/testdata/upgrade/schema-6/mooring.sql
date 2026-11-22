PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL);
INSERT INTO schema_migrations VALUES(1,1700000000);
INSERT INTO schema_migrations VALUES(2,1700000000);
INSERT INTO schema_migrations VALUES(3,1700000000);
INSERT INTO schema_migrations VALUES(4,1700000000);
INSERT INTO schema_migrations VALUES(5,1700000000);
INSERT INTO schema_migrations VALUES(6,1700000000);
CREATE TABLE projects (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	path       TEXT NOT NULL UNIQUE,
	position   INTEGER NOT NULL,
	created_at INTEGER NOT NULL
);
INSERT INTO projects VALUES('p1','demo-app','{{ROOT}}/work/demo-app',0,1700000100);
INSERT INTO projects VALUES('p2','notes','{{ROOT}}/work/notes',1,1700000200);
CREATE TABLE layouts (
	project_id TEXT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
	body       TEXT NOT NULL,
	updated_at INTEGER NOT NULL
);
INSERT INTO layouts VALUES('p1','{"tabs":[]}',1700000300);
CREATE TABLE commands (
	id        TEXT PRIMARY KEY,
	name      TEXT NOT NULL,
	args_hash TEXT NOT NULL,
	status    TEXT NOT NULL,
	outcome   TEXT,
	done_at   INTEGER NOT NULL DEFAULT 0
);
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
INSERT INTO chats VALUES('c1','p1','claude','Fix the login test','sess-1','{"agent":{"id":"claude"},"folder":{"path":""},"toolServers":[{"id":"t1"}],"account":{"id":"acct-work"},"filters":[]}','{"load":true,"resume":false,"image":false,"embeddedContext":true,"mcpHttp":true,"mcpSse":false}','default','sonnet','',1700000400000,1700000500000);
INSERT INTO chats VALUES('c2','p1','claude','Empty chat','','{"agent":{"id":"claude"},"folder":{"path":""},"toolServers":[],"account":{"id":""},"filters":[]}','{}','','','',1700000600000,1700000600000);
CREATE TABLE chat_messages (
	chat_id TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
	idx     INTEGER NOT NULL,
	body    TEXT NOT NULL,
	PRIMARY KEY (chat_id, idx)
) WITHOUT ROWID;
INSERT INTO chat_messages VALUES('c1',0,'{"id":0,"kind":"user","at":1700000400000,"text":"The login test fails on CI."}');
INSERT INTO chat_messages VALUES('c1',1,'{"id":1,"kind":"agent","at":1700000410000,"text":"The test reads the clock before the fixture sets it."}');
INSERT INTO chat_messages VALUES('c1',2,'{"id":2,"kind":"tool","at":1700000420000,"toolId":"tool-1","title":"Read login_test.go","toolKind":"read","status":"completed"}');
CREATE TABLE lsp_consent (
	server_id  TEXT PRIMARY KEY,
	command    TEXT NOT NULL,
	allowed_at INTEGER NOT NULL
);
INSERT INTO lsp_consent VALUES('gopls','["/usr/local/bin/gopls"]',1700000800);
CREATE TABLE lsp_custom (
	id          TEXT PRIMARY KEY,
	name        TEXT NOT NULL,
	endings     TEXT NOT NULL,
	language_id TEXT NOT NULL,
	command     TEXT NOT NULL
);
CREATE TABLE ssh_hosts (
	id            TEXT PRIMARY KEY,
	name          TEXT NOT NULL,
	address       TEXT NOT NULL,
	port          INTEGER NOT NULL,
	user          TEXT NOT NULL,
	auth          TEXT NOT NULL,
	key_path      TEXT NOT NULL DEFAULT '',
	agent_forward INTEGER NOT NULL DEFAULT 0,
	root          TEXT NOT NULL DEFAULT '',
	remembered    INTEGER NOT NULL DEFAULT 0,
	position      INTEGER NOT NULL
);
INSERT INTO ssh_hosts VALUES('h1','build box','build.example.com',22,'builder','key','',0,'',0,0);
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
INSERT INTO tool_servers VALUES('t1','docs','stdio','docs-server','["--stdio"]','[]','[]','',0,1,'sig-docs','{}','',0,1700000700,1,'["search"]',0);
INSERT INTO tool_servers VALUES('t2','issues','http','','[]','[]','[]','https://issues.example.com/mcp',0,0,'','{}','',0,0,0,'[]',1);
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
INSERT INTO accounts VALUES('default:claude','claude','Default','',0,'signedIn','',0);
INSERT INTO accounts VALUES('acct-work','claude','Work','{{ROOT}}/accounts/work',1,'unknown','',1);
CREATE TABLE account_defaults (
	agent      TEXT PRIMARY KEY,
	account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE
);
INSERT INTO account_defaults VALUES('claude','acct-work');
CREATE TABLE project_accounts (
	project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	agent      TEXT NOT NULL,
	account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
	PRIMARY KEY (project_id, agent)
);
INSERT INTO project_accounts VALUES('p1','claude','acct-work');
CREATE INDEX commands_status_done ON commands(status, done_at);
CREATE INDEX chats_project ON chats(project_id, updated_at);
COMMIT;
