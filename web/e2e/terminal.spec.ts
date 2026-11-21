import { writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import { collectProblems } from './helpers'
import { openTerminal, screen, text, type, waitForPrompt } from './termhelpers'

test('vim looks the same after a reload', async ({ page }) => {
  const folder = await openTerminal(page, 'term-vim')
  const problems = collectProblems(page)
  writeFileSync(join(folder, 'long.txt'), Array.from({ length: 400 }, (_, i) => `line ${i + 1} of the file`).join('\n') + '\n')

  await type(page, 'vim -u NONE -n long.txt')
  await expect.poll(async () => (await screen(page))?.alt).toBe(true)
  await expect.poll(() => text(page)).toContain('line 1 of the file')
  await page.keyboard.type('150Gzt')
  // Wait for zt to put line 150 at the top, not just for 150G to show it.
  await expect.poll(async () => (await screen(page))?.rows[0]).toBe('line 150 of the file')
  const before = await screen(page)

  await page.reload()
  await expect.poll(async () => (await screen(page))?.alt, { timeout: 15_000 }).toBe(true)
  await expect.poll(async () => (await screen(page))?.rows, { timeout: 15_000 }).toEqual(before?.rows)
  expect((await screen(page))?.cursor).toEqual(before?.cursor)

  // The reloaded view is live: leave vim and the shell prompt comes back.
  await page.keyboard.type(':q\n')
  await waitForPrompt(page)
  expect(problems).toEqual([])
})

test('a ten second flood keeps the page above 50 frames a second', async ({ page }) => {
  test.setTimeout(90_000)
  await openTerminal(page, 'term-flood')
  await page.evaluate(() => {
    const w = window as unknown as { __frames: number[] }
    w.__frames = []
    let last = performance.now()
    const tick = (t: number) => {
      w.__frames.push(t - last)
      last = t
      requestAnimationFrame(tick)
    }
    requestAnimationFrame(tick)
  })
  await type(page, "yes 'the quick brown fox jumps over the lazy dog 0123456789 the quick brown fox' & sleep 10; kill %1; wait; echo flood-$((6*7))")
  await expect.poll(() => text(page), { timeout: 40_000 }).toContain("flood-42")
  const gaps = await page.evaluate(() => (window as unknown as { __frames: number[] }).__frames)
  const fps = (gaps.length * 1000) / gaps.reduce((a, b) => a + b, 0)
  const slow = gaps.filter((g) => g > 50).length
  console.log(`flood: ${fps.toFixed(1)} fps over ${gaps.length} frames, ${slow} gaps over 50 ms`)
  expect(fps).toBeGreaterThan(50)
})

test('two windows share one terminal and the last to resize sets the size', async ({ page, context }) => {
  await page.setViewportSize({ width: 1400, height: 700 })
  await openTerminal(page, 'term-size')
  const first = (await screen(page))!
  await page.waitForTimeout(800) // the pane layout is saved 400 ms after the last edit
  const other = await context.newPage()
  await other.setViewportSize({ width: 1000, height: 500 })
  await other.goto('/?test')
  await waitForPrompt(other)

  // Joining does not change the size: the second window is letterboxed.
  expect((await screen(other))?.cols).toBe(first.cols)
  await expect(other.getByText('Sized by another window')).toBeVisible()

  // Typing in the second window takes over the size, for both.
  await type(other, 'stty size')
  await expect.poll(async () => (await screen(other))!.cols).toBeLessThan(first.cols)
  const taken = (await screen(other))!
  await expect.poll(async () => (await screen(page))!.cols).toBe(taken.cols)
  await expect.poll(() => text(page)).toContain(`${taken.height} ${taken.cols}`)

  // Resizing the first window takes it back, and both settle on one size.
  await page.setViewportSize({ width: 1300, height: 650 })
  await expect.poll(async () => (await screen(page))!.cols).toBeGreaterThan(taken.cols)
  const back = (await screen(page))!
  await expect.poll(async () => (await screen(other))!.cols).toBe(back.cols)
  await page.waitForTimeout(600)
  expect([(await screen(page))!.cols, (await screen(other))!.cols]).toEqual([back.cols, back.cols])
  await other.close()
})

test('keyboard only: leave a live terminal, then reach Start a new terminal after exit', async ({ page }) => {
  await openTerminal(page, 'term-keys')
  await expect(page.getByText(/(Alt\+|⌥)Esc leaves the terminal/)).toBeVisible()

  // Alt+Esc leaves the shell for the tab bar.
  await page.keyboard.press('Alt+Escape')
  await expect(page.getByRole('tab', { selected: true })).toBeFocused()

  // Back in, and exit: the restart button takes focus and Enter starts a new shell.
  await page.locator('[data-terminal] .xterm-helper-textarea').focus()
  await page.keyboard.type('exit 3\n')
  const restart = page.getByRole('button', { name: 'Start a new terminal' })
  await expect(restart).toBeFocused()
  await page.keyboard.press('Enter')
  await waitForPrompt(page)
})
