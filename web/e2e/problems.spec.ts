import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Locator, type Page } from '@playwright/test'
import { fixtures, signIn } from './helpers'

// The stand-in gopls (see serve.sh) reports "found ERROR" for each line containing ERROR.

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
async function allowIfAsked(page: Page, count: Locator) {
  const allow = page.getByRole('group', { name: 'Go (gopls) language server' }).getByRole('button', { name: 'Allow and start' })
  await expect(allow.or(count)).toBeVisible({ timeout: 30_000 })
  if (await allow.isVisible()) await allow.click()
}

test('problems: count in the status bar, list, open at the line', async ({ page }) => {
  test.setTimeout(120_000)
  const folder = join(fixtures, `problems-${Date.now()}-${test.info().repeatEachIndex}`)
  mkdirSync(folder, { recursive: true })
  writeFileSync(join(folder, 'bad.go'), 'package main\n\nfunc a() {}\n\nvar x = ERROR\n')
  await signIn(page)
  await addProject(page, folder)

  // Nothing runs yet: no count, and the list says why.
  await expect(page.getByRole('button', { name: /^Show problems:/ })).toHaveCount(0)
  await page.keyboard.press('Alt+P')
  const dialog = page.getByRole('dialog', { name: 'Problems' })
  await expect(dialog).toContainText('No running language server reports problems')
  await page.keyboard.press('Escape')
  await expect(dialog).toBeHidden()

  await tree(page).getByRole('button', { name: 'bad.go', exact: true }).click()
  const count = page.getByRole('button', { name: 'Show problems: 1 error', exact: true })
  await allowIfAsked(page, count)
  await expect(count).toBeVisible({ timeout: 30_000 })
  await count.click()
  await expect(dialog).toBeVisible()
  const row = dialog.getByRole('button', { name: /found ERROR/ })
  await expect(row).toContainText('Error')
  await expect(row).toContainText('bad.go:5:1')
  await row.focus()
  await page.keyboard.press('Enter')
  await expect(dialog).toBeHidden()
  await expect(page.locator('[data-editor-view] .cm-activeLine')).toContainText('var x = ERROR')

  // The palette has it too.
  await page.keyboard.press('ControlOrMeta+k')
  await page.keyboard.type('Show problems')
  await page.keyboard.press('Enter')
  await expect(dialog).toBeVisible()
})

test.describe('phone', () => {
  test.use({ hasTouch: true })
  test('problems: phone width has no sideways scroll and 44 px rows', async ({ page }) => {
    test.setTimeout(120_000)
    const folder = join(fixtures, `problems-phone-${Date.now()}-${test.info().repeatEachIndex}`)
    mkdirSync(folder, { recursive: true })
    writeFileSync(join(folder, 'bad.go'), 'package main\n\nvar averyveryverylongname_averyveryverylongname_averyveryverylongname = ERROR\n')
    await signIn(page)
    await addProject(page, folder)
    await tree(page).getByRole('button', { name: 'bad.go', exact: true }).click()
    const count = page.getByRole('button', { name: 'Show problems: 1 error', exact: true })
    await allowIfAsked(page, count)
    await expect(count).toBeVisible({ timeout: 30_000 })
    await page.setViewportSize({ width: 390, height: 800 })
    // The status bar lays out again after the resize; read the size once it has.
    await expect.poll(async () => (await count.boundingBox())?.height ?? 0).toBeGreaterThanOrEqual(44)
    await count.click()
    const dialog = page.getByRole('dialog', { name: 'Problems' })
    const row = dialog.getByRole('button', { name: /found ERROR/ })
    await expect.poll(async () => (await row.boundingBox())?.height ?? 0).toBeGreaterThanOrEqual(44)
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true)
    expect(await dialog.evaluate((d) => d.scrollWidth <= d.clientWidth + 1)).toBe(true)
  })
})
