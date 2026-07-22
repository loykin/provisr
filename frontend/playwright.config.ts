import { defineConfig } from '@playwright/test'
import { BASE_URL } from './e2e/env'

// No `webServer` entry here on purpose: the provisr daemon this suite tests
// against needs a bootstrapped admin user and a couple of fixture processes
// registered before any spec runs, which Playwright's built-in webServer
// (a single "start and wait for a URL" command) can't express. global-setup
// does all of that — build, boot, bootstrap, register — and
// global-teardown reverses it.
export default defineConfig({
  testDir: './e2e',
  globalSetup: './e2e/global-setup.ts',
  globalTeardown: './e2e/global-teardown.ts',
  timeout: 30_000,
  fullyParallel: false,
  workers: 1,
  reporter: 'list',
  use: {
    baseURL: `${BASE_URL}/ui/`,
    trace: 'retain-on-failure',
  },
})
