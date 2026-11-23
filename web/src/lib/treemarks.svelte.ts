import { git } from './gitstore.svelte'
import { app } from './state.svelte'
import { markChanges, NO_MARKS, type Marks } from './treemarks'

class TreeMarks {
  /** Only the project's own folder is marked: a worktree shown in Changes is another tree. */
  current = $derived.by<Marks>(() => (git.project === app.currentId && git.folder === '' ? markChanges(git.status) : NO_MARKS))
}

export const treeMarks = new TreeMarks()
