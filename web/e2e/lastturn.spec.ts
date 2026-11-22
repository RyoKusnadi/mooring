import { writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import { makeRepo, openProject } from './githelpers'
import { collectProblems, signIn } from './helpers'
import { stopChats } from './stopchats'

// "Last turn" in Changes: only what the agent wrote during its turn, not the user's own edits.
// The stand-in agent writes a file when the prompt is "WRITEFILE <path> <text>".

const used: string[] = []
test.afterEach(async ({ page }) => {
  if (used.length) await stopChats(page, used.splice(0))
})

test('last turn: lists only the file the agent wrote, with its diff', async ({ page }) => {
  test.setTimeout(120_000)
  const name = `turn-${Date.now() % 1_000_000}-${test.info().repeatEachIndex}`
  const repo = makeRepo(name, { 'a.txt': 'one\n' })
  used.push(name)
  await signIn(page)
  const problems = collectProblems(page)
  await openProject(page, repo)

  // The user edits a.txt before the turn; the agent then writes b.txt.
  writeFileSync(join(repo, 'a.txt'), 'one\nuser edit\n')
  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()

  await page.getByLabel('Message to Gemini').fill(`WRITEFILE ${join(repo, 'b.txt')} text from the agent`)
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect(panel.getByText(/^wrote:/)).toBeVisible()

  await page.keyboard.press('Alt+G')
  const changes = page.locator('section[aria-label^="Changes in"]')
  await changes.getByRole('button', { name: /^Last turn/ }).click()
  const files = changes.getByRole('list', { name: 'Files changed in the last turn' })
  await expect(files.getByRole('button', { name: /b\.txt/ })).toBeVisible()
  await expect(files.getByRole('listitem')).toHaveCount(1)
  await expect(changes.getByText('1 file changed by')).toBeVisible()
  await expect(changes.getByText('text from the agent')).toBeVisible()
  await expect(changes.getByText('user edit')).toHaveCount(0)

  // Back to uncommitted: the user's own edit is there.
  await changes.getByRole('button', { name: 'Uncommitted' }).click()
  await expect(changes.getByRole('button', { name: /a\.txt/ }).first()).toBeVisible()
  expect(problems).toEqual([])
})
