import type { IconName } from './icons'

/** The icon at the left of a tab, by the kind of tab. */
export function tabIcon(kind: string): IconName {
  switch (kind) {
    case 'terminal':
      return 'terminal'
    case 'chat':
      return 'chat'
    case 'file':
    case 'rfile':
      return 'file'
    case 'changes':
    case 'diff':
    case 'commit':
      return 'diff'
    case 'history':
      return 'history'
    case 'remote':
      return 'folder'
    default:
      return 'plus'
  }
}
