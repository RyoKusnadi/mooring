import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { collectProblems, fixtures, signIn } from './helpers'

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

function setup(name: string) {
  const folder = join(fixtures, name)
  mkdirSync(join(folder, 'src'), { recursive: true })
  writeFileSync(join(folder, 'notes.txt'), 'first line\nsecond line\n')
  writeFileSync(join(folder, 'src', 'main.go'), 'package main\n\nfunc main() {}\n')
  writeFileSync(join(folder, 'src', 'needle.txt'), 'one\ntwo\nfind-' + name + ' here\nfour\n')
  return folder
}

test('files: open, edit, save, external change, conflict, search, quick open', async ({ page }) => {
  test.setTimeout(90_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const name = `files-${run}`
  const folder = setup(name)
  await signIn(page)
  const problems = collectProblems(page)
  await addProject(page, folder)

  // Open from the tree; a second click focuses the same tab.
  await tree(page).getByRole('button', { name: 'notes.txt', exact: true }).click()
  await expect(editor(page)).toContainText('first line')
  await tree(page).getByRole('button', { name: 'notes.txt', exact: true }).click()
  await expect(page.getByRole('tab', { name: /notes\.txt/ })).toHaveCount(1)

  // Edit and save with the keyboard.
  await editor(page).click()
  await page.keyboard.press('ControlOrMeta+End')
  await page.keyboard.type('added by test')
  await expect(page.getByRole('tab', { name: /notes\.txt/ }).getByLabel(/unsaved/i)).toBeVisible()
  await page.keyboard.press('ControlOrMeta+s')
  await expect.poll(() => readFileSync(join(folder, 'notes.txt'), 'utf8')).toContain('added by test')
  await expect(page.getByRole('tab', { name: /notes\.txt/ }).getByLabel(/unsaved/i)).toHaveCount(0)

  // A change made outside shows up when there are no unsaved edits.
  writeFileSync(join(folder, 'notes.txt'), 'rewritten outside\n')
  await expect(editor(page)).toContainText('rewritten outside', { timeout: 10_000 })

  // A save over a newer file asks which version to keep.
  await editor(page).click()
  await page.keyboard.type('mine ')
  writeFileSync(join(folder, 'notes.txt'), 'theirs again\n')
  await expect(page.getByRole('button', { name: 'Overwrite the file on disk with my version' })).toBeVisible({ timeout: 10_000 })
  await page.getByRole('button', { name: 'Overwrite the file on disk with my version' }).click()
  await expect.poll(() => readFileSync(join(folder, 'notes.txt'), 'utf8')).toContain('mine ')

  // Taking the disk version keeps the keyboard in the editor.
  await editor(page).click()
  await page.keyboard.type('dropped ')
  writeFileSync(join(folder, 'notes.txt'), 'from disk\n')
  await page.getByRole('button', { name: 'Load the file from disk' }).click({ timeout: 10_000 })
  await expect(editor(page)).toContainText('from disk')
  await expect(page.locator('.cm-content')).toBeFocused()

  // Search across the project and open the line.
  await page.keyboard.press('Alt+F')
  const find = page.getByRole('dialog', { name: 'Find in project' })
  await find.getByLabel('Text to find').fill(`find-${name}`)
  const hit = find.getByRole('option', { name: /needle\.txt:3/ })
  await expect(hit).toBeVisible()
  await page.keyboard.press('Enter')
  await expect(page.getByRole('tab', { name: /needle\.txt/ })).toBeVisible()
  await expect(editor(page)).toContainText(`find-${name} here`)
  await expect(find).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(find).toHaveCount(0)

  // Quick open by file name.
  await page.keyboard.press('Alt+O')
  await find.getByLabel('File name').fill('main.go')
  await expect(find.getByRole('option').first()).toContainText('main.go')
  await page.keyboard.press('Enter')
  await expect(editor(page)).toContainText('package main')
  expect(problems).toEqual([])
})

// The editor keeps its state across tab switches; edits made after coming back must still save.
test('files: edits after switching tabs away and back still save', async ({ page }) => {
  const name = `switch-${Date.now()}-${test.info().repeatEachIndex}`
  const folder = setup(name)
  await signIn(page)
  await addProject(page, folder)
  // A double click keeps the tab; a single click would be a preview that main.go replaces.
  await tree(page).getByRole('button', { name: 'notes.txt', exact: true }).dblclick()
  await expect(editor(page)).toContainText('first line')
  await tree(page).getByRole('button', { name: 'src', exact: true }).click()
  await tree(page).getByRole('button', { name: 'main.go', exact: true }).click()
  await expect(editor(page)).toContainText('package main')
  await page.getByRole('tab', { name: /notes\.txt/ }).click()
  await expect(editor(page)).toContainText('first line')

  await editor(page).click()
  await page.keyboard.press('ControlOrMeta+End')
  await page.keyboard.type('after switching')
  await expect(page.getByRole('tab', { name: /notes\.txt/ }).getByLabel(/unsaved/i)).toBeVisible()
  await page.keyboard.press('ControlOrMeta+s')
  await expect.poll(() => readFileSync(join(folder, 'notes.txt'), 'utf8')).toContain('after switching')
  await expect(page.getByRole('tab', { name: /notes\.txt/ }).getByLabel(/unsaved/i)).toHaveCount(0)
})

test('files: a 5 MB file opens and typing works', async ({ page }) => {
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const folder = join(fixtures, `big-${run}`)
  mkdirSync(folder, { recursive: true })
  const line = 'const value = 1234567890 // padding to make the line a useful length\n'
  writeFileSync(join(folder, 'big.js'), line.repeat(Math.ceil((5 * 1024 * 1024) / line.length)))
  await signIn(page)
  await addProject(page, folder)
  const t0 = Date.now()
  await tree(page).getByRole('button', { name: 'big.js', exact: true }).click()
  await expect(editor(page)).toContainText('const value')
  await expect(page.locator('[data-editor-view] .state')).not.toContainText('Loading', { timeout: 15_000 })
  await editor(page).click()
  await page.keyboard.type('x')
  await expect(page.locator('[data-editor-view] .state')).toContainText('Unsaved changes')
  expect(Date.now() - t0).toBeLessThan(10_000)
})

test('files on a phone screen: the page does not scroll sideways', async ({ page, browser }) => {
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const name = `phonefiles-${run}`
  const folder = setup(name)
  writeFileSync(join(folder, 'wide.txt'), 'x'.repeat(400) + '\n')
  await signIn(page)
  await addProject(page, folder)
  await page.waitForTimeout(1000) // the layout is saved to the server a moment after an edit
  const phone = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true, storageState: await page.context().storageState() })
  const p = await phone.newPage()
  await p.goto('/')
  const wide = () => p.evaluate(() => document.documentElement.scrollWidth - innerWidth)
  await p.getByRole('button', { name: 'Projects', exact: true }).click()
  await p.getByRole('button', { name, exact: true }).click()
  if (!(await tree(p).isVisible())) await p.getByRole('button', { name: 'Projects', exact: true }).click()
  await tree(p).getByRole('button', { name: 'wide.txt', exact: true }).click()
  await expect(editor(p)).toContainText('xxxx')
  expect(await wide()).toBeLessThanOrEqual(0)
  await phone.close()
})

