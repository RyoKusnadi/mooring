// Replace across files: the project search finds the files; each file's whole text is matched
// again here, because the search clips long lines. The edits go through multiedit.ts.
import type { FileEdits, LspTextEdit } from './lspedits'

export interface ReplaceOptions {
  q: string
  regex?: boolean
  matchCase?: boolean
}

/** One changed line, for the preview. */
export interface LineChange {
  line: number
  before: string
  after: string
}

export interface FilePlan extends FileEdits {
  lines: LineChange[]
}

/** The search as a browser regular expression; regex mode reports what the browser cannot read. */
export function matcher(o: ReplaceOptions): RegExp | { error: string } {
  if (!o.q) return { error: 'Type what to find.' }
  const source = o.regex ? o.q : o.q.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  try {
    return new RegExp(source, o.matchCase ? 'gu' : 'giu')
  } catch (e) {
    return { error: `The pattern is not valid: ${e instanceof Error ? e.message : String(e)}` }
  }
}

/**
 * The edits that replace every match in `text`, line by line as the search matches. In regex
 * mode `$1` and `$<name>` take the groups; otherwise `with` goes in as typed.
 */
export function planText(path: string, text: string, re: RegExp, withText: string, regex: boolean): FilePlan {
  // Sticky and not global: replace() then expands `with` for the one match at lastIndex, with the
  // whole line around it, so ^, \b and lookaround behave as they did when matching.
  const one = new RegExp(re.source, re.flags.replace('g', '') + 'y')
  const edits: LspTextEdit[] = []
  const lines: LineChange[] = []
  text.split('\n').forEach((raw, n) => {
    const line = raw.endsWith('\r') ? raw.slice(0, -1) : raw
    let after = ''
    let last = 0
    let hit = false
    for (const m of line.matchAll(re)) {
      if (m[0] === '') continue
      const from = m.index ?? 0
      const to = from + m[0].length
      let insert = withText
      if (regex) {
        one.lastIndex = from
        const out = line.replace(one, withText)
        insert = out.slice(from, out.length - (line.length - to))
      }
      edits.push({ range: { start: { line: n, character: from }, end: { line: n, character: to } }, newText: insert })
      after += line.slice(last, from) + insert
      last = to
      hit = true
    }
    if (hit) lines.push({ line: n + 1, before: line, after: after + line.slice(last) })
  })
  return { path, edits, lines, planned: text }
}

export const countPlaces = (plans: FilePlan[]) => plans.reduce((n, p) => n + p.edits.length, 0)
