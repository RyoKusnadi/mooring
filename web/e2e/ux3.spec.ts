import { writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { makeRepo, openProject } from './githelpers'
import { collectProblems, openSettings, signIn } from './helpers'

// Phase 17: the shell. Sidebar width, footer, Settings, tabs, the + menu, palette order, appearance.

const names = () => {
  const run = `${Date.now() % 100_000}${test.info().workerIndex}${test.info().repeatEachIndex}`
  return { project: `ux3-fixture-project-${run}`, file: 'notes-for-the-release.md' }
}

const tabsOf = (page: Page) => page.getByRole('tab')

/** Every name in the sidebar that is cut with an ellipsis. */
const cutNames = (page: Page) =>
  page.getByRole('navigation', { name: 'Projects' }).evaluate((nav) => {
    const cut: string[] = []
    for (const el of nav.querySelectorAll<HTMLElement>('*')) {
      if (el.children.length === 0 && el.textContent && el.scrollWidth > el.clientWidth + 1 && getComputedStyle(el).textOverflow === 'ellipsis') cut.push(el.textContent)
    }
    return cut
  })

test('at 1440, 1024 and 390 px every feature is reachable from the gear or the palette, and no name is cut', async ({ page }) => {
  const n = names()
  const repo = makeRepo(n.project, { [n.file]: 'hello\n', 'main.go': 'package main\n' })
  await page.setViewportSize({ width: 1440, height: 900 })
  await signIn(page)
  const problems = collectProblems(page)
  await openProject(page, repo)
  const nav = page.getByRole('navigation', { name: 'Projects' })

  for (const [width, height] of [
    [1440, 900],
    [1024, 768],
    [390, 844],
  ]) {
    await page.setViewportSize({ width, height })
    const phone = width <= 900
    if (phone) await page.getByRole('button', { name: 'Projects', exact: true }).click()

    // No project or file name is cut at the default sidebar width.
    await expect(nav.getByRole('button', { name: n.project, exact: true })).toBeVisible()
    await expect(nav.getByRole('button', { name: n.file, exact: true })).toBeVisible()
    expect((await cutNames(page)).filter((t) => t === n.project || t === n.file)).toEqual([])
    // The gear: every section of Settings is there.
    await nav.getByRole('button', { name: 'Settings', exact: true }).click()
    const dialog = page.getByRole('dialog', { name: 'Settings' })
    for (const section of ['Accounts', 'Tool servers', 'SSH hosts', 'Language servers', 'Token saving', 'Appearance']) {
      await expect(dialog.getByRole('button', { name: section, exact: true })).toBeVisible()
    }
    await expect(dialog.getByRole('button', { name: 'Sign out' })).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(dialog).toBeHidden()

    if (phone) await page.keyboard.press('Escape')

    // The palette: the everyday commands, first.
    if (phone) await page.getByRole('banner').getByRole('button', { name: /^Commands/ }).click()
    else await page.keyboard.press('ControlOrMeta+k')
    const palette = page.getByRole('dialog', { name: 'Command palette' })
    const options = palette.getByRole('option')
    await expect(options.first()).toHaveText(/New terminal/)
    const first = (await options.allInnerTexts()).slice(0, 10).join('|')
    for (const label of ['New terminal', 'New chat', 'Open file by name', 'Search text in project', 'Show changes', 'Split right', 'Switch branch', 'Settings', 'Sign out']) {
      expect(first, label).toContain(label)
    }
    const all = (await options.allInnerTexts()).join('|')
    for (const label of ['Tool servers', 'Accounts and usage', 'SSH hosts', 'Language servers', 'Show history']) expect(all, label).toContain(label)
    await page.keyboard.press('Escape')
    await expect(palette).toBeHidden()
  }
  expect(problems).toEqual([])
})

test('the sidebar resizes by keyboard, stays within 220 and 480, and is remembered', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await signIn(page)
  const handle = page.getByRole('separator', { name: 'Sidebar width' })
  await expect(handle).toHaveAttribute('aria-valuenow', '264')
  await handle.focus()
  await page.keyboard.press('ArrowRight')
  await expect(handle).toHaveAttribute('aria-valuenow', '280')
  const box = await page.locator('aside#sidebar').boundingBox()
  expect(Math.round(box?.width ?? 0)).toBe(280)
  for (let i = 0; i < 20; i++) await page.keyboard.press('Shift+ArrowRight')
  await expect(handle).toHaveAttribute('aria-valuenow', '480')
  for (let i = 0; i < 20; i++) await page.keyboard.press('Shift+ArrowLeft')
  await expect(handle).toHaveAttribute('aria-valuenow', '220')
  await page.keyboard.press('ArrowRight')
  await page.reload()
  await expect(page.getByRole('separator', { name: 'Sidebar width' })).toHaveAttribute('aria-valuenow', '236')
  await page.getByRole('separator', { name: 'Sidebar width' }).focus()
  await page.keyboard.press('Home')
  await expect(page.getByRole('separator', { name: 'Sidebar width' })).toHaveAttribute('aria-valuenow', '264')
})

