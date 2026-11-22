// Shell side of terminal links: which project file a printed path names, and opening it.
// termlinkprovider.ts (terminal chunk) reaches this through terms.links, so the app state
// stays out of the terminal chunk's imports.
import { fileActions } from './fileactions'
import { findTab } from './layout'
import { app } from './state.svelte'
import { projectPath } from './termlinks'
import { terms, type TermLinks } from './termstore.svelte'

export const termLinks: TermLinks = {
  path(id, path) {
    const info = terms.list[id]
    if (!info || info.ssh || info.projectId !== app.currentId) return null // remote files, or another project
    const root = app.projects.find((p) => p.id === info.projectId)?.path ?? ''
    const layout = app.layouts[info.projectId]
    const folder = (layout && findTab(layout, id)?.folder) || ''
    return root ? projectPath(path, folder, root) : null
  },
  open: (path, line) => fileActions.openFile(path, line),
}
