// Dragging in the file tree: a row moved onto a folder, or files from the desktop dropped on one.
// A dragged tab (Pane and TabStrip use their own type) is neither, so the tree ignores it.
import { isInside, parentPath } from './paths'

export const ROW_TYPE = 'application/x-mooring-row'

export type DragKind = 'row' | 'files' | 'other'
export type Dropped = { kind: 'move'; from: string } | { kind: 'upload'; files: File[] } | { kind: 'folders' }

/** The browser hides the data of a drag until the drop, so the row being dragged is kept here. */
let dragging = ''

export function dragKind(types: readonly string[] | undefined): DragKind {
  if (!types) return 'other'
  if (types.includes(ROW_TYPE)) return 'row'
  return types.includes('Files') ? 'files' : 'other'
}

/** False when moving `from` into `toDir` changes nothing or puts a folder inside itself. */
export function canMove(from: string, toDir: string): boolean {
  if (!from || parentPath(from) === toDir) return false
  return !isInside(toDir, from)
}

export function startDrag(e: DragEvent, path: string) {
  dragging = path
  e.dataTransfer?.setData(ROW_TYPE, path)
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'copyMove'
}

export const endDrag = () => (dragging = '')

/** Call from dragover on a folder (or the empty space, where toDir is ""). True when a drop here would do something. */
export function acceptDrag(e: DragEvent, toDir: string): boolean {
  const kind = dragKind(e.dataTransfer?.types)
  if (kind === 'other' || (kind === 'row' && !canMove(dragging, toDir))) return false
  e.preventDefault()
  e.stopPropagation()
  if (e.dataTransfer) e.dataTransfer.dropEffect = kind === 'row' ? 'move' : 'copy'
  return true
}

/** Call from drop. Null when the drop is not for the tree. */
export function readDrop(e: DragEvent, toDir: string): Dropped | null {
  if (!acceptDrag(e, toDir)) return null
  const kind = dragKind(e.dataTransfer?.types)
  const files = [...(e.dataTransfer?.files ?? [])]
  // A folder dropped from the desktop shows up as an empty "file"; folders are not uploaded yet.
  const items = [...(e.dataTransfer?.items ?? [])]
  const plain = files.filter((_, i) => items[i]?.webkitGetAsEntry?.()?.isDirectory !== true)
  const from = dragging
  dragging = ''
  if (kind === 'row') return { kind: 'move', from }
  if (plain.length) return { kind: 'upload', files: plain }
  return files.length ? { kind: 'folders' } : null
}
