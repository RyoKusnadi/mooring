import { describe, expect, it } from 'vitest'
import { gitImageUrl } from './gitimage'

describe('gitImageUrl', () => {
  it('asks for the staged or the unstaged pair', () => {
    expect(gitImageUrl('p', '', 'a b.png', 'old', { staged: true })).toBe('/api/git/image?project=p&folder=&path=a%20b.png&side=old&staged=1')
    expect(gitImageUrl('p', 'w', 'x.png', 'new', { staged: false })).toBe('/api/git/image?project=p&folder=w&path=x.png&side=new&staged=0')
  })
  it('asks for a commit when there is one, and not for staged', () => {
    const u = gitImageUrl('p', '', 'x.png', 'new', { staged: false, commit: 'abc123' })
    expect(u).toContain('commit=abc123')
    expect(u).not.toContain('staged')
  })
})
