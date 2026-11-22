import { writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import { collectProblems, signIn } from './helpers'
import { git, makeRepo, openProject } from './githelpers'

test('blame names who changed each line, hides on edits, and explains an untracked file', async ({ page }) => {
  const repo = makeRepo(`blame-${Date.now() % 1_000_000}`, { 'a.txt': 'one\ntwo\nthree\n' })
  writeFileSync(join(repo, 'a.txt'), 'one\ntwo changed\nthree\n')
  git(repo, '-c', 'user.name=Second Author', '-c', 'user.email=second@example.com', 'commit', '-q', '-am', 'change line two')
  writeFileSync(join(repo, 'new.txt'), 'fresh\n')
  await signIn(page)
  const problems = collectProblems(page)
  await openProject(page, repo)
  const tree = page.getByRole('region', { name: /^Files in/ })

  await tree.getByRole('button', { name: 'a.txt', exact: true }).dblclick()
  const blame = page.getByRole('button', { name: 'Blame', exact: true })
  await expect(blame).toHaveAttribute('aria-pressed', 'false')
  await expect(page.locator('.cm-blame-row')).toHaveCount(0)
  await blame.click()
  await expect(blame).toHaveAttribute('aria-pressed', 'true')
  const rows = page.locator('.cm-blame-row:visible')
  await expect(rows.filter({ hasText: 'Test User' })).toHaveCount(2)
  await expect(rows.filter({ hasText: 'Second Author' })).toHaveCount(1)

  // The gutter is hidden from screen readers, so the bar says it for the line with the cursor.
  await page.locator('.cm-line', { hasText: 'two changed' }).click()
  await expect(page.getByText(/Line 2: change line two, by Second Author/)).toBeVisible()

  // The gutter is not a tab stop; the bar's button opens the line's commit.
  await expect(page.locator('button.cm-blame-row[title="change line two"]')).toHaveAttribute('tabindex', '-1')
  await expect(page.getByRole('button', { name: /^Open commit [0-9a-f]{7}: Line 2/ })).toBeVisible()

  // Unsaved edits: the lines no longer match, so the gutter hides and says why.
  await page.locator('.cm-content').click()
  await page.keyboard.type('x')
  await expect(page.getByText('Blame comes back after you save.')).toBeVisible()
  await expect(rows).toHaveCount(0)
  await page.keyboard.press('ControlOrMeta+s')
  await expect(rows.filter({ hasText: 'Not committed yet' })).toHaveCount(1)

  // The bar's button opens the commit of the line with the cursor.
  await page.locator('.cm-line', { hasText: 'two changed' }).click()
  await page.getByRole('button', { name: /^Open commit [0-9a-f]{7}: Line 2/ }).click()
  await expect(page.getByRole('tab', { name: /change line two|[0-9a-f]{7}/ }).first()).toBeVisible()

  await tree.getByRole('button', { name: 'new.txt', exact: true }).dblclick()
  await page.getByRole('button', { name: 'Blame', exact: true }).click()
  await expect(page.getByText('This file is not in Git yet, so there is nothing to show. Commit it first.')).toBeVisible()

  await page.setViewportSize({ width: 390, height: 844 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true)
  // The browser logs the 409 of the untracked file; that is the case under test.
  expect(problems.filter((m) => !/status of 409/.test(m))).toEqual([])
})
