import { describe, expect, it } from 'vitest'
import { lspStatus } from './lspstatus'

const go = { id: 'gopls', name: 'Go (gopls)', state: 'installing' }

describe('lspStatus', () => {
  it('names the install and its last line', () => {
    expect(lspStatus([go], { gopls: 'go: downloading x' }, {})).toBe('Installing Go (gopls)… go: downloading x')
    expect(lspStatus([go], {}, {})).toBe('Installing Go (gopls)…')
  })
  it('says reconnecting, and is empty when all is well', () => {
    expect(lspStatus([{ ...go, state: 'ready' }], {}, { k: 'waiting' })).toBe('Reconnecting to the language server…')
    expect(lspStatus([{ ...go, state: 'ready' }], {}, { k: 'open' })).toBe('')
  })
})
