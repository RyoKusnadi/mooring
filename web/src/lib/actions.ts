// Workspace actions shared by buttons, keyboard shortcuts and the command palette.
import { closeTab, cycleGroup, cycleTab, findGroup, findTab, focusTab, focusedGroup, groups, newTab, openTab, replaceTab, type Layout } from './layout'
import { moveTab, moveTabToPane, splitFocused, type Edge } from './layout-edit'
import { newCommandId } from './events'
import { app } from './state.svelte'
import { terms } from './termstore.svelte'
import { chats } from './chatstore.svelte'
import { files } from './filestore.svelte'
import { git } from './gitstore.svelte'
import { findDiffTab, findKindTab, flipDiff, newCommitDiffTab, newCommitTab, newDiffTab } from './difftabs'
import { isInside, joinPath } from './paths'
import { fileActions } from './fileactions'
import { termLinks } from './termplace'
import { sshActions } from './sshactions'
import { ssh } from './sshstore.svelte'
import { accounts } from './accountstore.svelte'
import { settings } from './settingsstore.svelte'
import { tools } from './toolstore.svelte'
import { chatHistory } from './chathistorystore.svelte'
import { problemsView } from './problemsview.svelte'
import { tick } from 'svelte'
import { chordOf, composing } from './keys'

export const paletteKey = /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘K' : 'Ctrl+K'

/** A terminal tab is named by what its shell reports; until then it is just "Terminal", never the project. */
const folderName = () => 'Terminal'

function startTerminal(id: string, folder = '') {
  const projectId = app.currentId
  terms.open((n, a) => app.run(n, a), projectId, id, folder).catch((e: unknown) => {
    app.notice = 'Could not start a terminal. ' + (e instanceof Error ? e.message : String(e))
  })
}

const runCmd = <T>(n: string, a: unknown) => app.run<T>(n, a)

/** After a pane move, DOM focus follows: into a terminal, else onto the pane's tab. */
function followFocus() {
  void tick().then(() => {
    const id = app.layout?.focus
    if (!id) return
    const view = terms.views.get(id)
    if (view) view.focus()
    else document.getElementById('tab-' + id)?.focus()
  })
}

/** Leaves a terminal for the focused tab in the pane's tab bar. */
function leaveTerminal() {
  const id = app.layout?.focus
  void tick().then(() => (id ? document.getElementById('tab-' + id)?.focus() : undefined))
}

terms.links = termLinks // file paths printed in a terminal open in the editor

const paneMove = (run: () => void) => () => {
  run()
  followFocus()
}