test('the + menu starts a terminal, a chat or the file finder, and Split opens a terminal', async ({ page }) => {
  const n = names()
  const repo = makeRepo(n.project, { [n.file]: 'hello\n' })
  await page.setViewportSize({ width: 1440, height: 900 })
  await signIn(page)
  await openProject(page, repo)
  await expect(tabsOf(page)).toHaveCount(0)
  await expect(page.getByRole('tabpanel').getByText(/Start a terminal or a chat/)).toBeVisible()

  await page.getByRole('button', { name: 'New tab' }).click()
  for (const name of ['Terminal', 'Chat', 'Open file']) await expect(page.getByRole('menuitem', { name })).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('menu')).toHaveCount(0)

  await page.getByRole('button', { name: 'New tab' }).click()
  await page.getByRole('menuitem', { name: 'Terminal' }).click()
  const term = page.locator('[data-tab][data-kind="terminal"]')
  await expect(term).toHaveCount(1)
  // A kind icon on the tab; the name is never the project's.
  await expect(term.locator('svg').first()).toBeVisible()
  await expect(term.getByRole('tab')).not.toContainText(n.project)

  await page.getByRole('button', { name: 'New tab' }).click()
  await page.getByRole('menuitem', { name: 'Open file' }).click()
  await expect(page.getByRole('dialog', { name: 'Find in project' })).toBeVisible()
  await page.keyboard.press('Escape')

  await page.getByRole('button', { name: 'Split right' }).click()
  await expect(page.getByRole('tablist', { name: 'Tabs' })).toHaveCount(2)
  await expect(page.locator('[data-tab][data-kind="terminal"]')).toHaveCount(2)
  // Pane buttons show in the focused pane only.
  await expect(page.getByRole('button', { name: 'New tab' })).toHaveCount(1)
  await expect(page.locator('[data-tab][data-kind="blank"]')).toHaveCount(0)
})

test('tabs: the close button shows on hover and the active tab has an accent underline', async ({ page }) => {
  const n = names()
  const repo = makeRepo(n.project, { [n.file]: 'hello\n' })
  await page.setViewportSize({ width: 1440, height: 900 })
  await signIn(page)
  await openProject(page, repo)
  await page.getByRole('navigation', { name: 'Projects' }).getByRole('button', { name: n.file, exact: true }).click()
  await page.getByRole('button', { name: 'New tab' }).click()
  await page.getByRole('menuitem', { name: 'Terminal' }).click()
  const file = page.locator('[data-tab][data-kind="file"]')
  const term = page.locator('[data-tab][data-kind="terminal"]')
  await expect(file.locator('svg').first()).toBeVisible()
  const close = file.getByRole('button', { name: /^Close / })
  await expect(close).toHaveCSS('opacity', '0')
  await file.hover()
  await expect(close).toHaveCSS('opacity', '1')
  const accent = await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--accent').trim())
  expect(accent).toMatch(/^#/)
  await expect(term).toHaveCSS('border-bottom-width', '2px')
  await expect(file).toHaveCSS('border-bottom-color', 'rgba(0, 0, 0, 0)')
})

test('the palette lists files for text that looks like a file name', async ({ page }) => {
  const n = names()
  const repo = makeRepo(n.project, { [n.file]: 'hello\n' })
  await page.setViewportSize({ width: 1440, height: 900 })
  await signIn(page)
  await openProject(page, repo)
  await page.keyboard.press('ControlOrMeta+k')
  await page.getByRole('combobox', { name: 'Type a command' }).fill(n.file)
  await expect(page.getByRole('option').first()).toHaveText(`List files named ${n.file}`)
  await page.keyboard.press('Enter')
  const finder = page.getByRole('dialog', { name: 'Find in project' })
  await expect(finder.locator('input').first()).toHaveValue(n.file)
})

test('Appearance: theme and text size apply at once, are kept per browser, and are set before first paint', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await signIn(page)
  const html = page.locator('html')
  const bg = () => page.evaluate(() => getComputedStyle(document.body).backgroundColor)
  await page.emulateMedia({ colorScheme: 'light' })
  await expect(html).toHaveAttribute('data-theme', 'light')

  const dialog = await openSettings(page, 'Appearance')
  await dialog.getByRole('radio', { name: 'Dark' }).check()
  await expect(html).toHaveAttribute('data-theme', 'dark')
  expect(await bg()).toBe('rgb(15, 20, 19)')
  await dialog.getByRole('radio', { name: 'Large' }).check()
  await expect(html).toHaveAttribute('data-size', 'large')
  expect(await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--fs-m').trim())).toBe('15px')

  // theme.js sets both before the app loads: look at the page while the app script is held back.
  await page.route('**/assets/*.js', (r) => r.abort())
  await page.reload()
  await expect(html).toHaveAttribute('data-theme', 'dark')
  await expect(html).toHaveAttribute('data-size', 'large')
  expect(await bg()).toBe('rgb(15, 20, 19)')
  await page.unroute('**/assets/*.js')

  await page.reload()
  await openSettingsAgain(page)
  async function openSettingsAgain(p: Page) {
    const d = await openSettings(p, 'Appearance')
    await expect(d.getByRole('radio', { name: 'Dark' })).toBeChecked()
    await d.getByRole('radio', { name: 'Same as the system' }).check()
    await d.getByRole('radio', { name: 'Default' }).check()
  }
  await expect(html).toHaveAttribute('data-theme', 'light')

  // The quick switch in the sidebar footer flips what is drawn, and its name says what it will do next.
  await page.keyboard.press('Escape')
  const nav = page.getByRole('navigation', { name: 'Projects' })
  await nav.getByRole('button', { name: 'Switch to dark theme' }).click()
  await expect(html).toHaveAttribute('data-theme', 'dark')
  await page.reload()
  await expect(html).toHaveAttribute('data-theme', 'dark')
  await nav.getByRole('button', { name: 'Switch to light theme' }).click()
  await expect(html).toHaveAttribute('data-theme', 'light')
})

test('the phone layout starts at 900 px', async ({ page }) => {
  await signIn(page)
  const header = page.getByRole('banner')
  await page.setViewportSize({ width: 900, height: 700 })
  await expect(header.getByRole('button', { name: 'Projects', exact: true })).toBeVisible()
  await page.setViewportSize({ width: 901, height: 700 })
  await expect(header).toHaveCount(0)
})
