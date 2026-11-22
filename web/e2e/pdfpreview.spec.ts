import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import { openProject } from './githelpers'
import { collectProblems, fixtures, signIn } from './helpers'

// A one-page PDF with no text, small enough to write out by hand.
const PDF = [
  '%PDF-1.1',
  '1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj',
  '2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj',
  '3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 100]>>endobj',
  'trailer<</Root 1 0 R>>',
  '%%EOF',
].join('\n')

test('a PDF opens in a frame on the raw route, and a text file named .pdf says it is not one', async ({ page }) => {
  const folder = join(fixtures, `pdf-${Date.now() % 1_000_000}`)
  mkdirSync(folder, { recursive: true })
  writeFileSync(join(folder, 'tiny.pdf'), PDF)
  writeFileSync(join(folder, 'fake.pdf'), 'this is only text\n')
  await signIn(page)
  const problems = collectProblems(page)
  await openProject(page, folder)
  const tree = page.getByRole('region', { name: /^Files in/ })

  await tree.getByRole('button', { name: 'tiny.pdf', exact: true }).dblclick()
  const frame = page.locator('iframe[title="tiny.pdf (PDF)"]')
  await expect(frame).toBeVisible()
  await expect(frame).toHaveAttribute('src', /\/api\/files\/raw\?project=.*&path=tiny\.pdf/)
  await expect(frame).not.toHaveAttribute('sandbox', /.*/)
  await expect(page.getByRole('link', { name: 'Open in a new tab' })).toHaveAttribute('target', '_blank')

  await tree.getByRole('button', { name: 'fake.pdf', exact: true }).dblclick()
  await expect(page.getByText('fake.pdf is not a PDF, whatever its name says.')).toBeVisible()
  await expect(page.locator('iframe[title="fake.pdf (PDF)"]')).toHaveCount(0)
  await page.getByRole('button', { name: 'Open as text' }).click()
  await expect(page.locator('.cm-content')).toContainText('this is only text')

  // On a phone the page does not scroll sideways.
  await page.setViewportSize({ width: 390, height: 844 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true)
  expect(problems.filter((m) => !/status of (415|404)/.test(m))).toEqual([])
})
