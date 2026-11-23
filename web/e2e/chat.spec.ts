import { mkdirSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { collectProblems, fixtures, signIn } from './helpers'
import { stopChats } from './stopchats'

// The server runs with a stand-in agent behind the name "gemini" (see e2e/serve.sh).
// Its prompts: "hello" streams two chunks, "ask" asks for permission, "long" streams
// until cancelled, anything else is echoed.

// Stop each test's chats so the shared limit of 20 has room for later specs.
const used: string[] = []
test.afterEach(async ({ page }) => {
  if (used.length) await stopChats(page, used.splice(0))
})

async function addProject(page: Page, name: string) {
  const folder = join(fixtures, name)
  mkdirSync(folder, { recursive: true })
  used.push(name)
  await page.getByRole('button', { name: 'Add project' }).first().click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Folder on this computer').fill(folder)
  await dialog.getByRole('button', { name: 'Go', exact: true }).click()
  await dialog.getByRole('button', { name: 'Open this folder' }).click()
  await expect(dialog).toBeHidden()
}

async function say(page: Page, text: string) {
  await page.getByLabel('Message to Gemini').fill(text)
  await page.getByRole('button', { name: 'Send', exact: true }).click()
}

test('chat: start, stream, permission, cancel, reload and resume, long history', async ({ page }) => {
  test.setTimeout(180_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  await signIn(page)
  const problems = collectProblems(page)
  await addProject(page, `chat-${run}`)

  // Start from a blank tab: the catalog shows what is installed and what is not.
  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New chat' }).click()
  await expect(panel.getByRole('heading', { name: 'Start a chat' })).toBeVisible()
  await expect(panel.getByText('Not installed.')).not.toHaveCount(0)
  await expect(panel.getByText('Not tested')).toHaveCount(0)
  await expect(panel.getByText(/npx|PATH/)).toHaveCount(0)
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()

  // Stream a reply into one message, with the context meter.
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await say(page, 'hello')
  await expect(panel.getByText('Hello world', { exact: true })).toBeVisible()
  await panel.getByRole('button', { name: /^Chat options/ }).click()
  await expect(panel.getByRole('meter')).toContainText('Context 1.2k of 200k')
  await page.keyboard.press('Escape')

  // A permission prompt shows on the tab, the project row and the sidebar, then is answered.
  await say(page, 'ask')
  const ask = panel.getByRole('alert', { name: 'Permission request' })
  await expect(ask).toBeVisible()
  await expect(page.getByRole('tab', { name: /Needs permission/ })).toBeVisible()
  await expect(page.getByRole('complementary').getByRole('img', { name: 'Needs permission' }).first()).toBeVisible()
  // One request, shown once: the banner has the full command and the folder; the transcript and header do not repeat it.
  await expect(ask).toContainText('npm test')
  await expect(ask).toContainText(' in ')
  await expect(panel.getByRole('alert', { name: 'Permission request' })).toHaveCount(1)
  await expect(panel.getByText('You allowed')).toHaveCount(0)
  await expect(page).toHaveTitle('(1) Mooring')
  await expect(page.getByRole('contentinfo').getByRole('button', { name: /needs permission/i })).toBeVisible()
  await ask.getByRole('button', { name: 'Allow' }).click()
  await expect(panel.getByText('ran it', { exact: true })).toBeVisible()
  await expect(ask).toBeHidden()
  await expect(panel.getByText('You allowed', { exact: true })).toBeVisible()
  await expect(page).toHaveTitle('Mooring')

  // Cancel a running reply.
  await say(page, 'long')
  await page.getByRole('button', { name: 'Stop reply' }).click()
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toBeVisible()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()

  // Reload: the chat is saved, opens from the sidebar and keeps its messages.
  await page.reload()
  const saved = page.getByRole('region', { name: /^Chats in/ }).locator('button.open').first()
  await expect(saved).toBeVisible()
  await saved.click()
  await expect(page.getByRole('tabpanel').getByText('Hello world', { exact: true })).toBeVisible()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await say(page, 'after reload')
  await expect(page.getByRole('tabpanel').getByText('echo: after reload', { exact: true })).toBeVisible()

  // A long history: 2,000 more messages, drawn by window, and still quick to move around.
  const id = await page.locator('[data-chat-view]').getAttribute('data-chat-view')
  await page.evaluate(async (chat) => {
    const ws = new WebSocket(`ws://${location.host}/api/events`)
    const pending = new Map<string, (f: { ok: boolean; error?: { code: string } }) => void>()
    await new Promise<void>((ok) => {
      ws.onopen = () => ws.send(JSON.stringify({ t: 'hello', epoch: '', pos: 0 }))
      ws.onmessage = (e) => {
        const f = JSON.parse(e.data)
        if (f.t === 'welcome') ok()
        if (f.t === 'result') pending.get(f.id)?.(f)
      }
    })
    for (let i = 0; i < 1000; i++) {
      for (;;) {
        const cid = crypto.randomUUID().replaceAll('-', '')
        const r = await new Promise<{ ok: boolean; error?: { code: string } }>((done) => {
          pending.set(cid, done)
          ws.send(JSON.stringify({ t: 'cmd', id: cid, name: 'chat.prompt', args: { id: chat, text: `line ${i}` } }))
        })
        if (r.ok) break
        if (r.error?.code !== 'BUSY') throw new Error('prompt failed: ' + r.error?.code)
        await new Promise((t) => setTimeout(t, 5))
      }
    }
    ws.close()
  }, id)
  await expect(page.getByRole('tabpanel').getByText('echo: line 999', { exact: true })).toBeVisible({ timeout: 60_000 })

  const rows = page.locator('[data-chat-view] .row[data-i]')
  expect(await rows.count()).toBeLessThan(120)
  const scroll = await page.evaluate(async () => {
    const vp = document.querySelector<HTMLElement>('[data-chat-view] .viewport')!
    const frame = () => new Promise((r) => requestAnimationFrame(() => r(null)))
    const t0 = performance.now()
    let worst = 0
    for (let y = vp.scrollHeight; y > 0; y -= vp.scrollHeight / 40) {
      const s = performance.now()
      vp.scrollTop = y
      await frame()
      worst = Math.max(worst, performance.now() - s)
    }
    vp.scrollTop = 0
    await frame()
    return { worst, total: performance.now() - t0, drawn: vp.querySelectorAll('.row').length }
  })
  expect(scroll.worst).toBeLessThan(250)
  expect(scroll.drawn).toBeLessThan(120)
  await page.getByRole('button', { name: 'Jump to latest' }).click()
  await expect(page.getByRole('tabpanel').getByText('echo: line 999', { exact: true })).toBeVisible()

  // Switching to another project and back stays quick: the open chat is kept in memory.
  await addProject(page, `chat-other-${run}`)
  const back = page.getByRole('button', { name: `chat-${run}`, exact: true })
  const ms = await back.evaluate(async (el) => {
    const t = performance.now()
    ;(el as HTMLElement).click()
    await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)))
    return performance.now() - t
  })
  expect(ms).toBeLessThan(250)
  expect(problems).toEqual([])
})

