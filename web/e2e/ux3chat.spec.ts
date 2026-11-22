import { mkdirSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import { collectProblems, fixtures, signIn } from './helpers'
import { openProject } from './githelpers'

// Runs with the chat tests: the stand-in agent "gemini" (see e2e/serve.sh).
test('phone chat: the header is 40px, its menu opens and closes, Model and Mode share a row', async ({ page }) => {
  const folder = join(fixtures, `ux3chat-${Date.now()}`)
  mkdirSync(folder, { recursive: true })
  await signIn(page)
  await page.setViewportSize({ width: 390, height: 844 })
  const problems = collectProblems(page)
  await page.getByRole('button', { name: 'Projects', exact: true }).click()
  await openProject(page, folder)
  await page.getByRole('button', { name: 'New tab' }).click()
  await page.getByRole('menuitem', { name: 'Chat' }).click()
  await page.getByRole('button', { name: 'Start chat with Gemini' }).click()
  const message = page.getByLabel('Message to Gemini')
  await expect(message).toBeEnabled()
  await page.getByLabel('Message to Gemini').fill('hello')
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect(page.getByRole('region', { name: /messages$/ }).getByText('Hello world', { exact: true })).toBeVisible()

  const header = page.locator('header.head')
  // The header is one 40px row: title, state and the menu. Model and Mode sit by the composer.
  const row = (await header.boundingBox())!
  expect(row.height).toBeLessThanOrEqual(41)
  const menu = page.getByRole('button', { name: /^Chat options/ })
  await menu.click()
  await expect(page.getByRole('checkbox', { name: 'Shorten long output' })).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('checkbox', { name: 'Shorten long output' })).toHaveCount(0)
  const model = page.getByRole('combobox', { name: /^Model/ })
  const mode = page.getByRole('combobox', { name: /^Mode(?!l)/ })
  const ys = await Promise.all([model, mode].map(async (l) => Math.round((await l.boundingBox())!.y)))
  expect(Math.abs(ys[0] - ys[1])).toBeLessThan(24)
  const tabs = (await page.getByRole('tablist', { name: 'Tabs' }).boundingBox())!
  expect(row.y + row.height - tabs.y).toBeLessThanOrEqual(100)
  expect(problems).toEqual([])
})
