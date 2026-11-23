# Manual test, November 2026

Hands-on checks before 1.0, on branch `release-prep`. Driven with Playwright on the
installed Chrome, plus curl and `docker exec` to check the result on disk. Screenshots
were kept outside the repo.

## Linux over HTTPS (Debian in Docker, arm64)

Mooring ran with `-addr 0.0.0.0:7443 -tls-cert -tls-key -secrets-passphrase-file`, a
self-made certificate for `localhost` and `127.0.0.1`, and a password from `mooring password`.

| Check | Result |
| --- | --- |
| Refuses a non-loopback address without a cert, without a key, without a password | pass, each with its own message |
| https by name and by IP with the cert trusted | pass |
| Foreign Host header | 403 |
| Plain http, untrusted cert, TLS 1.1 | refused |
| HSTS on localhost | not sent, as intended |
| 5 wrong passwords, then the 6th | 401 ×5, then 429; the right password also waits out the minute |
| A client that never finishes the handshake | closed after 10 s |
| Password sign-in in the browser, session cookie is `Secure` | pass |
| Add project, terminal runs a Linux shell | pass |
| Open, edit and save a file | pass |
| Delete a file: lands in `~/.local/share/Trash` with its `.trashinfo` | pass |
| Stage all and commit | pass |
| SSH host by password with "Remember on this computer" (passphrase-file store) | pass |
| Terminal on that host | pass |
| Restart Mooring, open the host again: no password prompt | pass |
| Terminal from before the restart | shows "This terminal is gone." |

## Mac to a real OpenSSH server (the same container, port 2222)

Mooring ran on a scratch data folder and a scratch home, so `known_hosts` and the key
file stayed out of the real home.

| Check | Result |
| --- | --- |
| Add host with a key file; the fingerprint shown matches `ssh-keygen -l` (ECDSA) | pass |
| Browse files over SFTP, edit and save | pass |
| Terminal on the host | pass |
| Kill the host's `sshd-session`: "The connection to the host ended." with Reconnect | pass |
| `exit 3` in the shell: "The shell exited with code 3." | pass |

No bugs found in this part.

## Real agents on the Mac (Claude, Antigravity)

Mooring ran from Terminal.app on a scratch data folder, so Claude could read its sign-in.
One Claude login (Claude Code through the npx adapter) and Antigravity CLI 1.5.14 (`agy acp`).
Gemini CLI was left out: Google stopped it for individual accounts in June 2026.

| Check | Result |
| --- | --- |
| Permission prompt, then the edit lands on disk | pass |
| Inline diff of the edit, chat title set from the first message | pass |
| Context and cost line | pass |
| Last turn view (checkpoint diff) | pass |
| Comment on a diff line, sent to Claude, which acts on it | pass |
| Tool server: the reference filesystem server, 14 tools, used by Claude behind a prompt | pass |
| Board: idle chats with reply boxes | pass |
| Group of two chats: a message reaches both; deleting a chat dissolves the group | pass |
| History search finds a word from an earlier chat | pass |
| Delete a chat: Claude's own session file is removed too | pass |
| Import with nothing new to import says so | pass |
| Import a session made with `claude -p` outside Mooring: the reply shows in a new chat | pass |
| Theme switch from the command palette, and back | pass |
| Second opinion from Antigravity: asks before each git command, reviews, changes no file | pass |
| Accounts and usage with one Claude login: signed in, 5-hour window, reset time | pass after bug 2 was fixed |

Bugs found and fixed:

1. The chat list line showed markdown as typed (`**Next step:**`, backticks, the
   first line of a code block). It now shows plain text and skips code blocks.
2. Claude reports a usage window with a status and no percentage. The row read
   "unknown used". It now says what the status means.
3. `mooring bench` refused the agent's own edits as "outside the folder" on macOS: the
   task folder was `/var/folders/...` and Claude names it `/private/var/folders/...`.
   The check now resolves links on both sides. The first real run was thrown away.

Not tried by hand: a second Claude account and switching (no second login; covered by
the tests with the stand-in agent), merge conflicts and blame (covered by the real-git
browser tests).
