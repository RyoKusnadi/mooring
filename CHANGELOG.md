# Changelog

All notable changes are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Mooring has no tagged release yet.

## [Unreleased]

First public version. What exists:

### Added

- One Go binary that serves a Svelte 5 web app on 127.0.0.1 for a single user. Everything starts from the browser.
- Projects: add a folder, clone a repository or make one. A file tree with edit, rename, move, upload,
  download, trash and search by name or text.
- Terminals in tabs and split panes that keep running when the page closes. A bell marks the tab that needs you.
- Agent chats over ACP for Claude, Gemini, Codex, Grok and Cursor. Each command asks for permission.
  Several accounts per agent with their limits, tool servers (MCP), queued messages, and token saving
  (output compaction and a ledger).
- Git: status, diff, stage whole files or single lines, commit, branches, push, fetch and pull (fast-forward
  only), stash, history and worktrees.
- SSH: saved hosts, terminals and remote files over SFTP. Remembered passwords go to the macOS keychain,
  `secret-tool` or an encrypted file.
- Language servers started on demand, with errors, hover, completion and go to definition in the editor.
- Sign-in by a login token or by a username and password. Sign-ins survive a restart and last 30 days.
  `mooring open`, `mooring token`, `mooring password` and `mooring autostart`.
- Light and dark themes, a command palette, Alt shortcuts, and a layout for narrow screens.
- `mooring bench`, a measurement of what the token-saving features save.
- Release builds for macOS and Linux, Intel and ARM, with `SHA256SUMS` and `THIRD_PARTY_NOTICES.txt`.

### Known limits

- The macOS binaries are not signed or notarised.
- Linux builds are vetted in CI but have had little hands-on use.
- Loopback only, no TLS.
