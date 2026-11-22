-- Full-text search over what the user and the agent wrote in every chat.
-- chat_search_rows maps one chat message to one row of the index, so a message
-- that grows while the agent replies replaces its own row and nothing else.
CREATE TABLE chat_search_rows (
	id      INTEGER PRIMARY KEY,
	chat_id TEXT NOT NULL,
	idx     INTEGER NOT NULL,
	UNIQUE (chat_id, idx)
);

CREATE VIRTUAL TABLE chat_search USING fts5(text, tokenize = 'unicode61 remove_diacritics 2');

-- Only plain user and agent messages are indexed: no jobs, tool output or thoughts.
CREATE TRIGGER chat_search_insert AFTER INSERT ON chat_messages
WHEN json_extract(new.body, '$.kind') IN ('user', 'agent')
	AND coalesce(json_extract(new.body, '$.job'), '') = ''
	AND coalesce(json_extract(new.body, '$.text'), '') != ''
BEGIN
	INSERT INTO chat_search_rows (chat_id, idx) VALUES (new.chat_id, new.idx);
	INSERT INTO chat_search (rowid, text) VALUES (last_insert_rowid(), json_extract(new.body, '$.text'));
END;

CREATE TRIGGER chat_search_update AFTER UPDATE OF body ON chat_messages
BEGIN
	DELETE FROM chat_search WHERE rowid =
		(SELECT id FROM chat_search_rows WHERE chat_id = old.chat_id AND idx = old.idx);
	DELETE FROM chat_search_rows WHERE chat_id = old.chat_id AND idx = old.idx;
	INSERT INTO chat_search_rows (chat_id, idx)
		SELECT new.chat_id, new.idx
		WHERE json_extract(new.body, '$.kind') IN ('user', 'agent')
			AND coalesce(json_extract(new.body, '$.job'), '') = ''
			AND coalesce(json_extract(new.body, '$.text'), '') != '';
	INSERT INTO chat_search (rowid, text)
		SELECT id, json_extract(new.body, '$.text') FROM chat_search_rows
		WHERE chat_id = new.chat_id AND idx = new.idx;
END;

-- Also runs when a removed chat takes its messages with it (ON DELETE CASCADE).
CREATE TRIGGER chat_search_delete AFTER DELETE ON chat_messages
BEGIN
	DELETE FROM chat_search WHERE rowid =
		(SELECT id FROM chat_search_rows WHERE chat_id = old.chat_id AND idx = old.idx);
	DELETE FROM chat_search_rows WHERE chat_id = old.chat_id AND idx = old.idx;
END;

-- Chats saved before this migration.
INSERT INTO chat_search_rows (chat_id, idx)
	SELECT chat_id, idx FROM chat_messages
	WHERE json_extract(body, '$.kind') IN ('user', 'agent')
		AND coalesce(json_extract(body, '$.job'), '') = ''
		AND coalesce(json_extract(body, '$.text'), '') != '';
INSERT INTO chat_search (rowid, text)
	SELECT r.id, json_extract(m.body, '$.text')
	FROM chat_search_rows r JOIN chat_messages m ON m.chat_id = r.chat_id AND m.idx = r.idx;
