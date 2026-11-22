// System notifications for chats and terminals that need the user. They fire only while the tab is
// hidden, once each time an item starts needing you, and say the project only, never what the agent wrote.
const BELL_KEY = 'mooring.notify.bell'

/** A terminal bell counts as needing you unless the user turned that off in this browser. */
export function bellCounts(store: Pick<Storage, 'getItem'> = localStorage): boolean {
  try {
    return store.getItem(BELL_KEY) !== 'off'
  } catch {
    return true
  }
}

export function setBellCounts(on: boolean, store: Pick<Storage, 'setItem'> = localStorage): void {
  try {
    store.setItem(BELL_KEY, on ? 'on' : 'off')
  } catch {
    // Storage is blocked: the switch lasts until the page reloads.
  }
}

export type Permission = 'granted' | 'denied' | 'default' | 'unsupported'

export function permission(): Permission {
  return typeof Notification === 'undefined' ? 'unsupported' : Notification.permission
}

export async function askPermission(): Promise<Permission> {
  if (typeof Notification === 'undefined') return 'unsupported'
  return await Notification.requestPermission()
}

export interface Needing {
  /** Stable per item: the tag, and what "starts needing you" is measured by. */
  key: string
  kind: 'chat' | 'terminal'
  project: string
}

export function notifyText(n: Pick<Needing, 'kind' | 'project'>): string {
  return `A ${n.kind} in ${n.project || 'a project'} needs you.`
}

interface Deps {
  hidden: () => boolean
  allowed: () => boolean
  show: (body: string, tag: string) => void
}

/**
 * Call `sync` with everything that needs you now. An item rising while the tab is visible is remembered,
 * not announced. Pass `ready` false until the first snapshot arrived: what already needed you at load
 * (a reload of a hidden tab) is remembered, not announced.
 */
export function makeNotifier(d: Deps) {
  const known = new Set<string>()
  let seeded = false
  return function sync(items: Needing[], ready = true): void {
    if (!ready) return
    if (!seeded) {
      seeded = true
      for (const i of items) known.add(i.key)
      return
    }
    const now = new Set(items.map((i) => i.key))
    for (const k of known) if (!now.has(k)) known.delete(k)
    for (const i of items) {
      if (known.has(i.key)) continue
      known.add(i.key)
      if (d.hidden() && d.allowed()) d.show(notifyText(i), `mooring-${i.key}`)
    }
  }
}

export const browserNotifier = () =>
  makeNotifier({
    hidden: () => document.hidden,
    allowed: () => permission() === 'granted',
    show: (body, tag) => {
      try {
        new Notification('Mooring', { body, tag })
      } catch {
        // Some phone browsers only allow notifications from a service worker.
      }
    },
  })

/** The body of the test notification Settings sends, so the user can see that notifications reach them. */
export const TEST_TEXT = 'Notifications from Mooring look like this.'

interface TestDeps {
  allowed: () => boolean
  make: (title: string, options: NotificationOptions) => unknown
}

const browserTest: TestDeps = {
  allowed: () => permission() === 'granted',
  make: (title, options) => new Notification(title, options),
}

/**
 * Shows one notification now, even while this tab is in front. False when notifications are not allowed
 * or the browser refused to make one (some phone browsers only allow them from a service worker).
 */
export function sendTest(d: TestDeps = browserTest): boolean {
  if (!d.allowed()) return false
  try {
    d.make('Mooring', { body: TEST_TEXT, tag: 'mooring-test' })
    return true
  } catch {
    return false
  }
}
