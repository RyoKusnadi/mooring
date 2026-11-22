import type { Page } from '@playwright/test'

// The shared test server runs at most 20 chats at once, and spec files run side by side.
// A spec that leaves chats running calls this at the end; it stops only its own projects' chats.
// null stops the chats of every project (stopall.setup.ts, between the test projects).
export async function stopChats(page: Page, projectNames: string[] | null): Promise<void> {
  const left = await page.evaluate(async (names) => {
    const ws = new WebSocket(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/api/events`)
    const waiting = new Set<string>()
    return await new Promise<string>((resolve) => {
      const done = (why: string) => {
        ws.close()
        resolve(why)
      }
      setTimeout(() => done(`timed out with ${waiting.size} chats not stopped`), 10_000)
      ws.onopen = () => ws.send(JSON.stringify({ t: 'hello', epoch: '', pos: 0 }))
      ws.onmessage = (e) => {
        const f = JSON.parse(String(e.data))
        if (f.t === 'welcome') {
          const s = f.snapshot ?? {}
          const ids = (s.projects ?? []).filter((p: { name: string }) => !names || names.includes(p.name)).map((p: { id: string }) => p.id)
          for (const pid of ids) {
            for (const c of s.chats?.[pid] ?? []) {
              if (c.state === 'stopped') continue
              const id = `stop-${c.id}-${Date.now()}`
              waiting.add(id)
              ws.send(JSON.stringify({ t: 'cmd', id, name: 'chat.close', args: { id: c.id } }))
            }
          }
          if (waiting.size === 0) done('')
        } else if (f.t === 'result' && waiting.delete(f.id) && waiting.size === 0) {
          done('')
        }
      }
    })
  }, projectNames)
  if (left) throw new Error(`stopChats: ${left}`)
}
