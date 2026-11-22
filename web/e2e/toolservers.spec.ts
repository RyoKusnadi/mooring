import { mkdirSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { mcpPort } from '../playwright.config'
import { collectProblems, fixtures, signIn } from './helpers'

// Stand-ins (see serve.sh): "fake-mcp" on the PATH is a stdio tool server with tools echo and add;
// http://127.0.0.1:mcpPort/mcp is an HTTP one that needs sign-in and a private-address allowance.
// The stand-in agent "gemini" answers the prompt word TOOLS with the servers it was given.

async function start(page: Page, name: string) {
  const folder = join(fixtures, name)
  mkdirSync(folder, { recursive: true })
  await signIn(page)
  await page.goto('/?test')
  await page.getByRole('button', { name: 'Add project' }).first().click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Folder on this computer').fill(folder)
  await dialog.getByRole('button', { name: 'Go', exact: true }).click()
  await dialog.getByRole('button', { name: 'Open this folder' }).click()
  await expect(dialog).toBeHidden()
}

async function openTools(page: Page) {
  await page.keyboard.press('Alt+m')
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByRole('heading', { name: 'Tool servers' })).toBeVisible()
  return dialog
}

async function chatWith(page: Page, name: string) {
  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New chat' }).click()
  await panel.getByText('Options', { exact: true }).click()
  await panel.getByLabel(name).check()
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await page.getByLabel('Message to Gemini').fill('TOOLS')
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  return panel
}

test('tool servers: add a local server through consent, check, enable, use it in a chat', async ({ page }) => {
  test.setTimeout(90_000)
  await start(page, `tools-stdio-${Date.now()}`)
  const problems = collectProblems(page)
  const dialog = await openTools(page)
  await expect(dialog.getByText('No tool servers yet')).toBeVisible()

  await dialog.getByRole('button', { name: 'Add your own' }).click()
  await dialog.getByLabel('Name', { exact: true }).fill('Notes')
  await dialog.getByLabel('Command', { exact: true }).fill('fake-mcp')
  await dialog.getByRole('button', { name: 'Review' }).click()

  // Consent shows the exact command and nothing runs yet.
  await expect(dialog.getByRole('heading', { name: 'Allow this tool server?' })).toBeVisible()
  await expect(dialog.getByText('Type', { exact: true })).toBeVisible()
  await expect(dialog.locator('[data-consent="command"]')).toHaveText('fake-mcp')
  await dialog.getByRole('button', { name: 'Allow and enable' }).click()

  // One click allowed it: the check runs, lists the tools and the server is enabled without a second step.
  await expect(dialog.getByText('Notes answered with 2 tools.')).toBeVisible()
  await expect(dialog.getByText('echo', { exact: true })).toBeVisible()
  await expect(dialog.getByText('It is allowed and enabled.')).toBeVisible()
  await expect(dialog.getByText(/^Tools listed at \d{1,2}[:.]\d{2}/)).toBeVisible()
  // Check again lists the tools anew instead of reusing the fresh list.
  await dialog.getByRole('button', { name: 'Check again' }).click()
  await expect(dialog.getByText('Notes answered with 2 tools.')).toBeVisible()
  await expect(dialog.getByText(/^Tools listed at /)).toBeVisible()
  await expect(dialog.getByRole('button', { name: 'Enable', exact: true })).toHaveCount(0)
  await dialog.getByRole('button', { name: 'Done' }).click()
  const row = dialog.getByRole('listitem').filter({ hasText: 'Notes' }).first()
  await expect(row.getByText('Enabled', { exact: true })).toBeVisible()
  await row.getByRole('button', { name: 'Show 2 tools' }).click()
  await expect(row.getByText('add', { exact: true })).toBeVisible()
  // A tool's description sits under its name, not run on after it.
  const stacked = await row.locator('.tl li').evaluateAll((items) => items.every((li) => { const d = li.querySelector('.tdesc'); return !d || d.getBoundingClientRect().top >= li.querySelector('code')!.getBoundingClientRect().bottom - 1 }))
  expect(stacked).toBe(true)

  // Escape closes the dialog and the page is usable again.
  await page.keyboard.press('Escape')
  await expect(dialog).toBeHidden()

  const panel = await chatWith(page, 'Notes')
  await expect(panel.getByText('tools:Notes(stdio,auth=no)', { exact: true })).toBeVisible()
  expect(problems).toEqual([])
})