export const actions = {
  /** Opens the "+" menu of the focused pane (Alt+T). */
  openPlusMenu: () => document.querySelector<HTMLElement>('[data-plus]')?.click(),
  newTerminal: () => {
    const tab = newTab(newCommandId(), 'terminal', folderName())
    app.edit((l) => openTab(l, tab))
    startTerminal(tab.id)
  },
  /** Replaces a blank tab with a terminal, in place. */
  terminalHere: (id: string) => {
    const tab = newTab(newCommandId(), 'terminal', folderName())
    app.edit((l) => replaceTab(l, id, tab))
    startTerminal(tab.id)
  },
  /** A chat tab starts on the agent picker; the tab id becomes the chat id. */
  newChat: () => app.edit((l) => openTab(l, newTab(newCommandId(), 'chat', 'New chat'))),
  /** A new chat tab whose start screen holds a summary of chat `from`, to edit before it starts. */
  chatFromSummary: (from: string, text: string, source: string, review = false) => {
    const tab = newTab(newCommandId(), 'chat', 'New chat')
    chats.seeds[tab.id] = { fromChat: from, text, source, ...(review ? { review } : {}) }
    app.edit((l) => openTab(l, tab))
  },
  /** Replaces a blank tab with a chat tab, in place. */
  chatHere: (id: string) => app.edit((l) => replaceTab(l, id, newTab(newCommandId(), 'chat', 'New chat'))),
  /** Shows a saved chat: its tab if one is open, else a new tab with its id. */
  openChat: (id: string) => {
    const l = app.layout
    if (l && findTab(l, id)) return app.edit((l) => focusTab(l, id))
    app.edit((l) => openTab(l, newTab(id, 'chat', chats.list[id]?.title || 'Chat')))
  },
  /** Shows the tab of a group of chats: the members side by side. */
  openGroup: (id: string) => {
    const l = app.layout
    if (l && findTab(l, id)) return app.edit((l) => focusTab(l, id))
    app.edit((l) => openTab(l, newTab(id, 'group', 'Group')))
  },
  /** Shows a terminal's tab when it is open in this project. */
  openTerminalTab: (id: string) => {
    const l = app.layout
    if (l && findTab(l, id)) app.edit((l) => focusTab(l, id))
  },
  /** Deletes a saved chat of the current project and closes its tab. */
  deleteChat: (id: string) => {
    const l = app.layout
    if (l && findTab(l, id)) app.edit((l) => closeTab(l, id))
    chats.remove(runCmd, id).catch((e: unknown) => {
      app.notice = 'Could not delete the chat. ' + (e instanceof Error ? e.message : String(e))
    })
  },
  /** A terminal that starts in a worktree (or any folder of the project). */
  terminalIn: (folder: string, title: string) => {
    const tab = { ...newTab(newCommandId(), 'terminal', title), folder }
    app.edit((l) => openTab(l, tab))
    startTerminal(tab.id, folder)
  },
  /** An exited terminal's tab gets a fresh shell in the same place. */
  restartTerminal: (id: string) => {
    const host = app.layout && findTab(app.layout, id)?.host
    if (host) return void sshActions.reconnectTerminal(id, host)
    const folder = (app.layout && findTab(app.layout, id)?.folder) || ''
    const tab = folder ? { ...newTab(newCommandId(), 'terminal', folderName()), folder } : newTab(newCommandId(), 'terminal', folderName())
    app.edit((l) => replaceTab(l, id, tab))
    void terms.close((n, a) => app.run(n, a), id).catch(() => {})
    startTerminal(tab.id, folder)
  },
  /** Opens (or focuses) the Changes view, for a worktree folder or the project's own. */
  showChanges: (folder?: string) => {
    git.show(folder)
    const l = app.layout
    const existing = l && groups(l.root).flatMap((g) => g.tabs).find((t) => t.kind === 'changes')
    if (existing) return app.edit((l) => focusTab(l, existing.id))
    app.edit((l) => openTab(l, { id: newCommandId(), kind: 'changes', title: 'Changes' }))
  },
  /** Opens (or focuses) the diff of one side of a file. */
  openDiff: (folder: string, path: string, staged: boolean) => {
    const l = app.layout
    const existing = l && findDiffTab(l, folder, path, staged)
    if (existing) return app.edit((l) => focusTab(l, existing.id))
    app.edit((l) => openTab(l, newDiffTab(newCommandId(), folder, path, staged)))
  },
  /** Opens (or focuses) the history of a place. */
  openHistory: (folder = git.folder) => {
    const l = app.layout
    const existing = l && findKindTab(l, 'history', folder)
    if (existing) return app.edit((l) => focusTab(l, existing.id))
    app.edit((l) => openTab(l, { id: newCommandId(), kind: 'history', title: folder ? 'History (worktree)' : 'History', folder }))
  },
  /** Opens a commit or a stash on its own tab. */
  openCommit: (folder: string, commit: string, title: string) => {
    const l = app.layout
    const existing = l && findKindTab(l, 'commit', folder, commit)
    if (existing) return app.edit((l) => focusTab(l, existing.id))
    app.edit((l) => openTab(l, newCommitTab(newCommandId(), folder, commit, title)))
  },
  /** Opens a file as a commit or stash changed it. */
  openCommitDiff: (folder: string, commit: string, path: string) => {
    const l = app.layout
    const existing = l && findDiffTab(l, folder, path, false, commit)
    if (existing) return app.edit((l) => focusTab(l, existing.id))
    app.edit((l) => openTab(l, newCommitDiffTab(newCommandId(), folder, commit, path)))
  },
  /** The diff tab shows the other side of its file, after a stage or an unstage. */
  flipDiff: (id: string, staged: boolean) => app.edit((l) => flipDiff(l, id, staged)),
  /** Opens a file of a place (paths in git are relative to the place). */
  openPlaceFile: (folder: string, path: string) => fileActions.openFile(joinPath(folder, path)),
  /** Closes the tabs of a removed worktree: its diffs, terminals and files (unsaved files stay open). */
  closePlace: (folder: string) => {
    const l = app.layout
    if (!l || !folder) return
    for (const t of groups(l.root).flatMap((g) => g.tabs)) {
      const inPlace = t.kind === 'file' ? !!t.path && isInside(t.path, folder) : (t.kind === 'diff' || t.kind === 'terminal') && t.folder === folder
      if (inPlace && !(t.kind === 'file' && files.dirty[t.id])) actions.closeTab(t.id)
    }
  },
  ...fileActions,
  ...sshActions,
  openTools: () => (tools.dialog = true),
  openAccounts: () => (accounts.dialog = true),
  openSettings: () => (settings.dialog = true),
  openChatHistory: () => (chatHistory.dialog = true),
  openProblems: () => (problemsView.dialog = true),
  /** A file tab with unsaved edits asks first, unless `force` (the editor's own buttons) says to drop them. */
  closeTab: (id?: string, force = false) => {
    const l = app.layout
    const tab = l && findTab(l, id ?? l.focus)
    if (tab?.kind === 'file' || tab?.kind === 'rfile') {
      if (files.dirty[tab.id] && !force) {
        files.closing = tab.id
        return app.edit((l) => focusTab(l, tab.id))
      }
      files.forget(tab.id)
    }
    if (tab?.kind === 'terminal') void terms.close((n, a) => app.run(n, a), tab.id).catch(() => {})
    // Closing a chat tab stops its agent; the saved chat stays in the project's list.
    if (tab?.kind === 'chat' && chats.list[tab.id]) void chats.close(runCmd, tab.id).catch(() => {})
    app.edit((l) => closeTab(l, id ?? l.focus))
    const host = tab?.host
    if (host && app.layout && !groups(app.layout.root).some((g) => g.tabs.some((t) => t.host === host))) ssh.forgetClosed(host)
  },
  focusTab: (id: string) => app.edit((l) => focusTab(l, id)),
  /** Splits the focused pane; the new pane starts a terminal. */
  split: (dir: 'row' | 'column') => {
    const tab = newTab(newCommandId(), 'terminal', folderName())
    app.edit((l) => splitFocused(l, dir, tab))
    startTerminal(tab.id)
  },
  nextTab: () => app.edit((l) => cycleTab(l, 1)),
  prevTab: () => app.edit((l) => cycleTab(l, -1)),
  nextPane: paneMove(() => app.edit((l) => cycleGroup(l, 1))),
  prevPane: paneMove(() => app.edit((l) => cycleGroup(l, -1))),
  moveToNextPane: paneMove(() => app.edit((l) => moveTabToPane(l, 1))),
  moveToPrevPane: paneMove(() => app.edit((l) => moveTabToPane(l, -1))),
  leaveTerminal,
  /** Opens the find bar of terminal `id` and puts the cursor in its field. */
  findInTerminal: (id: string) => {
    terms.patch(id, { find: true })
    void tick().then(() => {
      const field = document.querySelector<HTMLInputElement>(`[data-termfind="${CSS.escape(id)}"] input`)
      field?.focus()
      field?.select()
    })
  },
  moveToNewPane: (edge: Edge) => app.edit((l) => moveTab(l, l.focus, focusedGroup(l)?.active ?? '', edge)),
  drop: (tabId: string, anchorId: string, where: Edge | 'center', index?: number) =>
    app.edit((l) => moveTab(l, tabId, anchorId, where, index)),
}

