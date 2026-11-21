import { expect, test } from '@playwright/test'
import { token } from './helpers'

test('the page needs a login, and signing in and out works', async ({ page }) => {
  const problems: string[] = []
  page.on('console', (m) => {
    if (m.type() === 'error' || m.type() === 'warning') problems.push(m.text())
  })
  page.on('pageerror', (e) => problems.push(e.message))

  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Sign in to Mooring' })).toBeVisible()

  await page.getByText('Use the login token instead').click()
  await page.getByLabel('Login token').fill('not-the-token')
  await page.getByRole('button', { name: 'Sign in with token' }).click()
  await expect(page.getByRole('alert')).toContainText('does not match')

  await page.getByLabel('Login token').fill(token)
  await page.getByRole('button', { name: 'Sign in with token' }).click()
  await expect(page.getByRole('heading', { name: 'Projects', exact: true })).toBeVisible()

  await page.reload()
  await expect(page.getByRole('heading', { name: 'Projects', exact: true })).toBeVisible()

  await page.getByRole('navigation', { name: 'Projects' }).getByRole('button', { name: 'Settings', exact: true }).click()
  await page.getByRole('dialog', { name: 'Settings' }).getByRole('button', { name: 'Sign out' }).click()
  await expect(page.getByRole('heading', { name: 'Sign in to Mooring' })).toBeVisible()

  // The wrong-token attempt logs one expected 401; anything else is a bug,
  // including Content-Security-Policy violations.
  expect(problems.filter((p) => !p.includes('401'))).toEqual([])
})
