import { Badge } from '@/components/ui/badge'

const OUTLINE_STYLES = {
  blue: 'border-blue-600/30 bg-blue-600/15 text-blue-700 dark:text-blue-400',
  green: 'border-green-600/30 bg-green-600/15 text-green-700 dark:text-green-400',
  amber: 'border-amber-600/30 bg-amber-600/15 text-amber-700 dark:text-amber-400',
}

type OutlineColor = keyof typeof OUTLINE_STYLES

const STATUS_COLOR: Record<string, OutlineColor> = {
  succeeded: 'green', success: 'green', completed: 'green', active: 'green',
  running: 'blue', starting: 'blue', started: 'blue',
  stopping: 'amber', terminating: 'amber', degraded: 'amber',
}

function normalizeStatus(status: string): string {
  return status.trim().toLowerCase()
}

export function StatusBadge({ status }: { status: string }) {
  const value = normalizeStatus(status)
  const color = STATUS_COLOR[value]

  if (color) {
    return <Badge className={OUTLINE_STYLES[color]} variant="outline">{value}</Badge>
  }

  if (['failed', 'failure', 'error'].includes(value)) {
    return <Badge variant="destructive">{value}</Badge>
  }

  return <Badge variant="secondary">{value || 'unknown'}</Badge>
}
