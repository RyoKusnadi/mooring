// One readable line per tool call, what an edit changed, and the last lines of a command's output.
import type { ChatMessage } from './chatwire'
import { inputSummary } from './chatfmt'

interface DiffBlock {
  type?: string
  path?: string
  oldText?: string | null
  newText?: string
}

const diffsOf = (m: ChatMessage): DiffBlock[] => (m.content ?? []).filter((b): b is DiffBlock => (b as DiffBlock)?.type === 'diff')

/** A path relative to the project folder when it lies inside it, else as given. */
export function shortPath(path: string, root: string): string {
  const base = root.replace(/\/+$/, '')
  return base && path.startsWith(base + '/') ? path.slice(base.length + 1) : path
}

/** Lines added and removed between two texts, counted on the whole text. */
export function countChange(before: string, after: string): { added: number; removed: number } {
  const a = before === '' ? [] : before.replace(/\n$/, '').split('\n')
  const b = after === '' ? [] : after.replace(/\n$/, '').split('\n')
  let head = 0
  while (head < a.length && head < b.length && a[head] === b[head]) head++
  let tail = 0
  while (tail < a.length - head && tail < b.length - head && a[a.length - 1 - tail] === b[b.length - 1 - tail]) tail++
  return { added: b.length - head - tail, removed: a.length - head - tail }
}

/** The files this call edited, with the size of each change. A call with no diff but the edit kind names its locations. */
export function editsOf(m: ChatMessage, root = ''): { path: string; added: number; removed: number }[] {
  const diffs = diffsOf(m)
  if (diffs.length) return diffs.map((d) => ({ path: shortPath(d.path ?? '', root), ...countChange(d.oldText ?? '', d.newText ?? '') })).filter((e) => e.path)
  if (m.toolKind === 'edit' || m.toolKind === 'delete' || m.toolKind === 'move') {
    return (m.locations ?? []).filter((l) => l.path).map((l) => ({ path: shortPath(l.path!, root), added: 0, removed: 0 }))
  }
  return []
}

const VERB: Record<string, [string, string]> = {
  edit: ['Editing', 'Edited'],
  delete: ['Deleting', 'Deleted'],
  move: ['Moving', 'Moved'],
  read: ['Reading', 'Read'],
  search: ['Searching', 'Searched'],
  execute: ['Running', 'Ran'],
  fetch: ['Fetching', 'Fetched'],
  think: ['Thinking', 'Thought'],
}

/** "Edited web/src/a.ts +12 −3", "Ran npm test, failed", "Reading a.ts". */
export function toolLine(m: ChatMessage, root = ''): string {
  const status = m.status ?? 'pending'
  const done = status === 'completed' || status === 'failed'
  const edits = editsOf(m, root)
  let what = ''
  const verb = VERB[m.toolKind ?? '']?.[done ? 1 : 0]
  if (edits.length) {
    const e = edits[0]
    what = e.path + (e.added || e.removed ? ` +${e.added} −${e.removed}` : '')
    if (edits.length > 1) what = `${edits.length} files`
  } else if (m.toolKind === 'read' || m.toolKind === 'search') {
    what = shortPath(inputSummary(m.rawInput) || m.locations?.[0]?.path || '', root)
  } else what = inputSummary(m.rawInput)
  let line = verb && what ? `${verb} ${what}` : (m.title || verb || 'Tool call') + (what && !verb && what !== m.title ? ` ${what}` : '')
  if (status === 'failed') line += ', failed'
  return line
}

/** Colour and cursor codes, which a command prints but a chat line cannot show. */
export function stripAnsi(s: string): string {
  // eslint-disable-next-line no-control-regex
  return s.replace(/\u001b\[[0-?]*[ -/]*[@-~]|\u001b\][^\u0007\u001b]*(?:\u0007|\u001b\\)|\u001b[@-Z\\-_]/g, '').replace(/\r(?!\n)/g, '')
}

/** The status word to show beside a tool line, or '' when the line already says it ("Running make build"). */
export function stateLabel(line: string, label: string | undefined): string {
  if (!label || line.toLowerCase().startsWith(label.toLowerCase())) return ''
  return label
}

/** The last `n` non-empty lines of a command's output, and whether more came before. */
export function lastLines(text: string, n = 10): { lines: string[]; more: boolean } {
  const all = stripAnsi(text).replace(/\s+$/, '').split('\n')
  return { lines: all.slice(-n), more: all.length > n }
}

/** The text a tool call printed, from its text content blocks. */
export function outputText(m: ChatMessage): string {
  const out: string[] = []
  for (const raw of m.content ?? []) {
    const b = raw as { type?: string; content?: { type?: string; text?: string } }
    if (b?.type === 'content' && b.content?.type === 'text' && b.content.text) out.push(b.content.text)
  }
  return out.join('\n')
}

/** The distinct files edited by the calls in `list` (the messages of one turn), in the order first changed. */
export function changedFiles(list: (ChatMessage | undefined)[], root = ''): string[] {
  const seen = new Set<string>()
  for (const m of list) if (m?.kind === 'tool' && m.status !== 'failed') for (const e of editsOf(m, root)) seen.add(e.path)
  return [...seen]
}
