import { writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import { git, makeRepo, openProject } from './githelpers'
import { collectProblems, signIn } from './helpers'

// A 2 x 2 PNG and a 1 x 1 PNG.
const TWO = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAABytg0kAAAAFElEQVR4nGP8z8Dwn4EIwESMolGF5CkEAHpyAgeX0iDLAAAAAElFTkSuQmCC', 'base64')
const ONE = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==', 'base64')

const loaded = (el: HTMLImageElement) => el.complete && el.naturalWidth > 0

test('git: a changed picture shows Before and After, and a new one has no Before', async ({ page }) => {
  const run = `${Date.now()}-${test.info().repeatEachIndex}`
  const repo = makeRepo(`imgdiff-${run}`, { 'dot.png': 'placeholder' })
  writeFileSync(join(repo, 'dot.png'), TWO)
  git(repo, 'add', '-A')
  git(repo, 'commit', '-q', '-m', 'add picture')
  writeFileSync(join(repo, 'dot.png'), ONE)
  writeFileSync(join(repo, 'fresh.png'), TWO)
  await signIn(page)
  const problems = collectProblems(page)
  await openProject(page, repo)
  await page.keyboard.press('Alt+G')
  const changes = page.locator('section[aria-label^="Changes in"]')
  const diff = page.getByRole('tabpanel')

  await changes.getByRole('button', { name: 'dot.png, modified. Show changes' }).first().click()
  const before = diff.getByRole('img', { name: /^Before: dot\.png/ })
  const after = diff.getByRole('img', { name: /^After: dot\.png/ })
  await expect(before).toBeVisible()
  await expect(after).toBeVisible()
  expect(await before.evaluate(loaded)).toBe(true)
  expect(await before.evaluate((e: HTMLImageElement) => e.naturalWidth)).toBe(2)
  expect(await after.evaluate((e: HTMLImageElement) => e.naturalWidth)).toBe(1)
  await expect(diff.getByText('Stage it to include it in a commit.')).toBeVisible()

  // A new file has no earlier version.
  await changes.getByRole('button', { name: /^fresh\.png, .*Show changes/ }).first().click()
  await expect(diff.getByText('Not in this version')).toHaveCount(1)
  await expect(diff.getByRole('img', { name: /^After: fresh\.png/ })).toBeVisible()

  // On a phone the page does not scroll sideways.
  await page.setViewportSize({ width: 390, height: 844 })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true)
  expect(problems.filter((m) => !/status of 404/.test(m))).toEqual([])
})
