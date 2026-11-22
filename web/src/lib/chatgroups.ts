// Chats worked on together: 2 or 3 chats of one project share a groupId (set by the server).
import type { ChatInfo } from './chatstore.svelte'

export const MAX_GROUP = 3

export interface GroupRow {
  id: string
  title: string
  members: ChatInfo[]
  /** Any member waits for the user. */
  attention: boolean
}
export type ChatRow = { group: GroupRow } | { chat: ChatInfo }

const nameOf = (c: ChatInfo) => c.title || 'New chat'

/** The group's own title, or "Group: a, b" from its members. */
export function groupTitle(members: ChatInfo[]): string {
  const own = members.find((m) => m.groupTitle)?.groupTitle
  return own || 'Group: ' + members.map(nameOf).join(', ')
}

/** The list in its order, with a group's chats gathered under one row at the place of its newest chat. */
export function chatRows(list: ChatInfo[]): ChatRow[] {
  const rows: ChatRow[] = []
  const seen = new Set<string>()
  for (const c of list) {
    if (!c.groupId) {
      rows.push({ chat: c })
      continue
    }
    if (seen.has(c.groupId)) continue
    seen.add(c.groupId)
    const members = list.filter((m) => m.groupId === c.groupId)
    rows.push({ group: { id: c.groupId, title: groupTitle(members), members, attention: members.some((m) => m.attention) } })
  }
  return rows
}

/** The members of a group, oldest first; empty when the group is gone. */
export function membersOf(all: Record<string, ChatInfo>, groupId: string): ChatInfo[] {
  return Object.values(all)
    .filter((c) => c.groupId === groupId)
    .sort((a, b) => (a.createdAt ?? 0) - (b.createdAt ?? 0) || (a.id < b.id ? -1 : 1))
}

/** Chats a chat could join: same project, not in a group yet, and not itself. */
export function groupCandidates(all: Record<string, ChatInfo>, chat: ChatInfo): ChatInfo[] {
  return Object.values(all)
    .filter((c) => c.projectId === chat.projectId && c.id !== chat.id && !c.groupId)
    .sort((a, b) => b.updatedAt - a.updatedAt)
}

type Run = <T>(name: string, args: unknown) => Promise<T>

export async function makeGroup(run: Run, ids: string[], title: string): Promise<string> {
  const r = await run<{ groupId: string }>('chat.group', { ids, ...(title.trim() ? { title: title.trim() } : {}) })
  return r.groupId
}

export async function leaveGroup(run: Run, id: string): Promise<void> {
  await run('chat.ungroup', { id })
}
