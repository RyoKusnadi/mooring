// HTTP half of the tool server contract.
// Commands (save, check, sign in) go over the socket; the list and the catalog are read here.
import type { Fetcher } from './api'
import { getJSON, qs } from './filesapi'

/** An environment variable or header a server takes. `set` says a value is saved; the value never comes back. */
export interface Var {
  name: string
  description: string
  required: boolean
  secret: boolean
  set: boolean
}

export interface Tool {
  name: string
  description: string
}

export type Transport = 'stdio' | 'http' | 'sse' | 'npm' | 'pypi' | 'oci'

export interface ToolServer {
  id: string
  name: string
  kind: Transport
  command: string
  args: string[]
  env: Var[]
  headers: Var[]
  url: string
  allowPrivate: boolean
  enabled: boolean
  allowed: boolean
  source: { registry?: string; version?: string }
  signIn: '' | 'needed' | 'signedIn' | 'failed'
  expires?: number
  checkedAt: number
  /** Why a sign-in is needed, when more specific than the usual reason. */
  note?: string
  /** The last check was answered from a saved list. */
  cached?: boolean
  tools: Tool[]
}

/** What the forms edit: a server without the fields the server owns. */
export type ServerDraft = Omit<ToolServer, 'id' | 'enabled' | 'allowed' | 'signIn' | 'expires' | 'checkedAt' | 'tools' | 'note' | 'cached'> & { id?: string }

export interface CatalogKind {
  kind: Transport
  package?: string
  version?: string
  url?: string
  command?: string
  args?: string[]
  env: Omit<Var, 'set'>[]
  headers: Omit<Var, 'set'>[]
  available: boolean
  missing?: string
}

export interface CatalogEntry {
  name: string
  title: string
  description: string
  version: string
  website: string
  repository: string
  kinds: CatalogKind[]
}

export interface CatalogPage {
  entries: CatalogEntry[]
  next: number | null
  date: string
}

export interface Secrets {
  env: Record<string, string>
  headers: Record<string, string>
}

const browserFetch: Fetcher = (input, init) => fetch(input, init)

export const fetchToolServers = (f: Fetcher = browserFetch) => getJSON<{ servers: ToolServer[]; canSaveSecrets: boolean }>('/api/toolservers', f)

export const fetchCatalog = (q: string, cursor: number | null, f: Fetcher = browserFetch, signal?: AbortSignal) =>
  getJSON<CatalogPage>(`/api/toolservers/catalog?${qs({ q, cursor: cursor ?? undefined })}`, f, signal)
