import { useQuery } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { getProcess, getProcessLogsSince, listProcesses } from './api'
import type { LogLine } from './types'

const STATUS_POLL_MS = 3000
const LOGS_POLL_MS = 1000
const MAX_BUFFERED_LINES = 1000

export function useProcesses() {
  return useQuery({
    queryKey: ['processes'],
    queryFn: listProcesses,
    refetchInterval: STATUS_POLL_MS,
  })
}

export function useProcessStatus(name: string) {
  return useQuery({
    queryKey: ['process', name],
    queryFn: () => getProcess(name),
    refetchInterval: STATUS_POLL_MS,
  })
}

// Polls for new log lines since the last-seen offset and accumulates them
// client-side (the API only ever returns the delta, not the full history).
//
// The accumulation happens in an effect keyed off query.data, not inside
// queryFn itself: queryFn must stay a plain, repeatable read (react-query
// may invoke it more than once for the same tick, e.g. under React
// StrictMode), so mutating shared state there risks double-appending the
// same lines. The effect dedupes by offset, so a duplicate invocation is a
// harmless no-op instead of a "duplicate key" bug.
export function useProcessLogs(name: string, enabled: boolean) {
  const sinceRef = useRef(0)
  const [lines, setLines] = useState<LogLine[]>([])
  // Reset accumulated state when switching to a different process.
  const nameRef = useRef(name)
  if (nameRef.current !== name) {
    nameRef.current = name
    sinceRef.current = 0
    setLines([])
  }

  const query = useQuery({
    queryKey: ['processLogs', name],
    queryFn: () => getProcessLogsSince(name, sinceRef.current),
    refetchInterval: LOGS_POLL_MS,
    enabled,
  })

  useEffect(() => {
    if (!query.data || query.data.lines.length === 0) return
    setLines((prev) => {
      const lastOffset = prev.length > 0 ? prev[prev.length - 1].offset : -1
      const newLines = query.data.lines.filter((l) => l.offset > lastOffset)
      if (newLines.length === 0) return prev
      return [...prev, ...newLines].slice(-MAX_BUFFERED_LINES)
    })
    sinceRef.current = query.data.next
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only re-run when new data arrives
  }, [query.data])

  return { data: lines, error: query.error }
}
