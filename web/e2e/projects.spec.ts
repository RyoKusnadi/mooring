import { execFileSync } from 'node:child_process'
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import { collectProblems, fixtures as allFixtures, signIn } from './helpers'

test('add a folder, clone a repository and arrange panes from the browser', async ({ page }) => {
  // Each run gets its own folders, so the test can run again on the same data folder.
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const fixtures = join(allFixtures, `run-${run}`)
  const existing = `existing-${run}`
  const clonedName = `cloned-${run}`
  const folder = join(fixtures, existing)
  mkdirSync(folder, { recursive: true })
  const origin = join(fixtures, 'origin')
  mkdirSync(origin, { recursive: true })
  const git = (...args: string[]) => execFileSync('git', args, { cwd: origin, stdio: 'ignore' })
  git('init', '-q', '-b', 'main')
  writeFileSync(join(origin, 'README.md'), 'hello\n')
  git('add', '.')
  git('-c', 'user.name=Test', '-c', 'user.email=test@example.invalid', 'commit', '-q', '-m', 'first')

  await signIn(page)
  // The session check before sign-in answers 401 on purpose; collect from here on.
  const problems = collectProblems(page)

  // Open an existing folder by typing its path into the folder browser.
  await page.getByRole('button', { name: 'Add project' }).first().click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Folder on this computer').fill(folder)
  await dialog.getByRole('button', { name: 'Go', exact: true }).click()
  await dialog.getByRole('button', { name: 'Open this folder' }).click()
  await expect(dialog).toBeHidden()
  await expect(page.getByRole('navigation', { name: 'Projects' }).getByRole('button', { name: existing, exact: true })).toBeVisible()

  // Clone the local repository as a second project.
  await page.getByRole('button', { name: 'Add project' }).first().click()
  await dialog.getByRole('tab', { name: 'Clone a repository' }).click()
  await dialog.getByLabel('Repository address').fill('file://' + origin)
  await dialog.getByLabel('Parent folder').fill(fixtures)
  await dialog.getByLabel('Folder name (optional)').fill(clonedName)
  await dialog.getByRole('button', { name: 'Clone repository' }).click()
  await expect(dialog).toBeHidden({ timeout: 30_000 })
  const cloned = page.getByRole('navigation', { name: 'Projects' }).getByRole('button', { name: clonedName, exact: true })
  await expect(cloned).toBeVisible()

  // Arrange panes in the cloned project: split right, then down.
  await cloned.click()
  await page.getByRole('button', { name: 'New tab' }).click()
  await page.getByRole('menuitem', { name: 'Terminal' }).click()
  await page.getByRole('button', { name: 'Split right' }).first().click()
  await page.getByRole('button', { name: 'Split down' }).first().click()
  await expect(page.getByRole('tablist', { name: 'Tabs' })).toHaveCount(3)

  // The layout is kept by the server: it survives a reload.
  await page.waitForTimeout(800)
  await page.reload()
  await expect(page.getByRole('tablist', { name: 'Tabs' })).toHaveCount(3)

  // Closing a tab collapses its pane.
  await page.getByRole('button', { name: /^Close / }).first().click()
  await expect(page.getByRole('tablist', { name: 'Tabs' })).toHaveCount(2)

  expect(problems).toEqual([])
})
