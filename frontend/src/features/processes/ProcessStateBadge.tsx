import { StatusBadge } from '@/components/status-badge'

export function ProcessStateBadge({ state }: { state: string }) {
  return <StatusBadge status={state} />
}
