import { writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import { makeRepo, openProject } from './githelpers'
import { collectProblems, signIn } from './helpers'
import { stopChats } from './stopchats'

// Comments on diff lines go to a chat's message box as one text with file and line.

const used: string[] = []
test.afterEach(async ({ page }) => {
  if (used.length) await stopChats(page, used.splice(0))
})

test('diff comments: two comments become one draft with file and line', async ({ page }) => {
  test.setTimeout(120_000)
  const name = `cmt-${Date.now() % 1_000_000}-${test.info().repeatEachIndex}`
  const repo = makeRepo(name, { 'a.txt': 'alpha\nbeta\ngamma\n' })
  used.push(name)
  await signIn(page)
  const problems = collectProblems(page)
  await openProject(page, repo)
  writeFileSync(join(repo, 'a.txt'), 'alpha\nbeta two\ngamma\ndelta\n')

  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await page.keyboard.press('Alt+G')
  const changes = page.locator('section[aria-label^="Changes in"]')
  await changes.getByRole('button', { name: /a\.txt/ }).first().click()

  const comment = async (line: number, text: string) => {
    const row = changes.locator('.cl').filter({ has: page.getByRole('button', { name: `Comment on line ${line}`, exact: true }) }).first()
    await row.hover()
    await row.getByRole('button', { name: `Comment on line ${line}`, exact: true }).click()
    await row.getByPlaceholder('Comment for the agent').fill(text)
    await row.getByRole('button', { name: 'Add comment' }).click()
  }
  await comment(2, 'Why two?')
  await comment(4, 'Is delta needed?')
  await expect(changes.getByText('2 comments')).toBeVisible()

  await changes.getByRole('button', { name: 'Send to chat…' }).click()
  const draft = page.getByLabel('Message to Gemini')
  await expect(draft).toHaveValue(/a\.txt:2\n```\nbeta two\n```\nWhy two\?/)
  await expect(draft).toHaveValue(/a\.txt:4\n```\ndelta\n```\nIs delta needed\?/)
  // Sent comments are cleared.
  await page.keyboard.press('Alt+G')
  await expect(changes.getByText('2 comments')).toHaveCount(0)
  expect(problems).toEqual([])
})
