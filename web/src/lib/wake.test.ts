import { afterEach, describe, expect, it, vi } from 'vitest'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.resetModules()
})

describe('onWake', () => {
  it('runs on online and on the tab being shown, until released', async () => {
    const win = new EventTarget()
    const doc = Object.assign(new EventTarget(), { hidden: false })
    vi.stubGlobal('window', win)
    vi.stubGlobal('document', doc)
    const { onWake } = await import('./wake')
    let n = 0
    const off = onWake(() => n++)
    win.dispatchEvent(new Event('online'))
    doc.dispatchEvent(new Event('visibilitychange'))
    doc.hidden = true
    doc.dispatchEvent(new Event('visibilitychange'))
    expect(n).toBe(2)
    off()
    win.dispatchEvent(new Event('online'))
    expect(n).toBe(2)
  })
})
