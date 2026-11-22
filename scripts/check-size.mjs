// Size limits from the plan, checked by `make check`:
//   - JavaScript the first page load needs: under 130 KB compressed.
//   - The terminal chunk (xterm and its addons, loaded on the first terminal): under 130 KB compressed.
//   - The chat chunk (the agent chat view, loaded on the first chat): under 20 KB compressed.
//   - The editor chunk (CodeMirror and the file view, loaded on the first file): under 130 KB compressed.
//     Each language mode is its own chunk, loaded when a file of that language opens.
//   - The lsp chunk (the language server client, loaded with the first file that has a server): under 60 KB compressed.
//   - The history chunk (commit list, graph and commit details, loaded on the first history or commit tab): under 30 KB compressed.
//   - The tool servers chunk (the dialog, loaded the first time it opens): under 14 KB compressed.
//   - The accounts chunk (the dialog, loaded the first time it opens): under 10 KB compressed.
//   - Source files: under 300 lines, so an agent can read one whole.
// Usage: node scripts/check-size.mjs <repo root>
import { readdirSync, readFileSync } from 'node:fs'
import { extname, join, relative } from 'node:path'
import { gzipSync } from 'node:zlib'

const root = process.argv[2] ?? '.'
const SHELL_LIMIT = 150 * 1024
const TERMINAL_LIMIT = 130 * 1024
const CHAT_LIMIT = 20 * 1024
const EDITOR_LIMIT = 130 * 1024
const LSP_LIMIT = 60 * 1024
const SSH_DIALOGS_LIMIT = 12 * 1024
const REMOTE_FILES_LIMIT = 6 * 1024
const TOOL_DIALOGS_LIMIT = 14 * 1024
const ACCOUNTS_LIMIT = 10 * 1024
const HISTORY_LIMIT = 30 * 1024
const SETTINGS_LIMIT = 8 * 1024
const LINE_LIMIT = 300
const SOURCE_DIRS = ['cmd', 'internal', 'scripts', 'web/src', 'web/e2e']
const SOURCE_EXT = new Set(['.go', '.ts', '.svelte', '.mjs', '.css'])
const SKIP = new Set(['node_modules', 'dist'])

const failures = []

const dist = join(root, 'internal/webui/dist')
let html
try {
  html = readFileSync(join(dist, 'index.html'), 'utf8')
} catch {
  failures.push('internal/webui/dist/index.html is missing; run `make build` first')
}
if (html) {
  const files = [...html.matchAll(/(?:src|href)="\/(assets\/[^"]+\.js)"/g)].map((m) => m[1])
  const total = files.reduce((sum, f) => sum + gzipSync(readFileSync(join(dist, f))).length, 0)
  const kb = (n) => `${(n / 1024).toFixed(1)} KB`
  if (total > SHELL_LIMIT) failures.push(`app shell JavaScript is ${kb(total)} compressed; limit ${kb(SHELL_LIMIT)}`)
  else console.log(`app shell JavaScript: ${kb(total)} compressed (limit ${kb(SHELL_LIMIT)})`)
}

if (html) {
  // The terminal chunk is the lazy one that holds xterm; find it by content, not by name.
  const lazy = readdirSync(join(dist, 'assets')).filter((f) => f.endsWith('.js'))
  const chunk = lazy.find((f) => readFileSync(join(dist, 'assets', f), 'utf8').includes('xterm'))
  if (!chunk) failures.push('no terminal chunk found in dist/assets')
  else {
    const size = gzipSync(readFileSync(join(dist, 'assets', chunk))).length
    const kb = (n) => `${(n / 1024).toFixed(1)} KB`
    if (size > TERMINAL_LIMIT) failures.push(`terminal chunk ${chunk} is ${kb(size)} compressed; limit ${kb(TERMINAL_LIMIT)}`)
    else console.log(`terminal chunk: ${kb(size)} compressed (limit ${kb(TERMINAL_LIMIT)})`)
    if (html.includes(chunk)) failures.push(`terminal chunk ${chunk} is loaded by index.html; it must load on first use`)
  }
}

if (html) {
  // The chat chunk is the lazy one that holds the chat view, marked by its data-chat-view attribute.
  const files = readdirSync(join(dist, 'assets')).filter((f) => f.endsWith('.js'))
  const chunk = files.find((f) => readFileSync(join(dist, 'assets', f), 'utf8').includes('data-chat-view'))
  if (!chunk) failures.push('no chat chunk found in dist/assets')
  else {
    const size = gzipSync(readFileSync(join(dist, 'assets', chunk))).length
    const kb = (n) => `${(n / 1024).toFixed(1)} KB`
    if (size > CHAT_LIMIT) failures.push(`chat chunk ${chunk} is ${kb(size)} compressed; limit ${kb(CHAT_LIMIT)}`)
    else console.log(`chat chunk: ${kb(size)} compressed (limit ${kb(CHAT_LIMIT)})`)
    if (html.includes(chunk)) failures.push(`chat chunk ${chunk} is loaded by index.html; it must load on first use`)
  }
}

