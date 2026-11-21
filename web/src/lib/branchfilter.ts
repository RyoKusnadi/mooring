/** Branch names that contain what was typed, the ones that start with it first. Case does not matter. */
export function filterBranches(names: string[], query: string): string[] {
  const q = query.trim().toLowerCase()
  if (!q) return names
  const hits = names.filter((n) => n.toLowerCase().includes(q))
  return [...hits.filter((n) => n.toLowerCase().startsWith(q)), ...hits.filter((n) => !n.toLowerCase().startsWith(q))]
}

/** The typed text can become a new branch unless a branch already has exactly that name. */
export const canCreate = (names: string[], query: string): boolean => query.trim() !== '' && !names.includes(query.trim())

/** One plain line for a failed branch switch or create, whatever git said. */
export function branchFailureText(f: { code: string; message: string } | null): string {
  if (!f) return ''
  if (f.code === 'GIT_FAILED') return 'Git did not switch: you have uncommitted changes that this branch would overwrite. Commit, stash or discard them, then switch.'
  return f.message.split('\n')[0]
}
