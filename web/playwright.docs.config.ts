import { mkdirSync } from 'node:fs'
import { resolve } from 'node:path'
import { defineConfig } from '@playwright/test'

// Takes the pictures for docs/guide with `make screenshots`. It is not part of `make check`.
// It starts the same stand-ins as the browser tests (e2e/serve.sh), with its own data folder and a
// fake home folder, so no path from this machine shows on screen.
export const port = Number(process.env.MOORING_E2E_PORT || 7450)
export const dataDir = '.docs-data'
export const homeDir = resolve(dataDir, 'home')
mkdirSync(homeDir, { recursive: true })

export default defineConfig({
  testDir: 'e2e/docs',
  reporter: 'line',
  workers: 1,
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    channel: 'chrome',
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 1,
    colorScheme: 'light',
  },
  webServer: {
    command: `MOORING_E2E_PORT=${port} MOORING_E2E_DATA=${dataDir} MOORING_E2E_HOME=${homeDir} sh e2e/serve.sh`,
    url: `http://127.0.0.1:${port}/api/session`,
    reuseExistingServer: false,
  },
})
