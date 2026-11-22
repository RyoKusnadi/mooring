// Which language server a file belongs to, and the plain-text helpers around it: file URIs,
// places, the custom-server form, the restart notice. No editor code, so it stays in the app shell.

export interface LspServer {
  id: string
  name: string
  endings: string[]
  languageId: string
  state: 'ready' | 'needs_consent' | 'not_installed' | 'installing'
  command: string[]
  path: string
  installable: boolean
  installHint: string
  custom: boolean
}

export interface LspRunning {
  projectId: string
  folder: string
  id: string
  pid: number
  mb: number
  clients: number
  startedAt: string | number
}

/** Files over this size are not sent to a server (the contract's limit). */
export const MAX_LSP_BYTES = 2 * 1024 * 1024

const WORKTREE = /^\.mooring\/worktrees\/[^/]+(?=\/)/

/** The place a project file belongs to: "" or a chat's worktree folder, and the path inside it. */
export function splitPlace(path: string): { place: string; rel: string } {
  const m = WORKTREE.exec(path)
  return m ? { place: m[0], rel: path.slice(m[0].length + 1) } : { place: '', rel: path }
}

/** A custom server wins over a built-in one; then the longest matching ending wins. */
export function serverFor(servers: readonly LspServer[], path: string): LspServer | null {
  const name = path.slice(path.lastIndexOf('/') + 1).toLowerCase()
  let best: LspServer | null = null
  let bestLen = 0
  for (const s of servers) {
    for (const e of s.endings) {
      const ending = e.toLowerCase()
      if (!name.endsWith(ending) || name.length === ending.length) continue
      const better = ending.length > bestLen || (ending.length === bestLen && s.custom && !best?.custom)
      if (better) [best, bestLen] = [s, ending.length]
    }
  }
  return best
}

const TS_IDS: Record<string, string> = { tsx: 'typescriptreact', js: 'javascript', mjs: 'javascript', cjs: 'javascript', jsx: 'javascriptreact' }

/** The server's language id; the typescript server wants a different one for .tsx, .js and .jsx. */
export function languageIdFor(server: LspServer, path: string): string {
  if (server.languageId !== 'typescript') return server.languageId
  const ext = path.slice(path.lastIndexOf('.') + 1).toLowerCase()
  return TS_IDS[ext] ?? 'typescript'
}

// File URIs: written the way Go's url package writes them, which is what language servers send back.
const hex = (c: string) => '%' + c.charCodeAt(0).toString(16).toUpperCase().padStart(2, '0')
const encodeSegment = (s: string) => encodeURIComponent(s).replace(/[!'()*]/g, hex).replace(/%(24|26|2B|2C|3A|3B|3D|40)/g, (_, h) => String.fromCharCode(parseInt(h, 16)))
const encodePath = (p: string) => p.split('/').map(encodeSegment).join('/')

const trimSlash = (p: string) => (p.length > 1 ? p.replace(/\/+$/, '') : p)

/** `file://` + the project's folder + the project path of a file (which already holds its place). */
export function fileUri(projectPath: string, path: string): string {
  const base = trimSlash(projectPath)
  return 'file://' + encodePath(path ? `${base === '/' ? '' : base}/${path}` : base)
}

/** The project path of a `file://` URI, or null when it points outside the project. */
export function pathFromUri(projectPath: string, uri: string): string | null {
  const prefix = fileUri(projectPath, '') + '/'
  if (!uri.startsWith(prefix)) return null
  try {
    return decodeURIComponent(uri.slice(prefix.length))
  } catch {
    return null
  }
}

/** "go test -v" or `my-server --root "a b"` to an argument list. Quotes group words; there are no escapes. */
export function splitCommand(text: string): string[] {
  const out: string[] = []
  const re = /"([^"]*)"|'([^']*)'|(\S+)/g
  for (let m = re.exec(text); m; m = re.exec(text)) out.push(m[1] ?? m[2] ?? m[3])
  return out
}

/** ".rs, .rlib" or ".rs .rlib" to [".rs", ".rlib"]; a missing dot is added. */
export function parseEndings(text: string): string[] {
  return [...new Set(text.split(/[\s,]+/).filter(Boolean).map((e) => (e.startsWith('.') ? e : '.' + e).toLowerCase()))]
}

/** A custom server's id: 1 to 32 characters of a-z, 0-9 and "-". Empty when the name has none. */
export function slugId(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 32).replace(/-+$/, '')
}

export function formatMb(mb: number): string {
  if (mb < 1024) return `${Math.round(mb)} MB`
  return `${(mb / 1024).toFixed(1).replace(/\.0$/, '')} GB`
}

export function restartNotice(name: string, mb: number, limitMb: number): string {
  return `${name} used ${formatMb(mb)}, over its ${formatMb(limitMb)} limit, and was restarted.`
}

/** What one running server is called in lists: its place, or "whole project". */
export const placeLabel = (folder: string) => (folder ? folder.replace(/^\.mooring\/worktrees\//, 'worktree ') : 'whole project')

/** Shift-Alt-F by physical key, so it formats whatever character the layout types. */
export function isFormatKey(e: Pick<KeyboardEvent, 'code' | 'shiftKey' | 'altKey' | 'ctrlKey' | 'metaKey'>): boolean {
  return e.code === 'KeyF' && e.shiftKey && e.altKey && !e.ctrlKey && !e.metaKey
}