test('chat on a phone screen: the page does not scroll sideways', async ({ page, browser }) => {
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  await signIn(page)
  await addProject(page, `phone-${run}`)
  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await say(page, 'ask')
  await expect(panel.getByRole('alert', { name: 'Permission request' })).toBeVisible()

  // The same project on a phone: the dock, the header chip and three tabs are all showing.
  await page.waitForTimeout(1000) // the layout is saved to the server a moment after an edit
  const phone = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true, storageState: await page.context().storageState() })
  const p = await phone.newPage()
  await p.goto('/')
  await p.getByRole('button', { name: 'Projects', exact: true }).click()
  await p.getByRole('button', { name: `phone-${run}`, exact: true }).click()
  const wide = () => p.evaluate(() => document.documentElement.scrollWidth - innerWidth)
  await expect(p.getByRole('alert', { name: 'Permission request' })).toBeVisible()
  expect(await wide()).toBeLessThanOrEqual(0)
  await p.getByRole('button', { name: 'New tab' }).click()
  await p.getByRole('menuitem', { name: 'Terminal' }).click()
  expect(await wide()).toBeLessThanOrEqual(0)
  // Every way to start something stays reachable from the + menu.
  await p.getByRole('button', { name: 'New tab' }).click()
  for (const name of ['Terminal', 'Chat', 'Open file']) await expect(p.getByRole('menuitem', { name })).toBeVisible()
  await phone.close()
})

test('chat: a running command shows its output before it ends', async ({ page }) => {
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  await signIn(page)
  await addProject(page, `stream-${run}`)
  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await say(page, 'STREAM')
  const out = panel.getByLabel('Last lines of output')
  await expect(out).toContainText('step 2 done')
  await expect(panel.getByText('Build finished.', { exact: true })).toBeHidden()
  await expect(panel.getByText('Build finished.', { exact: true })).toBeVisible()
  await expect(out).toContainText('step 5 done')
  await expect(out).not.toContainText('step 5 done\nstep 5 done')
})
