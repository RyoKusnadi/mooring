import { mkdirSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { collectProblems, fixtures, signIn } from './helpers'
import { stopChats } from './stopchats'

// A group of two chats: one row in the sidebar, one tab with both side by side.
// Stand-in agents: "gemini" and "grok" (see e2e/serve.sh). "ask" asks for permission.

const used: string[] = []
test.afterEach(async ({ page }) => {
  if (used.length) await stopChats(page, used.splice(0))
})

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

test('chat group: two chats in one tab, one marker, ungroup', async ({ page }) => {
  test.setTimeout(120_000)
  const name = `group-${Date.now() % 1_000_000}-${test.info().repeatEachIndex}`
  used.push(name)
  await signIn(page)
  const problems = collectProblems(page)
  await addProject(page, name)
  const panel = page.getByRole('tabpanel')

  await panel.getByRole('button', { name: 'New chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await page.getByRole('button', { name: 'New tab' }).click()
  await page.getByRole('menuitem', { name: 'Chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Grok' }).click()
  await expect(page.getByLabel('Message to Grok')).toBeEnabled()

  const side = page.getByRole('region', { name: /^Chats in/ })
  await expect(side.locator('button.open')).toHaveCount(2)

  // Group them from the Grok chat's menu.
  await panel.getByRole('button', { name: /^Chat options/ }).click()
  await panel.getByRole('button', { name: 'Group with other chats…' }).click()
  const form = panel.getByRole('group', { name: 'Group with other chats' })
  await form.getByRole('checkbox').first().check()
  await panel.getByLabel('Group title (optional)').fill('Build together')
  await panel.getByRole('button', { name: 'Group chats' }).click()

  // The group tab shows both chats, each with its own composer.
  const view = page.locator('[data-group-view]')
  await expect(view.getByRole('heading', { name: 'Build together' })).toBeVisible()
  await expect(view.getByLabel('Message to Gemini')).toBeVisible()
  await expect(view.getByLabel('Message to Grok')).toBeVisible()

  // The sidebar has one group row with the two chats under it.
  const row = side.getByRole('button', { name: /Build together/ })
  await expect(row).toBeVisible()
  await expect(side.getByRole('list', { name: 'Chats in Build together' }).getByRole('listitem')).toHaveCount(2)

  // A permission request in one chat marks the group row.
  await view.getByLabel('Message to Gemini').fill('ask')
  await view.getByRole('button', { name: 'Send', exact: true }).first().click()
  const request = view.getByRole('alert', { name: 'Permission request' })
  await expect(request).toBeVisible()
  await expect(row.getByRole('img', { name: 'A chat in this group needs you' })).toBeVisible()
  await request.getByRole('button', { name: 'Allow' }).first().click()
  await expect(row.getByRole('img', { name: 'A chat in this group needs you' })).toBeHidden()

  // Ungroup: plain rows again, the group tab is gone.
  await view.getByRole('button', { name: 'Ungroup' }).click()
  await expect(side.getByRole('button', { name: /Build together/ })).toBeHidden()
  await expect(side.locator('button.open')).toHaveCount(2)
  await expect(page.locator('[data-group-view]')).toHaveCount(0)
  expect(problems).toEqual([])
})
