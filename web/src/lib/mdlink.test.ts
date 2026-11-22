import { describe, expect, it } from 'vitest'
import { classifyLink } from './mdlink'

const root = '/home/u/proj'
const none = { kind: 'none' }

describe('classifyLink', () => {
  it('lets http, https and mailto leave the page', () => {
    expect(classifyLink('https://example.com/a?b=1', root)).toEqual({ kind: 'web', href: 'https://example.com/a?b=1' })
    expect(classifyLink('http://example.com', root)).toMatchObject({ kind: 'web' })
    expect(classifyLink('mailto:a@b.test', root)).toEqual({ kind: 'web', href: 'mailto:a@b.test' })
  })
  it('refuses every other scheme, however it is written', () => {
    for (const h of ['javascript:alert(1)', 'JaVaScRiPt:alert(1)', ' javascript:alert(1)', 'java\nscript:alert(1)', 'java\tscript:1', 'data:text/html,<script>1</script>', 'vbscript:1', 'blob:https://x', 'ftp://x.test/a', 'tel:123', '//evil.test/x', '\u0001javascript:1']) {
      expect(classifyLink(h, root), h).toEqual(none)
    }
  })
  it('opens a path inside the project, with a line', () => {
    expect(classifyLink('src/a.ts', root)).toEqual({ kind: 'file', path: 'src/a.ts' })
    expect(classifyLink('./src/a.ts:12', root)).toEqual({ kind: 'file', path: 'src/a.ts', line: 12 })
    expect(classifyLink('src/a.ts#L30', root)).toEqual({ kind: 'file', path: 'src/a.ts', line: 30 })
    expect(classifyLink('/home/u/proj/src/a.ts', root)).toEqual({ kind: 'file', path: 'src/a.ts' })
    expect(classifyLink('file:///home/u/proj/a.go', root)).toEqual({ kind: 'file', path: 'a.go' })
  })
  it('keeps plain text for paths outside the project', () => {
    for (const h of ['../x', 'a/../../x', '/etc/passwd', '/home/u/proj-other/a', 'file:///etc/passwd', '..', '#top', '', '%00', 'a\\b']) {
      expect(classifyLink(h, root), h).toEqual(none)
    }
  })
  it('resolves dots inside the project', () => {
    expect(classifyLink('a/./b/../c.ts', root)).toEqual({ kind: 'file', path: 'a/c.ts' })
  })
})
