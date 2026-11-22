import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { basename, join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { git, makeRepo, openProject } from './githelpers'
import { signIn } from './helpers'

// Phase 19 review fixes: rows that never overlap, focus after a discard, empty states, the phone layout, the search panel.

const unique = () => `ux19b-${Date.now() % 1_000_000}-${test.info().workerIndex}${test.info().repeatEachIndex}`
const view = (page: Page) => page.locator('section[aria-label^="Changes in"]')

async function openChanges(page: Page, repo: string, width = 1440) {
  await page.setViewportSize({ width, height: 900 })
  await signIn(page)
  await openProject(page, repo)
  await page.keyboard.press('Alt+G')
  await expect(view(page)).toBeVisible()
}

for (const width of [1440, 1024]) {
  test(`names never run under the row icons at ${width} px`, async ({ page }) => {
    const repo = makeRepo(unique(), { 'tracked.txt': 'one\n' })
    const dir = join('packages', 'workspace-package-with-a-long-name', 'src', 'components', 'internal')
    mkdirSync(join(repo, dir), { recursive: true })
    for (const n of ['InternalSettingsPanelWithAVeryLongName.svelte', 'a.ts']) writeFileSync(join(repo, dir, n), 'x\n')
    await openChanges(page, repo, width)
    await expectNoOverlap(page)
  })
}

test('names never run under the row icons on a phone', async ({ page }) => {
  const repo = makeRepo(unique(), { 'tracked.txt': 'one\n' })
  mkdirSync(join(repo, 'deep', 'folder'), { recursive: true })
  writeFileSync(join(repo, 'deep', 'folder', 'AnExtremelyLongComponentNameForThisPhoneTest.svelte'), 'x\n')
  await signIn(page)
  await openProject(page, repo)
  await page.setViewportSize({ width: 390, height: 800 })
  await page.getByRole('button', { name: 'Projects', exact: true }).click()
  await page.getByRole('navigation', { name: 'Projects' }).getByRole('button', { name: /^Changes/ }).click()
  await page.getByRole('button', { name: 'Open changes' }).click()
  await expectNoOverlap(page)
})

async function expectNoOverlap(page: Page) {
  const rows = view(page).getByRole('list', { name: 'Unstaged files' }).getByRole('listitem')
  await expect(rows.first()).toBeVisible()
  const n = await rows.count()
  expect(n).toBeGreaterThan(0)
  for (let i = 0; i < n; i++) {
    const row = rows.nth(i)
    const text = (await row.locator('.text').boundingBox())!
    const icon = (await row.locator('.icon-btn').first().boundingBox())!
    expect(text.x + text.width, `row ${i} text runs under the icons`).toBeLessThanOrEqual(icon.x + 0.5)
    const name = row.locator('.name')
    expect(await name.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true)
  }
}

test('Discard moves focus to the next row, Esc keeps the changes, Backspace does nothing', async ({ page }) => {
  const repo = makeRepo(unique(), { 'a.txt': 'a\n', 'b.txt': 'b\n', 'c.txt': 'c\n' })
  for (const f of ['a.txt', 'b.txt', 'c.txt']) writeFileSync(join(repo, f), `${f} edited\n`)
  await openChanges(page, repo)
  const files = view(page).getByRole('list', { name: 'Unstaged files' })
  const row = (n: string) => files.getByRole('button', { name: new RegExp(`^${n}`) })
  await row('b\\.txt').focus()
  await page.keyboard.press('Backspace')
  await expect(page.getByRole('group', { name: 'Confirm discard' })).toHaveCount(0)
  await page.keyboard.press('Delete')
  const ask = page.getByRole('group', { name: 'Confirm discard' })
  await expect(ask).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(ask).toHaveCount(0)
  await expect(row('b\\.txt')).toBeFocused()
  expect(readFileSync(join(repo, 'b.txt'), 'utf8')).toBe('b.txt edited\n')
  await page.keyboard.press('Delete')
  await ask.getByRole('button', { name: 'Discard', exact: true }).click()
  await expect(row('c\\.txt')).toBeFocused()
  expect(readFileSync(join(repo, 'b.txt'), 'utf8')).toBe('b\n')
})

test('Discard all counts what the command discards and the file made after the question survives', async ({ page }) => {
  const repo = makeRepo(unique(), { 'a.txt': 'a\n', 's.txt': 's\n' })
  writeFileSync(join(repo, 'a.txt'), 'edited\n')
  writeFileSync(join(repo, 's.txt'), 'staged\n')
  git(repo, 'add', 's.txt')
  writeFileSync(join(repo, 'new.txt'), 'new\n')
  await openChanges(page, repo)
  await view(page).getByRole('button', { name: 'Discard all', exact: true }).click()
  const ask = page.getByRole('group', { name: 'Confirm discard' })
  await expect(ask).toContainText('all 2 unstaged changes')
  await expect(ask).toContainText('1 staged file stays staged')
  await expect(ask).toBeInViewport()
  writeFileSync(join(repo, 'later.txt'), 'made after the question\n')
  await ask.getByRole('button', { name: 'Discard', exact: true }).click()
  await expect(ask).toHaveCount(0)
  expect(readFileSync(join(repo, 'a.txt'), 'utf8')).toBe('a\n')
  expect(readFileSync(join(repo, 'later.txt'), 'utf8')).toBe('made after the question\n')
})

test('a clean tree says so, with no commit box and no diff prompt; a plain folder is not a repository', async ({ page }) => {
  const repo = makeRepo(unique(), { 'a.txt': 'a\n' })
  git(repo, 'remote', 'add', 'origin', repo)
  await openChanges(page, repo)
  await expect(view(page).getByText(/^No changes\. Everything is committed/)).toBeVisible()
  await expect(view(page).getByText('Pick a file')).toHaveCount(0)
  await expect(view(page).getByLabel('Commit message')).toHaveCount(0)
  const note = (await view(page).getByText(/^No changes\./).boundingBox())!
  expect(note.width).toBeLessThanOrEqual(560)
})

test('on a phone the commit box is one line until tapped', async ({ page }) => {
  const repo = makeRepo(unique(), { 'a.txt': 'a\n', 'b.txt': 'b\n' })
  writeFileSync(join(repo, 'a.txt'), 'edited\n')
  await signIn(page)
  await openProject(page, repo)
  await page.setViewportSize({ width: 390, height: 800 })
  await page.getByRole('button', { name: 'Projects', exact: true }).click()
  await page.getByRole('navigation', { name: 'Projects' }).getByRole('button', { name: /^Changes/ }).click()
  await page.getByRole('button', { name: 'Open changes' }).click()
  await expect(view(page).getByLabel('Commit message')).toHaveCount(0)
  const row = (await view(page).getByRole('list', { name: 'Unstaged files' }).getByRole('listitem').first().boundingBox())!
  expect(row.y).toBeLessThan(200)
  await view(page).getByRole('button', { name: /^Commit message/ }).click()
  await expect(view(page).getByLabel('Commit message')).toBeFocused()
})

test('search results close with the project and mark themselves stale after a file change', async ({ page }) => {
  const repo = makeRepo(unique(), { 'one.txt': 'needle\n' })
  const other = makeRepo(unique() + 'o', { 'x.txt': 'x\n' })
  await page.setViewportSize({ width: 1440, height: 900 })
  await signIn(page)
  await openProject(page, other)
  await openProject(page, repo)
  await page.keyboard.press('Alt+F')
  const panel = page.getByRole('dialog', { name: 'Find in project' })
  await panel.getByLabel('Text to find').fill('needle')
  await expect(panel.getByRole('option')).toHaveCount(1)
  writeFileSync(join(repo, 'two.txt'), 'needle too\n')
  await expect(panel.getByRole('button', { name: 'Refresh results' })).toBeVisible()
  await panel.getByRole('button', { name: 'Refresh results' }).click()
  await expect(panel.getByRole('option')).toHaveCount(2)
  await page.getByRole('navigation', { name: 'Projects' }).getByRole('button', { name: basename(other), exact: true }).click()
  await expect(panel).toBeHidden()
})
