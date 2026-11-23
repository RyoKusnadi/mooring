// Which entries of the file tree git calls changed. The data is the status the Changes view already reads.
import { sideLetter, type Change, type Status } from './gitapi'
import { parentPath } from './paths'

export interface Marks {
  /** Status letter by file path (M, A, D, R, T, ? or U). */
  files: Map<string, string>
  /** Folders that hold at least one changed file. */
  dirs: Set<string>
}

export const NO_MARKS: Marks = { files: new Map(), dirs: new Set() }

/** The side the user still has work on: a conflict or new file, else the unstaged side, else the staged one. */
export function letterOf(c: Change): string {
  if (c.kind || c.unstaged !== '') return sideLetter(c, false)
  return sideLetter(c, true)
}

export function markChanges(status: Status | null): Marks {
  if (!status?.repo || status.files.length === 0) return NO_MARKS
  const files = new Map<string, string>()
  const dirs = new Set<string>()
  for (const c of status.files) {
    files.set(c.path, letterOf(c))
    for (let d = parentPath(c.path); d && !dirs.has(d); d = parentPath(d)) dirs.add(d)
  }
  return { files, dirs }
}

/** The colour class of a letter: green for new, red for deleted or conflicted, plain text for edited (amber is for attention only). */
export const letterTone = (l: string): 'new' | 'gone' | 'edit' => (l === 'A' || l === '?' ? 'new' : l === 'D' || l === 'U' ? 'gone' : 'edit')

const WORDS: Record<string, string> = { M: 'Modified', A: 'New', D: 'Deleted', R: 'Renamed', T: 'Type changed', '?': 'New, not tracked', U: 'Conflict' }
export const letterWord = (l: string): string => WORDS[l] ?? 'Changed'
