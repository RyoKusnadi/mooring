-- SSH hosts. Passwords are never here: a remembered one is in the secret store
-- under the host's id, and `remembered` only says that it was put there.
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
