import { describe, expect, it } from 'vitest'
import type { Entry } from './filesapi'
import { crumbs, remoteTabsIn, retargetRemote, sortEntries, uploadMessage, uploadProblem } from './remotedir'

const e = (name: string, kind: Entry['kind'] = 'file'): Entry => ({ name, kind, size: 0, mtime: 0, hidden: false, ignored: false, link: false })

describe('remotedir', () => {
  it('lists folders first and numbers in order', () => {
    const names = sortEntries([e('b.txt'), e('src', 'dir'), e('file10'), e('file2'), e('A', 'dir')]).map((x) => x.name)
    expect(names).toEqual(['A', 'src', 'b.txt', 'file2', 'file10'])
  })

  it('builds breadcrumbs from the root', () => {
    expect(crumbs('', 'home')).toEqual([{ label: 'home', path: '' }])
    expect(crumbs('a/b', 'work')).toEqual([
      { label: 'work', path: '' },
      { label: 'a', path: 'a' },
      { label: 'b', path: 'a/b' },
    ])
  })

  it('refuses uploads over 10 MB and explains an existing name', () => {
    expect(uploadProblem('big.bin', 10 * 1024 * 1024 + 1)).toMatch(/over 10 MB/)
    expect(uploadProblem('ok.txt', 5)).toBe('')
    expect(uploadMessage('a.txt', 'ALREADY_EXISTS', 'x')).toMatch(/already exists/)
    expect(uploadMessage('a.txt', 'PERMISSION_DENIED', 'No.')).toBe('a.txt: No.')
  })
})

describe('remote tabs after rename', () => {
  const tab = (id: string, host: string, path: string) => ({ id, kind: 'rfile', title: 'Box: ' + path.split('/').pop(), host, path })
  const layout = { root: { tabs: [tab('a', 'h', 'dir/x.txt'), tab('b', 'other', 'dir/x.txt'), tab('c', 'h', 'dirty.txt')], active: 'a' }, focus: 'a' } as never
  it('finds and retargets only tabs of that host under the path', () => {
    expect(remoteTabsIn(layout, 'h', 'dir').map((t) => t.id)).toEqual(['a'])
    const out = retargetRemote(layout, 'h', 'dir', 'moved') as unknown as { root: { tabs: { id: string; path: string; title: string }[] } }
    expect(out.root.tabs.map((t) => t.path)).toEqual(['moved/x.txt', 'dir/x.txt', 'dirty.txt'])
    expect(out.root.tabs[0].title).toBe('Box: x.txt')
  })
})
