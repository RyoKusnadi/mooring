import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { collectProblems, fixtures, signIn } from './helpers'

// Phase 20, workspace tools: preview tabs, breadcrumbs, the tab menu
// The language server bar and Format are in lsp.spec.ts (the consent is shared, so one spec owns it); the tool server and SSH items are in toolservers.spec.ts and ssh.spec.ts.

const unique = () => `ux20-${Date.now() % 1_000_000}-${test.info().workerIndex}${test.info().repeatEachIndex}`
const tree = (page: Page) => page.getByRole('region', { name: /^Files in/ })
const editor = (page: Page) => page.locator('[data-editor-view] .cm-content')
const tabs = (page: Page) => page.locator('[data-tab]')
const previews = (page: Page) => page.locator('[data-tab][data-preview]')

async function openFolder(page: Page, folder: string) {
  await signIn(page)
  await page.getByRole('button', { name: 'Add project' }).first().click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Folder on this computer').fill(folder)
  await dialog.getByRole('button', { name: 'Go', exact: true }).click()
  await dialog.getByRole('button', { name: 'Open this folder' }).click()
  await expect(dialog).toBeHidden()
}

function makeFolder(files: Record<string, string>): string {
  const folder = join(fixtures, unique())
  for (const [name, body] of Object.entries(files)) {
    mkdirSync(join(folder, name, '..'), { recursive: true })
    writeFileSync(join(folder, name), body)
  }
  return folder
}

test('a single click previews a file, the next click replaces it, edit or double click keeps it', async ({ page }) => {
  const problems = collectProblems(page)
  await openFolder(page, makeFolder({ 'a.txt': 'alpha\n', 'b.txt': 'bravo\n', 'c.txt': 'charlie\n', 'd.txt': 'delta\n' }))

  await tree(page).getByRole('button', { name: 'a.txt', exact: true }).click()
  await expect(editor(page)).toContainText('alpha')
  await expect(tabs(page)).toHaveCount(1)
  await expect(previews(page)).toHaveCount(1)
  await expect(previews(page).getByRole('tab')).toHaveCSS('font-style', 'normal')
  await expect(previews(page).locator('.name')).toHaveCSS('font-style', 'italic')

  await tree(page).getByRole('button', { name: 'b.txt', exact: true }).click()
  await expect(editor(page)).toContainText('bravo')
  await expect(tabs(page)).toHaveCount(1)
  await expect(page.getByRole('tab', { name: /a\.txt/ })).toHaveCount(0)

  // A double click keeps it, so the next click opens a second tab.
  await tree(page).getByRole('button', { name: 'b.txt', exact: true }).dblclick()
  await expect(previews(page)).toHaveCount(0)
  await tree(page).getByRole('button', { name: 'c.txt', exact: true }).click()
  await expect(editor(page)).toContainText('charlie')
  await expect(tabs(page)).toHaveCount(2)
  await expect(previews(page)).toHaveCount(1)

  // Typing in a preview keeps it.
  await editor(page).click()
  await page.keyboard.type('x')
  await expect(previews(page)).toHaveCount(0)
  await tree(page).getByRole('button', { name: 'd.txt', exact: true }).click()
  await expect(tabs(page)).toHaveCount(3)

  // Enter on a tree row opens a normal tab.
  await tree(page).getByRole('button', { name: 'a.txt', exact: true }).focus()
  await page.keyboard.press('Enter')
  await expect(editor(page)).toContainText('alpha')
  await expect(previews(page)).toHaveCount(1)
  expect(problems.filter((p) => !/Failed to load resource/.test(p))).toEqual([])
})

test('breadcrumbs show the folders of the file and reveal one in the tree', async ({ page }) => {
  await openFolder(page, makeFolder({ 'src/lib/deep.txt': 'deep\n', 'top.txt': 'top\n' }))
  await tree(page).getByRole('button', { name: 'src', exact: true }).click()
  await tree(page).getByRole('button', { name: 'lib', exact: true }).click()
  await tree(page).getByRole('button', { name: 'deep.txt', exact: true }).click()
  await expect(editor(page)).toContainText('deep')

  const crumbs = page.getByRole('navigation', { name: 'Path of src/lib/deep.txt' })
  await expect(crumbs.getByRole('button')).toHaveText(['src', 'lib'])
  await expect(crumbs.getByText('deep.txt', { exact: true })).toBeVisible()

  // Fold the tree, then the crumb opens the folders again and puts focus on that folder's row.
  await tree(page).getByRole('button', { name: 'src', exact: true }).click()
  await expect(tree(page).getByRole('button', { name: 'deep.txt', exact: true })).toHaveCount(0)
  await crumbs.getByRole('button', { name: /^Show folder src\/lib/ }).click()
  await expect(tree(page).getByRole('button', { name: 'deep.txt', exact: true })).toBeVisible()
  await expect(page.locator('[data-path="src/lib"]')).toBeFocused()

  // Keyboard: Tab reaches the crumbs and Enter does the same.
  await tree(page).getByRole('button', { name: 'src', exact: true }).click()
  await crumbs.getByRole('button', { name: /^Show folder src in/ }).focus()
  await page.keyboard.press('Enter')
  await expect(page.locator('[data-path="src"]')).toBeFocused()
})

