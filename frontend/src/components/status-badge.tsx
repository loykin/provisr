import { Badge } from '@/components/ui/badge'

const OUTLINE_STYLES = {
  blue: 'border-blue-600/30 bg-blue-600/15 text-blue-700 dark:text-blue-400',
  green: 'border-green-600/30 bg-green-600/15 text-green-700 dark:text-green-400',
  amber: 'border-amber-600/30 bg-amber-600/15 text-amber-700 dark:text-amber-400',
}

function normalizeStatus(status: string): string {
  return status.trim().toLowerCase()
}

export function StatusBadge({ status }: { status: string }) {
  const value = normalizeStatus(status)

  if (['succeeded', 'success', 'completed', 'active'].includes(value)) {
    return (
      <Badge className={OUTLINE_STYLES.green} variant="outline">
        {value}
      </Badge>
    )
  }

  if (['running', 'starting', 'started'].includes(value)) {
    return (
      <Badge className={OUTLINE_STYLES.blue} variant="outline">
        {value}
      </Badge>
    )
  }

  if (['stopping', 'terminating'].includes(value)) {
    return (
      <Badge className={OUTLINE_STYLES.amber} variant="outline">
        {value}
      </Badge>
    )
  }

  if (['failed', 'failure', 'error'].includes(value)) {
    return <Badge variant="destructive">{value}</Badge>
  }

  return <Badge variant="secondary">{value || 'unknown'}</Badge>
}
