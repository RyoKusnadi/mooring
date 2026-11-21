import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { git, makeRepo, openProject } from './githelpers'
import { signIn } from './helpers'

// Phase 19, git as a full view: the Changes pane view, the branch popover, Discard all, search results kept open.

const unique = () => `ux19-${Date.now() % 1_000_000}-${test.info().workerIndex}${test.info().repeatEachIndex}`
const view = (page: Page) => page.getByRole('region', { name: /^Changes in/ })

async function openChanges(page: Page, repo: string) {
  await page.setViewportSize({ width: 1440, height: 900 })
  await signIn(page)
  await openProject(page, repo)
  await page.keyboard.press('Alt+G')
  await expect(view(page)).toBeVisible()
}

test('41 changes with long nested paths show every file name in full at 1440 px', async ({ page }) => {
  const repo = makeRepo(unique(), { 'tracked.txt': 'one\n', 'keep-this-one.txt': 'keep\n' })
  const names: string[] = []
  for (let i = 0; i < 41; i++) {
    const dir = join('packages', `workspace-package-with-a-long-name-${i}`, 'src', 'components', 'internal', 'sections')
    mkdirSync(join(repo, dir), { recursive: true })
    const name = `InternalSettingsPanel${i}.svelte`
    writeFileSync(join(repo, dir, name), 'x\n')
    names.push(name)
  }
  writeFileSync(join(repo, 'tracked.txt'), 'one\ntwo\n')
  await openChanges(page, repo)

  const rows = view(page).getByRole('list', { name: 'Unstaged files' }).getByRole('listitem')
  await expect(rows).toHaveCount(42)
  const left = (await view(page).locator('.left').boundingBox())!
  for (let i = 0; i < 42; i++) {
    const row = rows.nth(i)
    await row.scrollIntoViewIfNeeded()
    const name = row.locator('.name')
    const box = (await name.boundingBox())!
    expect(await name.evaluate((n) => n.scrollWidth <= n.clientWidth), `name ${i} is cut`).toBe(true)
    expect(box.x + box.width, `name ${i} reaches past the list`).toBeLessThanOrEqual(left.x + left.width)
    const text = await name.innerText()
    expect(text === 'tracked.txt' || names.includes(text), text).toBe(true)
  }
  // The folder wraps onto more lines instead of ending in an ellipsis; the name stays whole.
  const dir = rows.filter({ hasText: 'InternalSettingsPanel7.svelte' }).locator('.dir')
  expect(await dir.evaluate((n) => n.scrollWidth <= n.clientWidth)).toBe(true)
  // The diff of the first file shows beside the list, with the commit box under it.
  await expect(view(page).locator('.right h2')).toBeAttached()
  await expect(view(page).getByLabel('Commit message')).toBeVisible()
  // The sidebar keeps only a count and "Open changes".
  const side = page.getByRole('navigation', { name: 'Projects' })
  await side.getByRole('button', { name: /^Changes/ }).click()
  await expect(side.getByText('42 changed files')).toBeVisible()
  await expect(side.getByRole('button', { name: 'Open changes' })).toBeVisible()
  await expect(side.getByRole('list', { name: 'Unstaged files' })).toHaveCount(0)
})

