// The command palette's list. Everyday commands come first, then the rest, then one row per project.
import { actions } from './actions'
import { appearance } from './appearance.svelte'
import type { PaletteItem } from './filter'
import { files } from './filestore.svelte'
import { git } from './gitstore.svelte'
import { modLabel, showKey } from './keys'
import { focusedGroup } from './layout'
import { app } from './state.svelte'
import { ssh } from './sshstore.svelte'

export interface PaletteHooks {
  addProject: () => void
  languageServers: () => void
  signOut: () => void
}

export function paletteItems(hooks: PaletteHooks): PaletteItem[] {
  const open = !!app.layout
  const everyday: PaletteItem[] = open
    ? [
        { id: 'term-new', label: 'New terminal', hint: showKey('Alt+N'), run: actions.newTerminal },
        { id: 'chat-new', label: 'New chat', hint: showKey('Alt+A'), run: actions.newChat },
        { id: 'file-open', label: 'Open file by name', hint: `${showKey('Alt+O')} ${modLabel}P`, run: () => (files.finder = 'file') },
        { id: 'file-search', label: 'Search text in project', hint: `${showKey('Alt+F')} ${modLabel}${showKey('Shift+')}F`, run: () => (files.finder = 'text') },
        { id: 'changes-show', label: 'Show changes', hint: showKey('Alt+G'), run: () => actions.showChanges() },
        { id: 'split-right', label: 'Split right', hint: showKey('Alt+\\'), run: () => actions.split('row') },
        { id: 'split-down', label: 'Split down', hint: showKey('Alt+-'), run: () => actions.split('column') },
        {
          id: 'branch-switch',
          label: 'Switch branch',
          run: () => {
            actions.showChanges()
            git.branchRequest++
          },
        },
      ]
    : []
  const settings: PaletteItem[] = [
    { id: 'settings', label: 'Settings', hint: `${modLabel},`, run: actions.openSettings },
    { id: 'sign-out', label: 'Sign out', run: hooks.signOut },
  ]
  const more: PaletteItem[] = [
    { id: 'add', label: 'Add project', run: hooks.addProject },
    { id: 'theme-toggle', label: appearance.drawn === 'light' ? 'Switch to dark theme' : 'Switch to light theme', run: () => appearance.toggleTheme() },
    { id: 'tool-servers', label: 'Tool servers', hint: showKey('Alt+M'), run: actions.openTools },
    { id: 'accounts', label: 'Accounts and usage', run: actions.openAccounts },
    { id: 'ssh-hosts', label: 'SSH hosts', hint: showKey('Alt+S'), run: actions.openHosts },
    { id: 'lsp', label: 'Language servers', run: hooks.languageServers },
  ]
  if (open) {
    // Only when the editor in view has a language server that formats.
    const shown = app.layout ? focusedGroup(app.layout)?.active : undefined
    if (shown && files.formatters[shown]) more.unshift({ id: 'format', label: 'Format document', hint: showKey('Alt+Shift+F'), run: files.formatters[shown] })
    more.push(
      { id: 'history-show', label: 'Show history', hint: showKey('Alt+H'), run: () => actions.openHistory() },
      { id: 'changes-refresh', label: 'Refresh changes', run: () => git.refresh() },
      { id: 'tab-new', label: 'New tab menu', hint: showKey('Alt+T'), run: actions.openPlusMenu },
      { id: 'tab-close', label: 'Close tab', hint: showKey('Alt+W'), run: () => actions.closeTab() },
      { id: 'term-leave', label: 'Leave the terminal for its tab', hint: showKey('Alt+Esc'), run: actions.leaveTerminal },
      { id: 'pane-next', label: 'Focus next pane', hint: showKey('Alt+Down'), run: actions.nextPane },
      { id: 'tab-move-next', label: 'Move tab to next pane', hint: showKey('Alt+Shift+Right'), run: actions.moveToNextPane },
      { id: 'tab-move-new', label: 'Move tab to a new pane on the right', run: () => actions.moveToNewPane('right') },
    )
    for (const h of ssh.hosts) {
      more.push(
        { id: 'ssh-term-' + h.id, label: `Open a terminal on ${h.name}`, run: () => actions.terminalOn(h.id) },
        { id: 'ssh-files-' + h.id, label: `Browse files on ${h.name}`, run: () => actions.filesOn(h.id) },
      )
    }
    for (const w of git.worktrees) {
      more.push(
        { id: 'wt-changes-' + w.folder, label: `Show changes in worktree ${w.branch}`, run: () => actions.showChanges(w.folder) },
        { id: 'wt-history-' + w.folder, label: `Show history of worktree ${w.branch}`, run: () => actions.openHistory(w.folder) },
        { id: 'wt-term-' + w.folder, label: `New terminal in worktree ${w.branch}`, run: () => actions.terminalIn(w.folder, w.branch) },
      )
    }
  }
  const projects: PaletteItem[] = app.projects.map((p) => ({ id: 'project-' + p.id, label: `Switch to project ${p.name}`, run: () => app.select(p.id) }))
  return [...everyday, ...settings, ...more, ...projects]
}
