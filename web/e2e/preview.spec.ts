import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import { collectProblems, fixtures, signIn } from './helpers'
import { openProject } from './githelpers'

test('markdown, CSV and HTML files have a Preview that stays safe', async ({ page }) => {
  const name = `preview-${Date.now() % 1_000_000}`
  const folder = join(fixtures, name)
  mkdirSync(folder, { recursive: true })
  writeFileSync(join(folder, 'notes.md'), '# Plan\n\nSome **bold** text <b id="raw">raw</b> and [a link](https://example.com).\n')
  const rows = Array.from({ length: 1200 }, (_, i) => `r${i},${i}`).join('\n')
  writeFileSync(join(folder, 'data.csv'), `name,note\n"Lee, A","said ""hi"""\n${rows}\n`)
  writeFileSync(join(folder, 'page.html'), '<h1>Hello page</h1><script>window.parent.__ran = 1; document.title = "ran"</script>')
  writeFileSync(join(folder, 'plain.txt'), 'nothing to draw')
  await signIn(page)
  const problems = collectProblems(page)
  await openProject(page, folder)
  const tree = page.getByRole('region', { name: /^Files in/ })
  const view = page.getByRole('group', { name: 'View' })
  const preview = view.getByRole('button', { name: 'Preview' })

  await tree.getByRole('button', { name: 'plain.txt', exact: true }).click()
  await expect(page.locator('.cm-content')).toContainText('nothing to draw')
  await expect(view).toHaveCount(0)

  await tree.getByRole('button', { name: 'notes.md', exact: true }).dblclick()
  await expect(view.getByRole('button', { name: 'Source' })).toHaveAttribute('aria-pressed', 'true')
  await preview.click()
  await expect(page.getByRole('heading', { name: 'Plan' })).toBeVisible()
  await expect(page.locator('.md strong')).toHaveText('bold')
  await expect(page.locator('#raw')).toHaveCount(0)
  await expect(page.locator('.md p')).toContainText('<b id="raw">raw</b>')
  await view.getByRole('button', { name: 'Source' }).click()
  await expect(page.locator('.cm-content')).toContainText('# Plan')
  await view.getByRole('button', { name: 'Preview' }).click()

  await tree.getByRole('button', { name: 'data.csv', exact: true }).click()
  await view.getByRole('button', { name: 'Preview' }).click()
  await expect(page.getByRole('columnheader', { name: 'note' })).toBeVisible()
  await expect(page.getByRole('cell', { name: 'Lee, A' })).toBeVisible()
  await expect(page.getByRole('cell', { name: 'said "hi"' })).toBeVisible()
  await expect(page.locator('tbody tr')).toHaveCount(1000)
  await expect(page.getByText('Showing the header and the first 1000 rows')).toBeVisible()

  await tree.getByRole('button', { name: 'page.html', exact: true }).click()
  await view.getByRole('button', { name: 'Preview' }).click()
  const frame = page.locator('iframe[title^="Preview of"]')
  await expect(frame).toHaveAttribute('sandbox', '')
  await expect(page.getByText('Scripts do not run in this preview')).toBeVisible()
  await expect(page.frameLocator('iframe[title^="Preview of"]').getByRole('heading', { name: 'Hello page' })).toBeVisible()
  expect(await page.evaluate(() => (window as unknown as { __ran?: number }).__ran)).toBeUndefined()
  expect(await page.title()).not.toBe('ran')

  // A tab keeps its choice when you leave it and come back.
  await page.getByRole('tab', { name: /notes\.md/ }).click()
  await expect(page.getByRole('heading', { name: 'Plan' })).toBeVisible()

  await page.setViewportSize({ width: 390, height: 844 })
  await expect(page.getByRole('heading', { name: 'Plan' })).toBeVisible()
  for (const n of ['Source', 'Preview']) {
    const b = view.getByRole('button', { name: n })
    await expect.poll(async () => (await b.boundingBox())?.height ?? 0).toBeGreaterThanOrEqual(44)
  }
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true)
  // The browser reports the blocked script; that is the sandbox working, and the only message expected.
  expect(problems.filter((m) => !m.startsWith("Blocked script execution in 'about:srcdoc'"))).toEqual([])
})
