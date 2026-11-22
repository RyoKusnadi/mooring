import { test } from '@playwright/test'
import { signIn } from './helpers'
import { stopChats } from './stopchats'

test('stop the chats the main tests left running', async ({ page }) => {
  await signIn(page)
  await stopChats(page, null)
})
