import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import { git, makeRepo, openProject } from './githelpers'
import { collectProblems, fixtures, signIn } from './helpers'

// Fetch, pull and amend from the Changes view, against a bare repository on disk.

test('git: a commit pushed from outside shows as behind, then Pull brings it in; amend stops once pushed', async ({ page }) => {
  test.setTimeout(90_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const repo = makeRepo(`remote-${run}`, { 'a.txt': 'a\n' })
  const remote = join(fixtures, `remote-${run}.git`)
  const other = join(fixtures, `remote-${run}-other`)
  mkdirSync(remote, { recursive: true })
  git(remote, 'init', '--bare', '-q')
  git(repo, 'remote', 'add', 'origin', remote)
  git(repo, 'push', '-q', '-u', 'origin', 'main')
  git(fixtures, 'clone', '-q', '-b', 'main', remote, other)
  for (const [k, v] of Object.entries({ 'user.name': 'Other', 'user.email': 'other@example.com', 'commit.gpgsign': 'false' })) git(other, 'config', k, v)
  writeFileSync(join(other, 'from-outside.txt'), 'hello\n')
  git(other, 'add', '-A')
  git(other, 'commit', '-q', '-m', 'from outside')
  git(other, 'push', '-q')

  await signIn(page)
  await page.goto('/?test')
  const problems = collectProblems(page)
  // An edit keeps the commit box on screen, which is where Amend lives.
  writeFileSync(join(repo, 'b.txt'), 'b\n')
  await openProject(page, repo)
  await page.keyboard.press('Alt+G')
  const changes = page.locator('section[aria-label^="Changes in"]')
  await expect(changes.getByText('0 ahead, 0 behind')).toBeVisible()

  // Amend is off: the last commit is on origin/main.
  const amend = changes.getByLabel('Amend last commit')
  await expect(amend).toBeDisabled()
  await expect(amend).toHaveAccessibleDescription(/already on origin\/main.*force push/)
  await expect(changes.getByText(/The last commit is already on origin\/main/)).toBeVisible()

  // Fetch moves the count; nothing in the work tree changes yet.
  await changes.getByRole('button', { name: 'Fetch', exact: true }).click()
  await expect(changes.getByText('Fetched. origin/main has 1 new commit.')).toBeVisible()
  await expect(changes.getByText('0 ahead, 1 behind')).toBeVisible()
  expect(git(repo, 'ls-files')).not.toContain('from-outside.txt')

  await changes.getByRole('button', { name: 'Pull 1 commit' }).click()
  await expect(changes.getByText('Pulled 1 commit from origin/main.')).toBeVisible()
  await expect(changes.getByText('0 ahead, 0 behind')).toBeVisible()
  expect(git(repo, 'ls-files')).toContain('from-outside.txt')
  await expect(changes.getByRole('button', { name: /^Pull \d/ })).toHaveCount(0)

  // A local commit can be amended until it is pushed.
  await changes.getByRole('button', { name: 'b.txt, new. Show changes' }).waitFor()
  await changes.getByRole('button', { name: 'Stage all' }).click()
  await changes.getByLabel('Commit message').fill('add b')
  await changes.getByRole('button', { name: 'Commit', exact: true }).click()
  await expect(amend).toBeEnabled()
  await amend.check()
  await changes.getByLabel('Commit message').fill('add b, reworded')
  await changes.getByRole('button', { name: 'Amend commit' }).click()
  await expect.poll(() => git(repo, 'log', '-1', '--format=%s').trim()).toBe('add b, reworded')
  expect(git(repo, 'rev-list', '--count', 'HEAD').trim()).toBe('3')
  expect(problems).toEqual([])
})
