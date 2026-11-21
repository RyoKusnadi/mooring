import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
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
const row = (page: Page, name: string) => tree(page).getByRole('button', { name, exact: true })

async function menu(page: Page, name: string, item: string) {
  await row(page, name).hover()
  await tree(page).getByRole('button', { name: `Actions for ${name}` }).click()
  await page.getByRole('menu', { name: `Actions for ${name}` }).getByRole('menuitem', { name: item, exact: true }).click()
}

test('files: move by drag with undo, download, upload with replace, duplicate, desktop drop', async ({ page }) => {
  test.setTimeout(90_000)
  const folder = join(fixtures, `files2-${Date.now()}-${test.info().repeatEachIndex}`)
  mkdirSync(join(folder, 'src'), { recursive: true })
  mkdirSync(join(folder, 'docs'), { recursive: true })
  writeFileSync(join(folder, 'notes.txt'), 'first line\n')
  const bytes = Buffer.from(Array.from({ length: 4000 }, (_, i) => (i * 7) % 256))
  writeFileSync(join(folder, 'blob.bin'), bytes)

  await signIn(page)
  const problems = collectProblems(page)
  await addProject(page, folder)

  // Move by drag, then undo.
  // A drag that starts while the tree is still drawing can be dropped; drag again only while the file has not moved.
  await expect(async () => {
    if (existsSync(join(folder, 'notes.txt'))) await row(page, 'notes.txt').dragTo(row(page, 'src'))
    await expect(page.getByText('Moved notes.txt to src.')).toBeVisible({ timeout: 2000 })
  }).toPass({ timeout: 15_000 })
  expect(existsSync(join(folder, 'src', 'notes.txt'))).toBe(true)
  expect(existsSync(join(folder, 'notes.txt'))).toBe(false)
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await expect(page.getByText('Moved notes.txt to src.')).toBeHidden()
  expect(readFileSync(join(folder, 'notes.txt'), 'utf8')).toBe('first line\n')

  // Download comes back byte for byte.
  const download = page.waitForEvent('download')
  await menu(page, 'blob.bin', 'Download')
  const got = await (await download).path()
  expect(readFileSync(got).equals(bytes)).toBe(true)

  // Upload with the file chooser; a second upload of the same name asks first.
  const chooser = page.waitForEvent('filechooser')
  await menu(page, 'docs', 'Upload files…')
  await (await chooser).setFiles({ name: 'up.txt', mimeType: 'text/plain', buffer: Buffer.from('one') })
  await expect(page.getByText('Uploaded up.txt to docs.')).toBeVisible()
  expect(readFileSync(join(folder, 'docs', 'up.txt'), 'utf8')).toBe('one')

  const again = page.waitForEvent('filechooser')
  await menu(page, 'docs', 'Upload files…')
  await (await again).setFiles({ name: 'up.txt', mimeType: 'text/plain', buffer: Buffer.from('two') })
  await expect(page.getByText('up.txt already exists in docs.')).toBeVisible()
  expect(readFileSync(join(folder, 'docs', 'up.txt'), 'utf8')).toBe('one')
  await page.getByRole('button', { name: 'Keep both' }).click()
  await expect(page.getByText('Uploaded up copy.txt to docs.')).toBeVisible()
  expect(readFileSync(join(folder, 'docs', 'up copy.txt'), 'utf8')).toBe('two')

  const third = page.waitForEvent('filechooser')
  await menu(page, 'docs', 'Upload files…')
  await (await third).setFiles({ name: 'up.txt', mimeType: 'text/plain', buffer: Buffer.from('three') })
  await page.getByRole('button', { name: 'Replace', exact: true }).click()
  await expect(page.getByText('Uploaded up.txt to docs.')).toBeVisible()
  expect(readFileSync(join(folder, 'docs', 'up.txt'), 'utf8')).toBe('three')

  // Duplicate.
  await menu(page, 'notes.txt', 'Duplicate')
  await expect(row(page, 'notes copy.txt')).toBeVisible()
  expect(readFileSync(join(folder, 'notes copy.txt'), 'utf8')).toBe('first line\n')

  // Files dropped from the desktop onto a folder.
  await page.evaluate(() => {
    const dt = new DataTransfer()
    dt.items.add(new File(['dropped'], 'dropped.txt', { type: 'text/plain' }))
    const target = document.querySelector('[role="tree"] button.main[data-path="src"]')!.closest('.row')!
    target.dispatchEvent(new DragEvent('dragover', { bubbles: true, cancelable: true, dataTransfer: dt }))
    target.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer: dt }))
  })
  await expect(page.getByText('Uploaded dropped.txt to src.')).toBeVisible()
  expect(readFileSync(join(folder, 'src', 'dropped.txt'), 'utf8')).toBe('dropped')

  // The browser logs the two 409 answers for the taken name, which the page handles by asking.
  expect(problems.filter((p) => !/status of 409/.test(p))).toEqual([])
  expect(problems.filter((p) => /status of 409/.test(p))).toHaveLength(2)
})
