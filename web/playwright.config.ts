import { defineConfig } from '@playwright/test'

// The browser tests run the real binary (built by `make build`) with its own
// data folder, against the Google Chrome already installed on this machine.
// MOORING_E2E_PORT moves every test port at once, so two copies of the repo can run the
// tests side by side: the app on base, stand-in SSH on +1, stand-in tool server on +2,
// the sign-in restart test on +3 and the password test on +4.
export const port = Number(process.env.MOORING_E2E_PORT || 7341)
export const sshPort = port + 1
export const mcpPort = port + 2
export const signinPort = port + 3
export const passwordPort = port + 4
export const dataDir = '.e2e-data'

export default defineConfig({
  testDir: 'e2e',
  reporter: 'line',
  // Playwright's default is half the cores: one worker on the 3-core macOS runner.
  workers: process.env.CI ? 2 : undefined,
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    channel: 'chrome',
  },
  // The chat test writes thousands of messages. While it does, the server can answer another
  // test's project.add with "database is locked (SQLITE_BUSY)" (a Go-side bug),
  // so it runs after the others instead of beside them.
  // SSH questions (host key, password) show as a dialog in every window, which would take the
  // focus from the other tests' pages, so the SSH tests run last, on their own.
  projects: [
    { name: 'main', testIgnore: /(chat|tokens|ssh)\.spec|e2e\/docs\// },
    // The server runs at most 20 chats at once; this stops the ones the main tests left running.
    { name: 'stop chats', testMatch: /stopall\.setup\.ts/, dependencies: ['main'] },
    { name: 'chat', testMatch: /(chat|tokens)\.spec/, dependencies: ['stop chats'] },
    { name: 'ssh', testMatch: /ssh\.spec/, dependencies: ['chat'] },
  ],
  webServer: {
    // serve.sh puts a stand-in agent on the PATH as "gemini" for the chat tests.
    command: `MOORING_E2E_PORT=${port} sh e2e/serve.sh`,
    // Playwright counts a 401 as "up", which is what this answers before login.
    url: `http://127.0.0.1:${port}/api/session`,
    reuseExistingServer: false,
  },
})
