// Which chat tab gets something sent from a file or a terminal.
import { focusedGroup, groups, type Layout } from './layout'

/** The chat tab of the focused pane if it is one the server knows, else the first ready one, else the first. */
export function pickChatTab(l: Layout, known: (id: string) => boolean, ready: (id: string) => boolean): string | null {
  const ids = groups(l.root)
    .flatMap((g) => g.tabs)
    .filter((t) => t.kind === 'chat' && known(t.id))
    .map((t) => t.id)
  const shown = focusedGroup(l)?.active
  if (shown && ids.includes(shown)) return shown
  return ids.find(ready) ?? ids[0] ?? null
}

/** Every chat tab the server knows, in pane order. */
export function chatTabs(l: Layout, known: (id: string) => boolean): string[] {
  return groups(l.root)
    .flatMap((g) => g.tabs)
    .filter((t) => t.kind === 'chat' && known(t.id))
    .map((t) => t.id)
}
