import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { git, makeRepo, openProject } from './githelpers'
import { collectProblems, fixtures, openSettings, signIn } from './helpers'

// Token saving (phase 12) against the stand-in agent "gemini" (see e2e/serve.sh).
// Prompt words: "TERM <command>" runs the command through Mooring's terminal and reports its
// size; "Summarise this chat" gets a fixed summary; the stand-in has the models Fast and Deep;
// every turn reports 1,050 tokens.

async function say(page: Page, text: string) {
  await page.getByLabel('Message to Gemini').fill(text)
  await page.getByRole('button', { name: 'Send', exact: true }).click()
}

async function startChat(page: Page) {
  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  return panel
}

test('tokens: raw outputs, attachments, and a new chat from a summary', async ({ page }) => {
  test.setTimeout(120_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const folder = join(fixtures, `tokens-${run}`)
  mkdirSync(folder, { recursive: true })
  writeFileSync(join(folder, 'notes.txt'), 'first line\nsecond line\nthird line\n')
  await signIn(page)
  const problems = collectProblems(page)
  await openProject(page, folder)
  let panel = await startChat(page)

  // Shorten long output for this chat, then run a command with 1,000 lines of output.
  await panel.getByRole('button', { name: /^Chat options/ }).click()
  await panel.getByRole('checkbox', { name: 'Shorten long output' }).check()
  await page.keyboard.press('Escape')
  await say(page, 'TERM seq 1 1000')
  await expect(panel.getByText('lines=121', { exact: false }).first()).toBeVisible()
  await expect(panel.getByText('The agent got 120 of 1,000 lines')).toBeVisible()

  // Read all of it here, and send all of it with the next message.
  await panel.getByRole('switch', { name: 'Show raw output' }).check()
  await expect(panel.locator('pre').filter({ hasText: /^1\n2\n3/ })).toContainText('1000')
  await panel.getByRole('button', { name: 'Send full output' }).click()
  const composer = panel.getByRole('list', { name: 'Attached' })
  await expect(composer).toContainText('Command output, 1,000 lines')
  await say(page, 'look at this')
  // The chip left the composer and stays on the sent message: one list is left.
  await expect(panel.getByRole('list', { name: 'Attached' })).toHaveCount(1)
  await expect(panel.getByRole('region', { name: /messages$/ }).getByRole('list', { name: 'Attached' })).toContainText('Command output, 1,000 lines')

  // Lines of a file go to the chat as a chip with the path and the lines.
  await page.getByRole('button', { name: 'notes.txt' }).first().click()
  const editor = page.getByRole('tabpanel').locator('.cm-content')
  await editor.click()
  await page.keyboard.press('ControlOrMeta+a')
  await page.getByRole('button', { name: 'Ask the agent about these lines' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeVisible()
  const draft = page.locator('form').getByRole('list', { name: 'Attached' })
  await expect(draft).toContainText('notes.txt, lines 1 to 3')
  await page.getByRole('button', { name: 'Remove notes.txt, lines 1 to 3' }).click()
  await expect(draft).toHaveCount(0)

  // A new chat from a summary: edit the summary, start, and read it folded in the new chat.
  panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: /^Chat options/ }).click()
  await panel.getByRole('button', { name: 'New chat from summary' }).click()
  const summary = panel.getByRole('textbox', { name: /^Summary of/ })
  await expect(summary).toBeVisible()
  await expect(summary).not.toHaveValue('')
  await summary.fill((await summary.inputValue()) + '\nKeep the tests green.')
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  const ctx = panel.getByText('Summary this chat started from')
  await expect(ctx).toBeVisible()
  await ctx.click()
  await expect(panel.getByText('Keep the tests green.')).toBeVisible()

  // The old chat shows the prompt that asked for the summary, folded.
  await panel.getByRole('button', { name: /^Open / }).click()
  await expect(page.getByText('You asked for a summary of this chat')).toBeVisible()
  expect(problems).toEqual([])
})

test('tokens: settings, and a suggested commit message', async ({ page }) => {
  test.setTimeout(120_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const repo = makeRepo(`tokens-git-${run}`, { 'a.txt': 'one\n' })
  writeFileSync(join(repo, 'a.txt'), 'one\ntwo\n')
  git(repo, 'add', '-A')
  await signIn(page)
  const problems = collectProblems(page)
  await openProject(page, repo)

  // Settings: commit messages on, a small model for Gemini, a low daily limit.
  const dialog = await openSettings(page, 'Token saving')
  await expect(dialog.getByRole('heading', { name: 'Token saving' })).toBeVisible()
  await dialog.getByRole('checkbox', { name: /Suggest commit messages/ }).check()
  // Only Gemini's row: the other installed agents are real ones and must not be started.
  await dialog.getByRole('listitem').filter({ hasText: 'Gemini' }).getByRole('button', { name: 'Find models' }).click()
  const pick = dialog.getByLabel('Gemini')
  await expect(pick.getByRole('option', { name: 'Fast' })).toBeAttached()
  await pick.selectOption({ label: 'Fast' })
  await expect(dialog.getByRole('status').filter({ hasText: 'Saved.' })).toBeVisible()
  await dialog.getByLabel('Daily token limit for small jobs').fill('1000')
  await dialog.getByLabel('Daily token limit for small jobs').blur()
  await expect(dialog.getByRole('status').filter({ hasText: 'Saved.' })).toBeVisible()
  await dialog.getByLabel('Daily token limit for small jobs').fill('5')
  await dialog.getByLabel('Daily token limit for small jobs').blur()
  await expect(dialog.getByRole('alert')).toContainText('whole number from 1,000')
  await dialog.getByRole('button', { name: 'Close dialog' }).click()
  await expect(dialog).toBeHidden()

  // The commit box gets a Suggest message button. The first answer uses the limit up.
  await page.getByRole('button', { name: /^Changes/ }).click()
  await page.getByRole('button', { name: 'Open changes' }).click()
  const changes = page.getByRole('region', { name: /^Changes in tokens-git/ })
  await changes.getByRole('button', { name: 'Refresh' }).click()
  await changes.getByRole('button', { name: 'Suggest message' }).click()
  await expect(changes.getByLabel('Commit message')).toHaveValue('fake: commit subject')
  await expect(changes).toContainText('Read it before you commit.')
  await changes.getByLabel('Commit message').fill('')
  await changes.getByRole('button', { name: 'Suggest message' }).click()
  await expect(changes).toContainText('daily token limit for small jobs is used up')
  expect(problems).toEqual([])
})
