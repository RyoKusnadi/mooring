import { mkdirSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { collectProblems, fixtures, openSettings, signIn } from './helpers'

// serve.sh gives the stand-in agent "gemini" an account adapter. Its own login is signed in; an
// account made here starts signed out until the test creates <account folder>/signed-in.
// Prompt words: RATE sends a rate-limit window (five_hour, 42%), LIMIT fails the turn for a limit.

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

async function say(page: Page, text: string) {
  await page.getByLabel('Message to Gemini').fill(text)
  await page.getByRole('button', { name: 'Send', exact: true }).click()
}

async function openAccounts(page: Page) {
  const settings = await openSettings(page, 'Accounts')
  await settings.getByRole('button', { name: 'Open Accounts' }).click()
  const dialog = page.getByRole('dialog', { name: 'Accounts and usage' })
  await expect(dialog.getByRole('heading', { name: 'Accounts and usage' })).toBeVisible()
  return dialog
}

test('accounts: add, pick for a project, usage, limit and switch', async ({ page }) => {
  test.setTimeout(120_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const name = `Work ${run}`
  await signIn(page)
  const problems = collectProblems(page)
  await addProject(page, `accounts-${run}`)

  // Add an account in a folder Mooring makes. It starts with no sign-in known.
  let dialog = await openAccounts(page)
  await dialog.getByRole('button', { name: 'Add Gemini account' }).click()
  await dialog.getByLabel('Name', { exact: true }).fill(name)
  await dialog.getByRole('button', { name: 'Add account' }).click()
  const row = dialog.getByRole('listitem').filter({ hasText: name })
  await expect(row).toContainText('Folder managed by Mooring')
  await expect(row).toContainText('Sign-in is checked when a chat starts')
  await expect(row).toContainText('Usage: unknown')
  const dir = (await row.locator('code').innerText()).trim()
  writeFileSync(join(resolve(dir), 'signed-in'), '')
  await dialog.getByRole('button', { name: 'Close dialog' }).click()

  // A chat on the agent's own login runs a turn, so that login is known to be signed in.
  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await say(page, 'hello')
  await expect(panel.getByText('Hello world', { exact: true })).toBeVisible()

  // Pick the new account for this project. The picker says the open chat restarts.
  dialog = await openAccounts(page)
  await expect(dialog.getByText('restarts the 1 open Gemini chat')).toBeVisible()
  await dialog.getByLabel('Gemini account for this project').selectOption({ label: name })
  await expect(dialog.getByText('Saved. 1 chat restarted on this account.')).toBeVisible()
  await dialog.getByRole('button', { name: 'Close dialog' }).click()

  // Usage shows after the agent reports a window. The own login has reported nothing.
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await say(page, 'RATE')
  await expect(panel.locator('header.head .usage')).toContainText('5 hours: 42% used, resets')
  dialog = await openAccounts(page)
  const usage = dialog.getByRole('list', { name: `Usage of ${name}` })
  await expect(usage).toContainText('5 hours')
  await expect(usage).toContainText('42% used')
  await expect(usage).toContainText(/resets/)
  await expect(dialog.getByRole('region', { name: 'Gemini accounts' }).getByRole('listitem').filter({ hasText: "The agent's own login" })).toContainText('Usage: unknown')
  await dialog.getByRole('button', { name: 'Close dialog' }).click()

  // A limit shows the reset time and offers the account that has room. One click moves the project.
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await say(page, 'LIMIT')
  const limit = panel.getByRole('region', { name: 'Usage limit reached' })
  await expect(limit).toContainText(`The usage limit of ${name} was reached.`)
  await expect(limit).toContainText(/resets at|resets \w{3} /)
  await limit.getByRole('button', { name: 'Switch to Default' }).click()
  await expect(panel.getByText('This project now uses Default. Send your message again.')).toBeVisible()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await say(page, 'again')
  await expect(panel.getByText('echo: again', { exact: true })).toBeVisible()

  expect(problems).toEqual([])
})

test('accounts: sign-in opens a terminal tab, and a phone width does not scroll sideways', async ({ page }) => {
  test.setTimeout(90_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const name = `Phone ${run}`
  await signIn(page)
  const problems = collectProblems(page)
  await addProject(page, `accounts-phone-${run}`)

  let dialog = await openAccounts(page)
  await dialog.getByRole('button', { name: 'Add Gemini account' }).click()
  await dialog.getByLabel('Name', { exact: true }).fill(name)
  await dialog.getByRole('button', { name: 'Add account' }).click()
  await dialog.getByRole('button', { name: `Sign in to ${name}` }).click()
  await expect(dialog).toBeHidden()
  await expect(page.getByRole('tab', { name: `Sign in: ${name}` })).toBeVisible()

  await page.setViewportSize({ width: 390, height: 844 })
  await page.getByRole('button', { name: 'Projects' }).click()
  dialog = await openAccounts(page)
  await expect(dialog.getByRole('listitem').filter({ hasText: name })).toBeVisible()
  const wide = (sel: string) => page.evaluate((s) => (document.querySelector(s) as HTMLElement).scrollWidth - (document.querySelector(s) as HTMLElement).clientWidth, sel)
  expect(await wide('dialog[open]')).toBeLessThanOrEqual(0)
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(0)
  expect(problems).toEqual([])
})

test('accounts: on a phone the usage line is in the chat menu and the header stays 40px', async ({ page }) => {
  test.setTimeout(90_000)
  await signIn(page)
  await addProject(page, `accounts-usage-${Date.now()}-${test.info().repeatEachIndex}`)
  await page.setViewportSize({ width: 390, height: 844 })
  const panel = page.getByRole('tabpanel')
  await page.getByRole('button', { name: 'New tab' }).click()
  await page.getByRole('menuitem', { name: 'Chat' }).click()
  await page.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await say(page, 'RATE')
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  const header = panel.locator('header.head')
  await expect(header.locator('.usage')).toBeHidden()
  expect((await header.boundingBox())!.height).toBeLessThanOrEqual(41)
  await panel.getByRole('button', { name: /^Chat options/ }).click()
  await expect(page.getByRole('group', { name: 'Chat options' })).toContainText('5 hours: 42% used')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true)
})