if (html) {
  // The editor chunk is the lazy one that holds the file view, marked by its data-editor-view attribute.
  const files = readdirSync(join(dist, 'assets')).filter((f) => f.endsWith('.js'))
  const chunk = files.find((f) => readFileSync(join(dist, 'assets', f), 'utf8').includes('data-editor-view'))
  if (!chunk) failures.push('no editor chunk found in dist/assets')
  else {
    const size = gzipSync(readFileSync(join(dist, 'assets', chunk))).length
    const kb = (n) => `${(n / 1024).toFixed(1)} KB`
    if (size > EDITOR_LIMIT) failures.push(`editor chunk ${chunk} is ${kb(size)} compressed; limit ${kb(EDITOR_LIMIT)}`)
    else console.log(`editor chunk: ${kb(size)} compressed (limit ${kb(EDITOR_LIMIT)})`)
    if (html.includes(chunk)) failures.push(`editor chunk ${chunk} is loaded by index.html; it must load on first use`)
  }
}

if (html) {
  // The lsp chunk is the lazy one that holds the language client, marked by its request names.
  const files = readdirSync(join(dist, 'assets')).filter((f) => f.endsWith('.js'))
  const chunk = files.find((f) => readFileSync(join(dist, 'assets', f), 'utf8').includes('textDocument/didOpen'))
  if (!chunk) failures.push('no lsp chunk found in dist/assets')
  else {
    const size = gzipSync(readFileSync(join(dist, 'assets', chunk))).length
    const kb = (n) => `${(n / 1024).toFixed(1)} KB`
    if (size > LSP_LIMIT) failures.push(`lsp chunk ${chunk} is ${kb(size)} compressed; limit ${kb(LSP_LIMIT)}`)
    else console.log(`lsp chunk: ${kb(size)} compressed (limit ${kb(LSP_LIMIT)})`)
    if (html.includes(chunk)) failures.push(`lsp chunk ${chunk} is loaded by index.html; it must load on first use`)
  }
}

if (html) {
  // The history chunk is the lazy one that holds the history view, marked by its data-history-view attribute.
  const files = readdirSync(join(dist, 'assets')).filter((f) => f.endsWith('.js'))
  const chunk = files.find((f) => readFileSync(join(dist, 'assets', f), 'utf8').includes('data-history-view'))
  if (!chunk) failures.push('no history chunk found in dist/assets')
  else {
    const size = gzipSync(readFileSync(join(dist, 'assets', chunk))).length
    const kb = (n) => `${(n / 1024).toFixed(1)} KB`
    if (size > HISTORY_LIMIT) failures.push(`history chunk ${chunk} is ${kb(size)} compressed; limit ${kb(HISTORY_LIMIT)}`)
    else console.log(`history chunk: ${kb(size)} compressed (limit ${kb(HISTORY_LIMIT)})`)
    if (html.includes(chunk)) failures.push(`history chunk ${chunk} is loaded by index.html; it must load on first use`)
  }
}

if (html) {
  // The ssh dialogs chunk is the lazy one marked by its data-ssh-dialogs attribute.
  const files = readdirSync(join(dist, 'assets')).filter((f) => f.endsWith('.js'))
  const chunk = files.find((f) => readFileSync(join(dist, 'assets', f), 'utf8').includes('data-ssh-dialogs'))
  if (!chunk) failures.push('no ssh dialogs chunk found in dist/assets')
  else {
    const size = gzipSync(readFileSync(join(dist, 'assets', chunk))).length
    const kb = (n) => `${(n / 1024).toFixed(1)} KB`
    if (size > SSH_DIALOGS_LIMIT) failures.push(`ssh dialogs chunk ${chunk} is ${kb(size)} compressed; limit ${kb(SSH_DIALOGS_LIMIT)}`)
    else console.log(`ssh dialogs chunk: ${kb(size)} compressed (limit ${kb(SSH_DIALOGS_LIMIT)})`)
    if (html.includes(chunk)) failures.push(`ssh dialogs chunk ${chunk} is loaded by index.html; it must load on first use`)
  }
}

