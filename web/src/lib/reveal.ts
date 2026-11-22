// Shows a file or folder in the sidebar tree: opens the folders above it, scrolls to its row and focuses it.
import { tick } from 'svelte'
import { files } from './filestore.svelte'
import { git } from './gitstore.svelte'
import { parentPath } from './paths'
import { app } from './state.svelte'

/** The folders above `path`, outermost first. */
export function ancestors(path: string): string[] {
  const out: string[] = []
  for (let p = parentPath(path); p; p = parentPath(p)) out.unshift(p)
  return out
}

export async function reveal(path: string, folder = false) {
  const project = app.currentId
  for (const dir of ancestors(path)) files.expand(project, dir)
  if (folder) files.expand(project, path)
  git.view = 'files'
  files.cursor = path
  files.target = folder ? path : parentPath(path)
  files.revealTick++
  await tick()
  // Folders load on the server first, and on a phone the drawer opens first.
  for (let i = 0; i < 40; i++) {
    await new Promise((r) => setTimeout(r, 50))
    const row = [...document.querySelectorAll<HTMLElement>('#sidebar [data-path]')].find((e) => e.dataset.path === path)
    if (row && row.getClientRects().length > 0) {
      row.scrollIntoView({ block: 'nearest' })
      row.focus()
      return
    }
  }
}
