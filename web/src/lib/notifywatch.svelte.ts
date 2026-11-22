// Watches chats and terminals that need the user: shows a system notification while the tab is
// hidden, and clears a terminal bell right away when the user turned bells off. Installed from App.svelte.
import { app } from './state.svelte'
import { chats } from './chatstore.svelte'
import { terms } from './termstore.svelte'
import { bellCounts, browserNotifier, type Needing } from './notify'

export function watchAttention(): () => void {
  return $effect.root(() => {
    const sync = browserNotifier()
    const project = (id: string) => app.projects.find((p) => p.id === id)?.name ?? ''

    $effect(() => {
      const items: Needing[] = []
      for (const c of Object.values(chats.list)) if (c.attention) items.push({ key: `c:${c.id}`, kind: 'chat', project: project(c.projectId) })
      for (const t of Object.values(terms.list)) if (terms.needs(t.id)) items.push({ key: `t:${t.id}`, kind: 'terminal', project: project(t.projectId) })
      sync(items, terms.known)
    })

    // With bells off, a bell-only mark is cleared at the server; a real signal is never cleared this way.
    $effect(() => {
      if (bellCounts()) return
      for (const t of Object.values(terms.list)) if (t.attention && t.bell) void app.run('terminal.seen', { id: t.id, bellOnly: true }).catch(() => {})
    })
  })
}
