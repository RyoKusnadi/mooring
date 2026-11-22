import { describe, expect, it } from 'vitest'
import { operationText, sideLabels } from './gitoperation'

describe('gitoperation', () => {
  it('words the sides by operation', () => {
    expect(sideLabels('merge')).toEqual({ ours: 'Keep current branch', theirs: 'Keep incoming' })
    expect(sideLabels('rebase')).toEqual({ ours: 'Keep the base', theirs: 'Keep your commit' })
    expect(sideLabels(undefined).ours).toBe('Keep current branch')
  })
  it('names the finish button and the reason it waits', () => {
    expect(operationText('merge', 2).finish).toBe('Finish merge')
    expect(operationText('rebase', 1).finish).toBe('Continue rebase')
    expect(operationText('rebase', 1).why).toBe('Resolve the conflicted file first.')
    expect(operationText('revert', 0).why).toBe('')
    expect(operationText('merge', 3).body).toContain('3 files still have conflicts')
    expect(operationText('am', 0).title).toBe('Patch apply in progress.')
  })
})
