import { expect, test, type Page } from '@playwright/test'
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { dataDir } from '../../playwright.docs.config'
import { openDemo, scrubLive, shot, signIn, token } from './helpers'

// Takes the pictures for docs/guide/images. Run with `make screenshots`; it is not part of `make check`.
// Stand-ins (see e2e/serve.sh): "gemini" is the stand-in agent, "fake-mcp" a stand-in tool server.
// The prompt word "ask" makes the stand-in agent ask for permission.

const phone = { width: 390, height: 844 }
const base = 'http://127.0.0.1:' + (process.env.MOORING_E2E_PORT || 7450)

async function openFile(page: Page, part: string) {
  await page.keyboard.press('Alt+o')
  const dialog = page.getByRole('dialog', { name: 'Find in project' })
  await dialog.getByLabel('File name').fill(part)
  await expect(dialog.getByRole('option').first()).toContainText(part)
  await page.keyboard.press('Enter')
  await expect(dialog).toBeHidden()
}

test('login page', async ({ page }) => {
  await openDemo(page)
  // Set a password so the login page shows its password form. The token box stays closed.
  await page.getByRole('navigation', { name: 'Projects' }).getByRole('button', { name: 'Settings', exact: true }).click()
  const settings = page.getByRole('dialog', { name: 'Settings' })
  await settings.getByRole('button', { name: 'Sign-in', exact: true }).click()
  // A second run on the same data folder already has a password.
  await settings.getByRole('button', { name: /^(Set|Change) password$/ }).waitFor()
  if ((await settings.getByRole('button', { name: 'Set password' }).count()) > 0) {
    await settings.getByLabel('Username').fill('ada')
    await settings.getByLabel('New password', { exact: true }).fill('demo-password-1')
    await settings.getByLabel('New password again').fill('demo-password-1')
    await settings.getByRole('button', { name: 'Set password' }).click()
    await expect(settings.getByText('Password set.')).toBeVisible()
  }
  const fresh = await page.context().browser()!.newContext({ viewport: { width: 1280, height: 800 }, colorScheme: 'light', baseURL: 'http://127.0.0.1:' + (process.env.MOORING_E2E_PORT || 7450) })
  const login = await fresh.newPage()
  await login.goto('/')
  await expect(login.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible()
  await shot(login, 'getting-started-login')
  await fresh.close()
  expect(token).not.toBe('')
})

test('phone drawer', async ({ browser }) => {
  const ctx = await browser.newContext({ viewport: phone, colorScheme: 'light', baseURL: 'http://127.0.0.1:' + (process.env.MOORING_E2E_PORT || 7450) })
  const page = await ctx.newPage()
  await openDemo(page)
  await page.getByRole('button', { name: 'Projects', exact: true }).first().click()
  await expect(page.getByRole('navigation', { name: 'Projects' })).toBeVisible()
  await shot(page, 'getting-started-phone-drawer')
  await ctx.close()
})

test('the screen', async ({ page }) => {
  await openDemo(page)
  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New terminal' }).click()
  await expect(page.locator('[data-terminal] .xterm-helper-textarea')).toBeAttached()
  await page.waitForTimeout(1500)
  await page.locator('[data-terminal] .xterm-helper-textarea').focus()
  await page.keyboard.type("sleep 2; printf '\\a'\n")
  await page.keyboard.press('Alt+Escape')
  await openFile(page, 'README')
  await page.waitForTimeout(3500)
  await page.keyboard.press('Alt+\\')
  await page.waitForTimeout(800)
  await shot(page, 'getting-started-screen')
})

test('add project', async ({ page }) => {
  await openDemo(page)
  await page.getByRole('button', { name: 'Add project' }).first().click()
  const dialog = page.getByRole('dialog')
  await dialog.getByRole('tab', { name: 'Clone a repository' }).or(dialog.getByRole('button', { name: 'Clone a repository' })).first().click()
  await dialog.getByLabel('Repository address').fill('https://github.com/example/demo-app.git')
  await shot(page, 'projects-add-project')
  await page.keyboard.press('Escape')
})

test('file row menu', async ({ page }) => {
  await openDemo(page)
  const tree = page.getByRole('tree')
  await tree.getByText('src', { exact: true }).click()
  await expect(tree.getByText('greet.ts', { exact: true })).toBeVisible()
  await tree.getByText('src', { exact: true }).click({ button: 'right' })
  await expect(page.getByRole('menu')).toBeVisible()
  await shot(page, 'projects-file-menu')
  await page.keyboard.press('Escape')
})

test('find text', async ({ page }) => {
  await openDemo(page)
  await page.keyboard.press('Alt+f')
  const dialog = page.getByRole('dialog', { name: 'Find in project' })
  await dialog.getByLabel('Text to find').fill('greet')
  await expect(dialog.getByText(/\d+ lines?/)).toBeVisible()
  await shot(page, 'projects-find-text')
  await page.keyboard.press('Escape')
})

test('tool server consent', async ({ page }) => {
  await openDemo(page)
  await page.keyboard.press('Alt+m')
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByRole('heading', { name: 'Tool servers' })).toBeVisible()
  await dialog.getByRole('button', { name: 'Add your own' }).click()
  await dialog.getByLabel('Name', { exact: true }).fill('Notes')
  await dialog.getByLabel('Command', { exact: true }).fill('fake-mcp')
  await dialog.getByRole('button', { name: 'Review' }).click()
  await expect(dialog.getByRole('heading', { name: 'Allow this tool server?' })).toBeVisible()
  await shot(page, 'agent-chats-tool-consent')
  await page.keyboard.press('Escape')
})