export function paneCount(l: Layout | null): number {
  return l ? groups(l.root).filter((g) => g.tabs.length > 0).length : 0
}

export function paneNumber(l: Layout): number {
  const g = findGroup(l.root, l.focus)
  return g ? groups(l.root).filter((x) => x.tabs.length > 0).indexOf(g) + 1 : 0
}

const SHORTCUTS: Record<string, () => void> = {
  Backslash: actions.split.bind(null, 'row'),
  Minus: actions.split.bind(null, 'column'),
  KeyT: actions.openPlusMenu,
  KeyN: actions.newTerminal,
  KeyA: actions.newChat,
  KeyG: () => actions.showChanges(),
  KeyH: () => actions.openHistory(),
  KeyO: () => (files.finder = 'file'),
  // An open results panel takes focus back instead of starting over.
  KeyF: () => (files.finder === 'text' ? document.getElementById('find-q')?.focus() : (files.finder = 'text')),
  KeyS: sshActions.openHosts,
  KeyM: () => (tools.dialog = true),
  KeyY: actions.openChatHistory,
  KeyP: actions.openProblems,
  KeyW: () => actions.closeTab(),
  ArrowRight: actions.nextTab,
  ArrowLeft: actions.prevTab,
  ArrowDown: actions.nextPane,
  ArrowUp: actions.prevPane,
  Escape: actions.leaveTerminal,
}

/** Alt+key shortcuts for the workspace. Returns true when it handled the event. */
export function handleShortcut(e: KeyboardEvent, typing = false): boolean {
  if (composing(e) || !e.altKey || e.ctrlKey || e.metaKey) return false
  // In a text box the arrows and Esc keep their editing meaning; letters and \ - are free.
  if (typing && (e.code.startsWith('Arrow') || e.code === 'Escape')) return false
  if (e.shiftKey) {
    const move = e.code === 'ArrowRight' ? actions.moveToNextPane : e.code === 'ArrowLeft' ? actions.moveToPrevPane : null
    if (!move) return false
    move()
    return true
  }
  const run = SHORTCUTS[e.code]
  if (!run) return false
  run()
  return true
}

/** True for keys the workspace handles itself, so a terminal must not take them. */
export function isShortcut(e: KeyboardEvent): boolean {
  if (e.type !== 'keydown' || composing(e)) return false
  if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && e.code === 'KeyK') return true
  if (chordOf(e, true)) return true
  if (!e.altKey || e.ctrlKey || e.metaKey) return false
  if (e.shiftKey) return e.code === 'ArrowRight' || e.code === 'ArrowLeft'
  return e.code in SHORTCUTS
}
