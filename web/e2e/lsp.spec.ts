import { chmodSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { basename, join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { collectProblems, fixtures, signIn } from './helpers'

// The stand-in servers are on the PATH as gopls and typescript-language-server (see serve.sh), and
// run with a 3 s idle time and a 150 MB limit. Install is never clicked: nothing here downloads.

async function addProject(page: Page, folder: string) {
  await page.getByRole('button', { name: 'Add project' }).first().click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Folder on this computer').fill(folder)
  await dialog.getByRole('button', { name: 'Go', exact: true }).click()
  await dialog.getByRole('button', { name: 'Open this folder' }).click()
  await expect(dialog).toBeHidden()
}

const tree = (page: Page) => page.getByRole('region', { name: /^Files in/ })
const editor = (page: Page) => page.locator('[data-editor-view] .cm-content')
const bar = (page: Page, name: string) => page.getByRole('group', { name: `${name} language server` })
const errors = (page: Page) => page.locator('.cm-lintRange-error')
const completions = (page: Page) => page.locator('.cm-tooltip-autocomplete')

async function typeAtEnd(page: Page, text: string) {
  await editor(page).click()
  await page.keyboard.press('ControlOrMeta+End')
  await page.keyboard.type(text)
}

/** Types a word the stand-in completes, retrying while the server is still starting. */
async function expectCompletion(page: Page) {
  await typeAtEnd(page, '\nfake')
  await expect(async () => {
    await page.keyboard.press('Backspace')
    await page.keyboard.type('e')
    await expect(completions(page)).toContainText('fakeCompletion', { timeout: 2000 })
  }).toPass({ timeout: 30_000, intervals: [1000] })
  await page.keyboard.press('Escape')
}

/** Erases the last line. Focus is taken again first: a server restart can leave it outside the editor. */
async function eraseLine(page: Page) {
  await page.keyboard.press('Escape')
  await editor(page).click()
  await page.keyboard.press('ControlOrMeta+End')
  await page.keyboard.press('Shift+Home')
  await page.keyboard.press('Backspace')
  await page.keyboard.press('Backspace')
}

async function openServersDialog(page: Page) {
  const dialog = page.getByRole('dialog', { name: 'Language servers' })
  // The palette can miss a key press right after another dialog closes, so retry the whole sequence.
  await expect(async () => {
    await page.keyboard.press('Escape')
    await page.keyboard.press('ControlOrMeta+k')
    await page.keyboard.type('Language servers')
    await page.keyboard.press('Enter')
    await expect(dialog).toBeVisible({ timeout: 2000 })
  }).toPass({ timeout: 15_000 })
  return dialog
}

/** Checks the page, editor and open dialog; `dialogOnly` skips the page and editor. */
async function noSidewaysScroll(page: Page, dialogOnly = false) {
  const wide = await page.evaluate((dialogOnly) => {
    const name = (e: Element) => `${e.tagName}.${String(e.className)} right=${Math.round(e.getBoundingClientRect().right)}`
    const scrolls = [...(dialogOnly ? [] : [document.documentElement, document.querySelector('[data-editor-view]')]), document.querySelector('dialog[open]')]
    const over = scrolls.filter((e): e is Element => !!e && e.scrollWidth > e.clientWidth + 1).map(name)
    // Tab strips scroll on purpose; anything else past the screen edge is listed to find the cause.
    const past = [...document.querySelectorAll('body *')].filter((e) => e.getBoundingClientRect().right > innerWidth + 1 && !e.closest('[aria-label=Tabs]'))
    const chain: string[] = []
    for (let e = past[0]?.parentElement; e && e !== document.body; e = e.parentElement) chain.push(`${e.tagName}.${String(e.className)} w=${Math.round(e.getBoundingClientRect().width)}`)
    return over.length ? [...over, ...past.slice(0, 1).map(name), ...chain] : []
  }, dialogOnly)
  expect(wide).toEqual([])
}

test('language servers: consent, completion, errors, disk changes, idle stop, restart, typescript', async ({ page }) => {
  test.setTimeout(480_000)
  const folder = join(fixtures, `lsp-${Date.now()}-${test.info().repeatEachIndex}`)
  mkdirSync(folder, { recursive: true })
  writeFileSync(join(folder, 'main.go'), 'package main\n\nfunc main() {}\n')
  writeFileSync(join(folder, 'other.go'), 'package main\n')
  writeFileSync(join(folder, 'format.go'), 'package main\n\nfunc f() {}   \nvar x = 1\t\n')
  writeFileSync(join(folder, 'range.go'), 'package main // OUT_OF_RANGE   \n')
  writeFileSync(join(folder, 'ro.go'), 'package main   \n')
  chmodSync(join(folder, 'ro.go'), 0o444)
  writeFileSync(join(folder, 'app.ts'), 'export const answer = 42\n')
  await signIn(page)
  const problems = collectProblems(page)
  await addProject(page, folder)

  // The server is not allowed yet: the bar shows the command, and nothing runs until Allow.
  await tree(page).getByRole('button', { name: 'main.go', exact: true }).click()
  await expect(editor(page)).toContainText('package main')
  const goBar = bar(page, 'Go (gopls)')
  await expect(goBar).toContainText('Mooring wants to run')
  await expect(goBar).toContainText('gopls')
  await expect(goBar.getByRole('button', { name: /^Install/ })).toHaveCount(0)
  // The bar shows the short command, the full one on request, and is a row above the code that pushes it down.
  await expect(goBar.locator('.short')).toContainText('gopls')
  const firstLine = page.locator('[data-editor-view] .cm-line').first()
  const lineTop = (await firstLine.boundingBox())!.y
  await expect(goBar.locator('.exact code')).toBeHidden()
  await goBar.getByText('Show full command').click()
  await expect(goBar.locator('.exact code')).toBeVisible()
  expect(await goBar.evaluate((el) => getComputedStyle(el.parentElement!).position)).toBe('static')
  const barBottom = (await goBar.boundingBox())!.y + (await goBar.boundingBox())!.height
  expect((await firstLine.boundingBox())!.y).toBeGreaterThanOrEqual(barBottom)
  await expect(page.getByRole('button', { name: 'Format', exact: true })).toHaveCount(0)
  await goBar.getByRole('button', { name: 'Allow and start' }).click()
  await expect(goBar).toHaveCount(0)
  expect((await firstLine.boundingBox())!.y).toBeLessThan(lineTop)

  // Completion comes from the server: its own word plus the words of the open file.
  await expectCompletion(page)
  await expect(completions(page)).toHaveCount(0)
  await eraseLine(page)
  await expect(editor(page)).not.toContainText('fake')

  // An error appears while the line holds ERROR, with a mark and a gutter marker, and goes with it.
  await typeAtEnd(page, '\nERROR')
  await expect(errors(page)).toHaveCount(1, { timeout: 15_000 })
  await expect(page.locator('.cm-lint-marker-error')).toHaveCount(1)
  await errors(page).hover()
  await expect(page.locator('.cm-tooltip-lint')).toContainText('found ERROR')
  await eraseLine(page)
  await expect(errors(page)).toHaveCount(0, { timeout: 15_000 })

  // A change on disk to the open file shows fresh errors after the editor takes it.
  writeFileSync(join(folder, 'main.go'), 'package main\n\nfunc main() {}\n// ERROR from disk\n')
  await expect(editor(page)).toContainText('ERROR from disk')
  await expect(errors(page)).toHaveCount(1, { timeout: 15_000 })
  writeFileSync(join(folder, 'main.go'), 'package main\n\nfunc main() {}\n')
  await expect(errors(page)).toHaveCount(0, { timeout: 15_000 })

  // A file changed on disk that breaks other files marks the open one.
  writeFileSync(join(folder, 'other.go'), 'package main\n// BREAKS_OTHERS\n')
  await expect(errors(page).first()).toBeVisible({ timeout: 15_000 })
  // A later publish of the same errors can close an open tooltip on a busy machine: hover again until it shows.
  await expect(async () => {
    await page.mouse.move(0, 0)
    await errors(page).first().hover()
    await expect(page.locator('.cm-tooltip-lint')).toContainText('broken by other.go', { timeout: 2_000 })
  }).toPass({ timeout: 15_000 })
  writeFileSync(join(folder, 'other.go'), 'package main\n')
  await expect(errors(page)).toHaveCount(0, { timeout: 15_000 })

  // Over its memory limit the server is restarted, the notice says so, and completion works again.
  await page.mouse.move(0, 0)
  await typeAtEnd(page, '\nBLOAT')
  await expect(page.getByRole('contentinfo').getByText(/Go \(gopls\) used .* over its 150 MB limit, and was restarted\./)).toBeVisible({ timeout: 60_000 })
  // The bar may say the server stopped while it restarts; typing before it is back hits an editor without the client.
  await expect(page.getByText('was stopped.')).toHaveCount(0, { timeout: 30_000 })
  await eraseLine(page)
  await expect(editor(page)).not.toContainText('BLOAT')
  await expectCompletion(page)
  await eraseLine(page)

  // Format: the button shows once the server says it formats; one click is one edit, one undo brings it back.
  await tree(page).getByRole('button', { name: 'format.go', exact: true }).dblclick()
  await expect(editor(page)).toContainText('func f()')
  const format = page.getByRole('button', { name: 'Format', exact: true })
  await expect(format).toBeVisible({ timeout: 30_000 })
  const clean = 'package main\n\nfunc f() {}\nvar x = 1\n'
  const dirty = 'package main\n\nfunc f() {}   \nvar x = 1\t\n'
  const onDisk = () => readFileSync(join(folder, 'format.go'), 'utf8')
  // The server answers a moment after the click; the tab's mark shows the edit has landed.
  const unsaved = page.getByRole('img', { name: 'Unsaved changes' })
  await format.click()
  await expect(unsaved).toBeVisible()
  await page.keyboard.press('ControlOrMeta+s')
  await expect.poll(onDisk, { timeout: 10_000 }).toBe(clean)
  await expect(unsaved).toBeHidden()
  await editor(page).click()
  await page.keyboard.press('ControlOrMeta+z')
  await expect(unsaved).toBeVisible()
  await page.keyboard.press('ControlOrMeta+s')
  await expect.poll(onDisk, { timeout: 10_000 }).toBe(dirty)
  // Wait for the saved mark to clear, or the next check passes on the old mark and saves before Format lands.
  await expect(unsaved).toBeHidden()
  await page.keyboard.press('ControlOrMeta+k')
  await page.keyboard.type('Format document')
  await page.keyboard.press('Enter')
  await expect(unsaved).toBeVisible()
  await page.keyboard.press('ControlOrMeta+s')
  await expect.poll(onDisk, { timeout: 10_000 }).toBe(clean)
  // The file is on disk before the editor hears the save went through; closing earlier asks to discard.
  await expect(unsaved).toBeHidden()
  await page.getByRole('button', { name: 'Close format.go' }).click({ force: true })
  await expect(page.getByRole('tab', { name: /format\.go/ })).toHaveCount(0)

  // An answer with an edit outside the file is refused whole, with a note; the text stays as it was.
  await tree(page).getByRole('button', { name: 'range.go', exact: true }).dblclick()
  await expect(format).toBeVisible({ timeout: 30_000 })
  await format.click()
  await expect(page.getByText('The language server sent edits outside the file, so nothing was changed.')).toBeVisible()
  await expect(unsaved).toHaveCount(0)
  await expect(editor(page)).toContainText('OUT_OF_RANGE')

  // A read-only file is not formatted by the palette or the key either.
  await tree(page).getByRole('button', { name: 'ro.go', exact: true }).dblclick()
  await expect(page.getByText('Read-only')).toBeVisible()
  await expect(format).toBeDisabled()
  await page.keyboard.press('ControlOrMeta+k')
  await page.keyboard.type('Format document')
  await page.keyboard.press('Enter')
  await expect(page.getByText('This file is read-only, so it is not formatted.')).toBeVisible()
  await expect(unsaved).toHaveCount(0)
  await expect(editor(page)).toContainText('package main')
  // Their tabs go, or the server would not idle out below.
  for (const n of ['range.go', 'ro.go']) await page.getByRole('button', { name: `Close ${n}` }).click({ force: true })
  await expect(page.getByRole('tab', { name: /(range|ro)\.go/ })).toHaveCount(0)

  // The dialog lists the running server with a Stop button.
  const dialog = await openServersDialog(page)
  const running = dialog.getByTestId('lsp-running')
  await expect(running).toContainText('Go (gopls)')
  // Other tests' servers may be listed too, so look for this project's own.
  await expect(running.getByRole('button', { name: `Stop Go (gopls) in ${basename(folder)}`, exact: true })).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(dialog).toBeHidden()

  // One tab at a time from here: a second tab already crowds the pane buttons at 390 px (not an LSP matter).
  await page.getByRole('button', { name: 'Close main.go' }).click({ force: true })
  await expect(page.getByRole('tab', { name: /main\.go/ })).toHaveCount(0)

  // TypeScript: its own consent, then the same features. The bar also fits a phone screen.
  await tree(page).getByRole('button', { name: 'app.ts', exact: true }).click()
  await expect(editor(page)).toContainText('answer')
  const tsBar = bar(page, 'TypeScript and JavaScript')
  await expect(tsBar).toContainText('Mooring wants to run')
  await page.setViewportSize({ width: 390, height: 844 })
  await expect(page.getByRole('button', { name: 'Projects' })).toBeVisible()
  await expect(tsBar).toBeVisible()
  await noSidewaysScroll(page)
  await tsBar.getByRole('button', { name: 'Allow and start' }).click()
  await expect(tsBar).toHaveCount(0)
  await expectCompletion(page)
  const box = await completions(page).boundingBox({ timeout: 1000 }).catch(() => null)
  if (box) expect(box.x + box.width).toBeLessThanOrEqual(390)
  await eraseLine(page)
  await typeAtEnd(page, '\nERROR')
  await expect(errors(page)).toHaveCount(1, { timeout: 15_000 })
  await noSidewaysScroll(page)
  await page.setViewportSize({ width: 1280, height: 800 })

  // With every tab closed, the socket goes after 30 s and the stand-in after its 3 s idle time.
  for (const name of ['app.ts']) {
    // After the resize back from the phone width the close button can be hidden for a moment, so it is clicked in the page.
    const discard = page.getByRole('button', { name: 'Discard changes' })
    await expect(async () => {
      await page.getByRole('button', { name: `Close ${name}` }).evaluate((el: HTMLElement) => el.click())
      await expect(discard).toBeVisible({ timeout: 1500 })
    }).toPass({ timeout: 15_000 })
    await discard.click()
  }
  await expect(page.getByRole('tab', { name: /app\.ts/ })).toHaveCount(0)
  await expect(async () => {
    const d = await openServersDialog(page)
    await expect(d.getByRole('button', { name: new RegExp(`^Stop .* in ${basename(folder)}$`) })).toHaveCount(0, { timeout: 1000 })
    await page.keyboard.press('Escape')
  }).toPass({ timeout: 90_000, intervals: [5000] })

  // The dialog also fits a phone, with the add form in reach.
  await page.setViewportSize({ width: 390, height: 844 })
  const phone = await openServersDialog(page)
  await expect(phone.getByRole('button', { name: 'Add server' })).toBeVisible()
  await noSidewaysScroll(page, true)
  await page.keyboard.press('Escape')

  expect(problems.filter((p) => !/WebSocket|favicon|net::ERR|Failed to load resource/.test(p))).toEqual([])
})
