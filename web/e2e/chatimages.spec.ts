import { mkdirSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Locator, type Page } from '@playwright/test'
import { collectProblems, fixtures, signIn } from './helpers'
import { openProject } from './githelpers'
import { stopChats } from './stopchats'

// Images in agent chats. The stand-in "gemini" takes images (FAKE_CAPS has "image", see serve.sh) and
// answers "got N image(s)"; the stand-in "grok" takes none. Other files for the picture are in images.spec.ts.

// A 2 x 2 PNG.
const PNG = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAABytg0kAAAAFElEQVR4nGP8z8Dwn4EIwESMolGF5CkEAHpyAgeX0iDLAAAAAElFTkSuQmCC', 'base64')
const png = (name: string) => ({ name, mimeType: 'image/png', buffer: PNG })

const used: string[] = []
test.afterEach(async ({ page }) => {
  if (!used.length) return
  // A chat tab that is still shown starts a stopped chat again, so leave the app first.
  await page.goto('/theme.js')
  await stopChats(page, used.splice(0))
})

async function startChat(page: Page, agent: 'Gemini' | 'Grok') {
  const name = `chatimg-${Date.now() % 1_000_000}-${Math.random().toString(36).slice(2, 6)}`
  const folder = join(fixtures, name)
  mkdirSync(folder, { recursive: true })
  used.push(name)
  await openProject(page, folder)
  await page.getByRole('button', { name: 'New tab' }).click()
  await page.getByRole('menuitem', { name: 'Chat' }).click()
  await page.getByRole('button', { name: `Start chat with ${agent}` }).click()
  const box = page.getByLabel(`Message to ${agent}`)
  await expect(box).toBeEnabled()
  return box
}

/** Pastes or drops a PNG on the box the way a browser does: an event that carries a file. */
async function fire(target: Locator, kind: 'paste' | 'dragenter' | 'drop') {
  await target.evaluate(
    (el, { kind, b64 }) => {
      const dt = new DataTransfer()
      dt.items.add(new File([Uint8Array.from(atob(b64), (c) => c.charCodeAt(0))], 'pasted.png', { type: 'image/png' }))
      const init = { bubbles: true, cancelable: true }
      el.dispatchEvent(kind === 'paste' ? new ClipboardEvent('paste', { ...init, clipboardData: dt }) : new DragEvent(kind, { ...init, dataTransfer: dt }))
    },
    { kind, b64: PNG.toString('base64') },
  )
}

async function loaded(img: Locator) {
  await expect(img).toBeVisible()
  await expect.poll(() => img.evaluate((e: HTMLImageElement) => e.naturalWidth)).toBeGreaterThan(0)
}

test('chat images: attach, chip, send, thumbnail, viewer, and again after a reload', async ({ page }) => {
  test.setTimeout(120_000)
  await signIn(page)
  const problems = collectProblems(page)
  const box = await startChat(page, 'Gemini')
  const panel = page.getByRole('tabpanel')
  const field = panel.locator('.field')
  await expect(panel.getByText('You can also paste an image')).toBeVisible()

  // The button opens the file picker; a chip shows the thumbnail, the name and the size, and × removes it.
  const attachButton = panel.getByRole('button', { name: 'Attach image' })
  await expect(attachButton).toBeEnabled()
  const chooser = page.waitForEvent('filechooser')
  await attachButton.click()
  await (await chooser).setFiles(png('shot.png'))
  const chips = panel.getByRole('list', { name: 'Attached', exact: true })
  await expect(chips.getByText('shot.png')).toBeVisible()
  await expect(chips).toContainText('· 1 KB')
  await loaded(chips.locator('img'))
  await chips.getByRole('button', { name: 'Remove shot.png' }).click()
  await expect(chips).toHaveCount(0)

  // Paste and drop attach too; a drag shows the drop edge and its words.
  await fire(box, 'paste')
  await expect(chips.getByText('pasted.png')).toBeVisible()
  await fire(field, 'dragenter')
  await expect(panel.getByText('Drop images to attach')).toBeVisible()
  await fire(field, 'drop')
  await expect(panel.getByText('Drop images to attach')).toBeHidden()
  await expect(chips.getByRole('listitem')).toHaveCount(2)

  // Send: the chip row empties, the sent turn shows a thumbnail with the name, and the agent got the image.
  await box.fill('what is in these?')
  await panel.getByRole('button', { name: 'Send', exact: true }).click()
  const sent = panel.getByRole('list', { name: 'Attached images' })
  await expect(sent.getByRole('listitem')).toHaveCount(2)
  await expect(panel.getByText('got 2 image(s)', { exact: true })).toBeVisible()
  await expect(chips).toHaveCount(0)
  await loaded(sent.locator('img').first())

  // The viewer: a dialog with the name and Close; Esc closes it and focus goes back to the thumbnail.
  const open = sent.getByRole('button', { name: 'View pasted.png' }).first()
  await open.click()
  const viewer = page.getByRole('dialog')
  await expect(viewer.getByRole('heading', { name: /^pasted\.png · / })).toBeVisible()
  await loaded(viewer.locator('img'))
  await page.keyboard.press('Escape')
  await expect(viewer).toBeHidden()
  await expect(open).toBeFocused()
  await open.click()
  await viewer.getByRole('button', { name: /^Close/ }).click()
  await expect(viewer).toBeHidden()

  // The thumbnails come back from the stored files after a reload.
  await page.reload()
  const again = page.getByRole('list', { name: 'Attached images' })
  await loaded(again.locator('img').first())
  expect(problems).toEqual([])
})

