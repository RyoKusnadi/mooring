import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Locator, type Page } from '@playwright/test'
import { fixtures, signIn } from './helpers'

// The stand-in gopls (see serve.sh and internal/fakelsp/refs.go) answers references and rename with
// every whole-word match of the name under the cursor in the .go files of the folder.

async function addProject(page: Page, folder: string) {
  await page.getByRole('button', { name: 'Add project' }).first().click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Folder on this computer').fill(folder)
  await dialog.getByRole('button', { name: 'Go', exact: true }).click()
  await dialog.getByRole('button', { name: 'Open this folder' }).click()
  await expect(dialog).toBeHidden()
}

const tree = (page: Page) => page.getByRole('region', { name: /^Files in/ })

/** An earlier test on this server may have allowed gopls already; then the bar does not ask. */
async function allowIfAsked(page: Page, ready: Locator) {
  const allow = page.getByRole('group', { name: 'Go (gopls) language server' }).getByRole('button', { name: 'Allow and start' })
  await expect(allow.or(ready)).toBeVisible({ timeout: 30_000 })
  if (await allow.isVisible()) await allow.click()
  await expect(ready).toBeVisible({ timeout: 30_000 })
}

/** Two files that share the name `helper`; a.go is open with the cursor at the start of the name. */
async function setup(page: Page, label: string) {
  const folder = join(fixtures, `lsprefs-${label}-${Date.now()}-${test.info().repeatEachIndex}`)
  mkdirSync(folder, { recursive: true })
  writeFileSync(join(folder, 'a.go'), 'package main\n\nfunc helper() {}\n')
  writeFileSync(join(folder, 'b.go'), 'package main\n\nfunc run() {\n\thelper()\n\thelper()\n}\n')
  await signIn(page)
  await addProject(page, folder)
  await tree(page).getByRole('button', { name: 'a.go', exact: true }).click()
  await allowIfAsked(page, page.getByRole('button', { name: 'References', exact: true }))
  await placeCursor(page)
  return folder
}

async function placeCursor(page: Page) {
  await page.locator('[data-editor-view] .cm-content').click()
  await page.keyboard.press('ControlOrMeta+Home')
  await page.keyboard.press('ArrowDown')
  await page.keyboard.press('ArrowDown')
  for (let i = 0; i < 5; i++) await page.keyboard.press('ArrowRight')
}

test('references: Shift+F12 lists both files, Enter opens the line', async ({ page }) => {
  test.setTimeout(120_000)
  await setup(page, 'refs')
  await page.keyboard.press('Shift+F12')
  const dialog = page.getByRole('dialog', { name: 'References to helper' })
  await expect(dialog).toContainText('3 references in 2 files')
  await expect(dialog.getByRole('region', { name: 'a.go' })).toBeVisible()
  await expect(dialog.getByRole('region', { name: 'b.go' })).toBeVisible()
  const rows = dialog.getByRole('button', { name: /^b\.go:/ })
  await expect(rows).toHaveCount(2)
  await expect(rows.first()).toContainText('helper()')
  await rows.first().focus()
  await page.keyboard.press('Enter')
  await expect(dialog).toBeHidden()
  await expect(page.locator('[data-editor-view] .cm-activeLine')).toContainText('helper()')

  // The palette has it too.
  await page.keyboard.press('ControlOrMeta+k')
  await page.keyboard.type('Find references')
  await page.keyboard.press('Enter')
  await expect(page.getByRole('dialog', { name: /^References/ })).toBeVisible()
})

test('rename: open editor changes unsaved, closed file is saved', async ({ page }) => {
  test.setTimeout(120_000)
  const folder = await setup(page, 'rename')
  await page.keyboard.press('F2')
  const dialog = page.getByRole('dialog', { name: /^Rename/ })
  const name = dialog.getByLabel('New name')
  await expect(name).toHaveValue('helper')
  await expect(name).toBeFocused()

  await dialog.getByRole('button', { name: 'Preview' }).click()
  await expect(dialog).toContainText('The new name is the same')
  await name.fill('  ')
  await dialog.getByRole('button', { name: 'Preview' }).click()
  await expect(dialog).toContainText('Type a new name')

  await name.fill('assist')
  await page.keyboard.press('Enter')
  await expect(dialog).toContainText('Rename in 2 files, 3 places')
  await expect(dialog).toContainText('b.go')
  await dialog.getByRole('button', { name: 'Rename', exact: true }).click()
  await expect(dialog).toContainText('Changed 2 files')
  await dialog.getByRole('button', { name: 'Close' }).first().click()
  await expect(dialog).toBeHidden()

  await expect(page.locator('[data-editor-view] .cm-content')).toContainText('func assist()')
  await expect(page.getByText('Unsaved changes')).toBeVisible()
  expect(readFileSync(join(folder, 'a.go'), 'utf8')).toContain('func helper()')
  await expect.poll(() => readFileSync(join(folder, 'b.go'), 'utf8')).toBe('package main\n\nfunc run() {\n\tassist()\n\tassist()\n}\n')
})

test.describe('phone', () => {
  test.use({ hasTouch: true })
  test('references and rename: no sideways scroll, 44 px targets', async ({ page }) => {
    test.setTimeout(120_000)
    await setup(page, 'phone')
    await page.setViewportSize({ width: 390, height: 800 })
    const refs = page.getByRole('button', { name: 'References', exact: true })
    await expect(refs).toBeVisible()
    await placeCursor(page)
    expect((await refs.boundingBox())!.height).toBeGreaterThanOrEqual(44)
    await refs.click()
    const dialog = page.getByRole('dialog', { name: 'References to helper' })
    const row = dialog.getByRole('button', { name: /^b\.go:/ }).first()
    await expect(row).toBeVisible()
    expect((await row.boundingBox())!.height).toBeGreaterThanOrEqual(44)
    expect(await dialog.evaluate((d) => d.scrollWidth <= d.clientWidth + 1)).toBe(true)
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true)
    await page.keyboard.press('Escape')
    await expect(dialog).toBeHidden()

    await page.getByRole('button', { name: 'Rename', exact: true }).click()
    const rename = page.getByRole('dialog', { name: 'Rename helper' })
    const name = rename.getByLabel('New name')
    expect((await name.boundingBox())!.height).toBeGreaterThanOrEqual(44)
    expect((await rename.getByRole('button', { name: 'Preview' }).boundingBox())!.height).toBeGreaterThanOrEqual(44)
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true)
  })
})
