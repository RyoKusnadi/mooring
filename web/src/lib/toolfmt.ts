// Small pure helpers for the tool server screens: the command line as shown, state labels,
// drafts from a catalog entry or a saved server, form checks and the secrets to send.
import type { CatalogEntry, CatalogKind, Secrets, ServerDraft, ToolServer, Var } from './toolapi'

export const isRemote = (k: string) => k === 'http' || k === 'sse'

/** The words for how a server runs: "npx", "uvx", "docker", "Remote (http)", "Local command". */
export function kindLabel(k: CatalogKind | ToolServer | ServerDraft): string {
  switch (k.kind) {
    case 'npm':
      return 'npx'
    case 'pypi':
      return 'uvx'
    case 'oci':
      return 'docker'
    case 'stdio':
      return 'Local command'
    default:
      return `Remote (${k.kind})`
  }
}

/** The text of a way to run a catalog entry: the command line, or the address. */
export function kindLine(k: CatalogKind): string {
  return isRemote(k.kind) ? (k.url ?? '') : [k.command, ...(k.args ?? [])].join(' ')
}

/** The line the consent screen puts first: every argument on its own, so spaces inside one stay visible. */
export const argsOf = (d: { command: string; args: string[] }): string[] => [d.command, ...d.args]

export function needs(k: CatalogKind): string {
  return k.available ? '' : 'not installed here'
}

export interface Status {
  text: string
  /** Something the user has to do next: drawn with the attention tint. */
  attention: boolean
}

/** The state words of a saved server. `failure` is the last check or enable error held in the browser. */
export function statusOf(s: ToolServer, failure = ''): Status[] {
  const out: Status[] = []
  if (!s.allowed) out.push({ text: 'Not allowed yet', attention: true })
  else if (s.enabled) out.push({ text: 'Enabled', attention: false })
  else if (failure) out.push({ text: 'Check failed', attention: true })
  else if (!s.checkedAt) out.push({ text: 'Needs a check', attention: true })
  else out.push({ text: 'Checked, not enabled', attention: false })
  if (isRemote(s.kind) && s.signIn === 'needed') out.push({ text: 'Needs sign-in', attention: true })
  if (isRemote(s.kind) && s.signIn === 'failed') out.push({ text: 'Sign-in failed', attention: true })
  if (isRemote(s.kind) && s.signIn === 'signedIn') out.push({ text: 'Signed in', attention: false })
  return out
}

export const emptyDraft = (kind: 'stdio' | 'http' = 'stdio'): ServerDraft => ({ name: '', kind, command: '', args: [], env: [], headers: [], url: '', allowPrivate: false, source: {} })

export const draftOf = (s: ToolServer): ServerDraft => ({
  id: s.id,
  name: s.name,
  kind: s.kind,
  command: s.command,
  args: [...s.args],
  env: s.env.map((v) => ({ ...v })),
  headers: s.headers.map((v) => ({ ...v })),
  url: s.url,
  allowPrivate: s.allowPrivate,
  source: { ...s.source },
})

const asVar = (v: Omit<Var, 'set'>): Var => ({ name: v.name, description: v.description, required: v.required, secret: v.secret, set: false })

/** A saved server from one way of running a catalog entry. The command line is the server's own, not built here. */
export function draftOfEntry(e: CatalogEntry, k: CatalogKind): ServerDraft {
  const remote = isRemote(k.kind)
  return {
    name: (e.title || e.name).slice(0, 64),
    kind: k.kind,
    command: remote ? '' : (k.command ?? ''),
    args: remote ? [] : [...(k.args ?? [])],
    env: remote ? [] : k.env.map(asVar),
    headers: remote ? k.headers.map(asVar) : [],
    url: remote ? (k.url ?? '') : '',
    allowPrivate: false,
    source: { registry: e.name, version: e.version },
  }
}

const NAME = /^[A-Za-z_][A-Za-z0-9_]{0,63}$/
const HEADER = /^[A-Za-z][A-Za-z0-9-]{0,63}$/

/** Splits typed arguments, one per line. Blank lines are dropped. */
export const splitArgs = (text: string): string[] => text.split('\n').map((l) => l.replace(/\r$/, '')).filter((l) => l.trim() !== '')

