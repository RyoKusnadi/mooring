import { describe, expect, it } from 'vitest'
import { fileUri, formatMb, isFormatKey, languageIdFor, parseEndings, pathFromUri, restartNotice, serverFor, slugId, splitCommand, splitPlace, type LspServer } from './lspmatch'

const server = (id: string, endings: string[], extra: Partial<LspServer> = {}): LspServer => ({
  id,
  name: id,
  endings,
  languageId: id,
  state: 'ready',
  command: [id],
  path: '/bin/' + id,
  installable: false,
  installHint: '',
  custom: false,
  ...extra,
})

describe('serverFor', () => {
  const list = [server('gopls', ['.go']), server('typescript', ['.ts', '.tsx', '.js']), server('gotmpl', ['.go.tmpl'])]
  it('matches by file ending', () => {
    expect(serverFor(list, 'cmd/main.go')?.id).toBe('gopls')
    expect(serverFor(list, 'web/src/App.TSX')?.id).toBe('typescript')
  })
  it('prefers the longest ending', () => {
    expect(serverFor(list, 'x/page.go.tmpl')?.id).toBe('gotmpl')
  })
  it('lets a custom server win a tie', () => {
    const custom = server('mine', ['.go'], { custom: true })
    expect(serverFor([...list, custom], 'a.go')?.id).toBe('mine')
  })
  it('does not match a file that is only the ending, or no ending', () => {
    expect(serverFor(list, 'src/.go')).toBeNull()
    expect(serverFor(list, 'Makefile')).toBeNull()
    expect(serverFor(list, 'docs.go/readme')).toBeNull()
  })
})

describe('languageIdFor', () => {
  it('tells the typescript server about tsx and js files', () => {
    const ts = server('typescript', ['.ts'], { languageId: 'typescript' })
    expect(languageIdFor(ts, 'a.ts')).toBe('typescript')
    expect(languageIdFor(ts, 'a.tsx')).toBe('typescriptreact')
    expect(languageIdFor(ts, 'a.mjs')).toBe('javascript')
    expect(languageIdFor(server('gopls', ['.go']), 'a.go')).toBe('gopls')
  })
})

describe('places and uris', () => {
  it('splits a worktree path into place and file', () => {
    expect(splitPlace('.mooring/worktrees/fix-1/cmd/a.go')).toEqual({ place: '.mooring/worktrees/fix-1', rel: 'cmd/a.go' })
    expect(splitPlace('cmd/a.go')).toEqual({ place: '', rel: 'cmd/a.go' })
  })
  it('builds file uris from the project folder and the path', () => {
    expect(fileUri('/home/me/app', 'cmd/a.go')).toBe('file:///home/me/app/cmd/a.go')
    expect(fileUri('/home/me/app/', '.mooring/worktrees/w/a.go')).toBe('file:///home/me/app/.mooring/worktrees/w/a.go')
    expect(fileUri('/home/me/app', '')).toBe('file:///home/me/app')
  })
  it('escapes what a uri cannot hold, the way Go does', () => {
    expect(fileUri('/home/my app', 'a b/c#d?.go')).toBe('file:///home/my%20app/a%20b/c%23d%3F.go')
    expect(fileUri('/p', "it's(1).go")).toBe('file:///p/it%27s%281%29.go')
    expect(fileUri('/p', 'a+b,c@d:e.go')).toBe('file:///p/a+b,c@d:e.go')
  })
  it('turns uris back into project paths, and refuses outside ones', () => {
    const uri = fileUri('/home/my app', 'a b/ü.go')
    expect(pathFromUri('/home/my app', uri)).toBe('a b/ü.go')
    expect(pathFromUri('/home/me/app', 'file:///usr/lib/go/src/fmt/print.go')).toBeNull()
    expect(pathFromUri('/home/me/app', 'file:///home/me/app2/a.go')).toBeNull()
    expect(pathFromUri('/p', 'file:///p/%E0%A4%A')).toBeNull()
  })
})

describe('forms and notices', () => {
  it('splits commands with quotes', () => {
    expect(splitCommand('rust-analyzer --log "a b" \'c d\'')).toEqual(['rust-analyzer', '--log', 'a b', 'c d'])
    expect(splitCommand('   ')).toEqual([])
  })
  it('reads endings with or without dots', () => {
    expect(parseEndings('rs, .RLIB  rs')).toEqual(['.rs', '.rlib'])
  })
  it('makes a server id from a name', () => {
    expect(slugId('Rust Analyzer!')).toBe('rust-analyzer')
    expect(slugId('***')).toBe('')
    expect(slugId('x'.repeat(40)).length).toBe(32)
  })
  it('writes the restart notice', () => {
    expect(formatMb(150)).toBe('150 MB')
    expect(formatMb(2048)).toBe('2 GB')
    expect(restartNotice('gopls', 2150, 2048)).toBe('gopls used 2.1 GB, over its 2 GB limit, and was restarted.')
  })
})

describe('isFormatKey', () => {
  const key = (code: string, mods: Partial<KeyboardEvent> = {}) => ({ code, shiftKey: true, altKey: true, ctrlKey: false, metaKey: false, ...mods })
  it('takes Shift-Alt-F by physical key, whatever the layout types (Ï on macOS)', () => {
    expect(isFormatKey(key('KeyF'))).toBe(true)
    expect(isFormatKey(key('KeyF', { shiftKey: false }))).toBe(false)
    expect(isFormatKey(key('KeyF', { metaKey: true }))).toBe(false)
    expect(isFormatKey(key('KeyG'))).toBe(false)
  })
})