test('tool servers: a private address needs the box, then sign in by pasting the address', async ({ page }) => {
  test.setTimeout(90_000)
  await start(page, `tools-http-${Date.now()}`)
  const dialog = await openTools(page)

  await dialog.getByRole('button', { name: 'Add your own' }).click()
  await dialog.getByLabel('A remote address (http)', { exact: true }).check()
  await dialog.getByLabel('Name', { exact: true }).fill('Remote notes')
  await dialog.getByLabel('Address', { exact: true }).fill(`http://127.0.0.1:${mcpPort}/mcp`)
  await dialog.getByRole('button', { name: 'Review' }).click()
  // A plain http address is refused before the consent screen unless a private address is allowed.
  await expect(dialog.getByRole('alert')).toContainText('private')
  await dialog.getByLabel('Address', { exact: true }).fill(`https://127.0.0.1:${mcpPort}/mcp`)
  await dialog.getByRole('button', { name: 'Review' }).click()
  await dialog.getByRole('button', { name: 'Allow and enable' }).click()
  await expect(dialog.getByRole('alert')).toContainText('this machine or a private network')

  // Back to the form, tick the box and use the plain address.
  await dialog.getByRole('button', { name: 'Back' }).click()
  await dialog.getByLabel('Address', { exact: true }).fill(`http://127.0.0.1:${mcpPort}/mcp`)
  await dialog.getByLabel('Allow a private address').check()
  await dialog.getByRole('button', { name: 'Review' }).click()
  await expect(dialog.locator('[data-consent="address"]')).toHaveText(`http://127.0.0.1:${mcpPort}/mcp`)
  await expect(dialog.getByText('This server can connect to this machine or your home network.')).toBeVisible()
  await dialog.getByRole('button', { name: 'Allow and enable' }).click()

  // The check cannot get a tool list without signing in.
  await expect(dialog.getByText('The check failed.')).toBeVisible()
  await dialog.getByRole('button', { name: 'Sign in' }).click()
  const link = dialog.getByRole('link', { name: 'Open the sign-in page' })
  await expect(link).toHaveAttribute('rel', /noopener/)
  const first = (await link.getAttribute('href')) ?? ''
  // A reply that names another sign-in site is refused, with one message that says why.
  const wrong = (await page.request.get(first, { maxRedirects: 0 })).headers()['location']
  await dialog.getByLabel('Address of the page it ended on').fill(wrong + '&iss=https%3A%2F%2Fother-site.example')
  await dialog.getByRole('button', { name: 'Finish sign-in' }).click()
  await expect(dialog.getByRole('alert')).toHaveCount(1)
  await expect(dialog.getByRole('alert')).toContainText('different sign-in site than expected')
  await dialog.getByRole('button', { name: 'Start the sign-in again' }).click()
  await expect(dialog.getByRole('alert')).toHaveCount(0)
  await expect(link).not.toHaveAttribute('href', first)
  const href = (await link.getAttribute('href')) ?? ''
  // The stand-in approves at once and redirects to Mooring's callback; read where it points instead of following it.
  const res = await page.request.get(href, { maxRedirects: 0 })
  const back = res.headers()['location']
  expect(back).toContain('/api/toolservers/callback')
  await dialog.getByLabel('Address of the page it ended on').fill(back)
  await dialog.getByRole('button', { name: 'Finish sign-in' }).click()
  await expect(dialog.getByText('Signed in to Remote notes.')).toBeVisible()
  await dialog.getByRole('button', { name: 'Back to list' }).click()

  const row = dialog.getByRole('listitem').filter({ hasText: 'Remote notes' })
  await expect(row.getByText('Signed in', { exact: true })).toBeVisible()
  await row.getByRole('button', { name: 'Check Remote notes' }).click()
  await expect(dialog.getByText(/Remote notes answered with \d+ tools?\./)).toBeVisible()
  await dialog.getByRole('button', { name: 'Enable' }).click()
  await expect(row.getByText('Enabled', { exact: true })).toBeVisible()
  await page.keyboard.press('Escape')

  const panel = await chatWith(page, 'Remote notes')
  await expect(panel.getByText('tools:Remote-notes(http,auth=yes)', { exact: true })).toBeVisible()
})

test.describe('phone', () => {
  test.use({ hasTouch: true, colorScheme: 'dark' })

  test('tool servers: a long command wraps, targets are 44px, nothing scrolls sideways', async ({ page }) => {
    await start(page, `tools-phone-${Date.now()}`)
    await page.setViewportSize({ width: 390, height: 844 })
    await page.locator('header').getByRole('button', { name: /^Commands/ }).click()
    await expect(page.getByRole('dialog', { name: 'Command palette' })).toBeVisible()
    await page.keyboard.type('Tool servers')
    await page.keyboard.press('Enter')
    const dialog = page.getByRole('dialog')
    await expect(dialog.getByRole('heading', { name: 'Tool servers' })).toBeVisible()
    await dialog.getByRole('button', { name: 'Add your own' }).click()
    await dialog.getByLabel('Name', { exact: true }).fill('Long')
    await dialog.getByLabel('Command', { exact: true }).fill('/very/long/path/' + 'segment/'.repeat(20) + 'server')
    await dialog.getByLabel('Arguments (optional)').fill('--url=https://example.com/' + 'a'.repeat(120))
    await dialog.getByRole('button', { name: 'Review' }).click()
    await expect(dialog.getByRole('button', { name: 'Allow and enable' })).toBeVisible()
    const box = async (sel: string) => dialog.locator(sel).first().evaluate((el) => ({ sw: el.scrollWidth, cw: el.clientWidth }))
    const body = await dialog.evaluate((el) => ({ sw: el.scrollWidth, cw: el.clientWidth }))
    expect(body.sw).toBeLessThanOrEqual(body.cw)
    const arg = await box('[data-consent="arg"]')
    expect(arg.sw).toBeLessThanOrEqual(arg.cw)
    for (const name of ['Back', 'Allow and enable']) {
      const b = await dialog.getByRole('button', { name, exact: true }).boundingBox()
      expect(b!.height).toBeGreaterThanOrEqual(44)
    }
  })
})
