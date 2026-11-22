import { execFileSync } from 'node:child_process'
import { existsSync, mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, type Page } from '@playwright/test'
import { dataDir, homeDir } from '../../playwright.docs.config'

export const token = execFileSync('../bin/mooring', ['token', '-data', dataDir], { encoding: 'utf8' }).trim()
export const imagesDir = join('..', 'docs', 'guide', 'images')
mkdirSync(imagesDir, { recursive: true })

const demoPath = join(homeDir, 'work', 'demo-app')
const git = (...args: string[]) => execFileSync('git', args, { cwd: demoPath, encoding: 'utf8' })

const FILES: Record<string, string> = {
  'README.md': '# Demo app\n\nA small web app used to show Mooring.\n',
  'package.json': '{\n  "name": "demo-app",\n  "version": "1.0.0",\n  "scripts": { "test": "vitest run" }\n}\n',
  'src/main.ts': "import { greet } from './greet'\n\nconsole.log(greet('world'))\n",
  'src/greet.ts': 'export function greet(name: string): string {\n  return `Hello, ${name}!`\n}\n',
  'src/util.ts': 'export const clamp = (n: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, n))\n',
  'src/greet.test.ts': "import { greet } from './greet'\n\ntest('greets', () => expect(greet('Ada')).toBe('Hello, Ada!'))\n",
}

/** A small repository on branch main, then a staged edit, an unstaged edit and a new file. Safe to call twice. */
export function makeDemo(): string {
  mkdirSync(join(demoPath, 'src'), { recursive: true })
  if (!existsSync(join(demoPath, '.git'))) {
    git('init', '-q')
    git('symbolic-ref', 'HEAD', 'refs/heads/main')
    for (const [k, v] of Object.entries({ 'user.name': 'Ada Demo', 'user.email': 'ada@example.com', 'commit.gpgsign': 'false' })) git('config', k, v)
    for (const [p, t] of Object.entries(FILES)) writeFileSync(join(demoPath, p), t)
    git('add', '-A')
    git('commit', '-q', '-m', 'Add the greeting')
    writeFileSync(join(demoPath, 'src/greet.ts'), 'export function greet(name: string): string {\n  return `Hello, ${name}! Welcome back.`\n}\n')
    git('add', 'src/greet.ts')
    writeFileSync(join(demoPath, 'src/main.ts'), "import { greet } from './greet'\nimport { clamp } from './util'\n\nconsole.log(greet('world'), clamp(12, 0, 10))\n")
    writeFileSync(join(demoPath, 'NOTES.md'), 'Ideas\n\n- add a farewell\n')
  }
  return demoPath
}

/** Shows a stand-in home folder as /Users/demo, in text and in fields. Call it after the page settles. */
export async function scrub(page: Page) {
  await page.evaluate((home) => {
    const show = '/Users/demo'
    const w = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT)
    for (let n = w.nextNode(); n; n = w.nextNode()) if (n.nodeValue?.includes(home)) n.nodeValue = n.nodeValue.split(home).join(show)
    for (const el of document.querySelectorAll<HTMLInputElement>('input, textarea')) if (el.value.includes(home)) el.value = el.value.split(home).join(show)
    for (const el of document.querySelectorAll<HTMLElement>('[title]')) if (el.title.includes(home)) el.title = el.title.split(home).join(show)
  }, homeDir)
}

/** Keeps the scrub running while a video records. */
export async function scrubLive(page: Page) {
  await page.evaluate((home) => {
    setInterval(() => {
      const w = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT)
      for (let n = w.nextNode(); n; n = w.nextNode()) if (n.nodeValue?.includes(home)) n.nodeValue = n.nodeValue.split(home).join('/Users/demo')
    }, 50)
  }, homeDir)
}

export async function shot(page: Page, name: string) {
  await page.waitForTimeout(600)
  await scrub(page)
  await page.screenshot({ path: join(imagesDir, `${name}.png`) })
}

export async function signIn(page: Page) {
  await page.goto('/')
  await page.getByText('Use the login token instead').click()
  await page.getByLabel('Login token').fill(token)
  await page.getByRole('button', { name: 'Sign in with token' }).click()
  await expect(page.getByRole('heading', { name: 'Projects', exact: true }).or(page.getByRole('button', { name: 'Projects', exact: true }))).toBeVisible()
}

/** Signs in and opens the demo project, adding it the first time. */
export async function openDemo(page: Page) {
  const folder = makeDemo()
  await signIn(page)
  if ((page.viewportSize()?.width ?? 1280) <= 900) {
    await page.getByRole('button', { name: 'Projects', exact: true }).first().click()
    await page.getByRole('navigation', { name: 'Projects' }).getByText('demo-app', { exact: true }).click()
    return
  }
  const row = page.getByRole('navigation', { name: 'Projects' }).getByText('demo-app', { exact: true })
  await row.first().waitFor({ timeout: 2500 }).catch(() => undefined)
  if ((await row.count()) === 0) {
    await page.getByRole('button', { name: 'Add project' }).first().click()
    const dialog = page.getByRole('dialog')
    await dialog.getByLabel('Folder on this computer').fill(folder)
    await dialog.getByRole('button', { name: 'Go', exact: true }).click()
    await dialog.getByRole('button', { name: 'Open this folder' }).click()
    await expect(dialog).toBeHidden()
  } else {
    await row.click()
  }
  await expect(page.getByRole('navigation', { name: 'Projects' }).getByText('demo-app').first()).toBeVisible()
  await emptyPanes(page)
  await clearChats(page)
}

/** Deletes the chats an earlier picture left in the sidebar. */
export async function clearChats(page: Page) {
  const del = page.getByRole('button', { name: /^Delete chat / })
  for (let i = 0; i < 10 && (await del.count()) > 0; i++) {
    await del.first().click()
    await page.getByRole('button', { name: 'Delete', exact: true }).click()
    await page.waitForTimeout(300)
  }
}

/** Closes every tab of the open project, so a picture starts from an empty pane. */
export async function emptyPanes(page: Page) {
  await page.waitForTimeout(800)
  for (let i = 0; i < 12 && (await page.getByRole('tab').count()) > 0; i++) {
    await page.keyboard.press('Alt+Escape')
    await page.keyboard.press('Alt+w')
    await page.waitForTimeout(200)
  }
}
