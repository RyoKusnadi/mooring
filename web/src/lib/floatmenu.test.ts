import { describe, expect, it, vi } from 'vitest'
import { floatMenu, place } from './floatmenu'

const rect = (left: number, top: number, w = 28, h = 28) => ({ left, top, right: left + w, bottom: top + h, width: w, height: h, x: left, y: top, toJSON: () => ({}) }) as DOMRect
const win = { width: 390, height: 800 }

describe('place', () => {
  it('opens under the button, right edges aligned', () => {
    expect(place(rect(300, 100), { width: 200, height: 120 }, win)).toEqual({ left: 128, top: 130 })
  })
  it('flips above when there is no room below', () => {
    expect(place(rect(300, 740), { width: 200, height: 120 }, win)).toEqual({ left: 128, top: 618 })
  })
  it('stays inside the window on the left and when nothing fits', () => {
    expect(place(rect(10, 100), { width: 200, height: 120 }, win).left).toBe(4)
    expect(place(rect(300, 10), { width: 200, height: 900 }, win).top).toBe(4)
  })
})

describe('floatMenu', () => {
  it('removes exactly the listeners it added, even after update', () => {
    const added: [string, unknown][] = []
    const removed: [string, unknown][] = []
    const win = {
      innerWidth: 390,
      innerHeight: 800,
      addEventListener: (t: string, f: unknown) => void added.push([t, f]),
      removeEventListener: (t: string, f: unknown) => void removed.push([t, f]),
    }
    vi.stubGlobal('window', win)
    vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} })
    const node = { style: {}, offsetWidth: 100, offsetHeight: 50, contains: () => false, showPopover() {}, hidePopover() {} } as unknown as HTMLElement
    const anchor = { getBoundingClientRect: () => rect(10, 10) } as unknown as HTMLElement
    const a = floatMenu(node, { anchor: () => anchor, close: () => {} })
    a.update({ anchor: () => anchor, close: () => {} })
    a.destroy()
    expect(added.length).toBe(2)
    expect(removed).toEqual(added)
    vi.unstubAllGlobals()
  })
})
