import { X } from 'lucide-react'
import { PanelTemplate } from '@loykin/designkit'
import { useSidePanel } from '@loykin/side-panel'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useProcessStatus } from './useProcessStatus'
import { ProcessDetailBody } from './ProcessDetailBody'

export function ProcessDetailPanel({ name }: { name: string }) {
  const { close } = useSidePanel()
  const { status, error } = useProcessStatus(name)

  const closeBtn = (
    <Button variant="ghost" size="icon-sm" onClick={() => void close()}>
      <X className="h-3.5 w-3.5" />
    </Button>
  )

  if (!status) {
    return (
      <PanelTemplate title="Loading…" actions={closeBtn}>
        <p className="text-sm text-muted-foreground">{error ?? 'Loading…'}</p>
      </PanelTemplate>
    )
  }

  return (
    <PanelTemplate
      eyebrow="Process"
      title={name}
      status={<Badge variant={status.running ? 'default' : 'secondary'}>{status.state}</Badge>}
      actions={closeBtn}
    >
      <ProcessDetailBody status={status} />
    </PanelTemplate>
  )
}
