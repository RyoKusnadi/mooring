import { expect, test } from '@playwright/test'
import { makeRepo, openProject } from './githelpers'
import { collectProblems, signIn } from './helpers'
import { stopChats } from './stopchats'

// Chats left running count against the shared server's limit of 20 (see stopchats.ts).
const used: string[] = []
test.afterEach(async ({ page }) => {
  if (used.length) await stopChats(page, used.splice(0))
})

// "grok" is the stand-in agent that lists and reopens its own sessions (see e2e/serve.sh). Deleting
// a chat in Mooring leaves the agent's own session behind, which is what gets brought in.

test('a chat the agent kept can be brought in, with its messages', async ({ page }) => {
  test.setTimeout(120_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  used.push(`import-${run}`)
  const repo = makeRepo(`import-${run}`, { 'a.txt': 'one\n' })
  await signIn(page)
  const problems = collectProblems(page)
  await openProject(page, repo)
  const panel = page.getByRole('tabpanel')

  await panel.getByRole('button', { name: 'New chat' }).click()
  const importBox = panel.locator('details.import')
  await importBox.getByText("Bring in a chat from an agent's own program").click()
  await importBox.getByLabel('Agent').selectOption({ label: 'Grok' })
  await importBox.getByRole('button', { name: 'Find chats from Grok' }).click()
  await expect(importBox.getByText(/has no chats for this folder/)).toBeVisible()

  await panel.getByRole('button', { name: 'Start chat with Grok' }).click()
  await expect(page.getByLabel('Message to Grok')).toBeEnabled()
  await page.getByLabel('Message to Grok').fill('hello')
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect(panel.getByText('Hello world').first()).toBeVisible()

  const side = page.getByRole('region', { name: /^Chats in/ })
  await side.getByRole('button', { name: /^Delete chat/ }).first().click()
  // Grok cannot delete its own copy, so the question stays short.
  await expect(side.getByText('Delete this chat?', { exact: true })).toBeVisible()
  await side.getByRole('button', { name: 'Delete', exact: true }).click()
  await expect(side.getByRole('button', { name: /^Delete chat/ })).toHaveCount(0)

  await page.keyboard.press('Alt+A')
  const again = page.getByRole('tabpanel').locator('details.import')
  await again.getByText("Bring in a chat from an agent's own program").click()
  await again.getByLabel('Agent').selectOption({ label: 'Grok' })
  await again.getByRole('button', { name: 'Find chats from Grok' }).click()
  const list = again.getByRole('list', { name: 'Chats from Grok' })
  await expect(list.getByRole('listitem')).toHaveCount(1)
  await list.getByRole('button', { name: /^Bring in/ }).click()

  const chat = page.getByRole('tabpanel')
  await expect(chat.getByText('hello').first()).toBeVisible()
  await expect(chat.getByText('Hello world').first()).toBeVisible()
  await expect(page.getByLabel('Message to Grok')).toBeEnabled()

  // It is gone from the list now that a chat uses it.
  await page.keyboard.press('Alt+A')
  const third = page.getByRole('tabpanel').locator('details.import')
  await third.getByText("Bring in a chat from an agent's own program").click()
  await third.getByLabel('Agent').selectOption({ label: 'Grok' })
  await third.getByRole('button', { name: 'Find chats from Grok' }).click()
  await expect(third.getByText(/has no chats for this folder/)).toBeVisible()

  await page.setViewportSize({ width: 390, height: 844 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true)
  expect(problems).toEqual([])
})

test('deleting a chat says when the agent deletes its own copy too', async ({ page }) => {
  test.setTimeout(60_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  used.push(`delcopy-${run}`)
  const repo = makeRepo(`delcopy-${run}`, { 'a.txt': 'one\n' })
  await signIn(page)
  await openProject(page, repo)
  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  const side = page.getByRole('region', { name: /^Chats in/ })
  await side.getByRole('button', { name: /^Delete chat/ }).first().click()
  await expect(side.getByText('Delete this chat? Mooring removes it and asks Gemini to delete its own copy.')).toBeVisible()
  await page.setViewportSize({ width: 390, height: 844 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true)
})
