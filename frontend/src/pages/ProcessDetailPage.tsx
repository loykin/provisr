import { useParams, useNavigate } from '@tanstack/react-router'
import { DetailBodyTemplate, PageTopBar } from '@loykin/designkit'
import { Button } from '@/components/ui/button'
import { useProcessStatus } from '@/features/processes/queries'
import { ProcessDetailBody } from '@/features/processes/ProcessDetailBody'
import { ProcessStateBadge } from '@/features/processes/ProcessStateBadge'

export default function ProcessDetailPage() {
  const { name } = useParams({ from: '/processes/$name' })
  const navigate = useNavigate()
  const { data: status, error } = useProcessStatus(name)

  return (
    <DetailBodyTemplate
      topBar={
        <PageTopBar
          left={
            <Button variant="ghost" onClick={() => void navigate({ to: '/processes' })}>
              ← Processes
            </Button>
          }
        />
      }
      eyebrow="Process"
      title={name}
      status={status ? <ProcessStateBadge state={status.state} /> : undefined}
      description={
        error
          ? status
            ? 'Connection lost — showing last known status.'
            : 'Failed to load process status.'
          : undefined
      }
    >
      {status && <ProcessDetailBody name={name} status={status} />}
    </DetailBodyTemplate>
  )
}
