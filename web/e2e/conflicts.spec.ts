import { writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { git, makeRepo, openProject } from './githelpers'
import { collectProblems, signIn } from './helpers'

// A merge that stops on two files: settle each, then finish or abort from the banner in Changes.

function conflictedRepo(run: string) {
  const repo = makeRepo(`conflict-${run}`, { 'a.txt': 'one\n', 'b.txt': 'one\n' })
  git(repo, 'checkout', '-q', '-b', 'side')
  writeFileSync(join(repo, 'a.txt'), 'side a\n')
  writeFileSync(join(repo, 'b.txt'), 'side b\n')
  git(repo, 'commit', '-q', '-am', 'side change')
  git(repo, 'checkout', '-q', 'main')
  writeFileSync(join(repo, 'a.txt'), 'main a\n')
  writeFileSync(join(repo, 'b.txt'), 'main b\n')
  git(repo, 'commit', '-q', '-am', 'main change')
  try {
    git(repo, 'merge', 'side')
  } catch {
    // The merge stops on purpose.
  }
  return repo
}

async function open(page: Page, repo: string, watch?: (page: Page) => void) {
  await signIn(page)
  await page.goto('/?test')
  watch?.(page)
  await openProject(page, repo)
  await page.keyboard.press('Alt+G')
  return page.locator('section[aria-label^="Changes in"]')
}

test('conflicts: keep a side, mark an edited file, finish the merge', async ({ page }) => {
  test.setTimeout(90_000)
  const repo = conflictedRepo(`${Date.now()}-${test.info().repeatEachIndex}`)
  let problems: string[] = []
  const changes = await open(page, repo, (p) => (problems = collectProblems(p)))

  const banner = changes.getByRole('group', { name: 'Merge in progress.' })
  await expect(banner).toBeVisible()
  const finish = banner.getByRole('button', { name: 'Finish merge' })
  await expect(finish).toBeDisabled()

  // A file that still has markers is refused, and the reason shows.
  await changes.getByRole('group', { name: 'Resolve b.txt' }).getByRole('button', { name: 'Mark resolved' }).click()
  await expect(changes.getByText('still has conflict markers')).toBeVisible()

  await changes.getByRole('group', { name: 'Resolve a.txt' }).getByRole('button', { name: 'Keep incoming' }).click()
  await expect(changes.getByRole('group', { name: 'Resolve a.txt' })).toBeHidden()
  expect(git(repo, 'show', ':a.txt').trim()).toBe('side a')

  writeFileSync(join(repo, 'b.txt'), 'both\n')
  await changes.getByRole('group', { name: 'Resolve b.txt' }).getByRole('button', { name: 'Mark resolved' }).click()
  await expect(finish).toBeEnabled()
  await finish.click()

  await expect(banner).toBeHidden()
  await expect.poll(() => git(repo, 'log', '-1', '--format=%P').trim().split(' ').length).toBe(2)
  expect(git(repo, 'status', '--porcelain').trim()).toBe('')
  await expect(changes.getByText('No changes.')).toBeVisible()
  expect(problems).toEqual([])
})

test('conflicts: abort asks first, then puts the files back', async ({ page }) => {
  test.setTimeout(90_000)
  const repo = conflictedRepo(`abort-${Date.now()}-${test.info().repeatEachIndex}`)
  const changes = await open(page, repo)

  const banner = changes.getByRole('group', { name: 'Merge in progress.' })
  await banner.getByRole('button', { name: 'Abort merge' }).click()
  await banner.getByRole('button', { name: 'Keep going' }).click()
  await expect(banner).toBeVisible()
  await banner.getByRole('button', { name: 'Abort merge' }).click()
  await banner.getByRole('button', { name: 'Abort merge' }).click()

  await expect(banner).toBeHidden()
  expect(git(repo, 'status', '--porcelain').trim()).toBe('')
  expect(git(repo, 'show', 'HEAD:a.txt').trim()).toBe('main a')
})

test('conflicts: Space and Stage all leave conflicted files alone; rows fit at 390 px', async ({ page }) => {
  test.setTimeout(90_000)
  const repo = conflictedRepo(`keys-${Date.now()}-${test.info().repeatEachIndex}`)
  const changes = await open(page, repo)
  await expect(changes.getByRole('button', { name: 'Stage all' })).toBeDisabled()
  await changes.getByRole('button', { name: 'a.txt, conflict. Show changes' }).focus()
  await page.keyboard.press('Space')
  await page.waitForTimeout(300)
  expect(git(repo, 'ls-files', '-u').trim()).not.toBe('')
  await page.setViewportSize({ width: 390, height: 800 })
  await expect(changes.getByRole('group', { name: 'Resolve a.txt' })).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true)
})
