import { execFileSync } from 'node:child_process'
import { mkdirSync } from 'node:fs'
import { resolve } from 'node:path'
import { expect, type Page } from '@playwright/test'
import { dataDir } from '../playwright.config'

export const token = execFileSync('../bin/mooring', ['token', '-data', dataDir], { encoding: 'utf8' }).trim()

// Folders the tests add or clone live inside the e2e data folder, which is
// inside the home folder the server lets the browser use.
export const fixtures = resolve(dataDir, 'fixtures')
mkdirSync(fixtures, { recursive: true })

export function collectProblems(page: Page): string[] {
  const problems: string[] = []
  page.on('console', (m) => {
    // xterm warns when its idle-time work runs late, which happens on a busy machine (the whole suite at once).
    if (m.type() === 'warning' && /^task queue exceeded allotted deadline/.test(m.text())) return
    // Chrome without a GPU (CI runners) reports slow WebGL reads from the terminal; not an app problem.
    if (/GL Driver Message|GPU stall due to ReadPixels/.test(m.text())) return
    if (m.type() === 'error' || m.type() === 'warning') problems.push(m.text())
  })
  page.on('pageerror', (e) => problems.push(e.message))
  return problems
}

/** Opens Settings from the gear in the sidebar and picks a section. */
export async function openSettings(page: Page, section: string) {
  await page.getByRole('navigation', { name: 'Projects' }).getByRole('button', { name: 'Settings', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: 'Settings' })
  await expect(dialog).toBeVisible()
  await dialog.getByRole('button', { name: section, exact: true }).click()
  return dialog
}

export async function signIn(page: Page): Promise<void> {
  await page.goto('/')
  await page.getByText('Use the login token instead').click()
  await page.getByLabel('Login token').fill(token)
  await page.getByRole('button', { name: 'Sign in with token' }).click()
  await expect(page.getByRole('heading', { name: 'Projects', exact: true })).toBeVisible()
}
