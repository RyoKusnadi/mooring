# Mooring

One Go binary that serves a Svelte 5 web app: a browser workspace for terminals, AI coding
agents (any ACP agent: Claude, Grok, Cursor, Gemini, Codex, Antigravity, and others), files, git, SSH and
language servers. Single user, one machine. Everything is started from the browser.

This file is for coding agents and for people who want the short version. Human contributors:
see `CONTRIBUTING.md`. Security reports: see `SECURITY.md`. Pictures and walkthroughs of the
app: `docs/guide/`.

## Layout

- `cmd/mooring`: entry point, flags and the subcommands (`open`, `token`, `password`, `autostart`, `bench`, `version`).
- `internal/server`: HTTP server: login, host and origin guard, serving the web app.
- `internal/webui`: embeds the built web app (`internal/webui/dist`, generated).
- `internal/*`: one package per area (terminal, chat, files, git, ssh, lsp, mcp, secrets, and so on).
  `internal/fake*` are stand-ins for agents, SSH, tool servers and language servers, used by tests only.
- `web/`: Svelte 5 + Vite single-page app, no SvelteKit. `src/lib` holds non-UI code;
  `e2e/` holds browser tests.
- `scripts/`: helpers for `make check` and releases. `docs/guide/`: the user guide. `bench/`:
  fixtures for `mooring bench`.
- `DESIGN.md`: the visual rules. Colours come from the tokens in `web/src/app.css`, which follow it.

## Commands

- `make check`: gofmt, go vet, golangci-lint, `go test -race`, svelte-check, vitest, build,
  size limits, Playwright on the installed Chrome. Run it before you send a change.
- `make run`: build and serve on 127.0.0.1:7340. `bin/mooring open` opens the browser signed in;
  `bin/mooring token` prints the login token.
- `make dev`: Vite with hot reload on :5173, sending `/api` to a running `make run`.
- `make build-e2e`: the binary browser tests use. It is the release build plus test-only flags
  behind the `e2e` build tag. Release builds do not have those flags.
- One Go test: `go test ./internal/server -run TestName`.
- One web test: `cd web && pnpm exec vitest run src/lib/api.test.ts`.
- Browser tests on another port: `MOORING_E2E_PORT=<base>` uses ports base to base+4.
  The default base is 7341.

## Rules

- Source files stay under 300 lines (checked). Split by responsibility, not by size alone.
- Go: standard library first. A new dependency needs its reason in the commit message.
  Wrap errors with context. Log with `log/slog` at boundaries and on failures.
- Never log or store in SQLite: tokens, passwords, cookies, keys. Secrets go to the secret store.
- Every request and socket checks Host and Origin. Bind to loopback only unless a TLS cert and a password are set.
- Every file path from the browser goes through one resolve-then-check function.
- Every command from the browser carries a client id and runs once. Every stream has a position.
- Web: Svelte 5 runes. No UI kit, no CSS framework. Styles are scoped in components; colours
  come only from tokens in `web/src/app.css`.
- Terminal, editor, diff view and language features load on first use. The app shell stays
  under 150 KB of compressed JavaScript (checked).
- Tests sit next to code: Go `_test.go`, web `*.test.ts`, user flows in `web/e2e`.
  Tests never start a real agent, SSH server or language server; use the `internal/fake*` packages.
- Do not read or edit generated files: `internal/webui/dist`, `web/pnpm-lock.yaml`, `go.sum`,
  `THIRD_PARTY_NOTICES.txt` (made by `node scripts/notices.mjs`).
- UI text is plain and specific. Name the thing and the next step; no marketing tone.

## Words

- Project: a folder in the sidebar. Agent chat: the tab for one Session.
- Session: a stored conversation. Account: one login of a Provider, with its own limits.
- Tool server: an MCP server. Attention: something waiting for the user (a permission
  prompt, a question, a stopped chat).
