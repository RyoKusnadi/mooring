import { mkdirSync, writeFileSync } from 'node:fs'
import { basename, join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { collectProblems, fixtures, signIn } from './helpers'

// Phase 20 follow-ups for the language server: Hide on the stopped and crashed bars, Format while
// the server starts, symbols in the breadcrumbs. The stand-in (internal/fakelsp) crashes on CRASH in
// a file, answers initialize late when the folder holds .fakelsp-slow-init, and lists symbols from
// lines that start with func, type or class. lsp.spec.ts owns the first consent of Go (gopls); a
// run of this file alone grants it here.

const unique = () => `lspbar-${Date.now() % 1_000_000}-${test.info().workerIndex}${test.info().repeatEachIndex}`
const tree = (page: Page) => page.getByRole('region', { name: /^Files in/ })
const editor = (page: Page) => page.locator('[data-editor-view] .cm-content')
const format = (page: Page) => page.getByRole('button', { name: 'Format', exact: true })

function makeFolder(files: Record<string, string>): string {
  const folder = join(fixtures, unique())
  mkdirSync(folder, { recursive: true })
  for (const [name, body] of Object.entries(files)) writeFileSync(join(folder, name), body)
  return folder
}

async function openFile(page: Page, folder: string, file: string) {
  if (page.url() === 'about:blank') await signIn(page)
  await page.getByRole('button', { name: 'Add project' }).first().click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Folder on this computer').fill(folder)
  await dialog.getByRole('button', { name: 'Go', exact: true }).click()
  await dialog.getByRole('button', { name: 'Open this folder' }).click()
  await expect(dialog).toBeHidden()
  await tree(page).getByRole('button', { name: file, exact: true }).dblclick()
  await expect(editor(page)).toContainText('package main')
  await page.getByRole('button', { name: 'Allow and start' }).click({ timeout: 4000 }).catch(() => {})
}

async function stopFromDialog(page: Page, folder: string) {
  const dialog = page.getByRole('dialog', { name: 'Language servers' })
  await expect(async () => {
    await page.keyboard.press('Escape')
    await page.keyboard.press('ControlOrMeta+k')
    await page.keyboard.type('Language servers')
    await page.keyboard.press('Enter')
    await expect(dialog).toBeVisible({ timeout: 2000 })
  }).toPass({ timeout: 15_000 })
  await dialog.getByRole('button', { name: `Stop Go (gopls) in ${basename(folder)}`, exact: true }).click()
  await page.keyboard.press('Escape')
  await expect(dialog).toBeHidden()
}

test('the crashed and the stopped bar can be hidden', async ({ page }) => {
  test.setTimeout(180_000)
  const crashing = makeFolder({ 'crash.go': 'package main\n' })
  const problems = collectProblems(page)
  await openFile(page, crashing, 'crash.go')
  await expect(format(page)).toBeVisible({ timeout: 30_000 })
  // The text stays in the editor, so every restart crashes again until the connection gives up.
  await editor(page).click()
  await page.keyboard.press('ControlOrMeta+End')
  await page.keyboard.type('\nCRASH')
  const crashed = page.getByRole('alert').filter({ hasText: 'Go (gopls) stopped.' })
  await expect(crashed).toContainText('It crashed three times in two minutes.', { timeout: 120_000 })
  await expect(crashed.getByRole('button', { name: 'Start again' })).toBeVisible()
  await crashed.getByRole('button', { name: 'Hide' }).click()
  await expect(crashed).toHaveCount(0)
  await page.getByRole('button', { name: 'Close crash.go' }).click({ force: true })
  await page.getByRole('button', { name: 'Discard changes' }).click()

  const stopping = makeFolder({ 'stop.go': 'package main\n' })
  await openFile(page, stopping, 'stop.go')
  await expect(format(page)).toBeVisible({ timeout: 30_000 })
  await stopFromDialog(page, stopping)
  const stopped = page.getByRole('status').filter({ hasText: 'Go (gopls) was stopped.' })
  await expect(stopped).toBeVisible()
  await stopped.getByRole('button', { name: 'Hide' }).click()
  await expect(stopped).toHaveCount(0)
  expect(problems.filter((p) => !/WebSocket|favicon|net::ERR|Failed to load resource/.test(p))).toEqual([])
})

test('Format is disabled while the server starts, and says why from the key and the palette', async ({ page }) => {
  test.setTimeout(120_000)
  const folder = makeFolder({ 'slow.go': 'package main   \n', '.fakelsp-slow-init': '8s' })
  await openFile(page, folder, 'slow.go')
  await expect(format(page)).toBeDisabled()
  await expect(format(page)).toHaveAttribute('title', 'The language server is still starting.')
  await expect(page.getByText('The language server is starting…')).toBeVisible()
  const wait = 'The language server is still starting. Try again in a moment.'
  await editor(page).click()
  await page.keyboard.press('Alt+Shift+F')
  await expect(page.getByText(wait)).toBeVisible()
  await expect(format(page)).toBeDisabled()
  await page.keyboard.press('ControlOrMeta+k')
  await page.keyboard.type('Format document')
  await page.keyboard.press('Enter')
  await expect(page.getByText(wait)).toBeVisible()
  // Once the server answers, the button works and has no title.
  await expect(format(page)).toBeEnabled({ timeout: 30_000 })
  await expect(format(page)).not.toHaveAttribute('title', /starting/)
})

test('breadcrumbs show the symbols at the cursor, a click moves the cursor, the file name stays whole on a phone', async ({ page }) => {
  test.setTimeout(120_000)
  const body = 'package main\n\ntype Server struct {\n\tfunc handleLogin() {\n\t\tx := 1\n\t}\n}\n'
  const folder = makeFolder({ 'server.go': body })
  await openFile(page, folder, 'server.go')
  const symbols = page.getByRole('list', { name: 'Symbols at the cursor' })
  await editor(page).getByText('x := 1').click()
  await expect(symbols.getByRole('button')).toHaveText(['Server', 'handleLogin'], { timeout: 30_000 })
  await expect(page.getByRole('navigation', { name: /^Path of/ }).locator('.file')).toHaveText('server.go')
  // The symbol moves the cursor to its name, which is outside handleLogin.
  await symbols.getByRole('button', { name: 'Server', exact: true }).click()
  await expect(symbols.getByRole('button')).toHaveText(['Server'])
  await expect(page.locator('[data-editor-view] .cm-activeLine')).toContainText('type Server struct')
  await editor(page).getByText('x := 1').click()
  await expect(symbols.getByRole('button')).toHaveCount(2)

  await page.setViewportSize({ width: 390, height: 844 })
  const file = page.getByRole('navigation', { name: /^Path of/ }).locator('.file')
  await expect(file).toBeVisible()
  expect(await file.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true)
  const box = (await file.boundingBox())!
  expect(box.x + box.width).toBeLessThanOrEqual(390)
})
