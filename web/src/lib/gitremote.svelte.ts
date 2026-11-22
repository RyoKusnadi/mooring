// Fetch, pull and amend for the Changes view. The commands run through the git store, which keeps
// busy, failure and notice; this file only words the results.
import type { Status } from './gitapi'
import { git } from './gitstore.svelte'

const commits = (n: number) => `${n} ${n === 1 ? 'commit' : 'commits'}`

export function fetchNotice(upstream: string, added: number): string {
  if (!upstream) return 'Fetched. This branch has no upstream to compare with.'
  return added > 0 ? `Fetched. ${upstream} has ${added} new ${added === 1 ? 'commit' : 'commits'}.` : 'Already up to date.'
}

export function pullNotice(upstream: string, pulled: number): string {
  return pulled > 0 ? `Pulled ${commits(pulled)} from ${upstream}.` : 'Already up to date.'
}

/** A rejected push is answered with a Pull button, not with a command to type. */
export const offersPull = (f: { code: string } | null) => f?.code === 'PUSH_REJECTED'

export const pullLabel = (s: Status | null) => (s && s.behind > 0 ? `Pull ${commits(s.behind)}` : '')

/** The last commit is on a remote branch, so an amend would need a force push. */
export const headPushed = (s: Status | null) => !!s && s.head !== '' && s.pushed

export const amendBlocked = (s: Status | null) => (s ? `The last commit is already on ${s.upstream || 'a remote branch'}. Changing it would need a force push, which Mooring does not do.` : '')

export async function fetchRemote(): Promise<boolean> {
  const r = await git.act<{ upstream: string; new: number }>('git.fetch', {})
  if (!r) return false
  git.notice = fetchNotice(r.upstream, r.new)
  return true
}

export async function pullRemote(): Promise<boolean> {
  const r = await git.act<{ upstream: string; pulled: number }>('git.pull', {})
  if (!r) return false
  git.notice = pullNotice(r.upstream, r.pulled)
  return true
}
