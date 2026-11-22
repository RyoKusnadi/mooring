// Replace across files against this app's files: plan from the files the search found, then
// make the change through the same path as a rename.
import { currentText, projectTarget } from './edittarget'
import { applyFiles, type EditResult } from './multiedit'
import { matcher, planText, type FilePlan, type ReplaceOptions } from './replace'

/** A replace reads at most this many files; the search lists more only for a very common word. */
export const MAX_FILES = 500
const AT_ONCE = 6

export interface ReplacePlan {
  plans: FilePlan[]
  /** Files that could not be read, or were binary or too large. */
  skipped: string[]
}

export async function planReplace(projectId: string, paths: string[], o: ReplaceOptions, withText: string, onProgress?: (read: number, total: number) => void): Promise<ReplacePlan | { error: string }> {
  const re = matcher(o)
  if (!(re instanceof RegExp)) return re
  if (paths.length > MAX_FILES) return { error: `The search found ${paths.length} files; a replace can change ${MAX_FILES} at most. Narrow the search.` }
  const out: ReplacePlan = { plans: [], skipped: [] }
  for (let i = 0; i < paths.length; i += AT_ONCE) {
    const group = paths.slice(i, i + AT_ONCE)
    const texts = await Promise.all(group.map((path) => currentText(projectId, path).catch(() => null)))
    group.forEach((path, k) => {
      const text = texts[k]
      if (text === null) return void out.skipped.push(path)
      const p = planText(path, text, re, withText, !!o.regex)
      if (p.edits.length) out.plans.push(p)
    })
    onProgress?.(Math.min(i + AT_ONCE, paths.length), paths.length)
  }
  return out
}

export const applyReplace = (projectId: string, plans: FilePlan[]): Promise<EditResult | { refused: string }> =>
  applyFiles(plans, projectTarget(projectId, 'replace'))
