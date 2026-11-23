// Small rules for sending a reply to another chat.
import type { ChatMessage } from './chatwire'

type Row = Pick<ChatMessage, 'id' | 'kind' | 'text'>

/** The id of the last agent reply with text in the turn that ends at row `from`; a turn starts after a message of the user. */
export function lastReplyId(get: (i: number) => Row | undefined, from: number): number | null {
  for (let i = from; i >= 0; i--) {
    const m = get(i)
    if (m?.kind === 'user') return null
    if (m?.kind === 'agent' && m.text?.trim()) return m.id
  }
  return null
}

const REASON: Record<string, string> = { running: 'replying', starting: 'starting', needs_login: 'not signed in' }

/** Whether a reply can go to a chat in this state: an idle chat at once, a stopped chat after Mooring starts it again. */
export function canTake(state: string): boolean {
  return state === 'idle' || state === 'stopped'
}

/** Why a chat cannot take a reply now; empty when it can. */
export function busyReason(state: string): string {
  return canTake(state) ? '' : (REASON[state] ?? state)
}

type Resumable = { state: string; messages?: number; caps?: Record<string, boolean | string[]> | null }

/** What happens to a stopped chat on Send; empty for any other state. A new session only when the agent is known to lack resume and load. */
export function startNote(c: Resumable): string {
  if (c.state !== 'stopped') return ''
  const known = !!c.caps && 'load' in c.caps && 'resume' in c.caps
  if (known && !c.caps!.load && !c.caps!.resume && (c.messages ?? 0) > 0) return 'Starts a new session without the earlier messages'
  return 'Starts again when you send'
}
