// Opening hosts as tabs: a remote terminal, a remote files tab, a remote file in the editor.
// Signing in comes first, so a host that says no leaves no dead tab behind.
import { CommandError, newCommandId } from './events'
import { focusTab, groups, newTab, openTab, replaceTab, type Layout, type Tab } from './layout'
import { baseName } from './paths'
import { ssh } from './sshstore.svelte'
import { app } from './state.svelte'
import { terms } from './termstore.svelte'

const find = (l: Layout | null, kind: string, host: string, path?: string): Tab | undefined =>
  l ? groups(l.root).flatMap((g) => g.tabs).find((t) => t.kind === kind && t.host === host && (path === undefined || t.path === path)) : undefined

/** Signs in to a host. Returns the connection id, or '' when it did not work (the screens say why). */
export async function signIn(hostId: string): Promise<string> {
  try {
    return await ssh.connect(hostId)
  } catch (e) {
    const name = ssh.host(hostId)?.name ?? 'the host'
    if (e instanceof CommandError && (e.code === 'CANCELLED' || e.code === 'HOST_KEY_REJECTED'))
      app.notice = e.code === 'HOST_KEY_REJECTED' ? `Did not connect to ${name}. You chose not to trust its key.` : `Did not connect to ${name}.`
    else if (!ssh.failure) app.notice = `Could not connect to ${name}. ` + (e instanceof Error ? e.message : '')
    return ''
  }
}

function startShell(tab: Tab, conn: string) {
  terms.open((n, a) => app.run(n, a), app.currentId, tab.id, '', conn).catch((e: unknown) => {
    app.notice = 'Could not start the remote shell. ' + (e instanceof Error ? e.message : String(e))
  })
}

const hostName = (id: string) => ssh.host(id)?.name ?? 'Remote'

export const sshActions = {
  openHosts: () => (ssh.dialog = true),

  async terminalOn(hostId: string) {
    const conn = await signIn(hostId)
    if (!conn) return
    const tab: Tab = { ...newTab(newCommandId(), 'terminal', hostName(hostId) + ' terminal'), host: hostId }
    app.edit((l) => openTab(l, tab))
    startShell(tab, conn)
  },

  /** An ended remote shell becomes a new one, signing in again when the connection is gone. */
  async reconnectTerminal(oldId: string, hostId: string) {
    const conn = await signIn(hostId)
    if (!conn) return
    const tab: Tab = { ...newTab(newCommandId(), 'terminal', hostName(hostId) + ' terminal'), host: hostId }
    app.edit((l) => replaceTab(l, oldId, tab))
    void terms.close((n, a) => app.run(n, a), oldId).catch(() => {})
    startShell(tab, conn)
  },

  async filesOn(hostId: string) {
    const existing = find(app.layout, 'remote', hostId)
    if (existing) {
      app.edit((l) => focusTab(l, existing.id))
      if (!ssh.connFor(hostId)) await signIn(hostId)
      return
    }
    if (!(await signIn(hostId))) return
    app.edit((l) => openTab(l, { ...newTab(newCommandId(), 'remote', hostName(hostId) + ' files'), host: hostId }))
  },

  openRemoteFile(hostId: string, path: string) {
    const existing = find(app.layout, 'rfile', hostId, path)
    if (existing) return app.edit((l) => focusTab(l, existing.id))
    app.edit((l) => openTab(l, { ...newTab(newCommandId(), 'rfile', `${hostName(hostId)}: ${baseName(path)}`, path), host: hostId }))
  },
}