test('keyboard: Down shows the next diff, Space stages and moves on, Delete asks first', async ({ page }) => {
  const repo = makeRepo(unique(), { 'a.txt': 'a\n', 'b.txt': 'b\n', 'c.txt': 'c\n' })
  for (const f of ['a.txt', 'b.txt', 'c.txt']) writeFileSync(join(repo, f), `${f} changed\n`)
  await openChanges(page, repo)
  const files = view(page).getByRole('list', { name: 'Unstaged files' })
  const heading = (n: string) => page.getByRole('tabpanel').getByRole('heading', { name: n, exact: true })
  await expect(heading('a.txt')).toBeAttached()
  await files.getByRole('button', { name: /^a\.txt/ }).focus()
  await page.keyboard.press('ArrowDown')
  await expect(heading('b.txt')).toBeAttached()
  await expect(files.getByRole('button', { name: /^b\.txt/ })).toBeFocused()
  await page.keyboard.press('Space')
  await expect(view(page).getByRole('list', { name: 'Staged files', exact: true }).getByRole('button', { name: /^b\.txt/ })).toBeVisible()
  // Selection moved to the next file of the same list, so Space can go on.
  await expect(heading('c.txt')).toBeAttached()
  await expect(files.getByRole('button', { name: /^c\.txt/ })).toBeFocused()
  await page.keyboard.press('Space')
  await expect(view(page).getByRole('list', { name: 'Staged files', exact: true }).getByRole('listitem')).toHaveCount(2)
  expect(git(repo, 'diff', '--cached', '--name-only').split('\n').filter(Boolean).sort()).toEqual(['b.txt', 'c.txt'])
  // Delete on an unstaged file asks first and keeps the file when refused.
  await expect(files.getByRole('button', { name: /^a\.txt/ })).toBeFocused()
  await page.keyboard.press('Delete')
  const ask = view(page).getByRole('group', { name: 'Confirm discard' })
  await expect(ask).toContainText('Discard changes to a.txt?')
  await ask.getByRole('button', { name: 'Keep changes' }).click()
  expect(readFileSync(join(repo, 'a.txt'), 'utf8')).toBe('a.txt changed\n')
})

test('branch popover: filter, make from the typed name, refuse a dirty switch, close with Esc and outside click', async ({ page }) => {
  const repo = makeRepo(unique(), { 'a.txt': 'a\n' })
  git(repo, 'checkout', '-q', '-b', 'feature/other')
  writeFileSync(join(repo, 'a.txt'), 'other side\n')
  git(repo, 'commit', '-qam', 'other')
  git(repo, 'checkout', '-q', 'main')
  git(repo, 'branch', 'fix-menu')
  await openChanges(page, repo)
  const open = view(page).getByRole('button', { name: /main.*Switch branch/ })
  const pop = page.getByRole('group', { name: 'Branches' })
  await open.click()
  await expect(pop).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(pop).toBeHidden()
  await expect(open).toBeFocused()
  await open.click()
  await page.getByRole('heading', { name: /^Changes/ }).first().click({ force: true })
  await expect(pop).toBeHidden()

  await open.click()
  await pop.getByLabel('Switch to a branch, or name a new one').fill('feat')
  await expect(pop.getByRole('button', { name: 'Switch to feature/other' })).toBeVisible()
  await expect(pop.getByRole('button', { name: 'Switch to fix-menu' })).toHaveCount(0)

  // A dirty tree that the other branch would overwrite: refused, with a plain message.
  writeFileSync(join(repo, 'a.txt'), 'my edit\n')
  await pop.getByRole('button', { name: 'Switch to feature/other' }).click()
  await expect(pop.getByRole('alert')).toContainText('uncommitted changes')
  expect(git(repo, 'branch', '--show-current').trim()).toBe('main')
  // Focus stays in the popover, so Esc still closes it, and the plain message stays when it is closed.
  await expect(pop.getByLabel('Switch to a branch, or name a new one')).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(pop).toBeHidden()
  await expect(view(page).getByRole('alert')).toContainText('uncommitted changes')
  await expect(view(page).getByRole('alert')).not.toContainText('error:')
  git(repo, 'checkout', '-q', '--', 'a.txt')

  // The typed name becomes a new branch.
  await open.click()
  await pop.getByLabel('Switch to a branch, or name a new one').fill('brand-new')
  await expect(pop.getByRole('button', { name: /^Make branch brand-new/ })).toBeVisible()
  await pop.getByLabel('Switch to a branch, or name a new one').press('Enter')
  await expect(pop).toBeHidden()
  await expect(view(page).getByRole('group', { name: 'Branch', exact: true })).toContainText('brand-new')
  expect(git(repo, 'branch', '--show-current').trim()).toBe('brand-new')
})

