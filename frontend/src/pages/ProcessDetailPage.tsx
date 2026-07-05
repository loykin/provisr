import { useParams, useNavigate } from '@tanstack/react-router'
import { DetailBodyTemplate, PageTopBar } from '@loykin/designkit'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useProcessStatus } from '@/features/processes/useProcessStatus'
import { ProcessDetailBody } from '@/features/processes/ProcessDetailBody'

export default function ProcessDetailPage() {
  const { name } = useParams({ from: '/processes/$name' })
  const navigate = useNavigate()
  const { status, error } = useProcessStatus(name)

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
      status={
        status ? (
          <Badge variant={status.running ? 'default' : 'secondary'}>{status.state}</Badge>
        ) : undefined
      }
      description={error ?? undefined}
    >
      {status && <ProcessDetailBody status={status} />}
    </DetailBodyTemplate>
  )
}
