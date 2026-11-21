// The code symbols around the cursor, from the server's textDocument/documentSymbol. Part of the
// lazy lsp chunk. The server may answer a tree (DocumentSymbol) or a flat list (SymbolInformation);
// both become one tree here, and anything malformed is dropped rather than shown.
import type { Extension } from '@codemirror/state'
import { EditorView } from '@codemirror/view'

export interface SymbolStep {
  name: string
  /** Zero-based line and character of the symbol's name. */
  line: number
  character: number
}

interface Pos {
  line: number
  character: number
}

interface Node {
  name: string
  start: Pos
  end: Pos
  at: Pos
  children: Node[]
}

const isPos = (p: unknown): p is Pos => {
  const { line, character } = (p ?? {}) as Partial<Pos>
  return Number.isInteger(line) && Number.isInteger(character) && line! >= 0 && character! >= 0
}

const isRange = (r: unknown): r is { start: Pos; end: Pos } => isPos((r as { start?: unknown })?.start) && isPos((r as { end?: unknown })?.end)

const before = (a: Pos, b: Pos) => a.line < b.line || (a.line === b.line && a.character < b.character)
const within = (n: Node, p: Pos) => !before(p, n.start) && !before(n.end, p)

const MAX_NODES = 5000

function fromTree(list: unknown[], budget: { n: number }, depth = 0): Node[] {
  const out: Node[] = []
  for (const raw of list) {
    const s = raw as { name?: unknown; range?: unknown; selectionRange?: unknown; children?: unknown }
    if (budget.n <= 0 || depth > 30) break
    if (typeof s?.name !== 'string' || !isRange(s.range)) continue
    budget.n--
    const sel = isRange(s.selectionRange) ? s.selectionRange.start : s.range.start
    out.push({ name: s.name, start: s.range.start, end: s.range.end, at: sel, children: Array.isArray(s.children) ? fromTree(s.children, budget, depth + 1) : [] })
  }
  return out
}

function fromFlat(list: unknown[]): Node[] {
  const flat: Node[] = []
  for (const raw of list.slice(0, MAX_NODES)) {
    const s = raw as { name?: unknown; location?: { range?: unknown } }
    if (typeof s?.name !== 'string' || !isRange(s.location?.range)) continue
    const { start, end } = s.location.range
    flat.push({ name: s.name, start, end, at: start, children: [] })
  }
  // Outer symbols first: earlier start, then later end. A symbol joins the nearest open one that contains it.
  flat.sort((a, b) => (before(a.start, b.start) ? -1 : before(b.start, a.start) ? 1 : before(b.end, a.end) ? -1 : before(a.end, b.end) ? 1 : 0))
  const top: Node[] = []
  const open: Node[] = []
  for (const n of flat) {
    while (open.length && !within(open[open.length - 1], n.start)) open.pop()
    ;(open.length ? open[open.length - 1].children : top).push(n)
    open.push(n)
  }
  return top
}

/** The symbol tree in the server's answer; an empty list when the answer is not one. */
export function parseSymbols(answer: unknown): Node[] {
  if (!Array.isArray(answer)) return []
  const flat = answer.length > 0 && (answer[0] as { location?: unknown })?.location !== undefined
  return flat ? fromFlat(answer) : fromTree(answer, { n: MAX_NODES })
}

/** The symbols that contain the position, outermost first. */
export function symbolPath(nodes: Node[], at: Pos): SymbolStep[] {
  const out: SymbolStep[] = []
  let level = nodes
  for (;;) {
    // The innermost of overlapping siblings wins: the last one that contains the position.
    const hit = [...level].reverse().find((n) => within(n, at))
    if (!hit) return out
    out.push({ name: hit.name, line: hit.at.line, character: hit.at.character })
    level = hit.children
  }
}

const same = (a: SymbolStep[], b: SymbolStep[]) => a.length === b.length && a.every((s, i) => s.name === b[i].name && s.line === b[i].line && s.character === b[i].character)

export interface Tracker {
  /** Goes in the editor's language server slot; follows the cursor and edits. */
  extension: Extension
  /** Asks the server again, now. */
  refresh(): void
  go(step: SymbolStep): void
  dispose(): void
}

/**
 * `request` asks the server for the symbols of the file; it rejects or answers null when the
 * server cannot. `report` gets the path whenever it changes.
 */
export function trackSymbols(view: EditorView, request: () => Promise<unknown>, report: (steps: SymbolStep[]) => void): Tracker {
  let nodes: Node[] = []
  let shown: SymbolStep[] = []
  let asked = 0
  let gone = false
  let timer: ReturnType<typeof setTimeout> | null = null

  const update = () => {
    if (gone) return
    const head = view.state.selection.main.head
    const line = view.state.doc.lineAt(head)
    const next = symbolPath(nodes, { line: line.number - 1, character: head - line.from })
    if (same(next, shown)) return
    shown = next
    report(next)
  }
  const refresh = () => {
    if (gone) return
    const n = ++asked
    request().then(
      (answer) => {
        if (gone || n !== asked) return
        nodes = parseSymbols(answer)
        update()
      },
      () => {
        if (gone || n !== asked) return
        nodes = []
        update()
      },
    )
  }
  const extension = EditorView.updateListener.of((u) => {
    if (u.docChanged) {
      if (timer) clearTimeout(timer)
      timer = setTimeout(refresh, 600)
    } else if (u.selectionSet) update()
  })
  return {
    extension,
    refresh,
    go(step) {
      if (gone) return
      const doc = view.state.doc
      if (step.line + 1 > doc.lines) return
      const line = doc.line(step.line + 1)
      view.dispatch({ selection: { anchor: line.from + Math.min(step.character, line.length) }, scrollIntoView: true })
      view.focus()
    },
    dispose() {
      gone = true
      if (timer) clearTimeout(timer)
      nodes = []
      shown = []
    },
  }
}
