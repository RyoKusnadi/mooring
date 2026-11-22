// HTTP half of the SSH contract. Hosts, folders,
// file reads and writes, uploads and downloads. Commands (connect, rename, delete) go over the socket.
import type { Fetcher } from './api'
import { newCommandId } from './events'
import { getJSON, putText, qs, readVia, type FileSource, type Listing, type ReadChunk } from './filesapi'

export type Auth = 'auto' | 'agent' | 'key' | 'password'

export interface Host {
  id: string
  name: string
  address: string
  port: number
  user: string
  auth: Auth
  keyPath: string
  agentForward: boolean
  root: string
  remembered: boolean
  /** Where the host's saved key is: Mooring's own file, the user's ~/.ssh/known_hosts, or nowhere. */
  savedKey: '' | 'mooring' | 'user'
}
/** What the form edits: a saved host without the fields the server owns. */
export type HostDraft = Omit<Host, 'id' | 'remembered' | 'savedKey'> & { id?: string }

export interface ConfigHost {
  name: string
  address: string
  port: number
  user: string
  keyPath: string
  agentForward: boolean
  skipped?: string
}
export interface SshConn {
  id: string
  hostId: string
  state: string
  since: number
}
export interface SshPrompt {
  promptId: string
  connId: string
  host: string
  kind: 'hostkey' | 'password' | 'passphrase' | 'question'
  text?: string
  echo: boolean
  /** True when a password or passphrase is asked for again because the last one was refused. */
  retry?: boolean
  keyType?: string
  fingerprint?: string
}
export interface Forward {
  id: string
  conn: string
  localPort: number
  remoteHost: string
  remotePort: number
}

const browserFetch: Fetcher = (input, init) => fetch(input, init)

export const fetchHosts = (f: Fetcher = browserFetch) => getJSON<{ hosts: Host[]; canRemember: boolean }>('/api/ssh/hosts', f)
export const fetchConfig = (f: Fetcher = browserFetch) => getJSON<{ hosts: ConfigHost[] }>('/api/ssh/config', f)
export const listRemote = (conn: string, path: string, f: Fetcher = browserFetch) => getJSON<Listing>(`/api/ssh/files/list?${qs({ conn, path })}`, f)
export const downloadUrl = (conn: string, path: string) => `/api/ssh/files/download?${qs({ conn, path })}`

/** Uploads one file as a new file: the write refuses when the name is taken. */
export const uploadFile = (conn: string, path: string, file: Blob, f: Fetcher = browserFetch) =>
  putText(`/api/ssh/files/write?${qs({ conn, path, base: '' })}`, file, f, newCommandId(), 'application/octet-stream')

/** An editor source for a file on a host. */
export function remoteSource(conn: string, f: Fetcher = browserFetch): FileSource {
  const read = (path: string, o: { offset?: number; limit?: number; version?: string } = {}) =>
    getJSON<ReadChunk>(`/api/ssh/files/read?${qs({ conn, path, offset: o.offset, limit: o.limit, version: o.version })}`, f)
  return {
    readChunk: read,
    readFile: (path, h) => readVia(read, path, h),
    writeFile: (path, base, text) => putText(`/api/ssh/files/write?${qs({ conn, path, base })}`, text, f),
  }
}
