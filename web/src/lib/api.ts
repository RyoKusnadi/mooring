// Calls to the Mooring server's login endpoints. The session lives in an
// HttpOnly cookie, so the token is sent once at sign-in and never stored here.

export type Fetcher = (input: string, init?: RequestInit) => Promise<Response>

export type SessionState = 'signed-in' | 'signed-out'

export type LoginResult =
  | { ok: true }
  | { ok: false; reason: 'wrong-token' | 'wrong-credentials' | 'password-file' }
  | { ok: false; reason: 'too-many-attempts'; retryAfterSeconds: number }

const browserFetch: Fetcher = (input, init) => fetch(input, init)

export class ServerError extends Error {
  constructor(readonly status: number) {
    super(`Mooring server answered ${status}`)
  }
}

export async function checkSession(fetcher: Fetcher = browserFetch): Promise<SessionState> {
  const res = await fetcher('/api/session')
  if (res.status === 204) return 'signed-in'
  if (res.status === 401) return 'signed-out'
  throw new ServerError(res.status)
}

async function postLogin(body: object, wrong: 'wrong-token' | 'wrong-credentials', fetcher: Fetcher): Promise<LoginResult> {
  const res = await fetcher('/api/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (res.status === 204) return { ok: true }
  if (res.status === 401) {
    const answer = (await res.json().catch(() => null)) as { error?: string } | null
    return { ok: false, reason: answer?.error === 'password_file_unreadable' ? 'password-file' : wrong }
  }
  if (res.status === 429) {
    const seconds = Number(res.headers.get('Retry-After'))
    return { ok: false, reason: 'too-many-attempts', retryAfterSeconds: seconds > 0 ? seconds : 60 }
  }
  throw new ServerError(res.status)
}

export const login = (token: string, fetcher: Fetcher = browserFetch) => postLogin({ token: token.trim() }, 'wrong-token', fetcher)

/** Signs in with the username and password set in Settings. The password is sent as typed. */
export const loginWithPassword = (username: string, password: string, fetcher: Fetcher = browserFetch) =>
  postLogin({ username: username.trim(), password }, 'wrong-credentials', fetcher)

/** Whether a username and password exist, so the login page knows which form to show. */
export async function passwordIsSet(fetcher: Fetcher = browserFetch): Promise<boolean> {
  const res = await fetcher('/api/login/state')
  if (!res.ok) throw new ServerError(res.status)
  return ((await res.json()) as { passwordSet: boolean }).passwordSet === true
}

export async function logout(fetcher: Fetcher = browserFetch): Promise<void> {
  const res = await fetcher('/api/logout', { method: 'POST' })
  if (res.status !== 204) throw new ServerError(res.status)
}
