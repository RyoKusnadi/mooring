import { describe, expect, it } from 'vitest'
import { fetchCatalog, fetchToolServers } from './toolapi'

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

describe('tool server reads', () => {
  it('reads the saved servers', async () => {
    const r = await fetchToolServers(async (url) => {
      expect(url).toBe('/api/toolservers')
      return json({ servers: [], canSaveSecrets: false })
    })
    expect(r.canSaveSecrets).toBe(false)
  })

  it('encodes the search and carries the cursor', async () => {
    let seen = ''
    await fetchCatalog('read me & more', 50, async (url) => ((seen = url), json({ entries: [], next: null, date: '2026-11-01' })))
    expect(seen).toBe('/api/toolservers/catalog?q=read%20me%20%26%20more&cursor=50')
    await fetchCatalog('', null, async (url) => ((seen = url), json({ entries: [], next: null, date: '' })))
    expect(seen).toBe('/api/toolservers/catalog?q=')
  })

  it('throws on a failed answer', async () => {
    await expect(fetchCatalog('x', null, async () => json({ code: 'CATALOG_FAILED', message: 'Catalog is unreadable.' }, 500))).rejects.toThrow()
  })
})