test('a long path keeps the whole file name in view, at desktop and phone width', async ({ page }) => {
  const dirs = ['alpha', 'bravo', 'charlie', 'delta', 'echo', 'foxtrot', 'golf', 'hotel']
  const name = 'invoice_payment_failed.txt'
  await openFolder(page, makeFolder({ [`${dirs.join('/')}/${name}`]: 'x\n' }))
  let at = ''
  for (const d of dirs) {
    at = at ? `${at}/${d}` : d
    await page.locator(`[data-path="${at}"]`).click()
  }
  await tree(page).getByRole('button', { name, exact: true }).click()
  const crumbs = page.getByRole('navigation', { name: `Path of ${dirs.join('/')}/${name}` })
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 844 })
    const file = crumbs.getByText(name, { exact: true })
    await expect(file).toBeVisible()
    // The name is not clipped, and it ends inside the strip.
    await expect
      .poll(() =>
        file.evaluate((e) => {
          const nav = e.closest('nav')!.getBoundingClientRect()
          const box = e.getBoundingClientRect()
          return e.scrollWidth > e.clientWidth + 1 || box.right > nav.right + 1 ? `clipped ${e.scrollWidth}/${e.clientWidth}, nav ${Math.round(nav.width)}, name ends at ${Math.round(box.right)} of ${Math.round(nav.right)}` : 'whole'
        }),
      )
      .toBe('whole')
  }
})

test('the tab menu closes, pins and reveals, by mouse and by keyboard', async ({ page }) => {
  await openFolder(page, makeFolder({ 'one.txt': '1\n', 'two.txt': '2\n', 'three.txt': '3\n', 'sub/four.txt': '4\n' }))
  for (const n of ['one.txt', 'two.txt', 'three.txt']) {
    await tree(page).getByRole('button', { name: n, exact: true }).dblclick()
    await expect(page.getByRole('tab', { name: new RegExp(n.replace('.', '\\.')) })).toBeVisible()
  }
  await expect(tabs(page)).toHaveCount(3)

  // Pin: the close button goes and the tab moves to the front.
  await page.getByRole('tab', { name: /two\.txt/ }).click({ button: 'right' })
  const menu = page.getByRole('menu', { name: /^Tab / })
  await expect(menu.getByRole('menuitem')).toHaveText(['Close', 'Close others', 'Close to the right', 'Copy path', 'Copy full path', 'Reveal in files', 'Pin'])
  await menu.getByRole('menuitem', { name: 'Pin' }).click()
  await expect(menu).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Close two.txt' })).toBeHidden()
  await expect(tabs(page).first().getByRole('tab')).toHaveText(/two\.txt/)
  await expect(page.getByRole('img', { name: 'Pinned' })).toBeVisible()

  // Reveal puts focus in the tree, on that file.
  await page.getByRole('tab', { name: /three\.txt/ }).click({ button: 'right' })
  await menu.getByRole('menuitem', { name: 'Reveal in files' }).click()
  await expect(page.locator('[data-path="three.txt"]')).toBeFocused()

  // Copy path says so in the status bar.
  await page.context().grantPermissions(['clipboard-read', 'clipboard-write'])
  await page.getByRole('tab', { name: /three\.txt/ }).click({ button: 'right' })
  await menu.getByRole('menuitem', { name: 'Copy path' }).click()
  await expect(page.getByRole('contentinfo').getByText('Copied the path.')).toBeVisible()
  // The project-relative path; the full path is its own item.
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe('three.txt')
  await page.getByRole('tab', { name: /three\.txt/ }).click({ button: 'right' })
  await menu.getByRole('menuitem', { name: 'Copy full path' }).click()
  expect(await page.evaluate(() => navigator.clipboard.readText())).toMatch(/\/three\.txt$/)

  // Keyboard: the menu key opens it on the focused tab, arrows move, Escape gives the focus back.
  await page.getByRole('tab', { name: /one\.txt/ }).focus()
  await page.keyboard.press('Shift+F10')
  await expect(menu).toBeVisible()
  await expect(menu.getByRole('menuitem', { name: 'Close', exact: true })).toBeFocused()
  await page.keyboard.press('ArrowDown')
  await expect(menu.getByRole('menuitem', { name: 'Close others' })).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(menu).toHaveCount(0)
  await expect(page.getByRole('tab', { name: /one\.txt/ })).toBeFocused()

  // A tab with unsaved edits is not closed silently: it stays and asks, like a single close.
  await page.getByRole('tab', { name: /three\.txt/ }).click()
  await editor(page).click()
  await page.keyboard.type('x')
  await expect(page.getByRole('img', { name: 'Unsaved changes' })).toBeVisible()
  await page.getByRole('tab', { name: /one\.txt/ }).click({ button: 'right' })
  await menu.getByRole('menuitem', { name: 'Close to the right' }).click()
  await expect(page.getByText('Close three.txt without saving?')).toBeVisible()
  await page.getByRole('button', { name: 'Discard changes' }).click()
  // The unsaved tab was the last one to the right; the pinned tab stays.
  await expect(page.getByRole('tab', { name: /three\.txt/ })).toHaveCount(0)
  await page.getByRole('tab', { name: /one\.txt/ }).click({ button: 'right' })
  // Only the pinned tab is left to close, so the menu does not offer either.
  await expect(menu.getByRole('menuitem')).toHaveText(['Close', 'Copy path', 'Copy full path', 'Reveal in files', 'Pin'])
  await page.keyboard.press('Escape')
  await expect(tabs(page)).toHaveCount(2)
  await expect(page.getByRole('tab', { name: /two\.txt/ })).toBeVisible()

  // Phone width: the menu stays on screen and the page does not scroll sideways.
  await page.setViewportSize({ width: 390, height: 844 })
  await page.getByRole('tab', { name: /one\.txt/ }).click({ button: 'right' })
  const box = (await menu.boundingBox())!
  expect(box.x).toBeGreaterThanOrEqual(0)
  expect(box.x + box.width).toBeLessThanOrEqual(390)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  await page.keyboard.press('Escape')
})
