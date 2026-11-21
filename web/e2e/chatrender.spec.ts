import { mkdirSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { collectProblems, fixtures, signIn } from './helpers'
import { openProject } from './githelpers'

// The stand-in agent "gemini" (see e2e/serve.sh). "SHOWCASE" answers in markdown with an edit and a command.
async function startChat(page: Page, name: string) {
  const folder = join(fixtures, name)
  mkdirSync(folder, { recursive: true })
  await openProject(page, folder)
  await page.getByRole('button', { name: 'New tab' }).click()
  await page.getByRole('menuitem', { name: 'Chat' }).click()
  await page.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
}

test('chat shows markdown safely, tool lines, and a changed-files row', async ({ page }) => {
  test.setTimeout(120_000)
  await signIn(page)
  const problems = collectProblems(page)
  await startChat(page, `render-${Date.now()}`)
  await page.getByLabel('Message to Gemini').fill('SHOWCASE')
  await page.getByRole('button', { name: 'Send' }).click()
  const feed = page.getByRole('region', { name: /messages$/ })
  await expect(feed.getByRole('heading', { name: 'Plan' })).toBeVisible()
  await expect(feed.getByRole('listitem').filter({ hasText: 'Run the tests' })).toBeVisible()
  await expect(feed.getByRole('table')).toContainText('main_test.go')
  await expect(feed.locator('pre code')).toContainText('println("hello")')
  await expect(feed.getByRole('button', { name: /^Copy/ }).first()).toBeVisible()
  // A web link opens in a new tab and cannot reach the opener; markup in the reply stays text.
  const link = feed.getByRole('link', { name: 'the docs' })
  await expect(link).toHaveAttribute('target', '_blank')
  await expect(link).toHaveAttribute('rel', 'noopener noreferrer')
  await expect(feed.getByText('<script>alert(1)</script>')).toBeVisible()
  expect(await feed.locator('script').count()).toBe(0)
  // One line per tool call, the diff of an edit, and the last 10 lines of a command.
  await expect(feed.getByText('Edited main.go +1 −1')).toBeVisible()
  await expect(feed.getByText('test line N ok')).toBeVisible()
  await expect(feed.getByText('test line A ok')).toHaveCount(0)
  await expect(feed.getByRole('button', { name: /^Changed 1 file/ })).toBeVisible()
  expect(problems).toEqual([])
})

test('chat queues a message while the agent replies and sends it after', async ({ page }) => {
  test.setTimeout(120_000)
  await signIn(page)
  const problems = collectProblems(page)
  await startChat(page, `queue-${Date.now()}`)
  await page.getByLabel('Message to Gemini').fill('long')
  await page.getByRole('button', { name: 'Send' }).click()
  await expect(page.getByRole('button', { name: 'Stop reply' })).toBeVisible()
  await page.getByLabel('Message to Gemini').fill('hello')
  await page.getByRole('button', { name: 'Send after reply' }).click()
  await expect(page.getByText('Sends when the reply ends')).toBeVisible()
  await page.getByRole('button', { name: 'Stop reply' }).click()
  // Stop hands the queued text back to the box; nothing is sent behind the user's back.
  await expect(page.getByLabel('Message to Gemini')).toHaveValue('hello')
  await expect(page.getByText('You stopped this reply.')).toBeVisible()
  expect(problems).toEqual([])
})

test('a queued message is kept in this browser and comes back after a reload', async ({ page }) => {
  test.setTimeout(120_000)
  await signIn(page)
  const problems = collectProblems(page)
  await startChat(page, `queuekeep-${Date.now()}`)
  const box = page.getByLabel('Message to Gemini')
  await box.fill('long')
  await page.getByRole('button', { name: 'Send' }).click()
  await expect(page.getByRole('button', { name: 'Stop reply' })).toBeVisible()
  await expect(page.getByText('Queued messages stay if the page reloads, but their attachments do not.')).toBeVisible()
  await box.fill('kept across reload')
  await page.getByRole('button', { name: 'Send after reply' }).click()
  await expect(page.getByText('Sends when the reply ends')).toBeVisible()
  const stored = () => page.evaluate(() => Object.entries(localStorage).filter(([k]) => k.startsWith('mooring.queue.')).map(([, v]) => (JSON.parse(v) as { texts: string[] }).texts))
  expect(await stored()).toEqual([['kept across reload']])

  await page.reload()
  const saved = page.getByRole('region', { name: /^Chats in/ }).locator('button.open').first()
  await expect(saved).toBeVisible()
  await saved.click()
  const back = page.getByLabel('Message to Gemini')
  await expect(back).toBeVisible()
  // The reply it waited for is gone, so the text returns to the box with a note; it is not sent by itself.
  const queued = page.getByRole('list', { name: 'Queued messages' }).getByText('kept across reload')
  await expect(async () => {
    const inBox = (await back.inputValue()).includes('kept across reload')
    expect(inBox || (await queued.count()) > 0).toBe(true)
  }).toPass({ timeout: 15_000 })
  // Once it is out of the queue, nothing is left in storage.
  await expect.poll(async () => ((await queued.count()) > 0 ? ['queued'] : await stored()), { timeout: 30_000 }).toEqual([])
  expect(problems).toEqual([])
})

test('chat header menu shows the context meter and shortening', async ({ page }) => {
  await signIn(page)
  const problems = collectProblems(page)
  await startChat(page, `menu-${Date.now()}`)
  await page.getByLabel('Message to Gemini').fill('hello')
  await page.getByRole('button', { name: 'Send' }).click()
  await expect(page.getByRole('region', { name: /messages$/ }).getByText('Hello world', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: /^Chat options/ }).click()
  await expect(page.getByRole('meter', { name: 'Context used' })).toBeVisible()
  await expect(page.getByText(/Cost so far/)).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('meter', { name: 'Context used' })).toHaveCount(0)
  expect(problems).toEqual([])
})

test('a wide table scrolls sideways and keeps its words whole', async ({ page }) => {
  test.setTimeout(90_000)
  await signIn(page)
  const problems = collectProblems(page)
  await startChat(page, `table-${Date.now()}`)
  const cells = Array.from({ length: 8 }, (_, i) => `Column${i}LongName`)
  const table = [`| ${cells.join(' | ')} |`, `| ${cells.map(() => '---').join(' | ')} |`, `| ${cells.map(() => 'a value of some length').join(' | ')} |`].join('\n')
  await page.getByLabel('Message to Gemini').fill(`## Heading\n\n${table}`)
  await page.getByRole('button', { name: 'Send' }).click()
  const feed = page.getByRole('region', { name: /messages$/ })
  const wrap = feed.locator('.md-table').first()
  await expect(wrap).toBeVisible()
  expect(await wrap.evaluate((el) => el.scrollWidth > el.clientWidth)).toBe(true)
  expect(problems).toEqual([])
})
