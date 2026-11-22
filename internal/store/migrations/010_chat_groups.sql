-- A group is 2 or 3 chats of one project working on one task. A chat is in at
-- most one group; a group with fewer than 2 chats is deleted.
CREATE TABLE chat_groups (
	id         TEXT PRIMARY KEY,
	project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	title      TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL
);
ALTER TABLE chats ADD COLUMN group_id TEXT;
CREATE INDEX chats_group ON chats(group_id) WHERE group_id IS NOT NULL;

-- The text Mooring sent the agent as its extra instructions, fixed when the chat
-- was made so a restart or resume sends the same bytes. Empty: none. It holds no
-- secrets: the fixed text plus what the user wrote for the project.
ALTER TABLE chats ADD COLUMN instructions TEXT NOT NULL DEFAULT '';
