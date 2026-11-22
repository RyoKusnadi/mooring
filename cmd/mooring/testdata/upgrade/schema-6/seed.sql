-- Made-up rows for the schema 6 upgrade fixture. Nothing here is real data.
-- {{ROOT}} stands for the data folder of the test that loads the fixture.
INSERT INTO projects (id, name, path, position, created_at) VALUES
	('p1', 'demo-app', '{{ROOT}}/work/demo-app', 0, 1700000100),
	('p2', 'notes', '{{ROOT}}/work/notes', 1, 1700000200);
INSERT INTO layouts (project_id, body, updated_at) VALUES
	('p1', '{"tabs":[]}', 1700000300);
INSERT INTO chats (id, project_id, agent, title, session_id, spec, caps, mode, model, usage, created_at, updated_at) VALUES
	('c1', 'p1', 'claude', 'Fix the login test', 'sess-1',
	 '{"agent":{"id":"claude"},"folder":{"path":""},"toolServers":[{"id":"t1"}],"account":{"id":"acct-work"},"filters":[]}',
	 '{"load":true,"resume":false,"image":false,"embeddedContext":true,"mcpHttp":true,"mcpSse":false}',
	 'default', 'sonnet', '', 1700000400000, 1700000500000),
	('c2', 'p1', 'claude', 'Empty chat', '',
	 '{"agent":{"id":"claude"},"folder":{"path":""},"toolServers":[],"account":{"id":""},"filters":[]}',
	 '{}', '', '', '', 1700000600000, 1700000600000);
INSERT INTO chat_messages (chat_id, idx, body) VALUES
	('c1', 0, '{"id":0,"kind":"user","at":1700000400000,"text":"The login test fails on CI."}'),
	('c1', 1, '{"id":1,"kind":"agent","at":1700000410000,"text":"The test reads the clock before the fixture sets it."}'),
	('c1', 2, '{"id":2,"kind":"tool","at":1700000420000,"toolId":"tool-1","title":"Read login_test.go","toolKind":"read","status":"completed"}');
INSERT INTO accounts (id, agent, name, dir, managed, login, usage, position) VALUES
	('default:claude', 'claude', 'Default', '', 0, 'signedIn', '', 0),
	('acct-work', 'claude', 'Work', '{{ROOT}}/accounts/work', 1, 'unknown', '', 1);
INSERT INTO account_defaults (agent, account_id) VALUES ('claude', 'acct-work');
INSERT INTO project_accounts (project_id, agent, account_id) VALUES ('p1', 'claude', 'acct-work');
INSERT INTO tool_servers (id, name, kind, command, args, env, headers, url, allow_private, enabled, allowed_sig, source, sign_in, expires, checked_at, check_ok, tools, position) VALUES
	('t1', 'docs', 'stdio', 'docs-server', '["--stdio"]', '[]', '[]', '', 0, 1, 'sig-docs', '{}', '', 0, 1700000700, 1, '["search"]', 0),
	('t2', 'issues', 'http', '', '[]', '[]', '[]', 'https://issues.example.com/mcp', 0, 0, '', '{}', '', 0, 0, 0, '[]', 1);
INSERT INTO ssh_hosts (id, name, address, port, user, auth, key_path, agent_forward, root, remembered, position) VALUES
	('h1', 'build box', 'build.example.com', 22, 'builder', 'key', '', 0, '', 0, 0);
INSERT INTO lsp_consent (server_id, command, allowed_at) VALUES ('gopls', '["/usr/local/bin/gopls"]', 1700000800);
