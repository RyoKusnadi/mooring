import { execFileSync } from 'node:child_process'
import { existsSync, mkdirSync, readdirSync, renameSync, rmSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { collectProblems, fixtures, signIn } from './helpers'
import { text, type, waitForPrompt } from './termhelpers'

// An agent works in its own worktree; the page shows what it did, and the user reviews, commits and pushes.
// The server runs with a stand-in agent behind the name "gemini" (see e2e/serve.sh).

const git = (cwd: string, ...args: string[]) => execFileSync('git', args, { cwd, encoding: 'utf8' })

function makeRepo(name: string) {
  const remote = join(fixtures, `${name}-remote.git`)
  const repo = join(fixtures, name)
  mkdirSync(remote, { recursive: true })
  mkdirSync(repo, { recursive: true })
  git(remote, 'init', '--bare', '-q')
  git(repo, 'init', '-q')
  git(repo, 'symbolic-ref', 'HEAD', 'refs/heads/main')
  git(repo, 'config', 'user.name', 'Test User')
  git(repo, 'config', 'user.email', 'test@example.com')
  git(repo, 'config', 'commit.gpgsign', 'false')
  writeFileSync(join(repo, 'keep.txt'), ['keep', 'one', 'two', 'three', 'four', 'five', 'six', ''].join('\n'))
  mkdirSync(join(repo, 'src', 'deep'), { recursive: true })
  writeFileSync(join(repo, 'src', 'deep', 'app.txt'), 'app\n')
  writeFileSync(join(repo, 'old.txt'), 'old one\nold two\n')
  writeFileSync(join(repo, 'rename-me.txt'), 'same content\non both sides\n')
  git(repo, 'add', '-A')
  git(repo, 'commit', '-q', '-m', 'first')
  git(repo, 'remote', 'add', 'origin', remote)
  git(repo, 'push', '-q', 'origin', 'main')
  return { repo, remote }
}

async function openProject(page: Page, folder: string) {
  await page.getByRole('button', { name: 'Add project' }).first().click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Folder on this computer').fill(folder)
  await dialog.getByRole('button', { name: 'Go', exact: true }).click()
  await dialog.getByRole('button', { name: 'Open this folder' }).click()
  await expect(dialog).toBeHidden()
}

test('git: review, commit and push an agent\'s worktree', async ({ page }) => {
  test.setTimeout(120_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const { repo, remote } = makeRepo(`git-${run}`)
  const branch = `agent-${run}`
  await signIn(page)
  await page.goto('/?test')
  const problems = collectProblems(page)
  await openProject(page, repo)

  // Start a chat in its own worktree, on a named branch.
  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New chat' }).click()
  await expect(panel.getByRole('heading', { name: 'Start a chat' })).toBeVisible()
  await panel.getByText('Options', { exact: true }).click()
  await panel.getByLabel('Work in its own worktree').check()
  await panel.getByLabel('Branch name (optional)').fill(branch)
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await expect(panel.getByText(branch, { exact: true })).toBeVisible()

  // The agent's work, written into the worktree on disk: a new file, a deleted one, a renamed one.
  const trees = join(repo, '.mooring', 'worktrees')
  await expect.poll(() => (existsSync(trees) ? readdirSync(trees).length : 0)).toBe(1)
  const name = readdirSync(trees)[0]
  const tree = join(trees, name)
  writeFileSync(join(tree, 'new.txt'), 'hello from the agent\n')
  rmSync(join(tree, 'old.txt'))
  renameSync(join(tree, 'rename-me.txt'), join(tree, 'renamed.txt'))

  // Open the chat's changes: the place is that worktree.
  await panel.getByRole('button', { name: /Open changes/ }).click()
  const changes = page.locator('section[aria-label^="Changes in"]')
  await expect(changes.getByLabel('Worktree', { exact: true })).toHaveValue(`.mooring/worktrees/${name}`)
  await changes.getByRole('button', { name: 'Refresh' }).click()
  await expect(changes.getByLabel('Branch')).toContainText(branch)
  await expect(changes.getByLabel('Branch')).toContainText('No upstream')
  const unstaged = changes.getByRole('list', { name: 'Unstaged files' })
  await expect(unstaged.getByRole('button', { name: 'new.txt, new. Show changes' })).toBeVisible()
  await expect(unstaged.getByRole('button', { name: 'old.txt, deleted. Show changes' })).toBeVisible()
  await expect(unstaged.getByRole('button', { name: 'rename-me.txt, deleted. Show changes' })).toBeVisible()
  await expect(unstaged.getByRole('button', { name: 'renamed.txt, new. Show changes' })).toBeVisible()

  // A diff of a new file: added lines with new numbers only. Stage it from the diff tab.
  await unstaged.getByRole('button', { name: 'new.txt, new. Show changes' }).click()
  const diff = page.getByRole('tabpanel')
  await expect(diff.getByRole('heading', { name: 'new.txt' })).toBeVisible()
  await expect(diff.getByText('hello from the agent')).toBeVisible()
  await expect(diff.getByText('Added: ')).toHaveCount(1)
  await expect(diff.locator('.line .n').nth(0)).toHaveText('')
  await expect(diff.locator('.line .n').nth(1)).toHaveText('1')
  await diff.getByRole('button', { name: 'Stage file' }).click()
  await expect(diff.getByRole('button', { name: 'Unstage file' })).toBeVisible()
  await expect(diff.getByRole('button', { name: 'Open in editor' })).toBeEnabled()

  // A diff of a deleted file: removed lines with old numbers only, and nothing to open.
  await unstaged.getByRole('button', { name: 'old.txt, deleted. Show changes' }).click()
  await expect(diff.getByText('Removed: ')).toHaveCount(2)
  await expect(diff.locator('.line .n').nth(0)).toHaveText('1')
  await expect(diff.getByRole('button', { name: 'Open in editor' })).toBeDisabled()

  // Stage the rest: the rename joins up into one renamed file.
  await changes.getByRole('button', { name: 'Stage all', exact: true }).click()
  const staged = changes.getByRole('list', { name: 'Staged files' })
  await expect(staged.getByRole('button', { name: 'renamed.txt, renamed. Show changes' })).toBeVisible()
  await expect(staged.getByText('was rename-me.txt')).toBeVisible()
  await expect(staged.getByRole('button', { name: 'new.txt, new. Show changes' })).toBeVisible()
  await expect(staged.getByRole('button', { name: 'old.txt, deleted. Show changes' })).toBeVisible()
  await expect(changes.getByRole('list', { name: 'Unstaged files' })).toHaveCount(0)
  await staged.getByRole('button', { name: 'renamed.txt, renamed. Show changes' }).click()
  await expect(diff.getByText('with no change to the content')).toBeVisible()

  // A hook that refuses the commit: its output is shown. Then the commit goes through.
  const hook = join(repo, '.git', 'hooks', 'pre-commit')
  writeFileSync(hook, '#!/bin/sh\necho "lint: fix the trailing spaces" >&2\nexit 1\n', { mode: 0o755 })
  const message = changes.getByLabel('Commit message')
  await expect(changes.getByRole('button', { name: 'Commit', exact: true })).toBeDisabled()
  await message.fill('Agent work: add, delete, rename')
  await changes.getByRole('button', { name: 'Commit', exact: true }).click()
  const alert = changes.getByRole('alert')
  await expect(alert).toContainText('A commit hook refused the commit.')
  await expect(alert).toContainText('lint: fix the trailing spaces')
  rmSync(hook)
  await message.press('Control+Enter')
  await expect(changes.getByRole('status').filter({ hasText: /^Committed/ })).toBeVisible()
  await expect(changes.getByText('No changes.')).toBeVisible()
  await expect(changes.getByRole('list', { name: 'Staged files' })).toHaveCount(0)

  // Push with no upstream sets it.
  await changes.getByRole('button', { name: 'Push and set upstream' }).click()
  await expect(changes.getByRole('status').filter({ hasText: `Pushed to origin/${branch}` })).toBeVisible()
  await expect(changes.getByLabel('Branch')).toContainText(`0 ahead, 0 behind origin/${branch}`)
  const pushed = git(remote, 'ls-tree', '-r', '--name-only', branch).split('\n').filter(Boolean)
  expect(pushed.sort()).toEqual(['keep.txt', 'new.txt', 'renamed.txt', 'src/deep/app.txt'])
  expect(git(remote, 'log', '-1', '--format=%s', branch).trim()).toBe('Agent work: add, delete, rename')

  // Discard asks first. Keeping leaves the file; discarding removes it (a missing trash asks once more).
  writeFileSync(join(tree, 'scratch.txt'), 'throw away\n')
  await changes.getByRole('button', { name: 'Refresh' }).click()
  await changes.getByRole('button', { name: 'Discard changes to scratch.txt' }).click()
  const ask = changes.getByRole('group', { name: 'Confirm discard' })
  await expect(ask).toBeVisible()
  await ask.getByRole('button', { name: 'Keep changes' }).click()
  await expect(ask).toBeHidden()
  expect(existsSync(join(tree, 'scratch.txt'))).toBe(true)
  await changes.getByRole('button', { name: 'Discard changes to scratch.txt' }).click()
  await ask.getByRole('button', { name: 'Discard', exact: true }).click()
  await expect(async () => {
    const forever = ask.getByRole('button', { name: 'Discard for good' })
    if (await forever.isVisible()) await forever.click()
    expect(existsSync(join(tree, 'scratch.txt'))).toBe(false)
  }).toPass()
  await expect(changes.getByText('No changes.')).toBeVisible()

  // An edit deep in a folder shows up without Refresh. A rename with edits shows the old name and its
  // changes, and unstaging it puts both halves back.
  writeFileSync(join(tree, 'src', 'deep', 'app.txt'), 'app, edited by the agent\n')
  renameSync(join(tree, 'keep.txt'), join(tree, 'kept.txt'))
  writeFileSync(join(tree, 'kept.txt'), ['keep', 'one', 'two', 'three', 'four', 'five', 'six', 'seven', ''].join('\n'))
  const list = changes.getByRole('list', { name: 'Unstaged files' })
  await expect(list.getByRole('button', { name: /app\.txt, modified/ })).toBeVisible({ timeout: 10_000 })
  await changes.getByRole('button', { name: 'Stage all', exact: true }).click()
  await staged.getByRole('button', { name: 'kept.txt, renamed. Show changes' }).click()
  await expect(diff.locator('.sub code').first()).toHaveText('keep.txt')
  await expect(diff.getByText('Added: ')).toHaveCount(1)
  await diff.getByRole('button', { name: 'Unstage file' }).click()
  await expect(list.getByRole('button', { name: 'keep.txt, deleted. Show changes' })).toBeVisible()
  await expect(list.getByRole('button', { name: 'kept.txt, new. Show changes' })).toBeVisible()
  await expect(changes.getByRole('list', { name: 'Staged files', exact: true }).getByRole('button', { name: /kept\.txt|keep\.txt/ })).toHaveCount(0)
  await changes.getByRole('button', { name: 'Stage all', exact: true }).click()
  await expect(staged.getByRole('button', { name: 'kept.txt, renamed. Show changes' })).toBeVisible()
  await expect(changes.getByRole('list', { name: 'Unstaged files' })).toHaveCount(0)
  await message.fill('Agent work: rename with edits')
  await message.press('Control+Enter')
  await expect(changes.getByText('No changes.')).toBeVisible()

  // Alt+G shows the changes from the file tree, and a terminal can start in the worktree.
  await page.getByRole('button', { name: 'Files', exact: true }).click()
  await page.keyboard.press('Alt+G')
  await expect(changes).toBeVisible()
  await changes.getByRole('button', { name: 'Terminal here' }).click()
  await waitForPrompt(page)
  await type(page, 'basename "$PWD"')
  await expect.poll(() => text(page)).toContain(name)

  // Remove the worktree: its branch stays in the repository.
  await page.keyboard.press('Alt+G')
  await changes.getByRole('button', { name: 'Remove worktree' }).click()
  await changes.getByRole('group', { name: 'Confirm remove worktree' }).getByRole('button', { name: 'Remove folder' }).click()
  await expect(changes.getByRole('alert')).toContainText('A chat is running in this worktree')
  // Closing the chat's tab stops its agent, which frees the worktree.
  await page.getByRole('button', { name: 'Close New chat' }).click()
  await expect(page.locator('[data-chat-view]')).toHaveCount(0)
  // The agent takes a moment to stop; asking again is how the page is meant to be used. Then the
  // terminal started in the worktree is what refuses the removal.
  const yes = changes.getByRole('group', { name: 'Confirm remove worktree' }).getByRole('button', { name: 'Remove folder' })
  await expect(async () => {
    await yes.click()
    await expect(changes.getByRole('alert')).toContainText('A terminal is open in this worktree. Close it first.', { timeout: 2000 })
  }).toPass({ timeout: 40_000 })
  expect(existsSync(tree)).toBe(true)
  // Closing the terminal frees it.
  await page.getByRole('button', { name: new RegExp(`^Close .*${branch}`) }).click()
  await expect(page.locator('.xterm')).toHaveCount(0)
  await yes.click()
  await expect(changes.getByLabel('Worktree', { exact: true })).toHaveCount(0, { timeout: 10_000 })
  expect(existsSync(tree)).toBe(false)
  // Its diff tabs closed with it.
  await expect(page.getByRole('tab', { name: /kept\.txt|renamed\.txt|new\.txt|old\.txt/ })).toHaveCount(0)
  expect(git(repo, 'branch', '--list', branch)).toContain(branch)
  expect(problems).toEqual([])
})
