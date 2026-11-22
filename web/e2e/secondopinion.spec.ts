import { writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import { makeRepo, openProject } from './githelpers'
import { collectProblems, signIn } from './helpers'
import { stopChats } from './stopchats'

// Chats left running count against the shared server's limit of 20 (see stopchats.ts).
const used: string[] = []
test.afterEach(async ({ page }) => {
  if (used.length) await stopChats(page, used.splice(0))
})

// A second opinion: the summary and the uncommitted changes of a Gemini chat go to a chat with
// another agent. The stand-ins are "gemini" and "grok" (see e2e/serve.sh).

test('second opinion: another agent, the review request in the box, the changes in the context', async ({ page }) => {
  test.setTimeout(120_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  used.push(`opinion-${run}`)
  const repo = makeRepo(`opinion-${run}`, { 'plan.txt': 'one\n' })
  writeFileSync(join(repo, 'plan.txt'), `one\ntwo\n${'long'.repeat(80)}\n`)
  await signIn(page)
  const problems = collectProblems(page)
  await openProject(page, repo)

  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await page.getByLabel('Message to Gemini').fill('hello')
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect(panel.getByText('hello').first()).toBeVisible()

  await panel.getByRole('button', { name: /^Chat options/ }).click()
  await panel.getByRole('button', { name: 'Get a second opinion' }).click()
  await expect(panel.getByRole('textbox', { name: /^Second opinion on/ })).toBeVisible()
  await expect(panel.getByText('uncommitted changes in the earlier chat')).toBeVisible()
  await expect(panel.getByText('may include edits that are not from that chat')).toBeVisible()

  // The same agent is not the primary choice and has no "Start with" button.
  await expect(panel.getByRole('button', { name: /^Start with / })).toHaveCount(0)
  await expect(panel.locator('li button.primary')).toHaveCount(1)
  await expect(panel.locator('li button.primary')).not.toContainText('Gemini')
  await panel.getByRole('button', { name: 'Start chat with Grok' }).click()

  const box = page.getByLabel('Message to Grok')
  await expect(box).toBeEnabled()
  await expect(box).toHaveValue(/^Review the work of the earlier chat/)
  await page.getByRole('button', { name: 'Send', exact: true }).click()

  await expect(panel.getByText('echo: Summary of an earlier chat:').first()).toBeVisible()

  // On a phone the long diff line stays inside the block: the page does not scroll sideways.
  // The size changes first: the phone layout draws the chat again, which would close open details.
  await page.setViewportSize({ width: 390, height: 844 })
  await panel.getByText('Summary this chat started from').click()
  const changes = panel.getByText('Uncommitted changes sent with the summary')
  await expect(changes).toBeVisible()
  await changes.click()
  const pre = panel.locator('details.changes pre').filter({ hasText: 'plan.txt' })
  await expect(pre).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(0)
  await expect
    .poll(async () => {
      const b = await pre.boundingBox()
      return !!b && b.x >= 0 && b.x + b.width <= 390
    })
    .toBe(true)
  expect(problems).toEqual([])
})
