import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import { collectProblems, fixtures, signIn } from './helpers'
import { openProject } from './githelpers'

// Row menus sit in lists that scroll. They must not be clipped, at the smallest list and on a phone.
test('the project and file row menus are whole, inside the window, and reach Remove and Delete', async ({ page }) => {
  test.setTimeout(60_000)
  const name = `menus-${Date.now() % 1_000_000}`
  const folder = join(fixtures, name)
  mkdirSync(join(folder, 'sub'), { recursive: true })
  writeFileSync(join(folder, 'a.txt'), 'a\n')
  await signIn(page)
  const problems = collectProblems(page)
  await page.setViewportSize({ width: 1280, height: 480 })
  await openProject(page, folder)
  const inWindow = async (el: ReturnType<typeof page.locator>) => {
    const b = (await el.boundingBox())!
    const size = page.viewportSize()!
    expect(b.x).toBeGreaterThanOrEqual(0)
    expect(b.y).toBeGreaterThanOrEqual(0)
    expect(b.x + b.width).toBeLessThanOrEqual(size.width)
    expect(b.y + b.height).toBeLessThanOrEqual(size.height)
  }

  // Project: every item is visible, the list keeps at least one row, and Remove asks first.
  const nav = page.getByRole('navigation', { name: 'Projects' })
  const rows = nav.locator('ul').first()
  expect((await rows.boundingBox())!.height).toBeGreaterThanOrEqual(28)
  // The list holds projects from other tests. A scroll event arrives a frame after the scroll and closes
  // an open menu, so the row is brought into view and the frame passes before the click.
  await nav.getByRole('button', { name: `Actions for ${name}` }).scrollIntoViewIfNeeded()
  await page.evaluate(() => new Promise((done) => requestAnimationFrame(() => requestAnimationFrame(done))))
  await nav.getByRole('button', { name: `Actions for ${name}` }).click()
  const menu = page.getByRole('menu', { name: `Actions for ${name}` })
  await expect(menu.getByRole('menuitem')).toHaveCount(4)
  for (const item of await menu.getByRole('menuitem').all()) await expect(item).toBeInViewport({ ratio: 1 })
  await inWindow(menu)
  await menu.getByRole('menuitem', { name: 'Remove from list…' }).click()
  await expect(page.getByText(`Remove “${name}” from the list?`)).toBeVisible()
  await inWindow(page.getByRole('group', { name: `Actions for ${name}` }))
  await page.getByRole('button', { name: 'Cancel' }).click()
  await expect(nav.getByRole('button', { name: `Actions for ${name}` })).toBeFocused()

  // The same by keyboard: open on Enter, arrow down to the last item, Escape closes and keeps focus.
  await page.keyboard.press('Enter')
  await expect(menu).toBeVisible()
  await page.keyboard.press('ArrowUp')
  await expect(menu.getByRole('menuitem', { name: 'Remove from list…' })).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(menu).toHaveCount(0)
  await expect(nav.getByRole('button', { name: `Actions for ${name}` })).toBeFocused()

  // A folder in the tree, on a phone.
  await page.setViewportSize({ width: 390, height: 600 })
  const files = page.getByRole('region', { name: /^Files in/ })
  // The side panel is still moving after the resize; let it settle, then open it only if it is shut.
  const sub = files.getByRole('button', { name: 'sub', exact: true })
  await page.waitForTimeout(600)
  if (!(await sub.isVisible())) await page.getByRole('button', { name: 'Projects', exact: true }).click()
  await expect(sub).toBeVisible({ timeout: 10_000 })
  await sub.hover()
  await files.getByRole('button', { name: 'Actions for sub' }).click()
  const fileMenu = page.getByRole('menu', { name: 'Actions for sub' })
  await expect(fileMenu).toBeVisible()
  await inWindow(fileMenu)
  await fileMenu.getByRole('menuitem', { name: 'Delete…' }).click()
  await expect(files.getByRole('group', { name: 'Delete sub' })).toBeVisible()
  await page.keyboard.press('Escape')
  expect(problems).toEqual([])
})
