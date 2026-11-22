-- When a chat's last reply to the user ended, and when the user last looked at
-- it, both in Unix milliseconds. A chat that finished after it was last seen
-- shows on the status board under "Finished since you looked".
ALTER TABLE chats ADD COLUMN finished_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE chats ADD COLUMN seen_at INTEGER NOT NULL DEFAULT 0;
