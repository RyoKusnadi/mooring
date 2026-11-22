import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Page, type WebSocketRoute } from '@playwright/test'
import { collectProblems, fixtures, signIn } from './helpers'

// A switch for the events socket: while `down` is set every connection is closed at once.
async function flaky(page: Page) {
  const state = { down: false, open: [] as WebSocketRoute[] }
  await page.routeWebSocket(/\/api\/events/, (ws) => {
    if (state.down) return void ws.close()
    ws.connectToServer()
    state.open.push(ws)
  })
  return {
    cut() {
      state.down = true
      for (const w of state.open.splice(0)) void w.close()
    },
    restore() {
      state.down = false
    },
  }
}

async function addProject(page: Page, folder: string) {
  await page.getByRole('button', { name: 'Add project' }).first().click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Folder on this computer').fill(folder)
  await dialog.getByRole('button', { name: 'Go', exact: true }).click()
  await dialog.getByRole('button', { name: 'Open this folder' }).click()
  await expect(dialog).toBeHidden()
}

test('an unreachable server says so after 5 seconds, and Retry now connects', async ({ page }) => {
  test.setTimeout(60_000)
  const sock = await flaky(page)
  sock.cut()
  await signIn(page)
  const main = page.getByRole('main')
  await expect(main.getByText('Loading projects…')).toBeVisible()
  await expect(main.getByText(/not answering/)).toBeVisible({ timeout: 10_000 })
  sock.restore()
  await main.getByRole('button', { name: 'Retry now' }).click()
  await expect(main.getByText(/not answering|Loading projects/)).toHaveCount(0, { timeout: 5000 })
})

test('chat while offline: the header says Offline and a message waits to send', async ({ page }) => {
  test.setTimeout(90_000)
  const sock = await flaky(page)
  await signIn(page)
  const problems = collectProblems(page)
  const folder = join(fixtures, `ux2-${Date.now()}-${test.info().repeatEachIndex}`)
  mkdirSync(folder, { recursive: true })
  await addProject(page, folder)
  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  // Ready height only; Starting is not measured because the stand-in agent starts instantly.
  expect(await panel.locator('.tools').first().evaluate((el) => el.getBoundingClientRect().height)).toBeGreaterThanOrEqual(28)

  await expect(panel.getByText('Ready', { exact: true })).toBeVisible()
  sock.cut()
  await expect(panel.getByText('Offline', { exact: true })).toBeVisible({ timeout: 15_000 })
  await page.getByLabel('Message to Gemini').fill('hello')
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect(panel.getByText(/Waiting to send/)).toBeVisible()
  sock.restore()
  await expect(panel.getByText('Hello world', { exact: true })).toBeVisible({ timeout: 30_000 })
  expect(problems.filter((p) => !/WebSocket|ERR_|closed/i.test(p))).toEqual([])
})

test('on a phone, long lines in the editor wrap', async ({ page }) => {
  const folder = join(fixtures, `ux2wrap-${Date.now()}-${test.info().repeatEachIndex}`)
  mkdirSync(folder, { recursive: true })
  writeFileSync(join(folder, 'long.txt'), 'word '.repeat(200) + '\n')
  await signIn(page)
  await addProject(page, folder)
  await page.getByRole('region', { name: /^Files in/ }).getByRole('button', { name: 'long.txt', exact: true }).click()
  await expect(page.locator('[data-editor-view] .cm-content')).not.toHaveClass(/cm-lineWrapping/)
  await page.setViewportSize({ width: 390, height: 844 })
  await page.reload()
  await expect(page.locator('[data-editor-view] .cm-content')).toHaveClass(/cm-lineWrapping/, { timeout: 15_000 })
})
