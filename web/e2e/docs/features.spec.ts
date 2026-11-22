import { execFileSync } from 'node:child_process'
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { deflateSync } from 'node:zlib'
import { expect, test, type Page } from '@playwright/test'
import { homeDir } from '../../playwright.docs.config'
import { imagesDir, openDemo, scrub, shot } from './helpers'

// Pictures of the newer features for README.md and docs/guide. Run on its own (see the README of
// e2e/docs): `node node_modules/@playwright/test/cli.js test -c playwright.docs.config.ts e2e/docs/features.spec.ts`.
// Stand-ins: "gemini" and "grok" are stand-in agents; the prompt word "ask" asks for permission and
// "long" keeps the agent working. Other prompts are echoed back.

const git = (cwd: string, ...args: string[]) => execFileSync('git', args, { cwd, encoding: 'utf8' })

/** A small repository under the stand-in home folder, so the folder picker can open it. */
function repoAt(name: string, files: Record<string, string | Buffer>) {
  const dir = join(homeDir, 'work', name)
  mkdirSync(dir, { recursive: true })
  git(dir, 'init', '-q')
  git(dir, 'symbolic-ref', 'HEAD', 'refs/heads/main')
  for (const [k, v] of Object.entries({ 'user.name': 'Ada Demo', 'user.email': 'ada@example.com', 'commit.gpgsign': 'false' })) git(dir, 'config', k, v)
  for (const [p, t] of Object.entries(files)) writeFileSync(join(dir, p), t)
  git(dir, 'add', '-A')
  git(dir, 'commit', '-q', '-m', 'First commit')
  return dir
}

async function addProject(page: Page, dir: string) {
  await page.getByRole('button', { name: 'Add project' }).first().click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Folder on this computer').fill(dir)
  await dialog.getByRole('button', { name: 'Go', exact: true }).click()
  await dialog.getByRole('button', { name: 'Open this folder' }).click()
  await expect(dialog).toBeHidden()
}

async function startChat(page: Page, agent: 'Gemini' | 'Grok', text: string) {
  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New chat' }).click()
  await panel.getByRole('button', { name: `Start chat with ${agent}` }).click()
  await expect(page.getByLabel(`Message to ${agent}`)).toBeEnabled()
  await page.getByLabel(`Message to ${agent}`).fill(text)
  await page.getByRole('button', { name: 'Send', exact: true }).click()
}

test('status board', async ({ page }) => {
  await openDemo(page)
  const panel = page.getByRole('tabpanel')
  await startChat(page, 'Gemini', 'ask')
  await expect(panel.getByRole('alert', { name: 'Permission request' })).toBeVisible()
  // A second chat in a new tab starts a long answer, then the tab is left for a terminal while it is still working.
  await page.getByRole('button', { name: 'New tab' }).click()
  await page.getByRole('menuitem', { name: 'Chat' }).click()
  await panel.getByRole('button', { name: 'Start chat with Grok' }).click()
  await expect(page.getByLabel('Message to Grok')).toBeEnabled()
  await page.getByLabel('Message to Grok').fill('long')
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect(async () => {
    await page.getByRole('button', { name: 'New tab' }).click()
    await page.getByRole('menuitem', { name: 'Terminal' }).click({ timeout: 2_000 })
  }).toPass({ timeout: 20_000 })
  const side = page.getByRole('navigation', { name: 'Projects' })
  await side.getByRole('button', { name: /^Board/ }).click()
  const board = side.getByRole('region', { name: 'Status board' })
  await expect(board.getByRole('region', { name: /^Waiting for you/ })).toBeVisible()
  await expect(board.getByRole('region', { name: /^Running/ })).toBeVisible()
  // The sidebar is the point; the rest of the window is an empty terminal.
  await page.waitForTimeout(600)
  await scrub(page)
  await page.screenshot({ path: join(imagesDir, 'agent-chats-board.png'), clip: { x: 0, y: 0, width: 400, height: 440 } })
})

test('chat history search', async ({ page }) => {
  await openDemo(page)
  await startChat(page, 'Gemini', 'Add a farewell to greet.ts and a test for it')
  await expect(page.getByRole('tabpanel').getByText('farewell').first()).toBeVisible()
  await page.keyboard.press('Alt+Escape')
  await page.keyboard.press('Alt+y')
  const dialog = page.getByRole('dialog', { name: 'Chat history' })
  await dialog.getByLabel('Search all chats in every project').fill('farewell')
  await expect(dialog.locator('mark').first()).toBeVisible()
  await shot(page, 'agent-chats-history')
})

