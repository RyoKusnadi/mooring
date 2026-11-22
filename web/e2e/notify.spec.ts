import { expect, test, type Page } from '@playwright/test'
import { collectProblems, openSettings, signIn } from './helpers'

// A stand-in for the browser's Notification, so no real notification reaches the desktop.
// It records what was shown, and throws instead when `refuse` is set (like phone browsers that need a service worker).
async function fakeNotifications(page: Page, refuse: boolean) {
  await page.addInitScript((refuse) => {
    const shown: { title: string; body?: string; tag?: string }[] = []
    class Fake {
      static permission = 'granted'
      static requestPermission = async () => 'granted'
      constructor(title: string, options: NotificationOptions = {}) {
        if (refuse) throw new TypeError('Illegal constructor')
        shown.push({ title, body: options.body, tag: options.tag })
      }
    }
    Object.assign(window, { Notification: Fake, shownNotifications: shown })
  }, refuse)
}

const shown = (page: Page) => page.evaluate(() => (window as unknown as { shownNotifications: unknown[] }).shownNotifications)

test('settings: a test notification shows once per click and says what to check', async ({ page }) => {
  await fakeNotifications(page, false)
  await signIn(page)
  const problems = collectProblems(page)
  const dialog = await openSettings(page, 'Notifications')
  await expect(dialog.getByText('Notifications are on in this browser.')).toBeVisible()

  const send = dialog.getByRole('button', { name: 'Send a test notification' })
  const answer = dialog.getByRole('status').filter({ hasText: 'Mooring' })
  // The empty answer line is hidden until the first click, so only the green line is a status.
  await expect(dialog.getByRole('status')).toHaveCount(1)
  await send.click()
  await expect(answer).toContainText('A notification titled "Mooring" should show now.')
  expect(await shown(page)).toEqual([{ title: 'Mooring', body: 'Notifications from Mooring look like this.', tag: 'mooring-test' }])

  // Again: one more notification, still one answer, and focus stays on the button.
  await send.click()
  await expect.poll(async () => (await shown(page)).length).toBe(2)
  await expect(answer).toHaveCount(1)
  await expect(send).toBeFocused()
  expect(problems).toEqual([])
})

test('settings: a test notification the browser refuses says so', async ({ page }) => {
  await fakeNotifications(page, true)
  await signIn(page)
  const dialog = await openSettings(page, 'Notifications')
  await dialog.getByRole('button', { name: 'Send a test notification' }).click()
  await expect(dialog.getByRole('status').filter({ hasText: 'did not show it' })).toHaveText(
    'This browser did not show it. Check that notifications are allowed for this site, then try again.',
  )
})
