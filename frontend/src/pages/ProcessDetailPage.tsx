import { useParams } from '@tanstack/react-router'
import { DetailBodyTemplate } from '@loykin/designkit'
import { PageBreadcrumbTopBar } from '@/components/page-breadcrumb-topbar'
import { useProcessStatus } from '@/features/processes/queries'
import { ProcessDetailBody } from '@/features/processes/ProcessDetailBody'
import { ProcessStateBadge } from '@/features/processes/ProcessStateBadge'

export default function ProcessDetailPage() {
  const { name } = useParams({ from: '/processes/$name' })
  const { data: status, error } = useProcessStatus(name)

  return (
    <DetailBodyTemplate
      topBar={<PageBreadcrumbTopBar items={['provisr', 'Workloads', 'Processes', name]} />}
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
