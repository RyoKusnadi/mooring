// Wording for a merge, rebase, cherry-pick, revert or am that stopped for the user.
export type Operation = 'merge' | 'rebase' | 'cherry-pick' | 'revert' | 'am'

const NAMES: Record<Operation, string> = { merge: 'merge', rebase: 'rebase', 'cherry-pick': 'cherry-pick', revert: 'revert', am: 'patch apply' }

export const operationName = (op: Operation) => NAMES[op]

/** Button text for the sides. In a rebase "ours" is the base and "theirs" is the user's own commit. */
export function sideLabels(op: Operation | undefined): { ours: string; theirs: string } {
  return op === 'rebase' ? { ours: 'Keep the base', theirs: 'Keep your commit' } : { ours: 'Keep current branch', theirs: 'Keep incoming' }
}

const capital = (s: string) => s.charAt(0).toUpperCase() + s.slice(1)

export function operationText(op: Operation, left: number) {
  const name = NAMES[op]
  return {
    title: `${capital(name)} in progress.`,
    body: left > 0 ? `${left === 1 ? '1 file still has' : `${left} files still have`} conflicts. Resolve every file, then finish the ${name}.` : `All conflicts are resolved. Finish the ${name}.`,
    abort: `Abort ${name}`,
    finish: op === 'merge' ? 'Finish merge' : `Continue ${name}`,
    why: left > 0 ? `Resolve ${left === 1 ? 'the conflicted file' : 'the conflicted files'} first.` : '',
    confirm: op === 'merge' ? 'Abort the merge? The merge commit is not made and the files go back to how they were before it. Edits you made while resolving are lost.' : `Abort the ${name}? The branch goes back to how it was before the ${name} started. Edits you made while resolving are lost.`,
  }
}
