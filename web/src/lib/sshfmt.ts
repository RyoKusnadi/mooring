// Small pure helpers for the SSH screens: labels, form checks, and reading the server's host key message.
import type { Auth, ConfigHost, Host, HostDraft } from './sshapi'

export const AUTH_LABEL: Record<Auth, string> = {
  auto: 'Automatic (agent, keys, password)',
  agent: 'SSH agent only',
  key: 'Key file',
  password: 'Password',
}

/** `user@address:port`, as the server names a host in every prompt. */
export const hostLabel = (h: { user: string; address: string; port: number }) => `${h.user}@${h.address}:${h.port}`

export const emptyDraft = (): HostDraft => ({ name: '', address: '', port: 22, user: '', auth: 'auto', keyPath: '', agentForward: false, root: '' })

export const draftOf = (h: Host): HostDraft => ({ id: h.id, name: h.name, address: h.address, port: h.port, user: h.user, auth: h.auth, keyPath: h.keyPath, agentForward: h.agentForward, root: h.root })

export const draftOfConfig = (c: ConfigHost): HostDraft => ({ ...emptyDraft(), name: c.name, address: c.address, port: c.port || 22, user: c.user, keyPath: c.keyPath, agentForward: c.agentForward, auth: c.keyPath ? 'key' : 'auto' })

/** The first thing wrong with a host form, or '' when it can be saved. */
export function hostProblem(h: HostDraft): string {
  if (!h.name.trim() || h.name.trim().length > 100) return 'Give the host a name of 1 to 100 characters.'
  if (!h.address.trim()) return 'Enter the address of the host, such as example.com.'
  if (/\s/.test(h.address.trim())) return 'The address cannot contain spaces.'
  if (!Number.isInteger(h.port) || h.port < 1 || h.port > 65535) return 'The port is a number from 1 to 65535.'
  if (!h.user.trim()) return 'Enter the user name to sign in with.'
  if (h.auth === 'key' && !h.keyPath.trim()) return 'Enter the path of the key file, such as ~/.ssh/id_ed25519.'
  if (h.root.trim() && !h.root.trim().startsWith('/')) return 'The root folder is an absolute path, such as /home/me/work. Leave it empty for the home folder.'
  return ''
}

/** The first thing wrong with a port forward form, or ''. Local port 0 means any free port. */
export function forwardProblem(local: number, remoteHost: string, remotePort: number): string {
  if (!Number.isInteger(local) || local < 0 || local > 65535) return 'The local port is 0 (any free port) or a number up to 65535.'
  if (!remoteHost.trim()) return 'Enter the host to reach from the remote side, such as localhost.'
  if (!Number.isInteger(remotePort) || remotePort < 1 || remotePort > 65535) return 'The remote port is a number from 1 to 65535.'
  return ''
}

export interface KeyChange {
  was: string
  now: string
  /** The server's last sentence: where the old key is and what to do. */
  advice: string
}

/** Reads the fingerprints and advice out of a HOST_KEY_CHANGED message. Falls back to the whole text. */
export function readKeyChange(message: string): KeyChange {
  const m = /It was (SHA256:[^\s]+) and is now (SHA256:[^\s]+?)\./.exec(message)
  const advice = /(The old key is in .*)$/.exec(message)?.[1] ?? message
  return { was: m?.[1] ?? '', now: m?.[2] ?? '', advice }
}

/** Splits `localhost:8080` style text for the forward form. */
export const portOf = (text: string): number => (/^\d+$/.test(text.trim()) ? Number(text.trim()) : NaN)

export const localAddress = (port: number) => `127.0.0.1:${port}`

/** What to tell the user when a password or passphrase is asked for again. Empty for a first ask. */
export function refusal(p: { kind: string; retry?: boolean }): string {
  if (!p.retry) return ''
  if (p.kind === 'passphrase') return 'That passphrase did not unlock the key. Try again.'
  if (p.kind === 'password') return 'That password was refused. Try again.'
  return ''
}