test('changes view', async ({ page }) => {
  await openDemo(page)
  await page.keyboard.press('Alt+g')
  const view = page.getByRole('tabpanel')
  await expect(view.getByText('Commit message')).toBeVisible()
  await view.getByRole('button', { name: /greet\.ts/ }).first().click()
  await page.waitForTimeout(800)
  await shot(page, 'git-changes')
})

// The chat tests come last: they leave chats in the sidebar that the other pictures should not show.
const prompt = 'Run the tests and fix the failing one'

async function say(page: Page, text: string) {
  await page.getByLabel('Message to Gemini').fill(text)
  await page.getByRole('button', { name: 'Send', exact: true }).click()
}

test('chat picker and permission', async ({ page }) => {
  await openDemo(page)
  const panel = page.getByRole('tabpanel')
  await panel.getByRole('button', { name: 'New chat' }).click()
  await expect(panel.getByRole('heading', { name: 'Start a chat' })).toBeVisible()
  await expect(panel.getByText('Not installed.').first()).toBeVisible()
  await shot(page, 'agent-chats-picker')
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await say(page, prompt)
  await expect(panel.getByRole('alert', { name: 'Permission request' })).toBeVisible()
  await shot(page, 'agent-chats-permission')
})

// The front-page picture: a terminal and an agent chat side by side, in both themes.
for (const scheme of ['light', 'dark'] as const) {
  test(`hero ${scheme}`, async ({ browser }) => {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 }, colorScheme: scheme, baseURL: base })
    const page = await ctx.newPage()
    await openDemo(page)
    await page.getByRole('tabpanel').getByRole('button', { name: 'New terminal' }).click()
    await expect(page.locator('[data-terminal] .xterm-helper-textarea')).toBeAttached()
    await page.waitForTimeout(1500)
    await page.locator('[data-terminal] .xterm-helper-textarea').focus()
    await page.keyboard.type('git status -sb && git log --oneline -3\n')
    await page.waitForTimeout(800)
    await page.keyboard.press('Alt+\\')
    // The split starts a second terminal. Put a chat in that pane and close the terminal.
    await page.keyboard.press('Alt+Escape')
    await page.keyboard.press('Alt+a')
    await page.getByRole('region', { name: 'Pane 2 of 2' }).getByRole('button', { name: 'Close Terminal' }).click()
    const chat = page.getByRole('tabpanel').last()
    await chat.getByRole('button', { name: 'Start chat with Gemini' }).click()
    await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
    await say(page, prompt)
    await chat.getByRole('alert', { name: 'Permission request' }).getByRole('button', { name: 'Allow' }).click()
    await expect(chat.getByText('it passes now.').first()).toBeVisible()
    await page.waitForTimeout(1200)
    await shot(page, scheme === 'dark' ? 'hero-dark' : 'hero-light')
    await ctx.close()
  })
}

// A short recording of one chat. Playwright's own video needs a separate ffmpeg download, so this
// takes pictures in a loop and writes a frame list with each frame's length; `make screenshots`
// turns that into a GIF.
test('demo recording', async ({ browser }) => {
  const size = { width: 1280, height: 800 }
  const first = await browser.newContext({ viewport: size, colorScheme: 'light', baseURL: base })
  await openDemo(await first.newPage())
  const state = await first.storageState()
  await first.close()
  const ctx = await browser.newContext({ storageState: state, viewport: size, colorScheme: 'light', baseURL: base })
  const page = await ctx.newPage()
  await page.goto('/')
  await scrubLive(page)
  const frames = join(dataDir, 'frames')
  mkdirSync(frames, { recursive: true })
  const list: { file: string; at: number }[] = []
  const begin = Date.now()
  let recording = true
  const loop = (async () => {
    for (let i = 0; recording; i++) {
      const file = `f${String(i).padStart(4, '0')}.jpg`
      const at = Date.now() - begin
      await page.screenshot({ path: join(frames, file), type: 'jpeg', quality: 90 })
      list.push({ file, at })
    }
  })()
  await page.getByRole('navigation', { name: 'Projects' }).getByText('demo-app', { exact: true }).first().click()
  const panel = page.getByRole('tabpanel')
  await page.waitForTimeout(1200)
  await panel.getByRole('button', { name: 'New chat' }).click()
  await page.waitForTimeout(1500)
  await panel.getByRole('button', { name: 'Start chat with Gemini' }).click()
  await expect(page.getByLabel('Message to Gemini')).toBeEnabled()
  await page.getByLabel('Message to Gemini').pressSequentially(prompt, { delay: 45 })
  await page.waitForTimeout(500)
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  const alert = panel.getByRole('alert', { name: 'Permission request' })
  await expect(alert).toBeVisible()
  await page.waitForTimeout(2200)
  await alert.getByRole('button', { name: 'Allow' }).click()
  await expect(panel.getByText('it passes now.').first()).toBeVisible()
  await page.waitForTimeout(2500)
  recording = false
  await loop
  const lines = list.map((f, i) => `file '${f.file}'\nduration ${(((list[i + 1]?.at ?? f.at + 500) - f.at) / 1000).toFixed(3)}`)
  writeFileSync(join(frames, 'list.txt'), lines.join('\n') + `\nfile '${list[list.length - 1].file}'\n`)
  await ctx.close()
})

