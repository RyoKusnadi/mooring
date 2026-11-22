import { describe, expect, it } from 'vitest'
import type { LSPClient } from '@codemirror/lsp-client'
import type { EditorView } from '@codemirror/view'
import { Text } from '@codemirror/state'
import { MooringWorkspace } from './lspworkspace'

function setup(pathOf: (uri: string) => string | null = (u) => u.replace('file:///p/', '')) {
  const calls: string[] = []
  const client = {
    didOpen: (f: { uri: string; version: number }) => calls.push(`open ${f.uri} v${f.version}`),
    didClose: (uri: string) => calls.push(`close ${uri}`),
  } as unknown as LSPClient
  const said: string[] = []
  const shown: string[] = []
  const view = (text: string) => ({ state: { doc: Text.of(text.split('\n')) }, plugin: () => null }) as unknown as EditorView
  const ws = new MooringWorkspace(client, {
    pathOf,
    say: (t) => said.push(t),
    show: async (p) => (shown.push(p), p === 'gone.go' ? null : view('x')),
  })
  return { ws, calls, said, shown, view }
}

describe('MooringWorkspace', () => {
  it('opens a file with version 0 and counts up after each open', () => {
    const { ws, calls, view } = setup()
    const v = view('a')
    ws.openFile('file:///p/a.go', 'go', v)
    ws.closeFile('file:///p/a.go', v)
    ws.openFile('file:///p/a.go', 'go', v)
    expect(calls).toEqual(['open file:///p/a.go v0', 'close file:///p/a.go', 'open file:///p/a.go v1'])
  })

  it('lets a new editor take a file from an old one, and only the holder closes it', () => {
    const { ws, calls, view } = setup()
    const [one, two] = [view('a'), view('a')]
    ws.openFile('file:///p/a.go', 'go', one)
    ws.openFile('file:///p/a.go', 'go', two)
    ws.closeFile('file:///p/a.go', one)
    expect(ws.files).toHaveLength(1)
    ws.closeFile('file:///p/a.go', two)
    expect(ws.files).toHaveLength(0)
    expect(calls.filter((c) => c.startsWith('close'))).toHaveLength(2)
  })

  it('opens every held file again with the current text after a reconnect', () => {
    const { ws, calls, view } = setup()
    ws.openFile('file:///p/a.go', 'go', view('a'))
    ws.openFile('file:///p/b.go', 'go', view('b'))
    calls.length = 0
    ws.connected()
    expect(calls).toEqual(['open file:///p/a.go v1', 'open file:///p/b.go v1'])
  })

  it('finds only files an editor holds', async () => {
    const { ws, view } = setup()
    ws.openFile('file:///p/a.go', 'go', view('a'))
    expect(ws.getFile('file:///p/a.go')?.languageId).toBe('go')
    expect(await ws.requestFile('file:///p/other.go')).toBeNull()
  })

  it('shows another file by its project path', async () => {
    const { ws, shown, said } = setup()
    expect(await ws.displayFile('file:///p/lib/x.go')).not.toBeNull()
    expect(shown).toEqual(['lib/x.go'])
    expect(said).toEqual([])
  })

  it('says so when a file is outside the project or cannot open', async () => {
    const { ws, said } = setup((u) => (u.endsWith('gone.go') ? 'gone.go' : null))
    expect(await ws.displayFile('file:///usr/lib/x.go')).toBeNull()
    expect(await ws.displayFile('file:///p/gone.go')).toBeNull()
    expect(said).toEqual(['That file is outside this project, so Mooring cannot open it here.', 'Could not open gone.go.'])
  })
})
