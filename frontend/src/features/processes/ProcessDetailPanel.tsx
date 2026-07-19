import { X } from 'lucide-react'
import { PanelTemplate } from '@loykin/designkit'
import { useSidePanel } from '@loykin/side-panel'
import { Button } from '@/components/ui/button'
import { useProcessStatus } from './queries'
import { ProcessActions } from './ProcessActions'
import { ProcessDetailBody } from './ProcessDetailBody'
import { ProcessStateBadge } from './ProcessStateBadge'

export function ProcessDetailPanel({ name }: { name: string }) {
  const { close } = useSidePanel()
  const { data: status, error } = useProcessStatus(name)

  const closeBtn = (
    <Button variant="ghost" size="icon-sm" onClick={() => void close()}>
      <X className="h-3.5 w-3.5" />
    </Button>
  )

  if (!status) {
    return (
      <PanelTemplate title="Loading…" actions={closeBtn}>
        <p className="text-sm text-muted-foreground">
          {error ? 'Failed to load process status.' : 'Loading…'}
        </p>
      </PanelTemplate>
    )
  }

  return (
    <PanelTemplate
      eyebrow="Process"
      title={name}
      status={<ProcessStateBadge state={status.state} />}
      actions={
        <div className="flex items-center gap-2">
          <ProcessActions status={status} onUnregistered={() => void close()} />
          {closeBtn}
        </div>
      }
    >
      <ProcessDetailBody name={name} status={status} />
    </PanelTemplate>
  )
}
