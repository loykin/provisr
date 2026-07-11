import { StatusBadge } from '@/components/status-badge'
import type { JobStatus } from './types'

export function JobStateBadge({ phase }: { phase: JobStatus['phase'] }) {
  return <StatusBadge status={phase} />
}