test('chat images: refusals are inline, the good files stay, and eight is the most', async ({ page }) => {
  test.setTimeout(120_000)
  await signIn(page)
  const problems = collectProblems(page)
  await startChat(page, 'Gemini')
  const panel = page.getByRole('tabpanel')
  const picker = panel.locator('input[type="file"]')
  const chips = panel.getByRole('list', { name: 'Attached', exact: true })

  await picker.setInputFiles([png('ok.png'), { name: 'notes.pdf', mimeType: 'application/pdf', buffer: Buffer.from('%PDF-1.4') }])
  await expect(panel.getByRole('alert').filter({ hasText: 'notes.pdf is not an image. Attach PNG, JPEG, GIF or WebP.' })).toBeVisible()
  await expect(chips.getByText('ok.png')).toBeVisible()

  await picker.setInputFiles({ name: 'huge.png', mimeType: 'image/png', buffer: Buffer.alloc(6 * 1024 * 1024) })
  await expect(panel.getByRole('alert').filter({ hasText: 'huge.png is 6.0 MB. Images can be up to 5 MB.' })).toBeVisible()
  await expect(chips.getByRole('listitem')).toHaveCount(1)

  // A file that says "png" but is not one passes the browser's check and is refused by the server.
  await picker.setInputFiles({ name: 'fake.png', mimeType: 'image/png', buffer: Buffer.from('not really a picture') })
  await expect(panel.getByRole('alert').filter({ hasText: 'fake.png is not an image. Attach PNG, JPEG, GIF or WebP.' })).toBeVisible()

  await picker.setInputFiles(Array.from({ length: 8 }, (_, i) => png(`n${i}.png`)))
  await expect(panel.getByRole('alert').filter({ hasText: 'A message takes at most 8 attachments. Remove one first.' })).toBeVisible()
  await expect(chips.getByRole('listitem')).toHaveCount(8)
  // Removing a chip clears the problem line, the red edge and the alert.
  await chips.getByRole('button', { name: /^Remove / }).first().click()
  await expect(panel.getByRole('alert')).toHaveCount(0)
  await expect(panel.locator('.field.invalid')).toHaveCount(0)
  expect(problems.filter((p) => !/415|Unsupported Media Type|Failed to load resource/.test(p))).toEqual([])
})

test('chat images: an agent that takes none disables the button and says why', async ({ page }) => {
  test.setTimeout(120_000)
  await signIn(page)
  const problems = collectProblems(page)
  const box = await startChat(page, 'Grok')
  const panel = page.getByRole('tabpanel')
  const note = 'Grok does not take images. Start a chat with an agent that does, such as Claude.'

  await expect(panel.getByRole('button', { name: 'Attach image' })).toBeDisabled()
  await expect(panel.getByText(note)).toBeVisible()
  await fire(box, 'paste')
  await expect(panel.getByRole('alert').filter({ hasText: note })).toBeVisible()
  await expect(panel.getByRole('list', { name: 'Attached', exact: true })).toHaveCount(0)
  expect(problems).toEqual([])
})
