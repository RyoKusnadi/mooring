// Plain words for the token-saving features: job errors, attachment chips, output counts.
import { CommandError } from './events'

/** What a failed commit-message job means, and the next step. */
export function suggestError(err: unknown): string {
  if (!(err instanceof CommandError)) return 'Could not write a message. Write it yourself.'
  switch (err.code) {
    case 'NOTHING_TO_COMMIT':
      return 'Nothing is staged, so there is nothing to describe. Stage files first.'
    case 'TIMEOUT':
      return 'The agent took longer than 60 seconds, so Mooring stopped it. Try again, or write the message yourself.'
    case 'LIMIT_REACHED':
      return 'The daily token limit for small jobs is used up. Raise it in Settings, or write the message yourself.'
    case 'BUSY':
      return 'Another small job is running for this project. Try again when it ends.'
    case 'BAD_REQUEST':
      return 'Commit messages are off, or no agent has a small model set. Check Settings.'
    case 'CLOSED':
      return 'The connection was closed. Sign in again.'
    default:
      return err.message || 'Could not write a message. Write it yourself.'
  }
}

/** What a failed summary means. */
export function summarizeError(err: unknown): string {
  if (err instanceof CommandError && err.code === 'BUSY') return 'The chat is busy. Wait for the reply to end, or stop it, then try again.'
  return err instanceof Error ? err.message : 'Could not write a summary.'
}

export interface Chip {
  kind: string
  path?: string
  from?: number
  to?: number
  outputId?: string
  lines?: number
  imageId?: string
  name?: string
  size?: number
}

const base = (p: string) => p.slice(p.lastIndexOf('/') + 1) || p
const plural = (n: number, one: string, many: string) => `${n.toLocaleString('en-US')} ${n === 1 ? one : many}`

/** Short chip text: a file name with its lines, or the output's line count. */
export function chipText(c: Chip): string {
  if (c.kind === 'image') return c.name ?? 'Image'
  if (c.kind === 'output') return c.lines ? `Command output, ${plural(c.lines, 'line', 'lines')}` : 'Command output'
  const range = c.from === c.to ? `line ${c.from}` : `lines ${c.from} to ${c.to}`
  return `${base(c.path ?? '')}, ${range}`
}

/** The full path and lines, for the chip's title. */
export function chipTitle(c: Chip): string {
  if (c.kind === 'image') return c.name ?? 'Image'
  if (c.kind === 'output') return 'The saved output of a command the agent ran'
  return `${c.path}, lines ${c.from} to ${c.to}`
}

export interface OutputRef {
  id: string
  lines: number
  bytes: number
  keptLines: number
  compacted: boolean
}

/** "Kept 120 of 12,000 lines" when Mooring cut the output, else "12 lines". */
export function outputCount(o: OutputRef): string {
  if (!o.compacted || o.keptLines >= o.lines) return plural(o.lines, 'line', 'lines')
  return `The agent got ${o.keptLines.toLocaleString('en-US')} of ${plural(o.lines, 'line', 'lines')}`
}

/** 1-based first and last line of a selection; a selection that ends at the start of a line stops on the line before. */
export function lineRange(doc: { lineAt(pos: number): { number: number } }, from: number, to: number): { from: number; to: number } {
  const a = doc.lineAt(from).number
  const b = doc.lineAt(to > from ? to - 1 : to).number
  return { from: a, to: Math.max(a, b) }
}

/** The cost an agent reports, rounded for reading: "$0.13", or "<$0.01" for less than a cent. */
export function costText(cost: number, currency = 'USD'): string {
  const code = /^[A-Z]{3}$/.test(currency) ? currency : 'USD'
  const fmt = (n: number) => new Intl.NumberFormat('en-US', { style: 'currency', currency: code, minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(n)
  if (cost > 0 && cost < 0.01) return '<' + fmt(0.01)
  return fmt(cost)
}
