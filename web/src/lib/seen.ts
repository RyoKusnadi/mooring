// When to tell the server the user has looked at a chat (chat.seen).
import { groups, type Layout } from './layout'

/** The chats whose tab is the selected one in its pane; a group tab shows all its members. */
export function visibleChats(layout: Layout | null, membersOf: (groupId: string) => string[] = () => []): string[] {
  if (!layout) return []
  return groups(layout.root).flatMap((g) => g.tabs.filter((t) => t.id === g.active).flatMap((t) => (t.kind === 'chat' ? [t.id] : t.kind === 'group' ? membersOf(t.id) : [])))
}

/**
 * Which chats need a chat.seen now. `before` is what was visible at the last look: a chat that
 * just came into view is sent once, and one already in view is sent again when a reply
 * finished (unseen turned true). Chats the server does not know yet wait.
 */
export function seenToSend(before: ReadonlySet<string>, visible: string[], known: (id: string) => boolean, unseen: (id: string) => boolean): string[] {
  return visible.filter((id) => known(id) && (!before.has(id) || unseen(id)))
}
