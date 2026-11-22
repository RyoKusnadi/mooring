// The status board: every chat in every project, grouped by what it needs from the user.
import type { ChatInfo } from './chatstore.svelte'

/** What the board reads of a chat; unseen and finishedAt come from the server (chat.seen). */
export type BoardChat = Pick<ChatInfo, 'id' | 'projectId' | 'state' | 'attention' | 'updatedAt'> & {
  unseen?: boolean
  finishedAt?: number
}

export type GroupKey = 'waiting' | 'running' | 'finished' | 'idle'

export interface Group<T extends BoardChat> {
  key: GroupKey
  chats: T[]
  /** How many idle chats were left out; only the idle group is cut. */
  hidden: number
}

export const GROUP_ORDER: GroupKey[] = ['waiting', 'running', 'finished', 'idle']

export function groupOf(c: BoardChat): GroupKey {
  if (c.attention) return 'waiting'
  if (c.state === 'running' || c.state === 'starting') return 'running'
  if (c.unseen) return 'finished'
  return 'idle'
}

/**
 * Groups the chats in board order. Waiting chats come oldest first (the one that
 * has waited longest is on top); the rest come newest first. Empty groups are kept
 * out, and the idle group holds at most `idleMax` chats.
 */
export function boardGroups<T extends BoardChat>(list: T[], idleMax = 20): Group<T>[] {
  const by = new Map<GroupKey, T[]>(GROUP_ORDER.map((k) => [k, []]))
  for (const c of list) by.get(groupOf(c))!.push(c)
  const when = (c: T) => (groupOf(c) === 'finished' ? (c.finishedAt ?? c.updatedAt) : c.updatedAt)
  const out: Group<T>[] = []
  for (const key of GROUP_ORDER) {
    const chats = by.get(key)!.sort((a, b) => (key === 'waiting' ? when(a) - when(b) : when(b) - when(a)))
    if (chats.length === 0) continue
    const keep = key === 'idle' ? chats.slice(0, idleMax) : chats
    out.push({ key, chats: keep, hidden: chats.length - keep.length })
  }
  return out
}

/** The chats that finished since the user looked, for a count on the board's switch. */
export function unseenCount(list: BoardChat[]): number {
  return list.filter((c) => groupOf(c) === 'finished').length
}

/** The chats the board shows: those whose project still exists. The switch's count uses the same list. */
export function onBoard<T extends BoardChat>(list: T[], projectIds: Iterable<string>): T[] {
  const ids = new Set(projectIds)
  return list.filter((c) => ids.has(c.projectId))
}

/** How many chats wait for the user and how many finished since the user looked. */
export function boardCounts(list: BoardChat[]): { waiting: number; finished: number } {
  return { waiting: list.filter((c) => groupOf(c) === 'waiting').length, finished: unseenCount(list) }
}
