import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import { makeRepo, openProject } from './githelpers'
import { collectProblems, signIn } from './helpers'

const tree = (page: import('@playwright/test').Page) => page.getByRole('region', { name: /^Files in/ })
const editor = (page: import('@playwright/test').Page) => page.locator('[data-editor-view] .cm-content')

test('ux stage 1: tree order, blank tab, commit hint, unsaved edits, Alt in a text box', async ({ page }) => {
  test.setTimeout(90_000)
  const name = `ux1-${Date.now()}-${test.info().repeatEachIndex}`
  const repo = makeRepo(name, { 'notes.txt': 'one\n' })
  mkdirSync(join(repo, '.hidden'), { recursive: true })
  writeFileSync(join(repo, '.hidden', 'a.txt'), 'x')
  mkdirSync(join(repo, 'zdir'), { recursive: true })
  writeFileSync(join(repo, 'zdir', 'a.txt'), 'x')
  await signIn(page)
  const problems = collectProblems(page)
  await openProject(page, repo)
  writeFileSync(join(repo, 'notes.txt'), 'two\n')

  // The tree hides .git and puts dot folders after the others.
  await expect(tree(page).getByRole('button', { name: 'zdir', exact: true })).toBeVisible()
  await expect(tree(page).getByRole('button', { name: '.git', exact: true })).toHaveCount(0)
  const names = await tree(page).locator('button.open, [role="treeitem"]').allInnerTexts()
  const flat = names.join('|')
  expect(flat.indexOf('zdir')).toBeLessThan(flat.indexOf('.hidden'))

  // An empty pane says one short thing and offers three buttons; no worktree picker without worktrees.
  const panel = page.getByRole('tabpanel')
  await expect(panel.getByText(/Start a terminal or a chat/)).toBeVisible()
  await expect(panel.getByRole('button', { name: 'Open file' })).toBeVisible()
  await expect(panel.getByRole('button', { name: 'New terminal' })).toBeVisible()
  await expect(panel.getByRole('button', { name: 'New chat' })).toBeVisible()
  await expect(page.getByLabel('Worktree', { exact: true })).toHaveCount(0)

  // Save only shows once a file has unsaved edits, and then it is the primary button.
  await tree(page).getByRole('button', { name: 'notes.txt', exact: true }).click()
  await expect(editor(page)).toContainText('two')
  await expect(page.getByRole('button', { name: 'Save', exact: true })).toHaveCount(0)
  // The Tab hint shows the first time the editor has focus, and not again.
  const hint = page.getByText('Tab indents.')
  await expect(hint).toHaveCount(0)
  await editor(page).click()
  await expect(hint).toBeVisible()
  await page.getByRole('heading', { name: 'Projects', exact: true }).click()
  await editor(page).click()
  await expect(hint).toHaveCount(0)
  await page.keyboard.type('more')
  const save = page.getByRole('button', { name: 'Save', exact: true })
  await expect(save).toBeVisible()
  await expect(save).toHaveClass(/primary/)

  // The page asks before it goes away.
  const asked = await page.evaluate(() => {
    const e = new Event('beforeunload', { cancelable: true })
    window.dispatchEvent(e)
    return e.defaultPrevented
  })
  expect(asked).toBe(true)

  // An empty message says what is missing.
  await page.keyboard.press('Alt+G')
  await page.getByRole('button', { name: 'Stage all' }).click()
  await expect(page.getByText('Write a message to commit.')).toBeVisible()

  // Alt+letter works while typing in a box (the commit message).
  await page.getByLabel('Commit message').click()
  await page.keyboard.press('Alt+O')
  await expect(page.getByRole('dialog')).toBeVisible()
  await page.keyboard.press('Escape')

  expect(problems).toEqual([])
})
