import { describe, expect, it } from 'vitest'
import { findDiffTab, flipDiff, newDiffTab } from './difftabs'
import { parseLayout, singleTabLayout } from './layout'

describe('diff tabs', () => {
  const tab = newDiffTab('d1', '.mooring/worktrees/ab12', 'src/a.ts', false)
  const layout = singleTabLayout(tab)

  it('finds a tab by place, path and side', () => {
    expect(findDiffTab(layout, '.mooring/worktrees/ab12', 'src/a.ts', false)?.id).toBe('d1')
    expect(findDiffTab(layout, '', 'src/a.ts', false)).toBeNull()
    expect(findDiffTab(layout, '.mooring/worktrees/ab12', 'src/a.ts', true)).toBeNull()
  })

  it('flips to the staged side and renames the tab', () => {
    const next = flipDiff(layout, 'd1', true)
    expect(findDiffTab(next, '.mooring/worktrees/ab12', 'src/a.ts', true)?.title).toBe('Staged: a.ts')
    expect(layout).toEqual(singleTabLayout(tab))
  })

  it('keeps folder and side through a save and load', () => {
    expect(parseLayout(JSON.parse(JSON.stringify(flipDiff(layout, 'd1', true))))).toEqual(flipDiff(layout, 'd1', true))
  })
})
