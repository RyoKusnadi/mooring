import { expect, test } from '@playwright/test'
import { collectProblems } from './helpers'
import { openTerminal, type } from './termhelpers'

test('a notification escape from a background terminal marks its tab and the status bar', async ({ page }) => {
  await openTerminal(page, 'term-signal')
  const problems = collectProblems(page)
  // The escape comes from a job that outlives the tab switch.
  await type(page, "(sleep 2; printf '\\033]9;done\\a') &")

  await page.getByRole('button', { name: 'New tab' }).click()
  await page.getByRole('menuitem', { name: /^Terminal/ }).click()

  const bar = page.getByRole('contentinfo')
  await expect(bar.getByRole('button', { name: '1 terminal needs you' })).toBeVisible({ timeout: 15_000 })
  const mark = page.getByRole('tab').getByRole('img', { name: 'Needs you' })
  await expect(mark).toHaveCount(1)

  await page.getByRole('tab').first().click()
  await expect(mark).toHaveCount(0)
  await expect(bar.getByRole('button', { name: /needs you/ })).toHaveCount(0)
  expect(problems).toEqual([])
})
