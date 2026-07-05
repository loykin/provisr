import { useEffect, useState } from 'react'
import { getProcess } from './api'
import type { ProcessStatus } from './types'

const POLL_INTERVAL_MS = 3000

export function useProcessStatus(name: string) {
  const [status, setStatus] = useState<ProcessStatus | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const data = await getProcess(name)
        if (!cancelled) {
          setStatus(data)
          setError(null)
        }
      } catch {
        if (!cancelled) setError('Failed to load process status.')
      }
    }
    void load()
    const id = setInterval(load, POLL_INTERVAL_MS)
    return () => {
      cancelled = true
      clearInterval(id)
    }
  }, [name])

  return { status, error }
}
