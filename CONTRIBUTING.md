# Contributing

Mooring is a one-person project, so small, focused changes are easiest to review. For anything
bigger than a bug fix, open an issue first and say what you want to change.

## Set up

You need:

- Go, the version in `go.mod`.
- Node and pnpm (the web app builds with Vite). `make` runs `pnpm install --frozen-lockfile` for you.
- [golangci-lint](https://golangci-lint.run/) v2. `make check` looks for it in `$(go env GOPATH)/bin`.
  Set `GOLANGCI=/path/to/golangci-lint` if it lives somewhere else.
- Google Chrome, installed normally. The browser tests drive it through Playwright's `chrome` channel
  and download no browser.
- git. The tests start real `git`, `sh` and (stand-in) SSH and agent processes on 127.0.0.1.

macOS is the platform the project is developed and tested on. Linux builds and the Linux vet run
in CI; Linux has had little hands-on use.

```sh
make run           # build and serve on 127.0.0.1:7340
bin/mooring open   # open the browser, signed in
make dev           # Vite with hot reload on :5173, proxies /api to a running make run
```

## Checks

`make check` is what CI runs. It does, in order: gofmt, `go vet`, golangci-lint, a Linux vet and an
Intel macOS build, `go test -race`, svelte-check, vitest, the production build, the size limits, and
the Playwright suite. It must pass before a change is merged.

- One Go test: `go test ./internal/server -run TestName`
- One web test: `cd web && pnpm exec vitest run src/lib/api.test.ts`
- Browser tests use ports 7341 to 7345. If something of yours already listens there, set
  `MOORING_E2E_PORT=<base>` to move all five.

The browser tests run `bin/mooring-e2e`, which `make build-e2e` builds with `-tags e2e`. That tag adds
flags that only tests use. Release builds do not have them.

## Rules the checks enforce

- A source file stays under 300 lines. Split by responsibility, not by size alone.
- The app shell stays under 150 KB of compressed JavaScript. The terminal, editor, diff view and
  language features load on first use. `scripts/check-size.mjs` checks both limits.
- Tests sit next to the code: Go `_test.go`, web `*.test.ts`, user flows in `web/e2e`. A bug fix
  comes with a test that fails without it.
- Colours come from the tokens in `web/src/app.css`, which follow `DESIGN.md`. No UI kit, no CSS framework.
- Go standard library first. A new dependency needs its reason in the commit message.
- Never log or store tokens, passwords, cookies or keys. Every file path from the browser goes
  through the one resolve-then-check function. See `AGENTS.md` for the full list.

Dependencies are listed in `THIRD_PARTY_NOTICES.txt`. After you change `go.mod` or `web/package.json`,
run `node scripts/notices.mjs` and commit the result.

## Commits and pull requests

- Branch from `main`. One topic per pull request.
- Commit subject in the imperative, under about 70 characters ("Fix the stash list after a rename").
  Add a body when the reason is not obvious.
- Mention the issue it closes, if any.
- Fill in the pull request template: what changed, how you tested it, and a screenshot for UI changes.
- UI text is plain and specific: name the thing and the next step.

## Reporting problems

Bugs and ideas go in GitHub issues. Security problems do not: see `SECURITY.md`.
By taking part you agree to follow the `CODE_OF_CONDUCT.md`.
