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

// The stand-in agent echoes any prompt that is not "hello", "ask" or "long" (see chat.spec.ts).

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

async function chatAbout(page: Page, text: string) {
  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await page.getByLabel('Message to Gemini').fill(text)
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect(panel.getByText(text).first()).toBeVisible()
  // The saved chat is searchable once its row shows in the sidebar.
  await expect(page.getByRole('region', { name: /^Chats in/ }).locator('button.open')).toHaveCount(1)
}

test('chat history: find a chat in another project, open it, by button, palette and key', async ({ page, browser }) => {
  test.setTimeout(120_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const a = `hist-a-${run}`
  const b = `hist-b-${run}`
  used.push(a, b)
  const wordA = `quokka${Date.now() % 100000}`
  const wordB = `marmot${Date.now() % 100000}`
  await signIn(page)
  const problems = collectProblems(page)
  await addProject(page, a)
  await chatAbout(page, `${wordA} wetlands survey`)
  await addProject(page, b)
  await chatAbout(page, `${wordB} river notes`)
  const projects = page.getByRole('navigation', { name: 'Projects' })
  await expect(projects.getByRole('button', { name: b, exact: true })).toHaveAttribute('aria-current', 'true')

  // The button in the sidebar opens the dialog with the newest chats from both projects.
  await page.getByRole('region', { name: /^Chats in/ }).getByRole('button', { name: 'Search chat history', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: 'Chat history' })
  await expect(dialog).toBeVisible()
  expect((await dialog.boundingBox())?.width).toBe(640)
  await expect(dialog.getByRole('region', { name: a })).toBeVisible()
  await expect(dialog.getByRole('region', { name: b })).toBeVisible()

  // A word from the other project's chat shows that chat, with the word marked.
  await dialog.getByLabel('Search all chats in every project').fill(wordA)
  await expect(dialog.getByRole('region', { name: b })).toHaveCount(0)
  const group = dialog.getByRole('region', { name: a })
  await expect(group.locator('mark').first()).toHaveText(wordA)
  await dialog.getByLabel('Search all chats in every project').fill(`${wordA} nothinghere`)
  await expect(dialog.getByText('No chat has all of these words.')).toBeVisible()
  await dialog.getByLabel('Search all chats in every project').fill(wordA)
  await group.getByRole('button').first().click()
  await expect(dialog).toBeHidden()
  await expect(projects.getByRole('button', { name: a, exact: true })).toHaveAttribute('aria-current', 'true')
  await expect(page.getByRole('tab', { name: /quokka|wetlands/i })).toBeVisible()

  // The palette opens it too; Down moves to a result and Enter opens it.
  await page.keyboard.press('ControlOrMeta+k')
  await page.getByRole('dialog', { name: 'Command palette' }).getByRole('combobox').fill('chat history')
  await page.keyboard.press('Enter')
  await expect(dialog).toBeVisible()
  await dialog.getByLabel('Search all chats in every project').fill(wordB)
  await expect(dialog.getByRole('region', { name: b }).locator('mark').first()).toHaveText(wordB)
  await page.keyboard.press('ArrowDown')
  await expect(dialog.getByRole('region', { name: b }).getByRole('button').first()).toBeFocused()
  await page.keyboard.press('Enter')
  await expect(dialog).toBeHidden()
  await expect(projects.getByRole('button', { name: b, exact: true })).toHaveAttribute('aria-current', 'true')
  await expect(page.getByRole('tab', { name: /marmot|river/i })).toBeVisible()

  // Alt+Y opens it, Escape closes it.
  await page.keyboard.press('Alt+y')
  await expect(dialog).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(dialog).toBeHidden()
  expect(problems).toEqual([])

  // On a phone: reached from Commands, no sideways scroll, rows big enough to tap.
  const phone = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true, storageState: await page.context().storageState() })
  const p = await phone.newPage()
  const phoneProblems = collectProblems(p)
  await p.goto('/')
  await p.getByRole('button', { name: /^Commands/ }).click()
  await p.getByRole('dialog', { name: 'Command palette' }).getByRole('combobox').fill('chat history')
  await p.keyboard.press('Enter')
  const small = p.getByRole('dialog', { name: 'Chat history' })
  await small.getByLabel('Search all chats in every project').fill(`${wordA} wetlands`)
  const row = small.getByRole('region', { name: a }).getByRole('button').first()
  await expect(row.locator('mark').first()).toHaveText(wordA)
  expect(await p.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(0)
  const box = await small.boundingBox()
  expect(box && box.x >= 0 && box.x + box.width <= 390).toBe(true)
  expect((await row.boundingBox())?.height).toBeGreaterThanOrEqual(44)
  await row.tap()
  await expect(small).toBeHidden()
  await expect(p.getByRole('tab', { name: /quokka|wetlands/i })).toBeVisible()
  expect(phoneProblems).toEqual([])
  await phone.close()
})