test('Discard all sits away from Stage all, names the count, and asks first', async ({ page }) => {
  const repo = makeRepo(unique(), { 'a.txt': 'a\n', 'b.txt': 'b\n', 'c.txt': 'c\n' })
  for (const f of ['a.txt', 'b.txt', 'c.txt']) writeFileSync(join(repo, f), `${f} edited\n`)
  await openChanges(page, repo)
  const stage = view(page).getByRole('button', { name: 'Stage all', exact: true })
  const discard = view(page).getByRole('button', { name: 'Discard all', exact: true })
  const [s, d] = [(await stage.boundingBox())!, (await discard.boundingBox())!]
  expect(d.y - (s.y + s.height)).toBeGreaterThan(60)
  await discard.click()
  const ask = view(page).getByRole('group', { name: 'Confirm discard' })
  await expect(ask).toContainText('Discard all 3 unstaged changes')
  await expect(ask).not.toContainText('onflict')
  await ask.getByRole('button', { name: 'Keep changes' }).click()
  expect(readFileSync(join(repo, 'a.txt'), 'utf8')).toBe('a.txt edited\n')
  await discard.click()
  await ask.getByRole('button', { name: 'Discard', exact: true }).click()
  await expect(view(page).getByText('No changes.')).toBeVisible()
  expect(readFileSync(join(repo, 'a.txt'), 'utf8')).toBe('a\n')
  expect(existsSync(join(repo, 'b.txt'))).toBe(true)
})

test('on a phone the list and the diff are two steps with Back', async ({ page }) => {
  const repo = makeRepo(unique(), { 'a.txt': 'a\n', 'b.txt': 'b\n' })
  for (const f of ['a.txt', 'b.txt']) writeFileSync(join(repo, f), `${f} edited\n`)
  await signIn(page)
  await openProject(page, repo)
  await page.setViewportSize({ width: 390, height: 800 })
  await page.getByRole('button', { name: 'Projects', exact: true }).click()
  await page.getByRole('navigation', { name: 'Projects' }).getByRole('button', { name: /^Changes/ }).click()
  await page.getByRole('button', { name: 'Open changes' }).click()
  const files = view(page).getByRole('list', { name: 'Unstaged files' })
  await expect(files.getByRole('button', { name: /^b\.txt/ })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'b.txt', exact: true })).toHaveCount(0)
  await files.getByRole('button', { name: /^b\.txt/ }).click()
  await expect(page.getByRole('heading', { name: 'b.txt', exact: true })).toBeAttached()
  await expect(files).toBeHidden()
  await page.getByRole('button', { name: 'Back to files' }).click()
  await expect(files).toBeVisible()
})

test('search keeps its results in a panel while files open', async ({ page }) => {
  const repo = makeRepo(unique(), { 'one.txt': 'alpha needle\n', 'two.txt': 'beta needle\n', 'none.txt': 'nothing\n' })
  await page.setViewportSize({ width: 1440, height: 900 })
  await signIn(page)
  await openProject(page, repo)
  await page.keyboard.press('Alt+F')
  const panel = page.getByRole('dialog', { name: 'Find in project' })
  await panel.getByLabel('Text to find').fill('needle')
  const hits = panel.getByRole('option')
  await expect(hits).toHaveCount(2)
  await hits.filter({ hasText: 'one.txt' }).click()
  await expect(page.getByRole('tab', { name: 'one.txt' })).toBeVisible()
  await expect(panel).toBeVisible()
  await expect(hits).toHaveCount(2)
  await hits.filter({ hasText: 'two.txt' }).click()
  await expect(page.getByRole('tab', { name: 'two.txt' })).toBeVisible()
  await expect(panel).toBeVisible()
  await panel.getByLabel('Text to find').press('Escape')
  await expect(panel).toBeHidden()
})
