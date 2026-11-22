// What the status bar says about language servers: an install in progress, a socket that is reconnecting.
export interface StatusServer {
  id: string
  name: string
  state: string
}

export function lspStatus(servers: StatusServer[], progress: Record<string, string>, conn: Record<string, string>): string {
  const installing = servers.filter((s) => s.state === 'installing')
  if (installing.length > 0) {
    const s = installing[0]
    const line = progress[s.id]?.trim()
    return `Installing ${s.name}…${line ? ' ' + line : ''}`
  }
  return Object.values(conn).includes('waiting') ? 'Reconnecting to the language server…' : ''
}
