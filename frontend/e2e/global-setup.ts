import { execSync, spawn } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import {
  ADMIN_PASSWORD,
  ADMIN_USERNAME,
  BASE_URL,
  CONFIG_PATH,
  E2E_TMP_DIR,
  FRONTEND_ROOT,
  INLINE_PROCESS_NAME,
  API_PROCESS_NAME,
  PID_FILE,
  PROVISR_BINARY,
  REPO_ROOT,
  SERVER_PORT,
} from './env'

// Builds the current frontend + the provisr binary, boots a real daemon
// against a throwaway config, and registers the two fixture processes the
// specs compare: one declared inline in `[[processes]]` (must show up as
// "provisioned" in the UI) and one registered through the running API
// (must not). Runs once before the whole E2E suite; global-teardown.ts
// reverses every step.
export default async function globalSetup() {
  fs.rmSync(E2E_TMP_DIR, { recursive: true, force: true })
  fs.mkdirSync(E2E_TMP_DIR, { recursive: true })
  fs.mkdirSync(path.join(E2E_TMP_DIR, 'run'), { recursive: true })

  // 1) Build the frontend and embed it, exactly like `make ui` — the E2E
  // suite exercises the same single-binary /ui the real daemon serves, not
  // a dev server, so it catches "forgot to rebuild dist" mistakes too.
  execSync('npm run build', { cwd: FRONTEND_ROOT, stdio: 'inherit' })
  const embeddedDist = path.join(REPO_ROOT, 'internal/ui/dist')
  fs.rmSync(embeddedDist, { recursive: true, force: true })
  fs.cpSync(path.join(FRONTEND_ROOT, 'dist'), embeddedDist, { recursive: true })

  // 2) Build the provisr binary.
  execSync(`go build -o ${PROVISR_BINARY} ./cmd/provisr`, { cwd: REPO_ROOT, stdio: 'inherit' })

  // 3) Write a throwaway config: one inline process, auth enabled, no
  // programs directory needed since this suite only cares about the
  // inline-vs-API-registered distinction.
  const config = `
pid_dir = "${path.join(E2E_TMP_DIR, 'run')}"

[log]
dir = "${path.join(E2E_TMP_DIR, 'logs')}"

[[processes]]
type = "process"
[processes.spec]
name = "${INLINE_PROCESS_NAME}"
command = "sh -c 'while true; do sleep 5; done'"

[server]
listen = ":${SERVER_PORT}"
base_path = "/api"

[server.auth]
enabled = true

[server.auth.store]
type = "sqlite"
migrate = true
path = "${path.join(E2E_TMP_DIR, 'auth.db')}"
`
  fs.writeFileSync(CONFIG_PATH, config)

  // 4) Start the daemon.
  const child = spawn(PROVISR_BINARY, ['serve', CONFIG_PATH], {
    cwd: E2E_TMP_DIR,
    stdio: ['ignore', fs.openSync(path.join(E2E_TMP_DIR, 'server.log'), 'w'), 'inherit'],
    detached: true,
  })
  child.unref()
  fs.writeFileSync(PID_FILE, String(child.pid))

  // 5) Wait for it to accept connections.
  const deadline = Date.now() + 15_000
  for (;;) {
    try {
      const res = await fetch(`${BASE_URL}/api/auth/status`)
      if (res.ok) break
    } catch {
      // not up yet
    }
    if (Date.now() > deadline) throw new Error('provisr e2e server did not become ready in time')
    await new Promise((r) => setTimeout(r, 200))
  }

  // 6) Bootstrap the admin user, then register the API-managed fixture
  // process while we still have the token.
  const bootstrap = await fetch(`${BASE_URL}/api/auth/bootstrap`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username: ADMIN_USERNAME, password: ADMIN_PASSWORD }),
  })
  if (!bootstrap.ok) throw new Error(`bootstrap failed: ${bootstrap.status} ${await bootstrap.text()}`)
  const { token } = (await bootstrap.json()) as { token: { value: string } }

  const register = await fetch(`${BASE_URL}/api/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token.value}` },
    body: JSON.stringify({ name: API_PROCESS_NAME, command: "sh -c 'while true; do sleep 5; done'" }),
  })
  if (!register.ok) throw new Error(`register failed: ${register.status} ${await register.text()}`)
}