/** The first thing wrong with the add-your-own form, or '' when it can go to the consent screen. */
export function draftProblem(d: ServerDraft): string {
  const name = d.name.trim()
  if (!name || name.length > 64) return 'Give the server a name of 1 to 64 characters.'
  if (d.kind === 'stdio') {
    if (!d.command.trim()) return 'Enter the command to run, such as npx or /usr/local/bin/my-server.'
    if (d.args.length > 64) return 'A server takes at most 64 arguments.'
    for (const v of d.env) if (!NAME.test(v.name)) return `"${v.name}" is not a valid environment variable name. Use letters, digits and underscores.`
  } else {
    const url = d.url.trim()
    if (!/^https?:\/\/[^\s/]+/i.test(url)) return 'Enter the address starting with https://, such as https://example.com/mcp.'
    if (/^http:\/\//i.test(url) && !d.allowPrivate) return 'A plain http:// address is only allowed for a private address. Use https://, or allow a private address below.'
    for (const v of d.headers) if (!HEADER.test(v.name)) return `"${v.name}" is not a valid header name. Use letters, digits and hyphens.`
  }
  return ''
}

/** Names typed one per line into Var entries, keeping the ones already there. */
export function varsFromNames(text: string, old: Var[]): Var[] {
  const names = [...new Set(text.split(/[\s,]+/).filter(Boolean))]
  return names.map((n) => old.find((v) => v.name === n) ?? { name: n, description: '', required: false, secret: true, set: false })
}

export type Entry = { mode: 'keep' | 'replace' | 'clear'; value: string }

/**
 * The secrets to send with a save. A name left out keeps its value, "" clears it. A saved value stays
 * unless the user replaced or cleared it; a typed value for a name with nothing saved is sent as it is.
 */
export function secretsPayload(env: Var[], headers: Var[], envEntries: Record<string, Entry>, headerEntries: Record<string, Entry>): Secrets {
  const pick = (vars: Var[], entries: Record<string, Entry>) => {
    const out: Record<string, string> = {}
    for (const v of vars) {
      const e = entries[v.name]
      if (!e) continue
      if (e.mode === 'clear') out[v.name] = ''
      else if (e.mode === 'replace' || !v.set) {
        if (e.value !== '') out[v.name] = e.value
      }
    }
    return out
  }
  return { env: pick(env, envEntries), headers: pick(headers, headerEntries) }
}

/** The first required value that is neither saved nor typed. */
export function missingRequired(env: Var[], headers: Var[], envEntries: Record<string, Entry>, headerEntries: Record<string, Entry>): string {
  for (const [vars, entries] of [[env, envEntries], [headers, headerEntries]] as const) {
    for (const v of vars) {
      const e = entries[v.name]
      const has = e ? (e.mode === 'clear' ? false : e.mode === 'replace' || !v.set ? e.value !== '' : true) : v.set
      if (v.required && !has) return `Enter a value for ${v.name}, or remove it.`
    }
  }
  return ''
}

export const hasSecrets = (s: Secrets) => Object.keys(s.env).length + Object.keys(s.headers).length > 0

/** The date the catalog was made, as the server sends it (YYYY-MM-DD or a timestamp). */
export const dateOf = (d: string) => (d ? d.slice(0, 10) : '')

/** Words for the codes the tool server screens explain themselves. */
export function explain(code: string, message: string): string {
  switch (code) {
    case 'SECRETS_UNAVAILABLE':
      return 'This computer has no secret store to keep the values in, so nothing was saved. Mooring does not keep passwords or keys anywhere else. Remove the values, or set up a secret store on the computer Mooring runs on.'
    case 'PRIVATE_ADDRESS':
      return 'That address is on this machine or a private network. Go back and tick "Allow a private address" if you mean to reach it.'
    case 'BUSY':
      return 'A check of this server is already running, or four checks are running at once. Wait for one to finish, then check again.'
    case 'SIGN_IN_REQUIRED':
      // The server words these two itself; the sign-in site changing is not "sign in first".
      return /sign-in site changed/.test(message) ? message : 'This server needs you to sign in first.'
    case 'SIGN_IN_FAILED':
      if (/different sign-in site than expected/.test(message)) return message
      return 'The sign-in did not finish. It may have expired or been used already. Open the sign-in page again.'
    case 'NOT_ALLOWED':
      return 'This server has not been allowed yet. Review it and choose Allow.'
    default:
      return message
  }
}

/** A link from the catalog or the sign-in answer, only when it is http or https. */
export function safeUrl(u: string | undefined): string {
  try {
    const p = new URL(u ?? '')
    return p.protocol === 'https:' || p.protocol === 'http:' ? p.href : ''
  } catch {
    return ''
  }
}

/** The clock time a tool list was fetched, in the browser's own style, such as "1:26 AM". */
export const listedTime = (ms: number): string => new Date(ms).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })
