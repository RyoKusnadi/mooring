import { describe, expect, it } from 'vitest'
import { acceptDrag, canMove, dragKind, endDrag, readDrop, ROW_TYPE, startDrag } from './treedrag'

function drag(types: string[], files: File[] = []) {
  const data: Record<string, string> = {}
  const e = {
    dataTransfer: { types, files, items: [], dropEffect: 'none', effectAllowed: 'none', setData: (t: string, v: string) => ((data[t] = v), types.push(t)) },
    prevented: false,
    stopped: false,
    preventDefault() {
      this.prevented = true
    },
    stopPropagation() {
      this.stopped = true
    },
  }
  return e as unknown as DragEvent & { prevented: boolean; stopped: boolean }
}

describe('dragKind', () => {
  it('tells a tree row, desktop files and a tab apart', () => {
    expect(dragKind([ROW_TYPE])).toBe('row')
    expect(dragKind(['Files'])).toBe('files')
    expect(dragKind(['application/x-mooring-tab'])).toBe('other')
    expect(dragKind(['text/plain'])).toBe('other')
    expect(dragKind(undefined)).toBe('other')
  })
})

describe('canMove', () => {
  it('refuses a drop on itself, on a child and on the folder it is already in', () => {
    expect(canMove('src', 'src')).toBe(false)
    expect(canMove('src', 'src/lib')).toBe(false)
    expect(canMove('src/a.ts', 'src')).toBe(false)
    expect(canMove('a.ts', '')).toBe(false)
    expect(canMove('', 'src')).toBe(false)
  })
  it('allows other folders, and a name that only shares a prefix', () => {
    expect(canMove('a.ts', 'src')).toBe(true)
    expect(canMove('src/a.ts', '')).toBe(true)
    expect(canMove('src', 'src2')).toBe(true)
    expect(canMove('src/lib', 'docs')).toBe(true)
  })
})

describe('accepting a drag', () => {
  it('takes a dragged row for another folder and marks it as a move', () => {
    const start = drag([])
    startDrag(start, 'a.ts')
    const over = drag([ROW_TYPE])
    expect(acceptDrag(over, 'src')).toBe(true)
    expect(over.prevented && over.stopped).toBe(true)
    expect(over.dataTransfer!.dropEffect).toBe('move')
    expect(readDrop(drag([ROW_TYPE]), 'src')).toEqual({ kind: 'move', from: 'a.ts' })
    endDrag()
  })

  it('refuses a row dropped on itself or a child', () => {
    startDrag(drag([]), 'src')
    const self = drag([ROW_TYPE])
    expect(acceptDrag(self, 'src')).toBe(false)
    expect(self.prevented).toBe(false)
    expect(readDrop(drag([ROW_TYPE]), 'src/lib')).toBeNull()
    endDrag()
  })

  it('takes desktop files as an upload and leaves a tab drag alone', () => {
    const f = new File(['x'], 'x.txt')
    const e = drag(['Files'], [f])
    expect(readDrop(e, 'src')).toEqual({ kind: 'upload', files: [f] })
    const tab = drag(['application/x-mooring-tab'])
    expect(acceptDrag(tab, 'src')).toBe(false)
    expect(tab.prevented).toBe(false)
  })

  it('reports a drop that held only folders', () => {
    const f = new File([''], 'dir')
    const e = drag(['Files'], [f])
    Object.assign(e.dataTransfer as object, { items: [{ webkitGetAsEntry: () => ({ isDirectory: true }) }] })
    expect(readDrop(e, 'src')).toEqual({ kind: 'folders' })
  })
})
