// The username and password for signing in, set from Settings. The commands run over the events
// socket and are never stored by the server, so a password is only ever in these form fields.
import type { Fetcher } from './api'
import { getJSON } from './filesapi'

export const MIN_PASSWORD = 8
export const MAX_USERNAME = 64

export interface PasswordStatus {
  set: boolean
  username: string
}

const browserFetch: Fetcher = (input, init) => fetch(input, init)

export const fetchPasswordStatus = (f: Fetcher = browserFetch) => getJSON<PasswordStatus>('/api/password', f)

/** What is wrong with a new username and password, or '' when they can be saved. */
export function checkNew(username: string, password: string, again: string): string {
  const name = username.trim()
  if (name === '') return 'Enter a username.'
  if ([...name].length > MAX_USERNAME) return `The username can be at most ${MAX_USERNAME} characters.`
  if ([...password].length < MIN_PASSWORD) return `The password needs at least ${MIN_PASSWORD} characters.`
  if (password !== again) return 'The two passwords are not the same.'
  return ''
}

/** Makes a new login token on the server: every other browser is signed out, this one gets a fresh cookie. */
export async function signOutOthers(current: string, f: Fetcher = browserFetch): Promise<string> {
  const res = await f('/api/password/sign-out-others', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ current }),
  })
  if (res.status === 204) return ''
  if (res.status === 401) return 'The current password is not right.'
  if (res.status === 429) return `Too many wrong attempts. Try again in ${Number(res.headers.get('Retry-After')) || 60} seconds.`
  return 'Could not sign out the other browsers. Try again.'
}
