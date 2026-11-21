// Lanes for the history graph, computed in the browser from commits in topological order, newest first.
// A lane is a column that waits for one commit id (the next commit down the page that this line leads to).
// The state carries over between pages, so a page can end with lanes still waiting for parents that
// arrive on a later page, and computing page by page gives the same rows as computing the whole list.

export interface Edge {
  lane: number
  color: number
}
export interface GraphRow {
  /** The lane the commit's dot sits in. */
  col: number
  color: number
  /** Lines that run past this commit from top to bottom. */
  through: Edge[]
  /** Lines that arrive from above: the child lanes that end in this commit. */
  up: Edge[]
  /** Lines that leave downward, one per parent. */
  down: Edge[]
  /** Lanes drawn in this row. */
  width: number
  merge: boolean
}
export interface GraphState {
  lanes: (string | null)[]
  colors: number[]
  next: number
}
export interface GraphCommit {
  id: string
  parents: string[]
}

export const newGraph = (): GraphState => ({ lanes: [], colors: [], next: 0 })

function firstFree(lanes: (string | null)[]): number {
  const i = lanes.indexOf(null)
  return i < 0 ? lanes.length : i
}

/** Adds one commit to the graph and returns its row. Mutates `g`. */
export function addCommit(g: GraphState, c: GraphCommit): GraphRow {
  const { lanes, colors } = g
  const waiting: number[] = []
  lanes.forEach((id, i) => id === c.id && waiting.push(i))
  const up: Edge[] = waiting.map((lane) => ({ lane, color: colors[lane] }))
  const through: Edge[] = []
  lanes.forEach((id, lane) => id !== null && id !== c.id && through.push({ lane, color: colors[lane] }))

  let col: number
  if (waiting.length > 0) col = waiting[0]
  else {
    col = firstFree(lanes)
    lanes[col] = c.id
    colors[col] = g.next++
  }
  const color = colors[col]
  for (const i of waiting) lanes[i] = null
  lanes[col] = null

  const down: Edge[] = []
  c.parents.forEach((p, k) => {
    const have = lanes.indexOf(p)
    if (have >= 0) return void down.push({ lane: have, color: colors[have] })
    const lane = k === 0 ? col : firstFree(lanes)
    lanes[lane] = p
    colors[lane] = k === 0 ? color : g.next++
    down.push({ lane, color: colors[lane] })
  })

  while (lanes.length > 0 && lanes[lanes.length - 1] === null) lanes.pop()
  colors.length = lanes.length
  const width = Math.max(col, ...up.map((e) => e.lane), ...through.map((e) => e.lane), ...down.map((e) => e.lane)) + 1
  return { col, color, through, up, down, width, merge: c.parents.length > 1 }
}

/** Adds a page of commits. */
export function addPage(g: GraphState, page: GraphCommit[]): GraphRow[] {
  return page.map((c) => addCommit(g, c))
}
