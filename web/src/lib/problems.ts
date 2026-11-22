// Language-server diagnostics as a list: plain helpers for problems.svelte.ts.

/** LSP severity: 1 error, 2 warning, 3 information, 4 hint. */
export interface Problem {
  path: string
  /** 1-based, as the editor counts. */
  line: number
  col: number
  severity: number
  message: string
  source: string
}

interface LspDiagnostic {
  range?: { start?: { line?: number; character?: number } }
  severity?: number
  message?: string
  source?: string
}

const MAX_PER_FILE = 200

export function toProblems(path: string, diags: unknown): Problem[] {
  if (!Array.isArray(diags)) return []
  return (diags as LspDiagnostic[]).slice(0, MAX_PER_FILE).map((d) => ({
    path,
    line: (d.range?.start?.line ?? 0) + 1,
    col: (d.range?.start?.character ?? 0) + 1,
    severity: d.severity && d.severity >= 1 && d.severity <= 4 ? d.severity : 1,
    message: String(d.message ?? '').trim(),
    source: String(d.source ?? ''),
  }))
}

/** Errors first, then warnings and the rest; within a kind by path, then line. */
export function sortProblems(list: Problem[]): Problem[] {
  return [...list].sort((a, b) => a.severity - b.severity || a.path.localeCompare(b.path) || a.line - b.line || a.col - b.col)
}

export function countProblems(list: Problem[]): { errors: number; warnings: number } {
  let errors = 0
  let warnings = 0
  for (const p of list) {
    if (p.severity === 1) errors++
    else if (p.severity === 2) warnings++
  }
  return { errors, warnings }
}

const NAMES = ['Error', 'Warning', 'Information', 'Hint']
export const severityName = (severity: number): string => NAMES[severity - 1] ?? 'Error'

/** The status bar text: "2 errors, 5 warnings", or '' when there is nothing to show. */
export function countText({ errors, warnings }: { errors: number; warnings: number }): string {
  const parts: string[] = []
  if (errors) parts.push(`${errors} ${errors === 1 ? 'error' : 'errors'}`)
  if (warnings) parts.push(`${warnings} ${warnings === 1 ? 'warning' : 'warnings'}`)
  return parts.join(', ')
}

/** The first `max` problems, in order; `more` is how many were left out. */
export function limitProblems(list: Problem[], max: number): { shown: Problem[]; more: number } {
  return { shown: list.slice(0, max), more: Math.max(0, list.length - max) }
}
