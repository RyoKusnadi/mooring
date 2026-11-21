// Plain words for accounts, usage and the errors their commands give.
import type { Account, LoginState, Usage, UsageWindow } from './accountapi'

export const loginLabel: Record<LoginState, string> = {
  unknown: 'Sign-in is checked when a chat starts',
  signedIn: 'Signed in',
  needed: 'Sign-in needed',
}

const WINDOW_NAMES: Record<string, string> = {
  five_hour: '5 hours',
  seven_day: '7 days',
  seven_day_opus: '7 days, Opus',
  seven_day_sonnet: '7 days, Sonnet',
  overage: 'Extra usage',
}

export function windowName(name: string): string {
  return WINDOW_NAMES[name] ?? name.replace(/_/g, ' ')
}

/** "42%", or "unknown" when the agent did not say. Never a guess. */
export function percentText(w: UsageWindow): string {
  return w.usedPercent === null || w.usedPercent === undefined ? 'unknown' : `${Math.round(w.usedPercent)}%`
}

/** "resets at 14:05" today, "resets Mon 14:05" later, "reset time unknown" for 0. */
export function resetText(ms: number, now = Date.now()): string {
  if (!ms) return 'reset time unknown'
  const d = new Date(ms)
  const time = d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  if (ms <= now) return `reset at ${time}`
  const sameDay = d.toDateString() === new Date(now).toDateString()
  return sameDay ? `resets at ${time}` : `resets ${d.toLocaleDateString([], { weekday: 'short' })} ${time}`
}

/** True while the account's last limit still holds. */
export function limited(u: Usage, now = Date.now()): boolean {
  return u.limitHit > 0 && (u.limitResetsAt === 0 || u.limitResetsAt > now)
}

export const usageKnown = (u: Usage) => u.source === 'agent' && (u.windows?.length ?? 0) > 0

/** The account names the user told apart, with the built-in one first. */
export const sortAccounts = (list: Account[]): Account[] =>
  [...list].sort((a, b) => Number(b.id.startsWith('default:')) - Number(a.id.startsWith('default:')) || a.name.localeCompare(b.name))

export function explain(code: string, message: string): string {
  switch (code) {
    case 'BUSY':
      return 'A chat is running on this account. Stop or close it, then try again.'
    case 'ALREADY_EXISTS':
      return 'An account with this name exists for this agent. Pick another name.'
    case 'LIMIT_REACHED':
      return 'This agent already has 20 accounts. Remove one first.'
    case 'CLOSED':
      return 'Mooring is not connected. Try again when it is.'
    default:
      return message || 'That did not work. Try again.'
  }
}
