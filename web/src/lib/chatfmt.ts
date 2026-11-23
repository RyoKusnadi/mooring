// Small formatting helpers for chat rows. Pure, so they are easy to test.

export const TOOL_STATUS: Record<string, string> = {
  pending: 'Waiting',
  in_progress: 'Running',
  completed: 'Done',
  failed: 'Failed',
}

export const PLAN_STATUS: Record<string, string> = {
  pending: 'To do',
  in_progress: 'In progress',
  completed: 'Done',
}

/** The one line that says what a tool was asked to do: its command or path, else nothing. */
export function inputSummary(raw: unknown): string {
  if (typeof raw === 'string') return raw
  if (typeof raw !== 'object' || raw === null) return ''
  const o = raw as Record<string, unknown>
  for (const k of ['command', 'cmd', 'file_path', 'path', 'pattern', 'url', 'query']) {
    const v = o[k]
    if (typeof v === 'string' && v) return v
    if (Array.isArray(v) && v.every((x) => typeof x === 'string')) return v.join(' ')
  }
  return ''
}

export interface Block {
  kind: 'text' | 'diff'
  text?: string
  path?: string
  lines?: { sign: ' ' | '+' | '-'; text: string }[]
}

/** The agent's content blocks as things the page can draw. Unknown blocks are skipped. */
export function blocks(content: unknown[] | undefined): Block[] {
  const out: Block[] = []
  for (const raw of content ?? []) {
    const b = raw as { type?: string; content?: { type?: string; text?: string }; path?: string; oldText?: string | null; newText?: string }
    if (b?.type === 'diff') out.push({ kind: 'diff', path: b.path, lines: diffLines(b.oldText ?? '', b.newText ?? '') })
    else if (b?.type === 'content' && b.content?.type === 'text' && b.content.text) out.push({ kind: 'text', text: b.content.text })
  }
  return out
}

const CONTEXT = 2
const MAX_LINES = 60

/** Lines that differ, after the common start and end are cut, with a little context around them. */
export function diffLines(before: string, after: string): { sign: ' ' | '+' | '-'; text: string }[] {
  const a = before === '' ? [] : before.split('\n')
  const b = after === '' ? [] : after.split('\n')
  let head = 0
  while (head < a.length && head < b.length && a[head] === b[head]) head++
  let tail = 0
  while (tail < a.length - head && tail < b.length - head && a[a.length - 1 - tail] === b[b.length - 1 - tail]) tail++
  const out: { sign: ' ' | '+' | '-'; text: string }[] = []
  for (const t of a.slice(Math.max(0, head - CONTEXT), head)) out.push({ sign: ' ', text: t })
  for (const t of a.slice(head, a.length - tail)) out.push({ sign: '-', text: t })
  for (const t of b.slice(head, b.length - tail)) out.push({ sign: '+', text: t })
  for (const t of a.slice(a.length - tail, a.length - tail + CONTEXT)) out.push({ sign: ' ', text: t })
  return out.slice(0, MAX_LINES)
}

/** "1.2k" and "200k": token counts at the size a meter needs. */
export function tokens(n: number): string {
  if (n < 1000) return String(n)
  if (n < 100_000) return `${(n / 1000).toFixed(1).replace(/\.0$/, '')}k`
  if (n < 1_000_000) return `${Math.round(n / 1000)}k`
  return `${(n / 1_000_000).toFixed(1).replace(/\.0$/, '')}M`
}

/** The login command in an agent's catalog note, as in "Sign in with `claude auth login`." */
export function loginCommand(note: string | undefined): string {
  // One line only: it is typed into a terminal, so a line break or control character must not get in.
  return (/[Ss]ign in with `([^`]+)`/.exec(note ?? '')?.[1] ?? '').replace(/[\u0000-\u001f\u007f-\u009f\u2028\u2029]+/g, ' ').trim()
}

export interface Span {
  code: boolean
  text: string
}

/** Splits text on `backticks` into plain and code spans. */
export function inline(text: string): Span[] {
  return text
    .split(/`([^`\n]+)`/)
    .map((t, i): Span => ({ code: i % 2 === 1, text: t }))
    .filter((s) => s.text !== '')
}

