import { type ChildProcess, execFileSync, spawn } from 'node:child_process'
import { mkdirSync, readFileSync, rmSync, statSync } from 'node:fs'
import { resolve } from 'node:path'
import { expect, test } from '@playwright/test'
import { dataDir, passwordPort } from '../playwright.config'

// Own Mooring on its own port and data folder: setting a password changes the login page for everyone.
const port = passwordPort
const origin = `http://127.0.0.1:${port}`
const dir = resolve(dataDir, 'pwlogin')
const data = resolve(dir, 'data')
const mooring = resolve('../bin/mooring')
const secret = 'correct horse battery'

function start(): ChildProcess {
  return spawn(mooring, ['-addr', `127.0.0.1:${port}`, '-data', data, '-shell', resolve('e2e/shell.sh'), '-quiet'], { stdio: 'ignore' })
}

async function waitUp(): Promise<void> {
  for (let i = 0; i < 100; i++) {
    try {
      await fetch(`${origin}/api/session`)
      return
    } catch {
      await new Promise((r) => setTimeout(r, 100))
    }
  }
  throw new Error('the password test server did not start')
}

test('set a password in Settings, sign in with it, then remove it', async ({ browser }) => {
  rmSync(dir, { recursive: true, force: true })
  mkdirSync(dir, { recursive: true })
  const server = start()
  try {
    await waitUp()
    const token = execFileSync(mooring, ['token', '-data', data], { encoding: 'utf8' }).trim()
    const page = await (await browser.newContext()).newPage()

    // No password yet: the page is as before, plus one line about Settings.
    await page.goto(origin)
    await expect(page.getByText('you can set a username and password in Settings')).toBeVisible()
    await expect(page.getByLabel('Username')).toHaveCount(0)

    await page.getByText('Use the login token instead').click()
    await page.getByLabel('Login token').fill(token)
    await page.getByRole('button', { name: 'Sign in with token' }).click()
    await expect(page.getByRole('heading', { name: 'Projects', exact: true })).toBeVisible()

    await page.getByRole('navigation', { name: 'Projects' }).getByRole('button', { name: 'Settings', exact: true }).click()
    const dialog = page.getByRole('dialog', { name: 'Settings' })
    await dialog.getByRole('button', { name: 'Sign-in', exact: true }).click()
    await dialog.getByLabel('Username').fill('ada')
    await dialog.getByLabel('New password', { exact: true }).fill(secret)
    await dialog.getByLabel('New password again').fill('something else')
    await dialog.getByRole('button', { name: 'Set password' }).click()
    await expect(dialog.getByRole('alert')).toContainText('not the same')
    await dialog.getByLabel('New password again').fill(secret)
    await dialog.getByRole('button', { name: 'Set password' }).click()
    await expect(dialog.getByRole('status').filter({ hasText: 'Password set' })).toBeVisible()

    // The file holds a hash, readable by its owner only.
    const file = resolve(data, 'password')
    expect(statSync(file).mode & 0o777).toBe(0o600)
    expect(readFileSync(file, 'utf8')).not.toContain(secret)

    await dialog.getByRole('button', { name: 'Sign out', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Sign in to Mooring' })).toBeVisible()

    // The form fits a phone: nothing scrolls sideways at 390 px.
    await page.setViewportSize({ width: 390, height: 800 })
    await expect(page.getByLabel('Username')).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
    await page.setViewportSize({ width: 1280, height: 800 })

    // A wrong password gives one message and clears the field.
    await page.getByLabel('Username').fill('ada')
    await page.getByLabel('Password', { exact: true }).fill('not the password')
    await page.getByRole('button', { name: 'Sign in', exact: true }).click()
    await expect(page.getByRole('alert')).toHaveText('Wrong username or password.')
    await expect(page.getByLabel('Password', { exact: true })).toHaveValue('')
    await page.getByLabel('Username').fill('someone')
    await page.getByLabel('Password', { exact: true }).fill(secret)
    await page.getByRole('button', { name: 'Sign in', exact: true }).click()
    await expect(page.getByRole('alert')).toHaveText('Wrong username or password.')

    await page.getByLabel('Username').fill('ada')
    await page.getByLabel('Password', { exact: true }).fill(secret)
    await page.getByRole('button', { name: 'Sign in', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Projects', exact: true })).toBeVisible()

    // Sign out other browsers: the second browser loses its session, this one keeps it.
    const second = await (await browser.newContext()).newPage()
    await second.goto(origin)
    await second.getByLabel('Username').fill('ada')
    await second.getByLabel('Password', { exact: true }).fill(secret)
    await second.getByRole('button', { name: 'Sign in', exact: true }).click()
    await expect(second.getByRole('heading', { name: 'Projects', exact: true })).toBeVisible()
    await page.getByRole('navigation', { name: 'Projects' }).getByRole('button', { name: 'Settings', exact: true }).click()
    const others = page.getByRole('dialog', { name: 'Settings' })
    await others.getByRole('button', { name: 'Sign-in', exact: true }).click()
    await others.getByLabel('Password to confirm sign-out').fill('not the password')
    await others.getByRole('button', { name: 'Sign out other browsers' }).click()
    await expect(others.getByRole('alert')).toContainText('not right')
    await others.getByLabel('Password to confirm sign-out').fill(secret)
    await others.getByRole('button', { name: 'Sign out other browsers' }).click()
    await expect(others.getByRole('status').filter({ hasText: 'Other browsers are signed out' })).toBeVisible()
    // Its open socket is closed too, so it needs no reload to find out.
    await expect(second.getByRole('heading', { name: 'Sign in to Mooring' })).toBeVisible({ timeout: 15000 })
    await page.reload()
    await expect(page.getByRole('heading', { name: 'Projects', exact: true })).toBeVisible()

    // Remove needs the current password.
    await page.getByRole('navigation', { name: 'Projects' }).getByRole('button', { name: 'Settings', exact: true }).click()
    const again = page.getByRole('dialog', { name: 'Settings' })
    await again.getByRole('button', { name: 'Sign-in', exact: true }).click()
    await again.getByLabel('Password to confirm removal').fill('not the password')
    await again.getByRole('button', { name: 'Remove password' }).click()
    await expect(again.getByRole('alert')).toContainText('not right')
    await again.getByLabel('Password to confirm removal').fill(secret)
    await again.getByRole('button', { name: 'Remove password' }).click()
    await expect(again.getByRole('status').filter({ hasText: 'Password removed' })).toBeVisible()

    await again.getByRole('button', { name: 'Sign out', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Sign in to Mooring' })).toBeVisible()
    await expect(page.getByLabel('Username')).toHaveCount(0)
    await expect(page.getByText('Run mooring open in a terminal')).toBeVisible()
  } finally {
    server.kill('SIGTERM')
  }
})
