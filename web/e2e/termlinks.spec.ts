import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { collectProblems } from './helpers'
import { openTerminal, screen, type } from './termhelpers'

/** Clicks the cell at row and column of the visible terminal screen. */
async function clickCell(page: Page, row: number, col: number) {
  const s = await screen(page)
  const box = await page.locator('[data-terminal] .xterm-screen').boundingBox()
  if (!s || !box) throw new Error('no terminal screen')
  const w = box.width / s.cols
  const h = box.height / s.rows.length
  const x = box.x + (col + 0.5) * w
  const y = box.y + (row + 0.5) * h
  await page.mouse.move(x - w, y) // xterm finds links on mouse move
  await page.mouse.move(x, y)
  await page.mouse.click(x, y)
}

test('a path:line printed in the terminal opens the file at that line', async ({ page }) => {
  const folder = await openTerminal(page, 'term-links')
  const problems = collectProblems(page)
  mkdirSync(join(folder, 'src'), { recursive: true })
  writeFileSync(join(folder, 'src', 'app.ts'), Array.from({ length: 40 }, (_, i) => `const line${i + 1} = ${i + 1}`).join('\n') + '\n')

  await type(page, 'clear; echo "error at src/app.ts:12:3 here"')
  await expect.poll(async () => (await screen(page))?.rows[0]).toBe('error at src/app.ts:12:3 here')
  await clickCell(page, 0, 'error at '.length + 2)

  await expect(page.getByRole('tab', { name: /app\.ts/ })).toBeVisible()
  await expect(page.locator('[data-editor-view] .cm-activeLine')).toContainText('const line12 = 12')
  expect(problems).toEqual([])
})
