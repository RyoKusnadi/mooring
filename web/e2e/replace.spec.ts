import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Locator, type Page } from '@playwright/test'
import { fixtures, signIn } from './helpers'

async function addProject(page: Page, folder: string) {
  await page.getByRole('button', { name: 'Add project' }).first().click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Folder on this computer').fill(folder)
  await dialog.getByRole('button', { name: 'Go', exact: true }).click()
  await dialog.getByRole('button', { name: 'Open this folder' }).click()
  await expect(dialog).toBeHidden()
}

const tree = (page: Page) => page.getByRole('region', { name: /^Files in/ })
const panel = (page: Page) => page.getByRole('dialog', { name: 'Find in project' })

/** An earlier test on this server may have allowed gopls already; then the bar does not ask. */
async function allowIfAsked(page: Page, ready: Locator) {
  const allow = page.getByRole('group', { name: 'Go (gopls) language server' }).getByRole('button', { name: 'Allow and start' })
  await expect(allow.or(ready)).toBeVisible({ timeout: 30_000 })
  if (await allow.isVisible()) await allow.click()
  await expect(ready).toBeVisible({ timeout: 30_000 })
}

async function search(page: Page, q: string) {
  await page.keyboard.press('Alt+F')
  await expect(panel(page)).toBeVisible()
  await page.locator('#find-q').fill(q)
  await expect(panel(page).getByRole('status').first()).toContainText(/\d+ lines?/)
}

function project(label: string, files: Record<string, string>) {
  const folder = join(fixtures, `replace-${label}-${Date.now()}-${test.info().repeatEachIndex}`)
  mkdirSync(folder, { recursive: true })
  for (const [name, text] of Object.entries(files)) writeFileSync(join(folder, name), text)
  return folder
}

test('replace: an open editor changes unsaved, a closed file is saved', async ({ page }) => {
  test.setTimeout(120_000)
  const folder = project('open', { 'a.go': 'package main\n\nfunc greet() {}\n', 'b.go': 'package main\n\nfunc run() { greet() }\n' })
  await signIn(page)
  await addProject(page, folder)
  await tree(page).getByRole('button', { name: 'a.go', exact: true }).click()
  await allowIfAsked(page, page.getByRole('button', { name: 'References', exact: true }))
  await page.locator('[data-editor-view] .cm-content').click()
  await page.keyboard.press('ControlOrMeta+End')
  await page.keyboard.type('// greet me')
  await expect(page.getByText('Unsaved changes')).toBeVisible()

  await search(page, 'greet')
  // Alt+R is left to the search box on a Mac (Option+R types a character there), so click.
  await panel(page).getByRole('button', { name: 'Replace…' }).click()
  const field = panel(page).getByLabel('Replace with')
  await expect(field).toBeFocused()
  await field.fill('hello')
  await page.keyboard.press('Enter')
  await expect(panel(page)).toContainText('Replace in 2 files, 3 places')
  await expect(panel(page).getByRole('region', { name: 'a.go' })).toContainText('open in an editor, not saved')
  await expect(panel(page).getByRole('region', { name: 'b.go' })).toContainText('saved to disk')
  await expect(panel(page).getByRole('region', { name: 'b.go' })).toContainText('func run() { hello() }')
  const go = panel(page).getByRole('button', { name: 'Replace', exact: true })
  await expect(go).toBeFocused()
  await go.click()
  await expect(panel(page)).toContainText('Changed 2 files')
  await expect(panel(page).getByRole('button', { name: 'Done' })).toBeFocused()

  await expect(page.locator('[data-editor-view] .cm-content')).toContainText('func hello()')
  await expect(page.getByText('Unsaved changes')).toBeVisible()
  expect(readFileSync(join(folder, 'a.go'), 'utf8')).toContain('func greet()')
  await expect.poll(() => readFileSync(join(folder, 'b.go'), 'utf8')).toBe('package main\n\nfunc run() { hello() }\n')
})

test('replace: a regex takes $1, and Esc steps back', async ({ page }) => {
  test.setTimeout(120_000)
  const folder = project('regex', { 'v.txt': 'var v1 = 1\nvar v2 = 2\n' })
  await signIn(page)
  await addProject(page, folder)
  await page.keyboard.press('Alt+F')
  await panel(page).getByLabel('Regex').check()
  await page.locator('#find-q').fill('v(\\d)')
  await expect(panel(page).getByRole('status').first()).toContainText('2 lines')
  await panel(page).getByRole('button', { name: 'Replace…' }).click()
  await panel(page).getByLabel('Replace with').fill('w$1')
  await panel(page).getByRole('button', { name: 'Preview replace' }).click()
  await expect(panel(page)).toContainText('Replace in 1 file, 2 places')
  await page.keyboard.press('Escape')
  await expect(panel(page).getByLabel('Replace with')).toBeVisible()
  await panel(page).getByRole('button', { name: 'Preview replace' }).click()
  await panel(page).getByRole('button', { name: 'Replace', exact: true }).click()
  await expect(panel(page)).toContainText('Changed 1 file')
  await expect.poll(() => readFileSync(join(folder, 'v.txt'), 'utf8')).toBe('var w1 = 1\nvar w2 = 2\n')
})

test('replace: a file changed after the preview is left alone, and the search is frozen', async ({ page }) => {
  test.setTimeout(120_000)
  const folder = project('stale', { 'a.txt': 'one cat\n', 'b.txt': 'two cat\n' })
  await signIn(page)
  await addProject(page, folder)
  await search(page, 'cat')
  await panel(page).getByRole('button', { name: 'Replace…' }).click()
  await panel(page).getByLabel('Replace with').fill('dog')
  await panel(page).getByRole('button', { name: 'Preview replace' }).click()
  await expect(panel(page)).toContainText('Replace in 2 files, 2 places')
  await expect(page.locator('#find-q')).toBeDisabled()
  writeFileSync(join(folder, 'b.txt'), 'two cat, edited elsewhere\n')
  await panel(page).getByRole('button', { name: 'Replace', exact: true }).click()
  await expect(panel(page)).toContainText('Changed 1 file')
  await expect(panel(page)).toContainText('The file changed after the preview, so it was left alone')
  await expect.poll(() => readFileSync(join(folder, 'a.txt'), 'utf8')).toBe('one dog\n')
  expect(readFileSync(join(folder, 'b.txt'), 'utf8')).toBe('two cat, edited elsewhere\n')
})

test.describe('phone', () => {
  test.use({ hasTouch: true })
  test('replace: no sideways scroll, 44 px targets', async ({ page }) => {
    test.setTimeout(120_000)
    const folder = project('phone', { 'long.txt': `${'x'.repeat(120)} needle ${'y'.repeat(120)}\n` })
    await signIn(page)
    await addProject(page, folder)
    await page.setViewportSize({ width: 390, height: 800 })
    await search(page, 'needle')
    const open = panel(page).getByRole('button', { name: 'Replace…' })
    expect((await open.boundingBox())!.height).toBeGreaterThanOrEqual(44)
    await open.click()
    await panel(page).getByLabel('Replace with').fill('thread')
    const preview = panel(page).getByRole('button', { name: 'Preview replace' })
    expect((await preview.boundingBox())!.height).toBeGreaterThanOrEqual(44)
    await preview.click()
    const go = panel(page).getByRole('button', { name: 'Replace', exact: true })
    await expect(go).toBeVisible()
    expect((await go.boundingBox())!.height).toBeGreaterThanOrEqual(44)
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true)
    expect(await panel(page).evaluate((d) => d.scrollWidth <= d.clientWidth + 1)).toBe(true)
  })
})
