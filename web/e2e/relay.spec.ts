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

// Send a reply to another chat. Stand-in agents: "gemini" and "grok" (see e2e/serve.sh).
// "hello" replies "Hello world"; "long" keeps a chat running.

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

// A Gemini chat that has replied "Hello world", and an idle Grok chat in a second tab.
async function twoChats(page: Page, name: string) {
  await addProject(page, name)
  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await page.getByLabel('Message to Gemini').fill('hello')
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect(panel.getByText('Hello world', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'New tab' }).click()
  await page.getByRole('menuitem', { name: 'Chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Grok' }).click()
  await expect(page.getByLabel('Message to Grok')).toBeEnabled()
  return panel
}

test('relay: send Gemini reply to Grok with a note, label in Grok, running chat is disabled', async ({ page }) => {
  test.setTimeout(120_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const name = `relay-${run}`
  used.push(name)
  await signIn(page)
  const problems = collectProblems(page)
  const panel = await twoChats(page, name)

  // Back in the Gemini tab through its tab, then open the panel from the reply.
  await page.getByRole('tab').filter({ hasText: /hello/i }).first().click()
  const trigger = panel.getByRole('button', { name: 'Send to another chat' })
  await expect(trigger).toHaveCount(1)
  await trigger.click()
  const relay = panel.getByRole('region', { name: 'Send a reply to another chat' })
  await expect(relay.getByRole('button', { name: 'Send', exact: true })).toBeDisabled()
  await relay.getByRole('radio', { name: /^Grok/ }).check()
  await relay.getByLabel('Add a note (optional)').fill('Please check this')
  await relay.getByRole('button', { name: 'Send', exact: true }).click()
  await expect(relay.getByRole('status')).toContainText('Sent to Grok')
  await relay.getByRole('button', { name: 'Open that chat' }).click()

  // Grok shows who the reply came from, the note and the quoted reply.
  await expect(panel.getByText(/^From Gemini/)).toBeVisible()
  const sentText = panel.locator('p.text').filter({ hasText: 'Please check this' })
  await expect(sentText).toBeVisible()
  await expect(sentText).toContainText('> Hello world')
  await expect(page.getByLabel('Message to Grok')).toBeEnabled()

  // The label opens the source chat. A running Grok cannot be a target.
  await page.getByLabel('Message to Grok').fill('long')
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Stop' }).first()).toBeVisible()
  await panel.locator('.from button').first().click()
  await panel.getByRole('button', { name: 'Send to another chat' }).click()
  const again = panel.getByRole('region', { name: 'Send a reply to another chat' })
  await expect(again.getByRole('radio', { name: /^Grok.*replying/ })).toBeDisabled()

  // Escape closes the panel and focus goes back to the button.
  await page.keyboard.press('Escape')
  await expect(again).toBeHidden()
  await expect(panel.getByRole('button', { name: 'Send to another chat' })).toBeFocused()
  expect(problems).toEqual([])
})

test('relay: on a phone the page does not scroll sideways and targets are big enough', async ({ page, browser }) => {
  test.setTimeout(120_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const name = `relayphone-${run}`
  used.push(name)
  await signIn(page)
  await twoChats(page, name)

  const phone = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true, storageState: await page.context().storageState() })
  const p = await phone.newPage()
  const phoneProblems = collectProblems(p)
  await p.goto('/')
  await p.getByRole('tab').filter({ hasText: /hello/i }).first().click()
  const trigger = p.getByRole('tabpanel').getByRole('button', { name: 'Send to another chat' })
  await trigger.click()
  const relay = p.getByRole('region', { name: 'Send a reply to another chat' })
  await relay.getByRole('radio', { name: /^Grok/ }).check()
  await expect(relay.getByLabel('Add a note (optional)')).toBeVisible()
  expect(await p.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(0)
  expect((await trigger.boundingBox())?.height).toBeGreaterThanOrEqual(44)
  expect((await relay.getByRole('button', { name: 'Send', exact: true }).boundingBox())?.height).toBeGreaterThanOrEqual(44)
  expect((await relay.getByRole('radio', { name: /^Grok/ }).locator('xpath=ancestor::label').boundingBox())?.height).toBeGreaterThanOrEqual(44)
  expect(phoneProblems).toEqual([])
  await phone.close()
})

test('relay: a project with one chat says there is no other chat', async ({ page }) => {
  test.setTimeout(90_000)
  const name = `relayempty-${Date.now()}-${test.info().repeatEachIndex}`
  used.push(name)
  await signIn(page)
  await addProject(page, name)
  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await page.getByLabel('Message to Gemini').fill('hello')
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect(panel.getByText('Hello world', { exact: true })).toBeVisible()
  await panel.getByRole('button', { name: 'Send to another chat' }).click()
  const relay = panel.getByRole('region', { name: 'Send a reply to another chat' })
  await expect(relay).toContainText('There is no other chat in this project')
  await expect(relay.getByRole('button', { name: 'New chat' })).toBeVisible()
})
