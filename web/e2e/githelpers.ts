import { execFileSync } from 'node:child_process'
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, type Page } from '@playwright/test'
import { fixtures } from './helpers'

export const git = (cwd: string, ...args: string[]) => execFileSync('git', args, { cwd, encoding: 'utf8', maxBuffer: 1 << 28 })

/** A repository on branch main with one commit holding `files`, in a folder the server may open. */
export function makeRepo(name: string, files: Record<string, string>) {
  const repo = join(fixtures, name)
  mkdirSync(repo, { recursive: true })
  git(repo, 'init', '-q')
  git(repo, 'symbolic-ref', 'HEAD', 'refs/heads/main')
  for (const [k, v] of Object.entries({ 'user.name': 'Test User', 'user.email': 'test@example.com', 'commit.gpgsign': 'false', 'core.autocrlf': 'false' })) git(repo, 'config', k, v)
  for (const [path, text] of Object.entries(files)) writeFileSync(join(repo, path), text)
  git(repo, 'add', '-A')
  git(repo, 'commit', '-q', '-m', 'first')
  return repo
}

export async function openProject(page: Page, folder: string) {
  await page.getByRole('button', { name: 'Add project' }).first().click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Folder on this computer').fill(folder)
  await dialog.getByRole('button', { name: 'Go', exact: true }).click()
  await dialog.getByRole('button', { name: 'Open this folder' }).click()
  await expect(dialog).toBeHidden()
}

/**
 * A repository of `count` commits written with git fast-import: main has a merge of a side commit every
 * ten commits, so the graph has two lanes most of the way. Subjects are "commit N" and "side N".
 */
export function makeBigRepo(name: string, count: number) {
  const repo = join(fixtures, name)
  mkdirSync(repo, { recursive: true })
  git(repo, 'init', '-q')
  git(repo, 'symbolic-ref', 'HEAD', 'refs/heads/main')
  const out: string[] = []
  const commit = (ref: string, mark: number, subject: string, ts: number, from: number | null, merge: number | null) => {
    out.push(`commit ${ref}`, `mark :${mark}`, `committer Big Repo <big@example.com> ${ts} +0000`, `data ${Buffer.byteLength(subject)}`, subject)
    if (from !== null) out.push(`from :${from}`)
    if (merge !== null) out.push(`merge :${merge}`)
    const body = String(mark)
    out.push('M 644 inline a.txt', `data ${body.length}`, body, '')
  }
  const T = 1_700_000_000
  let side: number | null = null
  for (let i = 1; i <= count; i++) {
    commit('refs/heads/main', i, `commit ${i}`, T + i * 60, i > 1 ? i - 1 : null, side)
    side = null
    if (i % 10 === 0) {
      commit('refs/heads/side', 1_000_000 + i, `side ${i}`, T + i * 60 + 1, i, null)
      side = 1_000_000 + i
    }
  }
  execFileSync('git', ['fast-import', '--quiet'], { cwd: repo, input: out.join('\n') + '\n', maxBuffer: 1 << 28 })
  git(repo, 'reset', '-q', '--hard', 'main')
  for (const [k, v] of Object.entries({ 'user.name': 'Test User', 'user.email': 'test@example.com', 'commit.gpgsign': 'false' })) git(repo, 'config', k, v)
  return repo
}
