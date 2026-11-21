import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import { collectProblems, fixtures, signIn } from './helpers'

test('files: move a file with the keyboard only, then undo', async ({ page }) => {
  test.setTimeout(60_000)
  const folder = join(fixtures, `files3-${Date.now()}-${test.info().repeatEachIndex}`)
  mkdirSync(join(folder, 'src', 'lib'), { recursive: true })
  mkdirSync(join(folder, 'docs'), { recursive: true })
  writeFileSync(join(folder, 'docs', 'guide.txt'), 'g\n')
  writeFileSync(join(folder, 'src', 'lib', 'x.ts'), '1\n')
  writeFileSync(join(folder, 'notes.txt'), 'first line\n')

  await signIn(page)
  const problems = collectProblems(page)
  await page.getByRole('button', { name: 'Add project' }).first().click()
  const add = page.getByRole('dialog')
  await add.getByLabel('Folder on this computer').fill(folder)
  await add.getByRole('button', { name: 'Go', exact: true }).click()
  await add.getByRole('button', { name: 'Open this folder' }).click()
  await expect(add).toBeHidden()

  const tree = page.getByRole('region', { name: /^Files in/ })
  const notes = tree.getByRole('button', { name: 'notes.txt', exact: true })
  await notes.focus()
  await page.keyboard.press('m')
  const dlg = page.getByRole('dialog', { name: 'Move notes.txt' })
  await expect(dlg).toBeVisible()
  await expect(dlg.getByRole('combobox', { name: 'Folder' })).toBeFocused()
  await page.keyboard.type('lib')
  await expect(dlg.getByRole('option')).toHaveCount(1)
  await page.keyboard.press('Enter')
  await expect(dlg).toBeHidden()
  await expect(page.getByText('Moved notes.txt to lib.')).toBeVisible()
  expect(existsSync(join(folder, 'src', 'lib', 'notes.txt'))).toBe(true)
  expect(existsSync(join(folder, 'notes.txt'))).toBe(false)
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await expect.poll(() => existsSync(join(folder, 'notes.txt'))).toBe(true)
  expect(readFileSync(join(folder, 'notes.txt'), 'utf8')).toBe('first line\n')

  // Escape leaves everything where it was.
  await notes.focus()
  await page.keyboard.press('m')
  await expect(dlg).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(dlg).toBeHidden()
  await expect(notes).toBeFocused()
  expect(existsSync(join(folder, 'notes.txt'))).toBe(true)
  expect(problems).toEqual([])
})
