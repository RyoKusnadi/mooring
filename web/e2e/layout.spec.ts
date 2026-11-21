import { mkdirSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { fixtures, signIn } from './helpers'

async function addProject(page: Page, folder: string) {
  mkdirSync(folder, { recursive: true })
  await page.getByRole('button', { name: 'Add project' }).first().click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Folder on this computer').fill(folder)
  await dialog.getByRole('button', { name: 'Go', exact: true }).click()
  await dialog.getByRole('button', { name: 'Open this folder' }).click()
  await expect(dialog).toBeHidden()
}

// A second window that opens a project before the first window's layout is saved
// must not save its default blank layout over it.
test('a second window does not overwrite a new layout with a blank one', async ({ browser }) => {
  const run = Date.now()
  const name = `two-windows-${run}`
  const a = await (await browser.newContext()).newPage()
  const b = await (await browser.newContext()).newPage()
  await signIn(a)
  await signIn(b)
  // B looks at another project, so it makes its default layout only when it opens A's.
  await addProject(b, join(fixtures, `other-${run}`))
  await addProject(a, join(fixtures, name))

  const inB = b.getByRole('navigation', { name: 'Projects' }).getByRole('button', { name, exact: true })
  await expect(inB).toBeVisible()
  await a.getByRole('tabpanel').getByRole('button', { name: 'New terminal' }).click()
  await inB.click()

  await b.waitForTimeout(1_500)
  await a.reload()
  await expect(a.getByRole('tab')).toHaveCount(1)
  await expect(a.getByRole('tab', { name: 'Blank 1' })).toHaveCount(0)
  await expect(b.getByRole('tab')).toHaveCount(1)
})

// A short laptop screen and a phone drawer both keep the gear and "Search commands" on screen.
test('the gear and Search commands stay visible at 1280x640 and in the phone drawer', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 640 })
  await signIn(page)
  const nav = page.getByRole('navigation', { name: 'Projects' })
  const gear = nav.getByRole('button', { name: 'Settings', exact: true })
  await expect(gear).toBeInViewport({ ratio: 1 })
  await expect(nav.getByRole('button', { name: /^Search commands/ })).toBeInViewport({ ratio: 1 })

  await page.setViewportSize({ width: 390, height: 844 })
  await page.getByRole('button', { name: 'Projects', exact: true }).click()
  await expect(gear).toBeInViewport({ ratio: 1 })
  const box = await gear.boundingBox()
  expect(box?.height).toBeGreaterThanOrEqual(44)
})