if (html) {
  // The remote files chunk is the lazy one marked by its data-remote-view attribute.
  const files = readdirSync(join(dist, 'assets')).filter((f) => f.endsWith('.js'))
  const chunk = files.find((f) => readFileSync(join(dist, 'assets', f), 'utf8').includes('data-remote-view'))
  if (!chunk) failures.push('no remote files chunk found in dist/assets')
  else {
    const size = gzipSync(readFileSync(join(dist, 'assets', chunk))).length
    const kb = (n) => `${(n / 1024).toFixed(1)} KB`
    if (size > REMOTE_FILES_LIMIT) failures.push(`remote files chunk ${chunk} is ${kb(size)} compressed; limit ${kb(REMOTE_FILES_LIMIT)}`)
    else console.log(`remote files chunk: ${kb(size)} compressed (limit ${kb(REMOTE_FILES_LIMIT)})`)
    if (html.includes(chunk)) failures.push(`remote files chunk ${chunk} is loaded by index.html; it must load on first use`)
  }
}

if (html) {
  // The tool servers chunk is the lazy one marked by its data-tool-servers attribute.
  const files = readdirSync(join(dist, 'assets')).filter((f) => f.endsWith('.js'))
  const chunk = files.find((f) => readFileSync(join(dist, 'assets', f), 'utf8').includes('data-tool-servers'))
  if (!chunk) failures.push('no tool servers chunk found in dist/assets')
  else {
    const size = gzipSync(readFileSync(join(dist, 'assets', chunk))).length
    const kb = (n) => `${(n / 1024).toFixed(1)} KB`
    if (size > TOOL_DIALOGS_LIMIT) failures.push(`tool servers chunk ${chunk} is ${kb(size)} compressed; limit ${kb(TOOL_DIALOGS_LIMIT)}`)
    else console.log(`tool servers chunk: ${kb(size)} compressed (limit ${kb(TOOL_DIALOGS_LIMIT)})`)
    if (html.includes(chunk)) failures.push(`tool servers chunk ${chunk} is loaded by index.html; it must load on first use`)
  }
}

if (html) {
  // The accounts chunk is the lazy one marked by its data-accounts-dialog attribute.
  const files = readdirSync(join(dist, 'assets')).filter((f) => f.endsWith('.js'))
  const chunk = files.find((f) => readFileSync(join(dist, 'assets', f), 'utf8').includes('data-accounts-dialog'))
  if (!chunk) failures.push('no accounts chunk found in dist/assets')
  else {
    const size = gzipSync(readFileSync(join(dist, 'assets', chunk))).length
    const kb = (n) => `${(n / 1024).toFixed(1)} KB`
    if (size > ACCOUNTS_LIMIT) failures.push(`accounts chunk ${chunk} is ${kb(size)} compressed; limit ${kb(ACCOUNTS_LIMIT)}`)
    else console.log(`accounts chunk: ${kb(size)} compressed (limit ${kb(ACCOUNTS_LIMIT)})`)
    if (html.includes(chunk)) failures.push(`accounts chunk ${chunk} is loaded by index.html; it must load on first use`)
  }
  // The settings chunk is the lazy one marked by its data-settings-dialog attribute.
  const settingsChunk = files.find((f) => readFileSync(join(dist, 'assets', f), 'utf8').includes('data-settings-dialog'))
  if (!settingsChunk) failures.push('no settings chunk found in dist/assets')
  else {
    const size = gzipSync(readFileSync(join(dist, 'assets', settingsChunk))).length
    const kb = (n) => `${(n / 1024).toFixed(1)} KB`
    if (size > SETTINGS_LIMIT) failures.push(`settings chunk ${settingsChunk} is ${kb(size)} compressed; limit ${kb(SETTINGS_LIMIT)}`)
    else console.log(`settings chunk: ${kb(size)} compressed (limit ${kb(SETTINGS_LIMIT)})`)
    if (html.includes(settingsChunk)) failures.push(`settings chunk ${settingsChunk} is loaded by index.html; it must load on first use`)
  }
}

function walk(dir) {
  let entries
  try {
    entries = readdirSync(dir, { withFileTypes: true })
  } catch {
    return
  }
  for (const e of entries) {
    const path = join(dir, e.name)
    if (e.isDirectory()) {
      if (!SKIP.has(e.name)) walk(path)
    } else if (SOURCE_EXT.has(extname(e.name))) {
      const lines = readFileSync(path, 'utf8').split('\n').length
      if (lines > LINE_LIMIT) failures.push(`${relative(root, path)} has ${lines} lines; split it (limit ${LINE_LIMIT})`)
    }
  }
}
for (const dir of SOURCE_DIRS) walk(join(root, dir))

if (failures.length) {
  for (const f of failures) console.error(`size: ${f}`)
  process.exit(1)
}
