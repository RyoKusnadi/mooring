// Workspace actions for files: open (or focus) a file tab, and keep tabs right when files move or go.
import { noteOpened } from './recentfiles'
import { newCommandId } from './events'
import { files } from './filestore.svelte'
import { closeTab, fileTabsIn, findFileTab, findTab, focusTab, newTab, openTab, retargetFiles } from './layout'
import { baseName } from './paths'
import { app } from './state.svelte'
import { openPreview, patchTab, pinTab, previewIn } from './tabedit'

export const fileActions = {
  /**
   * One editor per file: an open file gets its tab focused instead of a second tab. A `preview` open
   * (one click in the tree) shows in a tab the next preview replaces; any other open keeps the tab.
   */
  openFile: (path: string, line?: number, preview = false) => {
    noteOpened(app.currentId, path)
    const l = app.layout
    const existing = l && findFileTab(l, path)
    if (existing) {
      app.edit((l) => (existing.preview && !preview ? patchTab(focusTab(l, existing.id), existing.id, { preview: false }) : focusTab(l, existing.id)))
      if (line) files.jump = { tabId: existing.id, line }
      return
    }
    const tab = newTab(newCommandId(), 'file', baseName(path), path)
    let old = preview && l ? previewIn(l) : undefined
    if (old && files.dirty[old.id]) {
      // A preview with unsaved edits is never replaced: it becomes a normal tab and the new file opens beside it.
      const keep = old.id
      app.edit((l) => patchTab(l, keep, { preview: false }))
      old = undefined
    }
    if (old) files.forget(old.id)
    app.edit((l) => (preview ? openPreview(l, tab) : openTab(l, tab)))
    files.jump = { tabId: tab.id, line: line ?? 1 }
  },

  /** A preview tab becomes a normal tab (an edit, a double-click or a pin does this). */
  keepTab: (id: string) => {
    if (app.layout && findTab(app.layout, id)?.preview) app.edit((l) => patchTab(l, id, { preview: false }))
  },
  keepFile: (path: string) => {
    const t = app.layout && findFileTab(app.layout, path)
    if (t?.preview) app.edit((l) => patchTab(l, t.id, { preview: false }))
  },
  pinTab: (id: string, pinned: boolean) => app.edit((l) => pinTab(l, id, pinned)),

  renameEntry: async (from: string, to: string) => {
    await files.rename(app.currentId, from, to)
    app.edit((l) => retargetFiles(l, from, to, baseName))
  },

  /** Deletes a file or folder and closes the tabs that edited it. Throws TRASH_UNAVAILABLE for the caller to ask. */
  deleteEntry: async (path: string, permanent = false) => {
    const l = app.layout
    const tabs = l ? fileTabsIn(l, path) : []
    const trashed = await files.remove(app.currentId, path, permanent)
    for (const t of tabs) files.forget(t.id)
    app.edit((l) => tabs.reduce((acc, t) => closeTab(acc, t.id), l))
    return trashed
  },
}