export type Prose = { kind: 'code'; text: string } | { kind: 'text'; spans: Span[] }

/** Agent text as paragraphs and code fences. Other markdown stays as typed. An open fence, as in a reply still streaming, counts as code. */
export function prose(text: string): Prose[] {
  const out: Prose[] = []
  const parts = text.split(/^```[^\n]*\n?/m)
  parts.forEach((part, i) => {
    if (i % 2 === 1) out.push({ kind: 'code', text: part.replace(/\n$/, '') })
    else if (part.trim() !== '') for (const p of part.split(/\n{2,}/)) if (p.trim()) out.push({ kind: 'text', spans: inline(p.replace(/^\n+|\n+$/g, '')) })
  })
  return out
}

/** What an answered permission row says. It does not matter which window answered. */
export function answerText(state: string | undefined, option: { name: string; kind: string } | undefined): string {
  if (state === 'cancelled') return 'Cancelled'
  if (!option) return 'Answered'
  if (option.kind.startsWith('allow')) return option.kind === 'allow_always' ? 'You allowed always' : 'You allowed'
  if (option.kind.startsWith('reject')) return option.kind === 'reject_always' ? 'You refused always' : 'You refused'
  return `You chose ${option.name}`
}

/** Modes known to ask before acting. Any other mode, or one nobody can name, gets a confirm. */
export function asksFirst(mode: { id: string; name: string } | undefined): boolean {
  return !!mode && /^(default|manual|ask|plan|read.?only|suggest)/i.test(mode.id + ' ' + mode.name) && !/bypass|auto|yolo|dont|accept/i.test(mode.id + ' ' + mode.name)
}

type Waiting = { attention: boolean; limitOnly?: boolean }

/** The marker label for a chat that waits on the user. */
export function attentionLabel(c: Waiting): string {
  return c.limitOnly ? 'Usage limit reached' : 'Needs permission'
}

/** The marker label for a project row: a waiting permission wins over a limit. */
export function projectAttentionLabel(list: Waiting[]): string {
  return list.some((c) => c.attention && !c.limitOnly) ? 'Needs permission' : 'Usage limit reached'
}

/** The status bar text for the chats that wait on the user. */
export function waitingText(list: Waiting[]): string {
  const waiting = list.filter((c) => c.attention)
  const limits = waiting.filter((c) => c.limitOnly).length
  const n = waiting.length
  if (limits === 0) return n === 1 ? '1 chat needs permission' : `${n} chats need permission`
  if (limits === n) return n === 1 ? '1 chat stopped at a usage limit' : `${n} chats stopped at a usage limit`
  return `${n} chats need you`
}

/** A chat can be summarised once it has messages and is not starting or replying: an idle chat asks its agent, any other is summarised from its messages. */
export function canSummarize(c: { state: string; messages: number }): boolean {
  return c.messages > 0 && c.state !== 'starting' && c.state !== 'running'
}

/** Why a chat cannot take a message right now, or '' when it can (or is replying). */
export function composerWhy(state: string): string {
  if (state === 'starting') return 'The agent is starting. Send turns on when it is ready.'
  if (state === 'needs_login') return 'Sign in first, then resume the chat and send.'
  if (state === 'stopped') return 'The agent stopped. Start a new chat to keep going.'
  return ''
}

/** The line under the box about what the box does now: an upload, a wait, a start, a reply or a stop. */
export function composerHint(state: string, agent: string, uploading: number, waiting: boolean): string {
  if (uploading) return `Uploading ${uploading === 1 ? 'an image' : uploading + ' images'}…`
  if (waiting) return 'Waiting to send. The message goes out when Mooring reconnects.'
  if (state === 'starting') return `Starting ${agent}…`
  if (state === 'running') return `${agent} is replying. A message you send now goes out when the reply ends. Queued messages stay if the page reloads, but their images and other attachments do not.`
  return state === 'idle' || state === 'needs_login' ? '' : `${agent} is not running. Resume the chat to send a message.`
}
