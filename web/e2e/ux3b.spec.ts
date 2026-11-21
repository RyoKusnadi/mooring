import { writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { makeRepo, openProject } from './githelpers'
import { openSettings, signIn } from './helpers'

// Phase 17, second part: Split in an empty pane, the dialog frame, the terminal, review fixes, the diff header.

const names = () => ({ project: `ux3b-fixture-project-${Date.now() % 1_000_000}-${test.info().workerIndex}${test.info().repeatEachIndex}`, file: 'notes-for-the-release.md' })
const tabsOf = (page: Page) => page.getByRole('tab')

test('Split works in an empty pane and opens a terminal', async ({ page }) => {
  const n = names()
  const repo = makeRepo(n.project, { [n.file]: 'hello\n' })
  await page.setViewportSize({ width: 1440, height: 900 })
  await signIn(page)
  await openProject(page, repo)
  await expect(tabsOf(page)).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Split down' })).toBeEnabled()
  await page.getByRole('button', { name: 'Split down' }).click()
  await expect(page.locator('[data-tab][data-kind="terminal"]')).toHaveCount(1)
})

test('every dialog shares one frame: 480 or 640 wide, top edge at 10dvh', async ({ page }) => {
  const n = names()
  const repo = makeRepo(n.project, { [n.file]: 'hello\n' })
  await page.setViewportSize({ width: 1440, height: 900 })
  await signIn(page)
  await openProject(page, repo)
  const frame = async (name: string) => {
    const box = await page.getByRole('dialog', { name }).boundingBox()
    return { w: Math.round(box?.width ?? 0), top: Math.round(box?.y ?? 0) }
  }
  await page.keyboard.press('ControlOrMeta+k')
  expect(await frame('Command palette')).toEqual({ w: 480, top: 90 })
  await page.keyboard.press('Escape')
  await page.getByRole('button', { name: 'New tab' }).click()
  await page.getByRole('menuitem', { name: 'Open file' }).click()
  expect(await frame('Find in project')).toEqual({ w: 640, top: 90 })
  await page.keyboard.press('Escape')
  await page.getByRole('button', { name: 'Add project' }).first().click()
  expect(await frame('Add project')).toEqual({ w: 640, top: 90 })
  await page.keyboard.press('Escape')
  await page.getByRole('navigation', { name: 'Projects' }).getByRole('button', { name: 'Settings', exact: true }).click()
  expect(await frame('Settings')).toEqual({ w: 640, top: 90 })
})

test('terminal: 16px gutter, a quiet hint bar, and the font follows the text size', async ({ page }) => {
  const n = names()
  const repo = makeRepo(n.project, { [n.file]: 'hello\n' })
  await page.setViewportSize({ width: 1440, height: 900 })
  await signIn(page)
  await openProject(page, repo)
  await page.getByRole('button', { name: 'New tab' }).click()
  await page.getByRole('menuitem', { name: 'Terminal' }).click()
  const stage = page.locator('.xterm').first()
  await expect(stage).toBeVisible()
  await expect(stage.locator('xpath=..')).toHaveCSS('padding-left', '16px')
  const send = page.getByRole('button', { name: 'Send selection to chat' })
  await expect(send).toHaveCSS('border-top-color', 'rgba(0, 0, 0, 0)')
  const rowHeight = () => page.locator('.xterm-rows > div, .xterm-screen').first().evaluate((el) => el.getBoundingClientRect().height)
  const small = await rowHeight()
  const dialog = await openSettings(page, 'Appearance')
  await dialog.getByRole('radio', { name: 'Large' }).check()
  await expect.poll(rowHeight).not.toBe(small)
  await dialog.getByRole('radio', { name: 'Default' }).check()
})

test('review fixes: radios use the accent, tab close by keyboard, menu keys, short-screen dialog footer, diff header', async ({ page }) => {
  const n = names()
  const repo = makeRepo(n.project, { [n.file]: 'hello\n' })
  await page.setViewportSize({ width: 1440, height: 900 })
  await signIn(page)
  await openProject(page, repo)

  // Radios: accent-color comes from the token, not the browser blue.
  const dialog = await openSettings(page, 'Appearance')
  const accent = await dialog.getByRole('radio', { name: 'Light' }).evaluate((el) => getComputedStyle(el).accentColor)
  expect(accent).not.toBe('auto')
  await page.keyboard.press('Escape')

  // The + menu: ArrowDown on the button opens it on the first item, End goes to the last.
  await page.getByRole('button', { name: 'New tab' }).focus()
  await page.keyboard.press('ArrowDown')
  await expect(page.getByRole('menuitem', { name: 'Terminal' })).toBeFocused()
  await page.keyboard.press('End')
  await expect(page.getByRole('menuitem', { name: 'Open file' })).toBeFocused()
  await page.keyboard.press('Home')
  await page.keyboard.press('Enter')

  // Closing a tab by keyboard: Delete on the tab, or Tab to its close button.
  await expect(tabsOf(page)).toHaveCount(1)
  // The terminal mounts late and takes focus; wait for that, then move focus back to the tab.
  await expect(page.locator('.xterm-helper-textarea')).toBeFocused()
  await tabsOf(page).first().focus()
  await expect(tabsOf(page).first()).toBeFocused()
  await page.keyboard.press('Delete')
  await expect(tabsOf(page)).toHaveCount(0)

  // A short landscape screen keeps the footer in view.
  await page.setViewportSize({ width: 1000, height: 390 })
  const short = await openSettings(page, 'Appearance')
  await expect(short.getByRole('button', { name: 'Sign out' })).toBeInViewport({ ratio: 1 })
})

test('diff beside the list: the first code line starts under the header, two rows at most', async ({ page }) => {
  const n = names()
  const repo = makeRepo(n.project, { [n.file]: 'hello\n' })
  writeFileSync(join(repo, n.file), 'hello\nchanged\n')
  await page.setViewportSize({ width: 1440, height: 900 })
  await signIn(page)
  await openProject(page, repo)
  await page.keyboard.press('Alt+G')
  await page.locator('section[aria-label^="Changes in"]').getByRole('button', { name: `${n.file}, modified. Show changes` }).first().click()
  const row = page.getByRole('tabpanel').getByRole('checkbox', { name: 'Added: changed' })
  await expect(row).toBeVisible()
  const top = (await page.getByRole('tabpanel').locator('.right').boundingBox())!.y
  expect((await row.boundingBox())!.y - top).toBeLessThan(125)
})
