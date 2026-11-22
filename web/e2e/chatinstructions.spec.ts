import { mkdirSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { openProject } from './githelpers'
import { collectProblems, fixtures, openSettings, signIn } from './helpers'
import { stopChats } from './stopchats'

// Mooring's instructions for agents. The stand-in "gemini" does not take extra instructions
// (FAKE_CAPS has no systemPrompt), so a chat started with the switch on says so once.

const NOTICE = 'Gemini does not take extra instructions, so this chat starts without them.'
const used: string[] = []
test.afterEach(async ({ page }) => {
  if (used.length) await stopChats(page, used.splice(0))
})

async function startChat(page: Page) {
  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  return panel
}

test('instructions: the switch decides the notice, and a project text is kept', async ({ page }) => {
  test.setTimeout(90_000)
  const name = `instr-${Date.now() % 1_000_000}`
  const folder = join(fixtures, name)
  mkdirSync(folder, { recursive: true })
  used.push(name)
  await signIn(page)
  const problems = collectProblems(page)
  await openProject(page, folder)

  // On by default: the stand-in cannot take them, and the chat says so.
  let panel = await startChat(page)
  await expect(panel.getByText(NOTICE)).toBeVisible()

  const dialog = await openSettings(page, 'Token saving')
  const sw = dialog.getByRole('checkbox', { name: "Add Mooring's instructions to new chats" })
  await expect(sw).toBeChecked()
  await dialog.getByText('What is sent').click()
  await expect(dialog.getByText('You are running inside Mooring')).toBeVisible()

  // The project text: a count against 2,000, saved when the box is left, still there after a reload.
  const extra = dialog.getByLabel(`Extra instructions for ${name}`)
  await extra.fill('Answer in short sentences.')
  await expect(dialog.getByText('26 of 2,000 characters.')).toBeVisible()
  await extra.blur()
  await expect(dialog.getByRole('status').filter({ hasText: 'Saved.' })).toBeVisible()

  // Switched off: a new chat starts with no notice.
  await sw.uncheck()
  await expect(dialog.getByRole('status').filter({ hasText: 'Saved.' })).toBeVisible()
  await page.keyboard.press('Escape')
  await page.getByRole('button', { name: 'New tab' }).click()
  await page.getByRole('menuitem', { name: 'Chat' }).click()
  panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await expect(panel.getByText(NOTICE)).toHaveCount(0)

  await page.reload()
  const again = await openSettings(page, 'Token saving')
  await expect(again.getByRole('checkbox', { name: "Add Mooring's instructions to new chats" })).not.toBeChecked()
  await expect(again.getByLabel(`Extra instructions for ${name}`)).toHaveValue('Answer in short sentences.')

  // Put the shared server back as it was.
  await again.getByLabel(`Extra instructions for ${name}`).fill('')
  await again.getByLabel(`Extra instructions for ${name}`).blur()
  // Wait for that save, so the switch is not clicked while the dialog redraws.
  await expect(again.getByRole('status').filter({ hasText: 'Saved.' })).toBeVisible()
  await again.getByRole('checkbox', { name: "Add Mooring's instructions to new chats" }).check()
  await expect(again.getByRole('status').filter({ hasText: 'Saved.' })).toBeVisible()
  expect(problems).toEqual([])
})
