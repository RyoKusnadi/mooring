import { createServer } from 'node:http'
import { mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { randomBytes } from 'node:crypto'
import { join, resolve } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import { dataDir, sshPort } from '../playwright.config'
import { collectProblems, fixtures, signIn } from './helpers'
import { text, type } from './termhelpers'

// The stand-in SSH server (see serve.sh): 127.0.0.1:sshPort, user e2e, password e2e-pass, home below.
const home = resolve(dataDir, 'fakessh-home')
const known = resolve(dataDir, 'known_hosts')
const editor = (page: Page) => page.locator('[data-editor-view] .cm-content')

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

/** Saves a password host. It is tested at once: the key is trusted and the password typed in one dialog. */
async function addHost(page: Page, name: string, how: { password: string | null } = { password: 'e2e-pass' }) {
  await page.keyboard.press('Alt+s')
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByRole('heading', { name: 'SSH hosts' })).toBeVisible()
  await dialog.getByRole('button', { name: 'Add host' }).click()
  await dialog.getByLabel('Name').fill(name)
  await dialog.getByLabel('Address').fill('127.0.0.1')
  await dialog.getByLabel('Port').fill(String(sshPort))
  await dialog.getByLabel('User').fill('e2e')
  await dialog.getByLabel('Sign-in method').selectOption('password')
  await dialog.getByRole('button', { name: 'Add and test' }).click()
  if (how.password !== null) {
    const hostkey = page.getByRole('dialog', { name: 'Trust this host?' })
    await expect(hostkey).toContainText(`e2e@127.0.0.1:${sshPort}`)
    await expect(hostkey).toContainText('SHA256:')
    await hostkey.getByLabel(/^Password for/).fill(how.password)
    await hostkey.getByRole('button', { name: 'Trust and sign in' }).click()
    if (how.password !== 'e2e-pass') return dialog
    await expect(page.getByRole('status').filter({ hasText: 'The host works.' })).toBeVisible({ timeout: 15_000 })
  } else {
    await expect(page.getByRole('status').filter({ hasText: /\w/ }).first()).toBeVisible({ timeout: 15_000 })
    await expect(dialog.getByRole('button', { name: 'Test again' })).toBeVisible({ timeout: 15_000 })
  }
  await dialog.getByRole('button', { name: 'Done' }).click()
  await expect(dialog.getByText(name, { exact: true })).toBeVisible()
  return dialog
}

async function signInWithPassword(page: Page) {
  const prompt = page.getByRole('dialog').last()
  await prompt.getByLabel(/^Password for/).fill('e2e-pass')
  await prompt.getByRole('button', { name: 'Sign in' }).click()
}

test('ssh: trust the host key, sign in, run a command, reload', async ({ page }) => {
  test.setTimeout(90_000)
  rmSync(known, { force: true })
  await start(page, `ssh-term-${Date.now()}`)
  const problems = collectProblems(page)
  // A wrong password typed with the trust is refused once, then asked for again.
  const dialog = await addHost(page, 'Fake one', { password: 'wrong' })
  await expect(page.getByRole('alert').filter({ hasText: 'That password was refused. Try again.' })).toBeVisible()
  await signInWithPassword(page)
  await expect(page.getByRole('status').filter({ hasText: 'The host works.' })).toBeVisible({ timeout: 15_000 })
  await dialog.getByRole('button', { name: 'Done' }).click()
  await dialog.getByRole('button', { name: 'Open a terminal on Fake one' }).click()

  await expect(page.locator('[data-terminal] .xterm-helper-textarea')).toBeAttached({ timeout: 15_000 })
  // A tab on another computer says which one, with a marker.
  await expect(page.getByRole('tab', { name: /^SSH Fake one terminal$/ })).toBeVisible()
  await expect(page.getByRole('img', { name: 'SSH' })).toBeVisible()
  await type(page, 'echo $((20+22))')
  await expect.poll(() => text(page), { timeout: 15_000 }).toMatch(/\n42\b/)

  await page.reload()
  await expect(page.getByRole('tab', { name: /Fake one/ })).toBeVisible()
  await expect.poll(() => text(page), { timeout: 15_000 }).toMatch(/\n42\b/)
  await type(page, 'echo $((6*7+1))')
  await expect.poll(() => text(page), { timeout: 15_000 }).toMatch(/\n43\b/)
  expect(problems).toEqual([])
})

