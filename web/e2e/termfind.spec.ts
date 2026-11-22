import { expect, test, type Page } from '@playwright/test'
import { collectProblems } from './helpers'
import { openTerminal, screen, type } from './termhelpers'

const findKey = process.platform === 'darwin' ? 'Meta+F' : 'Control+Shift+F'
// The word is split in the command so only the output holds it: five hits.
const print = 'clear; for i in 1 2 3 4 5; do echo "nee""dle $i"; done; echo done-printing'

async function printed(page: Page) {
  await type(page, print)
  await expect.poll(async () => (await screen(page))?.rows.some((r) => r.startsWith('done-printing'))).toBe(true)
}

test('find in the terminal: count, older and newer wrap, Escape returns to the shell', async ({ page }) => {
  await openTerminal(page, 'term-find')
  const problems = collectProblems(page)
  await printed(page)

  const bar = page.getByRole('group', { name: 'Find in output' })
  const field = bar.getByRole('textbox', { name: 'Find in output' })
  await expect(bar).toBeHidden()
  await page.keyboard.press(findKey)
  await expect(field).toBeFocused()

  await field.fill('needle')
  await expect(bar.getByRole('status')).toHaveText('5 of 5')
  await page.keyboard.press('Enter')
  await expect(bar.getByRole('status')).toHaveText('4 of 5')
  await page.keyboard.press('Shift+Enter')
  await page.keyboard.press('Shift+Enter')
  await expect(bar.getByRole('status')).toHaveText('1 of 5') // wraps from the newest to the oldest
  await page.keyboard.press('Enter')
  await expect(bar.getByRole('status')).toHaveText('5 of 5') // and back
  await bar.getByRole('button', { name: 'Older match' }).click()
  await expect(bar.getByRole('status')).toHaveText('4 of 5')

  await field.fill('Needle') // a capital makes it exact
  await expect(bar.getByRole('status')).toHaveText('No matches')
  await field.fill('zzzz')
  await expect(bar.getByRole('status')).toHaveText('No matches')

  await page.keyboard.press('Escape')
  await expect(bar).toBeHidden()
  await expect(page.locator('[data-terminal] .xterm-helper-textarea')).toBeFocused()
  await page.keyboard.type('echo still-typing\n')
  await expect.poll(async () => (await screen(page))?.rows.some((r) => r.startsWith('still-typing'))).toBe(true)

  await page.getByRole('button', { name: 'Find in output' }).click()
  await expect(field).toBeFocused()
  await bar.getByRole('button', { name: 'Close find' }).click()
  await expect(bar).toBeHidden()
  expect(problems).toEqual([])
})

test('find in the terminal on a phone: fits, 44 px targets, clear of the keys row', async ({ page, browser }) => {
  test.setTimeout(60_000)
  const name = 'term-find-phone'
  await openTerminal(page, name)
  await printed(page)
  await page.waitForTimeout(1000) // the layout is saved to the server a moment after an edit
  const phone = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true, storageState: await page.context().storageState() })
  const p = await phone.newPage()
  await p.goto('/?test')
  await p.getByRole('button', { name: 'Projects', exact: true }).click()
  await p.getByRole('button', { name, exact: true }).click()
  await expect(p.getByRole('toolbar', { name: 'Terminal keys' })).toBeVisible()

  await p.getByRole('button', { name: 'Find in output' }).click()
  const bar = p.getByRole('group', { name: 'Find in output' })
  await bar.getByRole('textbox', { name: 'Find in output' }).fill('needle')
  await expect(bar.getByRole('status')).toHaveText(/^\d of 5$/)

  expect(await p.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(0)
  const keys = await p.getByRole('toolbar', { name: 'Terminal keys' }).boundingBox()
  const box = await bar.boundingBox()
  expect(box && keys && box.y + box.height <= keys.y && box.x >= 0 && box.x + box.width <= 390).toBe(true)
  for (const el of [bar.getByRole('textbox'), ...(await bar.getByRole('button').all())]) {
    const b = await el.boundingBox()
    expect(b && b.height >= 44 && (b.width >= 44 || (await el.getAttribute('type')) === 'text')).toBe(true)
  }
  await phone.close()
})
