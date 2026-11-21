import { describe, expect, it } from 'vitest'
import { languageFor } from './langs'

describe('languageFor', () => {
  it('goes by extension, any case, any folder', () => {
    expect(languageFor('src/lib/a.ts')).toBe('ts')
    expect(languageFor('web/App.SVELTE')).toBe('svelte')
    expect(languageFor('x.test.tsx')).toBe('tsx')
    expect(languageFor('internal/a_test.go')).toBe('go')
  })
  it('knows files without an extension', () => {
    expect(languageFor('Dockerfile')).toBe('dockerfile')
    expect(languageFor('a/b/Makefile')).toBe('shell')
  })
  it('leaves unknown files as plain text', () => {
    expect(languageFor('notes.txt')).toBeNull()
    expect(languageFor('LICENSE')).toBeNull()
    expect(languageFor('a.unknownext')).toBeNull()
  })
})
