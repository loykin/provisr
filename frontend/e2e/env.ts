import path from 'node:path'
import { fileURLToPath } from 'node:url'

// Shared constants between global-setup, global-teardown, and the specs
// themselves — kept in one place so the port/paths can't drift apart.
const __dirname = path.dirname(fileURLToPath(import.meta.url))

export const REPO_ROOT = path.resolve(__dirname, '../..')
export const FRONTEND_ROOT = path.resolve(__dirname, '..')
export const E2E_TMP_DIR = path.join(FRONTEND_ROOT, '.e2e-tmp')
export const PROVISR_BINARY = path.join(E2E_TMP_DIR, 'provisr-e2e')
export const CONFIG_PATH = path.join(E2E_TMP_DIR, 'config.toml')
export const PID_FILE = path.join(E2E_TMP_DIR, 'server.pid')

export const SERVER_PORT = 8199
export const BASE_URL = `http://localhost:${SERVER_PORT}`

export const ADMIN_USERNAME = 'admin'
export const ADMIN_PASSWORD = 'e2e-test-password'

// Names of the two fixture processes the specs assert against: one declared
// in the main config file's `[[processes]]` array (must show the
// "provisioned" lock in the UI), one registered through the running API
// after boot (must behave like any normal process).
export const INLINE_PROCESS_NAME = 'inline-demo'
export const API_PROCESS_NAME = 'api-demo'
