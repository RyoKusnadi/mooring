import { mkdirSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { collectProblems, fixtures, signIn } from './helpers'
import { stopChats } from './stopchats'

// Chats left running count against the shared server's limit of 20 (see stopchats.ts).
const used: string[] = []
test.afterEach(async ({ page }) => {
  if (used.length) await stopChats(page, used.splice(0))
})

// The status board lists chats of every project by what they need. Stand-in agents: "gemini" and
// "grok" (see e2e/serve.sh). "ask" asks for permission, "hello" replies "Hello world".

async function addProject(page: Page, name: string) {
  const folder = join(fixtures, name)
  mkdirSync(folder, { recursive: true })
  await page.getByRole('button', { name: 'Add project' }).first().click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Folder on this computer').fill(folder)
  await dialog.getByRole('button', { name: 'Go', exact: true }).click()
  await dialog.getByRole('button', { name: 'Open this folder' }).click()
  await expect(dialog).toBeHidden()
}

test('board: answer a prompt, reply to a finished chat, opening a chat clears it', async ({ page }) => {
  test.setTimeout(120_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const name = `board-${run}`
  used.push(name)
  await signIn(page)
  const problems = collectProblems(page)
  await addProject(page, name)
  const panel = page.getByRole('tabpanel')

  // Chat A (Gemini) asks for permission, then the tab is left for a blank one.
  await panel.getByRole('button', { name: 'New chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await page.getByLabel('Message to Gemini').fill('ask')
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect(panel.getByRole('alert', { name: 'Permission request' })).toBeVisible()

  // Chat B (Grok) is started in a new tab and left for a terminal tab, idle.
  await page.getByRole('button', { name: 'New tab' }).click()
  await page.getByRole('menuitem', { name: 'Chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Grok' }).click()
  await expect(page.getByLabel('Message to Grok')).toBeEnabled()
  // The new chat can still take the focus as it settles, which closes the menu: open it again then.
  await expect(async () => {
    await page.getByRole('button', { name: 'New tab' }).click()
    await page.getByRole('menuitem', { name: 'Terminal' }).click({ timeout: 2_000 })
  }).toPass({ timeout: 20_000 })

  const side = page.getByRole('navigation', { name: 'Projects' })
  await side.getByRole('button', { name: /^Board/ }).click()
  const board = side.getByRole('region', { name: 'Status board' })
  const waiting = board.getByRole('region', { name: /^Waiting for you/ })
  const finished = board.getByRole('region', { name: /^Finished since you looked/ })
  const idle = board.getByRole('region', { name: /^Idle/ })
  const mine = (g: ReturnType<typeof board.getByRole>, agent: string) => g.getByRole('listitem').filter({ hasText: `${agent} · ${name}` })

  // The prompt is answered in the row; no chat tab opens.
  const tabs = await page.getByRole('tab').count()
  const rowA = mine(waiting, 'Gemini')
  await expect(rowA).toBeVisible()
  await rowA.getByRole('group', { name: 'Permission request' }).getByRole('button', { name: 'Allow' }).click()
  await expect(rowA).toBeHidden()
  await expect(page.getByRole('tab')).toHaveCount(tabs)

  // A reply sent from the board finishes in a chat nobody is looking at.
  const rowB = mine(idle, 'Grok')
  await rowB.getByLabel(/^Reply to Grok/).fill('hello')
  await rowB.getByRole('button', { name: 'Send', exact: true }).click()
  const doneB = mine(finished, 'Grok')
  await expect(doneB).toBeVisible()
  await expect(side.getByRole('button', { name: /^Board, .*finished since you looked/ })).toBeVisible()
  await expect(page.getByRole('tab')).toHaveCount(tabs)

  // Opening the chat moves it out of the group.
  await doneB.getByRole('button').first().click()
  await expect(page.getByRole('tab', { name: /Grok|hello/i }).first()).toBeVisible()
  await expect(page.getByRole('tabpanel').getByText('Hello world', { exact: true })).toBeVisible()
  await expect(mine(finished, 'Grok')).toHaveCount(0)
  await expect(mine(idle, 'Grok')).toBeVisible()
  expect(problems).toEqual([])
})

test('board: on a phone the page does not scroll sideways and targets are big enough', async ({ page, browser }) => {
  test.setTimeout(90_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const name = `boardphone-${run}`
  used.push(name)
  await signIn(page)
  await addProject(page, name)
  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await page.getByLabel('Message to Gemini').fill('hello')
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect(panel.getByText('Hello world', { exact: true })).toBeVisible()

  const phone = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true, storageState: await page.context().storageState() })
  const p = await phone.newPage()
  const phoneProblems = collectProblems(p)
  await p.goto('/')
  await p.getByRole('button', { name: 'Projects' }).click()
  const side = p.getByRole('navigation', { name: 'Projects' })
  await side.getByRole('button', { name: /^Board/ }).click()
  const row = side.getByRole('listitem').filter({ hasText: `Gemini · ${name}` })
  await expect(row).toBeVisible()
  expect(await p.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(0)
  expect(await side.evaluate((n) => n.scrollWidth - n.clientWidth)).toBeLessThanOrEqual(0)
  expect((await row.getByRole('button').first().boundingBox())?.height).toBeGreaterThanOrEqual(44)
  expect((await row.getByRole('button', { name: 'Send', exact: true }).boundingBox())?.height).toBeGreaterThanOrEqual(44)
  expect((await row.getByLabel(/^Reply to Gemini/).boundingBox())?.height).toBeGreaterThanOrEqual(44)
  expect(phoneProblems).toEqual([])
  await phone.close()
})