test('merge conflicts', async ({ page }) => {
  const dir = repoAt('greeter', { 'greet.ts': 'export const greet = () => "Hello"\n', 'README.md': '# Greeter\n\nSays hello.\n' })
  git(dir, 'checkout', '-q', '-b', 'friendly')
  writeFileSync(join(dir, 'greet.ts'), 'export const greet = () => "Hello, friend"\n')
  writeFileSync(join(dir, 'README.md'), '# Greeter\n\nSays hello to a friend.\n')
  git(dir, 'commit', '-q', '-am', 'Greet friends')
  git(dir, 'checkout', '-q', 'main')
  writeFileSync(join(dir, 'greet.ts'), 'export const greet = () => "Hello, world"\n')
  writeFileSync(join(dir, 'README.md'), '# Greeter\n\nSays hello to the world.\n')
  git(dir, 'commit', '-q', '-am', 'Greet the world')
  try {
    git(dir, 'merge', 'friendly')
  } catch {
    // The merge stops on purpose.
  }
  await openDemo(page)
  await addProject(page, dir)
  await page.keyboard.press('Alt+g')
  const changes = page.locator('section[aria-label^="Changes in"]')
  await expect(changes.getByRole('group', { name: 'Merge in progress.' })).toBeVisible()
  await expect(changes.getByRole('group', { name: 'Resolve greet.ts' })).toBeVisible()
  await changes.getByRole('button', { name: 'greet.ts, conflict. Show changes' }).click()
  await expect(page.getByText('This file has merge conflicts')).toBeVisible()
  await shot(page, 'git-conflicts')
})

// A colour block as a PNG, made here so no picture file has to be kept.
function png(w: number, h: number, pixel: (x: number, y: number) => [number, number, number]) {
  const raw = Buffer.alloc((w * 3 + 1) * h)
  for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) raw.set(pixel(x, y), y * (w * 3 + 1) + 1 + x * 3)
  const table = Array.from({ length: 256 }, (_, n) => {
    for (let k = 0; k < 8; k++) n = n & 1 ? 0xedb88320 ^ (n >>> 1) : n >>> 1
    return n >>> 0
  })
  const crc = (b: Buffer) => {
    let c = 0xffffffff
    for (const v of b) c = table[(c ^ v) & 255] ^ (c >>> 8)
    return (c ^ 0xffffffff) >>> 0
  }
  const chunk = (type: string, data: Buffer) => {
    const body = Buffer.concat([Buffer.from(type), data])
    const out = Buffer.alloc(body.length + 8)
    out.writeUInt32BE(data.length, 0)
    body.copy(out, 4)
    out.writeUInt32BE(crc(body), body.length + 4)
    return out
  }
  const head = Buffer.alloc(13)
  head.writeUInt32BE(w, 0)
  head.writeUInt32BE(h, 4)
  head.set([8, 2, 0, 0, 0], 8)
  return Buffer.concat([Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]), chunk('IHDR', head), chunk('IDAT', deflateSync(raw)), chunk('IEND', Buffer.alloc(0))])
}

const logo = (accent: [number, number, number]) => png(240, 150, (x, y) => (x > 70 && x < 170 && y > 40 && y < 110 ? accent : [30 + (x * 60) / 240, 80 + (y * 80) / 150, 190]))

test('image diff and preview', async ({ page }) => {
  const dir = repoAt('brand-kit', {
    'logo.png': logo([240, 240, 240]),
    'README.md': '# Brand kit\n\nLogos and colours for the demo app.\n',
  })
  writeFileSync(join(dir, 'logo.png'), logo([235, 130, 40]))
  await openDemo(page)
  await addProject(page, dir)
  await page.keyboard.press('Alt+g')
  const changes = page.locator('section[aria-label^="Changes in"]')
  await changes.getByRole('button', { name: 'logo.png, modified. Show changes' }).first().click()
  const diff = page.getByRole('tabpanel')
  await expect(diff.getByRole('img', { name: /^After: logo\.png/ })).toBeVisible()
  await expect(diff.getByRole('img', { name: /^Before: logo\.png/ })).toBeVisible()
  await expect(diff.getByText('Loading…')).toHaveCount(0)
  await shot(page, 'git-image-diff')

  writeFileSync(
    join(dir, 'README.md'),
    '# Brand kit\n\nLogos and colours for the demo app.\n\n## Colours\n\n| Name | Hex |\n| --- | --- |\n| Sea | `#1e50be` |\n| Sun | `#eb8228` |\n\n## Use\n\n- Keep clear space around the logo.\n- Use the orange mark on light backgrounds only.\n\n> Ask before changing the logo.\n',
  )
  await page.keyboard.press('Alt+o')
  const find = page.getByRole('dialog', { name: 'Find in project' })
  await find.getByLabel('File name').fill('README')
  await expect(find.getByRole('option').first()).toContainText('README')
  await page.keyboard.press('Enter')
  await expect(find).toBeHidden()
  await page.getByRole('group', { name: 'View' }).getByRole('button', { name: 'Preview' }).click()
  await expect(page.getByRole('heading', { name: 'Colours' })).toBeVisible()
  await shot(page, 'files-preview')
})
