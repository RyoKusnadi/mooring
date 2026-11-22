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

const REASON: Record<string, string> = { running: 'answering now', starting: 'starting', stopped: 'stopped', needs_login: 'not signed in' }

/** Why a chat cannot take a reply now; empty when it is idle. */
export function busyReason(state: string): string {
  return state === 'idle' ? '' : (REASON[state] ?? state)
}
