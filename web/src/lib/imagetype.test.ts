import { describe, expect, it } from 'vitest'
import { isImagePath, isPdfPath, rawUrl } from './imagetype'

describe('image files', () => {
  it('knows the picture types by extension, in any case', () => {
    for (const p of ['a.png', 'x/b.JPG', 'c.jpeg', 'd.gif', 'e.webp', 'f.avif', 'g.bmp', 'h.ico', 'i.SVG']) expect(isImagePath(p)).toBe(true)
    for (const p of ['a.txt', 'png', 'a.png.html', 'a.pdf']) expect(isImagePath(p)).toBe(false)
  })
  it('builds an address with the parts escaped', () => {
    expect(rawUrl('p 1', 'a b/c&d.png', 3)).toBe('/api/files/raw?project=p%201&path=a%20b%2Fc%26d.png&v=3')
  })
  it('knows a PDF by its name', () => {
    expect(isPdfPath('a.pdf')).toBe(true)
    expect(isPdfPath('docs/B.PDF')).toBe(true)
    expect(isPdfPath('a.pdf.txt')).toBe(false)
  })
})
