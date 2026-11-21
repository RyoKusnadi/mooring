import { type ChildProcess, execFileSync, spawn } from 'node:child_process'
import { chmodSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { expect, test } from '@playwright/test'
import { dataDir, signinPort } from '../playwright.config'

// This test runs its own Mooring on another port and data folder, because it restarts the server.
// `mooring open` finds its browser through PATH, so a stand-in named open/xdg-open records the
// link instead of opening one.
const port = signinPort
const origin = `http://127.0.0.1:${port}`
const dir = resolve(dataDir, 'signin')
const bin = resolve(dir, 'bin')
const linkFile = resolve(dir, 'link.txt')
const mooring = resolve('../bin/mooring')

function start(): ChildProcess {
  return spawn(mooring, ['-addr', `127.0.0.1:${port}`, '-data', resolve(dir, 'data'), '-shell', resolve('e2e/shell.sh'), '-quiet'], {
    stdio: 'ignore',
  })
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
  throw new Error('the sign-in test server did not start')
}

async function stop(p: ChildProcess): Promise<void> {
  const exited = new Promise((r) => p.once('exit', r))
  p.kill('SIGTERM')
  await exited
}

test('sign in by link, restart the server, still signed in', async ({ browser }) => {
  rmSync(dir, { recursive: true, force: true })
  mkdirSync(bin, { recursive: true })
  for (const name of ['open', 'xdg-open']) {
    writeFileSync(resolve(bin, name), `#!/bin/sh\nprintf '%s' "$1" > "${linkFile}"\n`)
    chmodSync(resolve(bin, name), 0o755)
  }
  let server = start()
  try {
    await waitUp()
    execFileSync(mooring, ['open', '-data', resolve(dir, 'data')], {
      env: { ...process.env, PATH: `${bin}:${process.env.PATH}` },
    })
    // The opener runs after `mooring open` exits, so wait for the stand-in to write the link.
    await expect.poll(() => existsSync(linkFile) && readFileSync(linkFile, 'utf8')).toBeTruthy()
    const link = readFileSync(linkFile, 'utf8')
    expect(link).toMatch(new RegExp(`^${origin}/login\\?code=v1\\.`))

    const context = await browser.newContext()
    const page = await context.newPage()
    await page.goto(link)
    await expect(page.getByRole('heading', { name: 'Projects', exact: true })).toBeVisible()
    expect(page.url()).toBe(`${origin}/`)

    await stop(server)
    server = start()
    await waitUp()
    await page.reload()
    await expect(page.getByRole('heading', { name: 'Projects', exact: true })).toBeVisible()

    // The link works once: a second browser is told so.
    const other = await (await browser.newContext()).newPage()
    await other.goto(link)
    await expect(other.getByRole('heading', { name: 'Sign in to Mooring' })).toBeVisible()
    await expect(other.getByRole('status')).toContainText('already used or has expired')
    await context.close()
  } finally {
    server.kill('SIGTERM')
  }
})
