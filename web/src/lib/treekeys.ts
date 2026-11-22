// Keyboard moves in the file tree: arrows, Home and End walk the visible rows; F2, Delete and M (move) act on one.

export interface RowKeys {
  isDir: boolean
  open: boolean
  usable: boolean
  toggle: () => void
  rename: () => void
  remove: () => void
  move: () => void
}

const visible = (el: HTMLElement) => [...(el.closest('[role="tree"]')?.querySelectorAll<HTMLButtonElement>('button.main:not(:disabled)') ?? [])]
const go = (el: HTMLElement | null | undefined) => el?.focus()

export function treeKeys(e: KeyboardEvent, c: RowKeys) {
  if (e.altKey || e.ctrlKey || e.metaKey) return
  const here = e.currentTarget as HTMLElement
  const all = visible(here)
  const i = all.indexOf(here as HTMLButtonElement)
  const handled = () => e.preventDefault()
  switch (e.key) {
    case 'ArrowDown':
      handled()
      return go(all[i + 1])
    case 'ArrowUp':
      handled()
      return go(all[i - 1])
    case 'Home':
      handled()
      return go(all[0])
    case 'End':
      handled()
      return go(all[all.length - 1])
    case 'ArrowRight':
      handled()
      if (c.isDir && !c.open) c.toggle()
      else if (c.isDir) go(all[i + 1])
      return
    case 'ArrowLeft':
      handled()
      if (c.isDir && c.open) return c.toggle()
      return go(here.closest('ul')?.closest('li')?.querySelector<HTMLElement>('button.main'))
    case 'F2':
      if (!c.usable) return
      handled()
      return c.rename()
    case 'm':
    case 'M':
      if (!c.usable || e.shiftKey) return
      handled()
      return c.move()
    case 'Delete':
      if (!c.usable) return
      handled()
      return c.remove()
  }
}