test('ssh: a changed host key is refused', async ({ page }) => {
  test.setTimeout(60_000)
  // A saved key for this address that is not the one the server has.
  const blob = Buffer.concat([Buffer.from([0, 0, 0, 11]), Buffer.from('ssh-ed25519'), Buffer.from([0, 0, 0, 32]), randomBytes(32)])
  writeFileSync(known, `[127.0.0.1]:${sshPort} ssh-ed25519 ${blob.toString('base64')}\n`)
  await start(page, `ssh-key-${Date.now()}`)
  const dialog = await addHost(page, 'Fake two', { password: null })
  await dialog.getByRole('button', { name: 'Open a terminal on Fake two' }).click()
  const problem = page.getByRole('dialog', { name: 'The host key changed' })
  await expect(problem).toContainText('different key')
  await expect(problem).toContainText('Saved key')
  await expect(problem).toContainText('SHA256:')
  await expect(problem.getByRole('button', { name: 'Forget the saved key' })).toBeVisible()
  await expect(page.getByRole('dialog', { name: 'Trust this host?' })).toHaveCount(0)
  await problem.getByRole('button', { name: 'Forget the saved key' }).click()
  await expect(problem).toContainText('The saved key is gone')
  await expect(problem.getByRole('button', { name: 'Connect again' })).toBeFocused()
  await problem.getByRole('button', { name: 'Connect again' }).click()
  const again = page.getByRole('dialog', { name: 'Trust this host?' })
  await expect(again).toContainText('SHA256:')
  // Prompts show in every window, so one left open would cover the other tests' pages.
  await again.getByRole('button', { name: "Don't connect" }).click()
  await expect(again).toHaveCount(0)
})

test('ssh: browse, edit, save, conflict, forward', async ({ page }) => {
  test.setTimeout(120_000)
  const dir = `e2e-${Date.now()}`
  mkdirSync(join(home, dir, 'sub'), { recursive: true })
  const file = join(home, dir, 'notes.txt')
  writeFileSync(file, 'first line\n')
  rmSync(known, { force: true })
  await start(page, `ssh-files-${Date.now()}`)
  const problems = collectProblems(page)
  const dialog = await addHost(page, 'Fake three')
  await dialog.getByRole('button', { name: 'Browse files on Fake three' }).click()

  const view = page.locator('[data-remote-view]')
  await expect(view).toBeVisible({ timeout: 15_000 })
  await view.getByRole('button', { name: `Open folder ${dir}` }).click()
  await expect(view.getByRole('button', { name: 'Open notes.txt' })).toBeVisible()
  await view.getByRole('button', { name: 'Open notes.txt' }).click()
  await expect(editor(page)).toContainText('first line')

  await editor(page).click()
  await page.keyboard.press('ControlOrMeta+End')
  await page.keyboard.type('added remotely')
  await page.keyboard.press('ControlOrMeta+s')
  await expect.poll(() => readFileSync(file, 'utf8')).toContain('added remotely')

  // Changed behind the editor's back: the save is refused and the disk keeps the other version.
  await page.keyboard.type(' mine')
  writeFileSync(file, 'theirs\n')
  await page.keyboard.press('ControlOrMeta+s')
  await expect(page.getByRole('button', { name: 'Overwrite the file on disk with my version' })).toBeVisible({ timeout: 10_000 })
  expect(readFileSync(file, 'utf8')).toBe('theirs\n')
  await page.getByRole('button', { name: 'Load the file from disk' }).click()
  await expect(editor(page)).toContainText('theirs')

  // A forward carries bytes from a server that only the host can reach.
  const web = createServer((_, res) => res.end('through the tunnel')).listen(0, '127.0.0.1')
  await new Promise((r) => web.once('listening', r))
  const port = (web.address() as { port: number }).port
  await page.keyboard.press('Alt+s')
  const hosts = page.getByRole('dialog')
  await hosts.getByRole('button', { name: 'Port forwards on Fake three' }).click()
  await hosts.getByLabel('Local port').fill('0')
  await hosts.getByLabel('Remote host').fill('127.0.0.1')
  await hosts.getByLabel('Remote port').fill(String(port))
  await hosts.getByRole('button', { name: 'Add forward' }).click()
  const addr = await hosts.getByTestId('forwards').locator('code').innerText()
  expect(addr).toMatch(/^127\.0\.0\.1:\d+$/)
  const body = await (await fetch(`http://${addr}/`)).text()
  expect(body).toBe('through the tunnel')
  await hosts.getByRole('button', { name: /^Stop forward/ }).click()
  await expect(hosts.getByTestId('forwards')).toHaveCount(0)
  web.close()
  // The refused save is a 409 the browser logs; anything else is a real problem.
  expect(problems.filter((m) => !/409 \(Conflict\)/.test(m))).toEqual([])
})
