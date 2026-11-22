// Words for the status board.
import type { GroupKey } from './board'

export const GROUP_TITLE: Record<GroupKey, string> = {
  waiting: 'Waiting for you',
  running: 'Running',
  finished: 'Finished since you looked',
  idle: 'Idle',
}

/** The part of a row's meta line that says why a chat is not simply idle, or ''. */
export function chatState(c: { state: string; limitOnly?: boolean }, group: GroupKey): string {
  if (group === 'waiting') return c.limitOnly ? 'Usage limit reached' : 'Needs permission'
  if (c.state === 'stopped') return 'Stopped'
  if (c.state === 'needs_login') return 'Not signed in'
  if (c.state === 'starting') return 'Starting'
  return ''
}

/** The accessible name of the Board switch when it carries a count, else undefined. */
export function switchLabel(c: { waiting: number; finished: number }): string | undefined {
  const parts = [c.waiting > 0 ? `${c.waiting} waiting for you` : '', c.finished > 0 ? `${c.finished} finished since you looked` : ''].filter(Boolean)
  return parts.length ? `Board, ${parts.join(', ')}` : undefined
}
