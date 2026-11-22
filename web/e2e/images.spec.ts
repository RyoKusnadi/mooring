import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import { collectProblems, fixtures, signIn } from './helpers'
import { openProject } from './githelpers'

// A 2 x 2 PNG.
const PNG = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAABytg0kAAAAFElEQVR4nGP8z8Dwn4EIwESMolGF5CkEAHpyAgeX0iDLAAAAAElFTkSuQmCC', 'base64')

test('images show in the file tab, and an svg with a script runs nothing', async ({ page }) => {
  const name = `images-${Date.now() % 1_000_000}`
  const folder = join(fixtures, name)
  mkdirSync(folder, { recursive: true })
  writeFileSync(join(folder, 'dot.png'), PNG)
  writeFileSync(join(folder, 'evil.svg'), '<svg xmlns="http://www.w3.org/2000/svg"><script>window.__ran = 1; document.title = "ran"</script></svg>')
  writeFileSync(join(folder, 'blob.bin'), Buffer.from([0, 1, 2, 3, 0, 255]))
  await signIn(page)
  const problems = collectProblems(page)
  await openProject(page, folder)
  const tree = page.getByRole('region', { name: /^Files in/ })

  await tree.getByRole('button', { name: 'dot.png', exact: true }).click()
  const img = page.getByRole('img', { name: 'dot.png' })
  await expect(img).toBeVisible()
  expect(await img.evaluate((e: HTMLImageElement) => e.naturalWidth)).toBeGreaterThan(0)
  await expect(page.getByRole('status').filter({ hasText: '2 × 2 px' })).toBeVisible()
  await page.getByRole('button', { name: '100%' }).click()
  await expect(page.getByRole('button', { name: '100%' })).toHaveAttribute('aria-pressed', 'true')

  await tree.getByRole('button', { name: 'blob.bin', exact: true }).click()
  await expect(page.getByText('This is a binary file, so it is not shown here.')).toBeVisible()

  await tree.getByRole('button', { name: 'evil.svg', exact: true }).click()
  await expect(page.getByRole('tab', { name: /evil\.svg/ })).toBeVisible()
  expect(await page.evaluate(() => (window as unknown as { __ran?: number }).__ran)).toBeUndefined()
  expect(await page.title()).not.toBe('ran')
  expect(problems).toEqual([])
})
