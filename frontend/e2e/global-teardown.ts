import fs from 'node:fs'
import { E2E_TMP_DIR, PID_FILE } from './env'

export default async function globalTeardown() {
  if (fs.existsSync(PID_FILE)) {
    const pid = Number(fs.readFileSync(PID_FILE, 'utf8'))
    try {
      // Negative pid: signal the whole process group so the daemon's own
      // child processes (the fixture "sleep" loops) die with it instead of
      // being orphaned — global-setup spawned it with detached: true, which
      // makes it its own group leader.
      process.kill(-pid, 'SIGTERM')
    } catch {
      // already gone
    }
  }
  fs.rmSync(E2E_TMP_DIR, { recursive: true, force: true })
}
