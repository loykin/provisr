import { Badge } from '@/components/ui/badge'

// shadcn's Badge only ships semantic variants (default/secondary/destructive/
// outline/ghost/link) — there's no built-in "success green" for status
// indicators, so the state → color mapping is done here via className on
// top of the "outline" variant, not a Badge variant itself.
const STATE_STYLES: Record<string, string> = {
  running: 'border-transparent bg-green-600 text-white dark:bg-green-500',
  starting: 'border-transparent bg-blue-600 text-white dark:bg-blue-500',
  stopping: 'border-transparent bg-amber-600 text-white dark:bg-amber-500',
  stopped: 'border-transparent bg-neutral-400 text-white dark:bg-neutral-600',
}

export function ProcessStateBadge({ state }: { state: string }) {
  return (
    <Badge variant="outline" className={STATE_STYLES[state] ?? STATE_STYLES.stopped}>
      {state}
    </Badge>
  )
}
