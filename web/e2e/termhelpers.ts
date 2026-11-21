import { mkdirSync } from 'node:fs'
import { join } from 'node:path'
import { expect, type Page } from '@playwright/test'
import { fixtures, signIn } from './helpers'

export interface Screen {
  rows: string[]
  cols: number
  height: number
  alt: boolean
  cursor: number[]
}

/** Adds a folder as a project, opens it, and starts a terminal in its first tab. */
export async function openTerminal(page: Page, name: string): Promise<string> {
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
  await page.getByRole('tabpanel').getByRole('button', { name: 'New terminal' }).click()
  await waitForPrompt(page)
  return folder
}

export const screen = (page: Page) =>
  page.evaluate(() => (window as unknown as { __mooringTest?: { screen(): Screen | null } }).__mooringTest?.screen() ?? null)

export async function waitForPrompt(page: Page) {
  await expect.poll(async () => (await screen(page))?.rows.some((r) => r.startsWith('$ ')), { timeout: 15_000 }).toBe(true)
}

export async function type(page: Page, line: string) {
  await page.locator('[data-terminal] .xterm-helper-textarea').focus()
  await page.keyboard.type(line + '\n')
}

export const text = async (page: Page) => (await screen(page))?.rows.join('\n') ?? ''
