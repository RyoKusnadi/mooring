import { writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import { git, makeBigRepo, makeRepo, openProject } from './githelpers'
import { collectProblems, signIn } from './helpers'

// History with a graph, search, stash, branches, and choosing lines of a diff.

const ten = (f: (n: number) => string) => Array.from({ length: 10 }, (_, i) => f(i + 1) + '\n').join('')

test('git: choose two lines of a ten-line change, with keys and with the mouse, and in a CRLF file', async ({ page }) => {
  test.setTimeout(90_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const crlf = (f: (n: number) => string) => Array.from({ length: 6 }, (_, i) => f(i + 1) + '\r\n').join('')
  const repo = makeRepo(`lines-${run}`, { 'ten.txt': ten((n) => `line ${n}`), 'dos.txt': crlf((n) => `row ${n}`) })
  writeFileSync(join(repo, 'ten.txt'), ten((n) => (n % 2 === 0 ? `LINE ${n}` : `line ${n}`)))
  writeFileSync(join(repo, 'dos.txt'), crlf((n) => (n === 2 || n === 5 ? `ROW ${n}` : `row ${n}`)))
  await signIn(page)
  await page.goto('/?test')
  const problems = collectProblems(page)
  await openProject(page, repo)
  await page.keyboard.press('Alt+G')
  const changes = page.locator('section[aria-label^="Changes in"]')
  await changes.getByRole('button', { name: 'ten.txt, modified. Show changes' }).first().click()
  const diff = page.getByRole('tabpanel')
  await expect(diff.getByRole('heading', { name: 'ten.txt' })).toBeVisible()
  const line = (kind: 'Added' | 'Removed', text: string) => diff.getByRole('checkbox', { name: `${kind}: ${text}` })

  // Mouse: the gutter box of a removed line, then the added line it became.
  await line('Removed', 'line 4').locator('.box').click()
  await line('Added', 'LINE 4').locator('.box').click()
  await expect(line('Added', 'LINE 4')).toBeChecked()
  await expect(diff.getByText('2 lines chosen.')).toBeVisible()
  await diff.getByRole('button', { name: 'Stage lines (2)' }).click()
  await expect(diff.getByText('2 lines staged.')).toBeVisible()
  const cached = git(repo, 'diff', '--cached', '-U0', 'ten.txt')
  expect(cached).toContain('-line 4\n+LINE 4\n')
  expect(cached.match(/^@@/gm)).toHaveLength(1)
  expect(git(repo, 'diff', '-U0', 'ten.txt').match(/^\+LINE/gm)).toHaveLength(4)

  // Keyboard: arrows move between changed lines, Space chooses, B chooses the whole block.
  await line('Removed', 'line 2').focus()
  await page.keyboard.press('Space')
  await page.keyboard.press('ArrowDown')
  await expect(line('Added', 'LINE 2')).toBeFocused()
  await page.keyboard.press('Space')
  await expect(diff.getByText('2 lines chosen.')).toBeVisible()
  await page.keyboard.press('b')
  await expect(diff.getByText(/^\d+ lines chosen\.$/)).not.toHaveText('2 lines chosen.')
  await page.keyboard.press('Escape')
  await diff.getByRole('button', { name: 'How to choose lines' }).click()
  await expect(diff.getByText(/Choose lines by clicking/)).toBeVisible()
  // The help sits in the header row and does not cover the first block's button. Esc closes it, so does a click elsewhere.
  const tip = (await diff.getByText(/Choose lines by clicking/).boundingBox())!
  const stage = (await diff.getByRole('button', { name: /^Stage block/ }).first().boundingBox())!
  expect(tip.y + tip.height <= stage.y || stage.y + stage.height <= tip.y).toBe(true)
  await page.keyboard.press('Escape')
  await expect(diff.getByText(/Choose lines by clicking/)).toHaveCount(0)
  await diff.getByRole('button', { name: 'How to choose lines' }).click()
  await expect(diff.getByText(/Choose lines by clicking/)).toBeVisible()
  await page.getByRole('heading').first().click({ force: true })
  await expect(diff.getByText(/Choose lines by clicking/)).toHaveCount(0)

  // A block at a time, from its header.
  await diff.getByRole('button', { name: /^Stage block/ }).first().click()
  // With nothing left unstaged the diff shows the staged side instead of an empty note.
  await expect(diff.getByRole('button', { name: 'Unstage file' })).toBeVisible()
  expect(git(repo, 'diff', 'ten.txt')).toBe('')
  expect(git(repo, 'diff', '--cached', '-U0', 'ten.txt').match(/^\+LINE/gm)).toHaveLength(5)

  // A file with CRLF line endings: the staged lines keep their \r\n byte for byte.
  await changes.getByRole('button', { name: 'dos.txt, modified. Show changes' }).first().click()
  await expect(diff.getByRole('heading', { name: 'dos.txt' })).toBeVisible()
  await line('Removed', 'row 2').locator('.box').click()
  await line('Added', 'ROW 2').locator('.box').click()
  await diff.getByRole('button', { name: 'Stage lines (2)' }).click()
  await expect(diff.getByText('2 lines staged.')).toBeVisible()
  expect(git(repo, 'show', ':dos.txt')).toBe(crlf((n) => (n === 2 ? `ROW ${n}` : `row ${n}`)))
  expect(git(repo, 'diff', '--name-only')).toContain('dos.txt')
  expect(problems).toEqual([])
})

test('git: history pages past the first, lanes, search and a commit\'s diff', async ({ page }) => {
  test.setTimeout(240_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const repo = makeBigRepo(`big-${run}`, 100_050)
  await signIn(page)
  await page.goto('/?test')
  const problems = collectProblems(page)
  await openProject(page, repo)
  await page.keyboard.press('Alt+H')
  const history = page.getByRole('tabpanel')
  const list = history.getByRole('region', { name: 'Commits' })
  await expect(history.getByText(/commits read/)).toBeVisible({ timeout: 60_000 })
  await expect(list.getByRole('button', { name: /commit 100050/ })).toBeVisible()
  await expect(list.locator('svg.graph').first()).toBeVisible()
  // A row is 32px (48 on a phone), not the 150px an unsized svg gives.
  const rowBox = await list.getByRole('button', { name: /commit 100049/ }).boundingBox()
  expect(rowBox?.height).toBeLessThan(60)

  // Scroll to the end of what is read; the list fetches the next page near the end, several times.
  await expect(async () => {
    await list.evaluate((el) => (el.scrollTop = el.scrollHeight))
    await expect(history.getByText(/^(1,\d{3}|[6-9]\d{2}) of/)).toBeVisible({ timeout: 3000 })
  }).toPass({ timeout: 60_000 })
  // Lanes: a merge is a ring, and a second lane has lines running past commits.
  await expect(list.locator('svg.graph circle[stroke-width="2"]').first()).toBeVisible()
  expect(await list.locator('svg.graph line').count()).toBeGreaterThan(8)

  // Search the messages: the match shows without lanes; its commit opens with files, then a diff.
  await history.getByLabel('Search the history').fill('commit 99995')
  await history.getByRole('button', { name: 'Search', exact: true }).click()
  await expect(history.getByRole('status').filter({ hasText: /^\s*1 match/ })).toBeVisible({ timeout: 60_000 })
  await expect(history.locator('svg.graph')).toHaveCount(0)
  await history.getByRole('region', { name: 'Search results' }).getByRole('button', { name: /commit 99995/ }).click()
  const detail = history.getByRole('region', { name: 'Selected commit' })
  await expect(detail.getByRole('heading', { name: 'commit 99995' })).toBeVisible()
  await detail.getByRole('button', { name: /a\.txt/ }).click()
  await expect(page.getByRole('tab', { name: /^[0-9a-f]{7}: a\.txt$/ })).toBeVisible()
  await expect(page.getByRole('tabpanel').getByText(/As changed in [0-9a-f]{7}/)).toBeVisible()
  await expect(page.getByRole('tabpanel').getByRole('button', { name: 'Stage file' })).toHaveCount(0)
  expect(problems).toEqual([])
})

test('git: stash save, view, apply and drop; a conflicting apply; switch and make branches; search code', async ({ page }) => {
  test.setTimeout(120_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const repo = makeRepo(`stash-${run}`, { 'a.txt': 'one\ntwo\nthree\n' })
  writeFileSync(join(repo, 'a.txt'), 'one\ntwo changed\nthree\n')
  writeFileSync(join(repo, 'fresh.txt'), 'brand new\n')
  await signIn(page)
  await page.goto('/?test')
  const problems = collectProblems(page)
  await openProject(page, repo)
  await page.keyboard.press('Alt+G')
  const changes = page.locator('section[aria-label^="Changes in"]')
  await expect(changes.getByRole('button', { name: 'a.txt, modified. Show changes' })).toBeVisible()

  // Save with a message and untracked files: the working copy is clean afterwards.
  await changes.getByRole('button', { name: 'Stash changes' }).click()
  await changes.getByLabel('Message (optional)').fill('wip one')
  await changes.getByLabel('Include untracked files').check()
  await changes.getByRole('button', { name: 'Stash', exact: true }).click()
  const stashes = changes.getByRole('list', { name: /^Stashes/ })
  await expect(stashes.getByText('wip one')).toBeVisible()
  await expect(changes.getByText(/^No changes\./)).toBeVisible()
  expect(git(repo, 'stash', 'list')).toContain('wip one')

  // View it like a commit: its files, the untracked one as new.
  await stashes.getByRole('button', { name: 'View stash wip one' }).click()
  const tab = page.getByRole('tabpanel')
  await expect(tab.getByRole('button', { name: /fresh\.txt/ })).toBeVisible()
  await expect(tab.getByRole('button', { name: /a\.txt/ })).toBeVisible()
  await page.getByRole('tab', { name: 'Changes', exact: true }).click()

  // Apply keeps the stash; the changes are back.
  await stashes.getByRole('button', { name: 'Apply stash wip one' }).click()
  await expect(changes.getByRole('button', { name: 'a.txt, modified. Show changes' })).toBeVisible()
  await expect(stashes.getByText('wip one')).toBeVisible()
  git(repo, 'checkout', '-q', '--', 'a.txt')
  git(repo, 'clean', '-fdq')

  // Drop asks first, and says there is no undo.
  await stashes.getByRole('button', { name: 'Drop stash wip one' }).click()
  const ask = changes.getByRole('group', { name: 'Confirm drop stash' })
  await expect(ask).toContainText('no undo')
  await expect(ask.getByRole('button', { name: 'Keep it' })).toBeFocused()
  await ask.getByRole('button', { name: 'Keep it' }).click()
  expect(git(repo, 'stash', 'list')).toContain('wip one')
  await stashes.getByRole('button', { name: 'Drop stash wip one' }).click()
  await ask.getByRole('button', { name: 'Drop stash' }).click()
  await expect(stashes).toHaveCount(0)
  expect(git(repo, 'stash', 'list')).toBe('')

  // An apply that conflicts says what happened and keeps the stash.
  writeFileSync(join(repo, 'a.txt'), 'one\ntwo from stash\nthree\n')
  git(repo, 'stash', 'push', '-q', '-m', 'clash')
  writeFileSync(join(repo, 'a.txt'), 'one\ntwo from main\nthree\n')
  git(repo, 'commit', '-qam', 'main edits line two')
  await changes.getByRole('button', { name: 'Refresh' }).click()
  await changes.getByRole('button', { name: 'Apply stash clash' }).click()
  await expect(changes.getByRole('alert').filter({ hasText: 'did not apply cleanly' })).toBeVisible()
  await expect(changes.getByRole('button', { name: /^a\.txt, conflict/ })).toBeVisible()
  expect(git(repo, 'stash', 'list')).toContain('clash')
  git(repo, 'reset', '-q', '--hard')

  // Branches: make one from the popover, switch back from its list.
  const pop = page.getByRole('group', { name: 'Branches' })
  await changes.getByRole('button', { name: /main.*Switch branch/ }).click()
  await pop.getByLabel('Switch to a branch, or name a new one').fill('feature/one')
  await pop.getByRole('button', { name: /^Make branch feature\/one/ }).click()
  await expect(pop).toBeHidden()
  await expect(changes.getByRole('group', { name: 'Branch', exact: true })).toContainText('feature/one')
  expect(git(repo, 'branch', '--show-current').trim()).toBe('feature/one')
  await changes.getByRole('button', { name: /feature\/one.*Switch branch/ }).click()
  await pop.getByRole('button', { name: 'Switch to main' }).click()
  await expect(pop).toBeHidden()
  await expect(changes.getByRole('group', { name: 'Branch', exact: true })).toContainText('main')
  expect(git(repo, 'branch', '--show-current').trim()).toBe('main')

  // History search by the code a commit added.
  await page.keyboard.press('Alt+H')
  const history = page.getByRole('tabpanel')
  await history.getByLabel('Look in').selectOption('code')
  await history.getByLabel('Search the history').fill('two from main')
  await history.getByRole('button', { name: 'Search', exact: true }).click()
  await expect(history.getByRole('region', { name: 'Search results' }).getByRole('button', { name: /main edits line two/ })).toBeVisible()
  expect(problems).toEqual([])
})