test('files: no request loop, prompts stay, tree keys, plain errors', async ({ page }) => {
  test.setTimeout(60_000)
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const name = `prompts-${run}`
  const folder = setup(name)
  let lists = 0
  page.on('request', (r) => r.url().includes('/api/files/list') && lists++)
  await signIn(page)
  await addProject(page, folder)
  await expect(tree(page).getByRole('treeitem', { name: /notes\.txt/ })).toBeVisible()

  // Idle: the tree loaded once, and nothing keeps asking.
  await page.waitForTimeout(500)
  const before = lists
  await page.waitForTimeout(2000)
  expect(lists - before).toBeLessThanOrEqual(3)
  expect(lists).toBeLessThan(15)

  // Rename stays open past the old reset, with the name selected up to the extension.
  const row = tree(page).getByRole('button', { name: 'notes.txt', exact: true })
  await row.focus()
  await page.keyboard.press('F2')
  const input = tree(page).getByLabel('New name for notes.txt')
  await expect(input).toBeFocused()
  await page.waitForTimeout(700)
  await expect(input).toBeVisible()
  await page.keyboard.type('renamed')
  await expect(input).toHaveValue('renamed.txt')
  await page.keyboard.press('Enter')
  await expect(tree(page).getByRole('button', { name: 'renamed.txt', exact: true })).toBeVisible()
  expect(readFileSync(join(folder, 'renamed.txt'), 'utf8')).toContain('first line')

  // New file prompt stays and creates.
  await tree(page).getByRole('button', { name: 'New file' }).click()
  const made = tree(page).getByLabel('Name of the new file')
  await page.waitForTimeout(700)
  await expect(made).toBeVisible()
  await made.fill('fresh.txt')
  await page.keyboard.press('Enter')
  await expect(tree(page).getByRole('button', { name: 'fresh.txt', exact: true })).toBeVisible()

  // Delete confirm stays, starts on the action for a trash move, and works.
  await tree(page).getByRole('button', { name: 'renamed.txt', exact: true }).focus()
  await page.keyboard.press('Delete')
  const ask = tree(page).getByRole('group', { name: 'Delete renamed.txt' })
  await page.waitForTimeout(700)
  await expect(ask).toBeVisible()
  await expect(ask.getByRole('button')).toHaveCount(2)
  await ask.getByRole('button', { name: 'Keep' }).click()
  await expect(ask).toBeHidden()
  await tree(page).getByRole('button', { name: 'renamed.txt', exact: true }).focus()
  await page.keyboard.press('Delete')
  await expect(ask).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(ask).toBeHidden()
  await expect(tree(page).getByRole('button', { name: 'renamed.txt', exact: true })).toBeVisible()

  // One tab stop, arrow keys move between rows.
  await expect(tree(page).locator('button.main[tabindex="0"]')).toHaveCount(1)
  await tree(page).getByRole('button', { name: 'src', exact: true }).focus()
  await page.keyboard.press('ArrowRight')
  await expect(tree(page).getByRole('button', { name: 'main.go', exact: true })).toBeVisible()
  await page.keyboard.press('ArrowDown')
  await expect(tree(page).getByRole('button', { name: 'main.go', exact: true })).toBeFocused()
  await page.keyboard.press('ArrowLeft')
  await expect(tree(page).getByRole('button', { name: 'src', exact: true })).toBeFocused()

  // A failed file list says so and can be tried again.
  await page.route('**/api/files/names*', (r) => r.abort())
  await page.keyboard.press('Alt+O')
  const open = page.getByRole('dialog', { name: 'Find in project' })
  await expect(open.getByRole('alert')).toContainText('Could not list the files')
  await page.unroute('**/api/files/names*')
  await open.getByRole('button', { name: 'Try again' }).click()
  await open.getByLabel('File name').fill('main.go')
  await expect(open.getByRole('option').first()).toContainText('main.go')
  await page.keyboard.press('Escape')
  // The close event comes a moment after the dialog hides; shortcuts wait for it.
  await expect(page.locator('dialog')).toHaveCount(0)

  // A bad regular expression says so in plain words.
  await page.keyboard.press('Alt+F')
  const find = page.getByRole('dialog', { name: 'Find in project' })
  await find.getByLabel('Regex').check()
  await find.getByLabel('Text to find').fill('(')
  await expect(find.getByRole('alert')).toContainText('not a valid regular expression')
})
